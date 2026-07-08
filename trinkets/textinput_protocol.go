package trinkets

import (
	"github.com/phroun/kittytk/core"
	"github.com/phroun/kittytk/protocol"
)

// Wire registration for TextInput (see docs/property-vocabulary.md).
// cursor/selection/readonly/mask arrive with the set verb and event
// slices (they are interaction state, not construction state).
func init() {
	regTrinket("textinput",
		func() core.Trinket { return NewTextInput() },
		map[string]protocol.PropertyApplier{
			"text":        stringProp("text", (*TextInput).SetText),
			"placeholder": stringProp("placeholder", (*TextInput).SetPlaceholder),
		},
		nil,
		func(ctx *protocol.BindContext, w core.Trinket) {
			t := w.(*TextInput)
			id := trinketID(t)
			t.SetOnTextChanged(func(text string) {
				ctx.EmitEvent(protocol.NewEvent("change").
					WithUint("trinket", id).WithString("text", text))
			})
		},
	)
}
