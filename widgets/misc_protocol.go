package widgets

import (
	"fmt"

	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/protocol"
)

// Wire registrations for the small widgets: spacer, separator,
// progress (see docs/property-vocabulary.md).
func init() {
	regWidget("spacer",
		func() core.Widget { return NewSpacer() },
		map[string]protocol.PropertyApplier{
			"width": wprop("width", func(_ *protocol.BindContext, s *Spacer, v *protocol.Value, f protocol.FlagState) error {
				n, err := protocol.AsInt("width", v, f)
				if err != nil {
					return err
				}
				size := s.Size()
				size.Width = core.Unit(n)
				s.SetSize(size)
				return nil
			}),
			"height": wprop("height", func(_ *protocol.BindContext, s *Spacer, v *protocol.Value, f protocol.FlagState) error {
				n, err := protocol.AsInt("height", v, f)
				if err != nil {
					return err
				}
				size := s.Size()
				size.Height = core.Unit(n)
				s.SetSize(size)
				return nil
			}),
		},
		nil,
		nil,
	)

	regWidget("separator",
		func() core.Widget { return NewLineSeparator() },
		map[string]protocol.PropertyApplier{
			"caption":     stringProp("caption", (*LineSeparator).SetTitle),
			"orientation": orientationProp[*LineSeparator]((*LineSeparator).SetOrientation),
		},
		nil,
		nil,
	)

	regWidget("progress",
		func() core.Widget { return NewProgressBar() },
		map[string]protocol.PropertyApplier{
			"value":         intProp("value", (*ProgressBar).SetValue),
			"minimum":       intProp("minimum", (*ProgressBar).SetMinimum),
			"maximum":       intProp("maximum", (*ProgressBar).SetMaximum),
			"caption":       stringProp("caption", (*ProgressBar).SetFormat),
			"indeterminate": boolProp("indeterminate", (*ProgressBar).SetIndeterminate),
		},
		nil,
		nil,
	)
}

// orientationProp maps the horizontal/vertical enum onto a setter.
func orientationProp[T any](set func(w T, o core.Orientation)) protocol.PropertyApplier {
	return wprop("orientation", func(_ *protocol.BindContext, w T, v *protocol.Value, f protocol.FlagState) error {
		word, err := protocol.AsWord("orientation", v, f)
		if err != nil {
			return err
		}
		switch word {
		case "horizontal":
			set(w, core.Horizontal)
		case "vertical":
			set(w, core.Vertical)
		default:
			return fmt.Errorf("orientation: unknown value %q", word)
		}
		return nil
	})
}
