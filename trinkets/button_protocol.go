package trinkets

import (
	"github.com/phroun/kittytk/core"
	"github.com/phroun/kittytk/protocol"
)

// Wire registration for Button (see docs/property-vocabulary.md).
func init() {
	regTrinket("button",
		func() core.Trinket { return NewButton("") },
		map[string]protocol.PropertyApplier{
			"caption": stringProp("caption", (*Button).SetText),
			"default": boolProp("default", (*Button).SetDefault),
			// action is OPTIONAL: when set, clicking dispatches the
			// command ID (via BindContext.FireAction in the click
			// wiring below).
			"action": actionProp("action"),
		},
		nil, // buttons take no children
		func(ctx *protocol.BindContext, w core.Trinket) {
			b := w.(*Button)
			id := trinketID(b)
			b.SetOnClick(func() {
				ctx.FireAction(id)
				ctx.EmitEvent(protocol.NewEvent("click").WithUint("trinket", id))
			})
		},
	)
}
