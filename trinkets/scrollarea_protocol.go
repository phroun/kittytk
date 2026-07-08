package trinkets

import (
	"fmt"

	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/protocol"
)

// Wire registration for ScrollArea: a viewport around exactly one
// content trinket.
//
//	new scrollarea resizable children={new panel layout=vbox children={...}}
func init() {
	protocol.RegisterType("scrollarea", &protocol.TypeSpec{
		New: func() any { return NewScrollArea() },
		ID: func(t any) uint64 {
			return uint64(t.(*ScrollArea).ObjectID())
		},
		Props: map[string]protocol.PropertyApplier{
			"scroll_x":  intProp("scroll_x", (*ScrollArea).SetScrollX),
			"scroll_y":  intProp("scroll_y", (*ScrollArea).SetScrollY),
			"resizable": boolProp("resizable", (*ScrollArea).SetTrinketResizable),
		},
		Append: func(parent, child any) error {
			sa, ok := parent.(*ScrollArea)
			if !ok {
				return fmt.Errorf("scrollarea: wrong parent type %T", parent)
			}
			w, ok := child.(core.Trinket)
			if !ok {
				return fmt.Errorf("scrollarea: content must be a trinket, got %T", child)
			}
			if sa.Content() != nil {
				return fmt.Errorf("scrollarea: only one content trinket (wrap several in a panel)")
			}
			sa.SetContent(w)
			return nil
		},
		Destroy: func(t any) error {
			return destroyTrinket(t.(*ScrollArea))
		},
	})
}
