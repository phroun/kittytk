package widgets

import (
	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/protocol"
)

// Wire registration for Button (see docs/property-vocabulary.md).
func init() {
	regWidget("button",
		func() core.Widget { return NewButton("") },
		map[string]protocol.PropertyApplier{
			"caption": stringProp("caption", (*Button).SetText),
			"default": boolProp("default", (*Button).SetDefault),
			// action is OPTIONAL (D11 note): when set, clicking the
			// button dispatches the command ID.
			"action": actionProp("action", (*Button).SetOnClick),
		},
		nil, // buttons take no children
	)
}
