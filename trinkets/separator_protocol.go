package trinkets

import (
	"fmt"

	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/protocol"
)

// Wire registration for the separator trinket (see docs/property-vocabulary.md).
func init() {
	regTrinket("separator",
		func() core.Trinket { return NewLineSeparator() },
		map[string]protocol.PropertyApplier{
			"caption":     stringProp("caption", (*LineSeparator).SetTitle),
			"orientation": orientationProp[*LineSeparator]((*LineSeparator).SetOrientation),
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
