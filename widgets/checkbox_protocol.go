package widgets

import (
	"fmt"

	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/protocol"
)

// Wire registration for Checkbox (see docs/property-vocabulary.md).
// `checked` is tri-capable per D16: checked / !checked / ?checked.
func init() {
	regWidget("checkbox",
		func() core.Widget { return NewCheckbox("") },
		map[string]protocol.PropertyApplier{
			"caption": stringProp("caption", (*Checkbox).SetText),
			"checked": wprop("checked", func(_ *protocol.BindContext, c *Checkbox, v *protocol.Value, f protocol.FlagState) error {
				switch f {
				case protocol.FlagTrue:
					c.SetCheckState(Checked)
				case protocol.FlagFalse:
					c.SetCheckState(Unchecked)
				case protocol.FlagIndeterminate:
					c.SetCheckState(PartiallyChecked)
				default:
					// Long forms for generic tooling (D12/D16).
					w, err := protocol.AsWord("checked", v, f)
					if err != nil {
						return err
					}
					switch w {
					case "true":
						c.SetCheckState(Checked)
					case "false":
						c.SetCheckState(Unchecked)
					case "mixed":
						c.SetCheckState(PartiallyChecked)
					default:
						return fmt.Errorf("checked: unknown value %q", w)
					}
				}
				return nil
			}),
			"tristate": boolProp("tristate", (*Checkbox).SetTriState),
			"wrap":     boolProp("wrap", (*Checkbox).SetWordWrap),
			"action":   actionProp("action"),
		},
		nil,
		func(ctx *protocol.BindContext, w core.Widget) {
			c := w.(*Checkbox)
			id := widgetID(c)
			c.SetOnStateChanged(func(state CheckState) {
				ctx.FireAction(id)
				flag := protocol.FlagFalse
				switch state {
				case Checked:
					flag = protocol.FlagTrue
				case PartiallyChecked:
					flag = protocol.FlagIndeterminate
				}
				ctx.EmitEvent(protocol.NewEvent("toggle").
					WithUint("widget", id).WithFlag("checked", flag))
			})
		},
	)
}
