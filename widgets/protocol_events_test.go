package widgets

import (
	"testing"

	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/protocol"
)

// buildWithEvents builds protocol UI with an event-collecting context.
func buildWithEvents(t *testing.T, commands *core.CommandRegistry, src string) (*captureFactory, *[]*protocol.Event) {
	t.Helper()
	events := &[]*protocol.Event{}
	ctx := &protocol.BindContext{
		Emit: func(ev *protocol.Event) { *events = append(*events, ev) },
	}
	if commands != nil {
		ctx.Dispatch = func(id string) { commands.Dispatch(id) }
	}
	f := &captureFactory{inner: protocol.NewRegistryFactory(ctx)}
	script, err := protocol.Parse(src)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, err := protocol.NewSession().Execute(script, f); err != nil {
		t.Fatalf("execute: %v", err)
	}
	return f, events
}

func eventsOfType(events []*protocol.Event, typ string) []*protocol.Event {
	var out []*protocol.Event
	for _, ev := range events {
		if ev.Type == typ {
			out = append(out, ev)
		}
	}
	return out
}

func TestCheckboxTogglesEmitTriStateEvents(t *testing.T) {
	f, events := buildWithEvents(t, nil, `new checkbox caption="t" tristate`)
	cb := f.targets[0].(*Checkbox)
	*events = nil // ignore construction-time emissions

	// Toggle cycle: Unchecked -> Checked -> Partially -> Unchecked.
	cb.Toggle()
	cb.Toggle()
	cb.Toggle()

	got := eventsOfType(*events, "toggle")
	if len(got) != 3 {
		t.Fatalf("toggle events = %d, want 3", len(got))
	}
	want := []protocol.FlagState{protocol.FlagTrue, protocol.FlagIndeterminate, protocol.FlagFalse}
	for i, ev := range got {
		if id, ok := ev.Widget(); !ok || id != uint64(cb.ObjectID()) {
			t.Errorf("event %d widget = %d", i, id)
		}
		if ev.Flag("checked") != want[i] {
			t.Errorf("event %d checked = %v, want %v", i, ev.Flag("checked"), want[i])
		}
	}
}

func TestButtonClickEmitsClickAndCommandEvents(t *testing.T) {
	commands := core.NewCommandRegistry()
	fired := 0
	commands.Register("do.it", func() { fired++ })

	f, events := buildWithEvents(t, commands, `new button caption="Go" action=do.it`)
	btn := f.targets[0].(*Button)
	*events = nil

	btn.Click()

	if fired != 1 {
		t.Errorf("registry fired = %d, want 1", fired)
	}
	cmds := eventsOfType(*events, "command")
	clicks := eventsOfType(*events, "click")
	if len(cmds) != 1 || len(clicks) != 1 {
		t.Fatalf("command=%d click=%d, want 1 each", len(cmds), len(clicks))
	}
	if action, ok := cmds[0].Word("action"); !ok || action != "do.it" {
		t.Errorf("command action = %q", action)
	}
	if id, ok := clicks[0].Widget(); !ok || id != uint64(btn.ObjectID()) {
		t.Errorf("click widget = %d", id)
	}
}

func TestTextInputChangeEvents(t *testing.T) {
	f, events := buildWithEvents(t, nil, `new textinput text="start"`)
	ti := f.targets[0].(*TextInput)

	// Construction applied text="start" through the wire - that
	// emission is expected (suppression policy is a later decision).
	if built := eventsOfType(*events, "change"); len(built) != 1 {
		t.Fatalf("construction change events = %d, want 1", len(built))
	}

	*events = nil
	ti.SetText("edited")

	got := eventsOfType(*events, "change")
	if len(got) != 1 {
		t.Fatalf("change events = %d, want 1", len(got))
	}
	if s, ok := got[0].Text("text"); !ok || s != "edited" {
		t.Errorf("text = %q", s)
	}
}

func TestComboBoxSelectionChangeEvents(t *testing.T) {
	f, events := buildWithEvents(t, nil, `
new combobox children={new item caption="A"; new item caption="B"} selected=1
`)
	combo := f.targets[0].(*ComboBox)
	*events = nil

	combo.SetCurrentIndex(0)

	got := eventsOfType(*events, "change")
	if len(got) != 1 {
		t.Fatalf("change events = %d, want 1", len(got))
	}
	if sel, ok := got[0].Int("selected"); !ok || sel != 0 {
		t.Errorf("selected = %d", sel)
	}
}

func TestEventsRouteThroughDispatcher(t *testing.T) {
	// The full loop: protocol-built widget -> event record ->
	// dispatcher -> app handler keyed by ObjectID.
	dispatcher := protocol.NewEventDispatcher()
	ctx := &protocol.BindContext{
		Emit: func(ev *protocol.Event) { dispatcher.Dispatch(ev) },
	}
	f := &captureFactory{inner: protocol.NewRegistryFactory(ctx)}
	script, err := protocol.Parse(`new checkbox caption="c"`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := protocol.NewSession().Execute(script, f); err != nil {
		t.Fatal(err)
	}
	cb := f.targets[0].(*Checkbox)

	toggled := 0
	dispatcher.On(uint64(cb.ObjectID()), "toggle", func(ev *protocol.Event) {
		if ev.Flag("checked") == protocol.FlagTrue {
			toggled++
		}
	})

	cb.Toggle()
	if toggled != 1 {
		t.Errorf("handler toggled = %d, want 1", toggled)
	}
}
