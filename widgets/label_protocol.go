package widgets

import (
	"fmt"

	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/protocol"
)

// Wire registration for Label (see docs/property-vocabulary.md).
func init() {
	regWidget("label",
		func() core.Widget { return NewLabel("") },
		map[string]protocol.PropertyApplier{
			"caption": stringProp("caption", (*Label).SetText),
			"wrap":    boolProp("wrap", (*Label).SetWordWrap),
			// text_align is the TEXT alignment within the label;
			// the common `align` property is the layout-item hint.
			// Distinct concepts, distinct names.
			"text_align": wprop("text_align", func(_ *protocol.BindContext, l *Label, v *protocol.Value, f protocol.FlagState) error {
				w, err := protocol.AsWord("text_align", v, f)
				if err != nil {
					return err
				}
				switch w {
				case "left":
					l.SetAlignment(core.AlignLeft)
				case "center":
					l.SetAlignment(core.AlignCenter)
				case "right":
					l.SetAlignment(core.AlignRight)
				default:
					return fmt.Errorf("text_align: unknown value %q", w)
				}
				return nil
			}),
		},
		nil,
		nil,
	)
}
