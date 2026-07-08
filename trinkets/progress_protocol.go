package trinkets

import (
	"github.com/phroun/kittytk/core"
	"github.com/phroun/kittytk/protocol"
)

// Wire registration for the progress bar trinket (see docs/property-vocabulary.md).
func init() {
	regTrinket("progress",
		func() core.Trinket { return NewProgressBar() },
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
