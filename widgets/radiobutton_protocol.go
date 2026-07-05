package widgets

import (
	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/protocol"
)

// Wire registration for RadioButton (see docs/property-vocabulary.md).
// `group` membership over the wire arrives with a later slice (radio
// groups need identity of their own).
func init() {
	regWidget("radiobutton",
		func() core.Widget { return NewRadioButton("") },
		map[string]protocol.PropertyApplier{
			"caption": stringProp("caption", (*RadioButton).SetText),
			"checked": boolProp("checked", (*RadioButton).SetChecked),
			"wrap":    boolProp("wrap", (*RadioButton).SetWordWrap),
		},
		nil,
	)
}
