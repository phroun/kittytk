package widgets

import (
	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/protocol"
)

// Wire registration for the progress bar widget (see docs/property-vocabulary.md).
func init() {
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
