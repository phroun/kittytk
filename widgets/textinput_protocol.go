package widgets

import (
	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/protocol"
)

// Wire registration for TextInput (see docs/property-vocabulary.md).
// cursor/selection/readonly/mask arrive with the set verb and event
// slices (they are interaction state, not construction state).
func init() {
	regWidget("textinput",
		func() core.Widget { return NewTextInput() },
		map[string]protocol.PropertyApplier{
			"text":        stringProp("text", (*TextInput).SetText),
			"placeholder": stringProp("placeholder", (*TextInput).SetPlaceholder),
		},
		nil,
		func(ctx *protocol.BindContext, w core.Widget) {
			t := w.(*TextInput)
			id := widgetID(t)
			t.SetOnTextChanged(func(text string) {
				ctx.EmitEvent(protocol.NewEvent("change").
					WithUint("widget", id).WithString("text", text))
			})
		},
	)
}
