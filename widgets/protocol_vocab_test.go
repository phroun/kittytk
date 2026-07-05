package widgets

import (
	"testing"

	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/protocol"
	"github.com/phroun/tuitk/style"
)

func TestTabsBuildFromProtocol(t *testing.T) {
	f, _ := buildUI(t, nil, `
tw=new tabs position=bottom children={
	new tab caption="One" children={new label caption="first"}
	new tab caption="Two" children={new panel layout=vbox children={new label caption="second"}}
} selected=1
`)
	tw := f.targets[0].(*TabWidget)
	if tw.Count() != 2 {
		t.Fatalf("tabs = %d, want 2", tw.Count())
	}
	if tw.CurrentIndex() != 1 {
		t.Errorf("selected = %d, want 1", tw.CurrentIndex())
	}
	if tw.TabPosition() != TabsBottom {
		t.Errorf("position = %v, want TabsBottom", tw.TabPosition())
	}
}

func TestListViewBuildAndEvents(t *testing.T) {
	f, events := buildWithEvents(t, nil, `
lv=new listview children={
	new item caption="Alpha"
	new item caption="Beta"
	new item caption="Gamma"
} selected=0
`)
	lv := f.targets[0].(*ListView)
	if lv.Count() != 3 {
		t.Fatalf("items = %d, want 3", lv.Count())
	}

	*events = nil
	lv.SetCurrentIndex(2)
	got := eventsOfType(*events, "change")
	if len(got) != 1 {
		t.Fatalf("change events = %d, want 1", len(got))
	}
	if sel, ok := got[0].Int("selected"); !ok || sel != 2 {
		t.Errorf("selected = %d, want 2", sel)
	}
}

func TestTreeViewBuildsNestedItems(t *testing.T) {
	f, _ := buildUI(t, nil, `
tv=new treeview children={
	new item caption="Fruit" expanded children={
		new item caption="Apple"
		new item caption="Pear" children={new item caption="Bosc"}
	}
	new item caption="Roots"
}
`)
	tv := f.targets[0].(*TreeView)
	roots := tv.RootItems()
	if len(roots) != 2 {
		t.Fatalf("root items = %d, want 2", len(roots))
	}
	fruit := roots[0]
	if fruit.Text != "Fruit" || !fruit.Expanded {
		t.Errorf("fruit = %q expanded=%v", fruit.Text, fruit.Expanded)
	}
	if len(fruit.Children) != 2 {
		t.Fatalf("fruit children = %d, want 2", len(fruit.Children))
	}
	if len(fruit.Children[1].Children) != 1 || fruit.Children[1].Children[0].Text != "Bosc" {
		t.Errorf("nested grandchild missing")
	}
}

func TestScrollAreaSingleContent(t *testing.T) {
	f, _ := buildUI(t, nil, `
sa=new scrollarea resizable children={new label caption="inside"}
`)
	sa := f.targets[0].(*ScrollArea)
	if sa.Content() == nil {
		t.Fatal("no content set")
	}
	if _, err := protocol.Parse(`x=new scrollarea children={new label caption="a"; new label caption="b"}`); err != nil {
		t.Fatalf("parse: %v", err)
	} else {
		script, _ := protocol.Parse(`x=new scrollarea children={new label caption="a"; new label caption="b"}`)
		f2 := &captureFactory{inner: protocol.NewRegistryFactory(&protocol.BindContext{})}
		if _, err := protocol.NewSession().Execute(script, f2); err == nil {
			t.Error("second content should error")
		}
	}
}

func TestRadioGroupProperty(t *testing.T) {
	f, _ := buildUI(t, nil, `
new panel layout=vbox children={
	a=new radiobutton caption="A" group=juice checked
	b=new radiobutton caption="B" group=juice
	c=new radiobutton caption="C" group=other checked
}
`)
	a := f.targets[1].(*RadioButton)
	b := f.targets[2].(*RadioButton)
	c := f.targets[3].(*RadioButton)

	b.SetChecked(true)
	if a.IsChecked() {
		t.Error("a should be unchecked after b checked (same group)")
	}
	if !c.IsChecked() {
		t.Error("c is in another group and must be unaffected")
	}
}

func TestWindowBuildFromProtocol(t *testing.T) {
	f, reply := buildUI(t, nil, `
win=new window title="Tools" x=64 y=32 width=400 height=240 no_resize children={
	new label caption="body"
}
`)
	// The window is targets[0] via the capture factory.
	w := f.targets[0].(interface {
		Title() string
		Bounds() core.UnitRect
	})
	if w.Title() != "Tools" {
		t.Errorf("title = %q", w.Title())
	}
	if b := w.Bounds(); b.X != 64 || b.Y != 32 || b.Width != 400 || b.Height != 240 {
		t.Errorf("bounds = %+v", b)
	}
	if reply.IDs["win"] == 0 {
		t.Error("window not surfaced in reply")
	}
}

func TestStretchAndAlignTravelWithChild(t *testing.T) {
	f, _ := buildUI(t, nil, `
new panel layout=hbox children={
	new label caption="fixed"
	new spacer stretch=1
	new button caption="OK" align=right
}
`)
	spacer := f.targets[2].(*Spacer)
	btn := f.targets[3].(*Button)
	if spacer.LayoutStretch() != 1 {
		t.Errorf("spacer stretch = %d, want 1", spacer.LayoutStretch())
	}
	if a, set := btn.LayoutAlignment(); !set || a != core.AlignRight {
		t.Errorf("button align = %v/%v, want AlignRight", a, set)
	}
}

func TestColorProperties(t *testing.T) {
	f, _ := buildUI(t, nil, `
new label caption="tinted" fg=bright_yellow bg="#334455"
`)
	lbl := f.targets[0].(*Label)
	s := lbl.Style()
	if s == nil {
		t.Fatal("no custom style set")
	}
	if s.Fg != style.ColorBrightYellow {
		t.Errorf("fg = %v, want bright yellow", s.Fg)
	}
	if s.Bg != style.RGB(0x33, 0x44, 0x55) {
		t.Errorf("bg = %v, want RGB 334455", s.Bg)
	}
}
