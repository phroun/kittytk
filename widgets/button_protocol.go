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
			// action is OPTIONAL: when set, clicking dispatches the
			// command ID (via BindContext.FireAction in the click
			// wiring below).
			"action": actionProp("action"),
		},
		nil, // buttons take no children
		func(ctx *protocol.BindContext, w core.Widget) {
			b := w.(*Button)
			id := widgetID(b)
			b.SetOnClick(func() {
				ctx.FireAction(id)
				ctx.EmitEvent(protocol.NewEvent("click").WithUint("widget", id))
			})
		},
	)
}
