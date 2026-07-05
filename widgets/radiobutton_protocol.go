package widgets

import (
	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/protocol"
)

// Wire registration for RadioButton (see docs/property-vocabulary.md).
// Group membership is a plain named property: buttons sharing a
// group= word on the same connection exclude each other. The groups
// themselves live in the connection's stash - no container widget,
// no positional coupling.
func init() {
	regWidget("radiobutton",
		func() core.Widget { return NewRadioButton("") },
		map[string]protocol.PropertyApplier{
			"caption": stringProp("caption", (*RadioButton).SetText),
			"checked": boolProp("checked", (*RadioButton).SetChecked),
			"wrap":    boolProp("wrap", (*RadioButton).SetWordWrap),
			"group": wprop("group", func(ctx *protocol.BindContext, r *RadioButton, v *protocol.Value, f protocol.FlagState) error {
				word, err := protocol.AsWord("group", v, f)
				if err != nil {
					return err
				}
				g := ctx.Stash("radiogroup:"+word, func() any { return NewRadioGroup() }).(*RadioGroup)
				g.AddButton(r)
				return nil
			}),
		},
		nil,
		func(ctx *protocol.BindContext, w core.Widget) {
			r := w.(*RadioButton)
			id := widgetID(r)
			r.SetOnToggled(func(checked bool) {
				flag := protocol.FlagFalse
				if checked {
					flag = protocol.FlagTrue
				}
				ctx.EmitEvent(protocol.NewEvent("toggle").
					WithUint("widget", id).WithFlag("checked", flag))
			})
		},
	)
}
