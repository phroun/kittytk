package widgets

import (
	"fmt"

	"github.com/phroun/tuitk/protocol"
)

// The virtual `item` type unifies list-shaped children (D13): combobox
// entries, listview rows, and treeview nodes are all items; trees nest
// them with children={} blocks:
//
//	new treeview children={
//	    new item caption="Fruit" expanded children={
//	        new item caption="Apple"
//	        new item caption="Pear"
//	    }
//	}
//
// An item is a record, not a widget; each consuming parent converts it
// to its native form on Append.
type wireItem struct {
	caption  string
	expanded bool
	children []*wireItem
}

func init() {
	protocol.RegisterType("item", &protocol.TypeSpec{
		Virtual: true,
		New:     func() any { return &wireItem{} },
		Props: map[string]protocol.PropertyApplier{
			"caption": wprop("caption", func(_ *protocol.BindContext, it *wireItem, v *protocol.Value, f protocol.FlagState) error {
				s, err := protocol.AsString("caption", v, f)
				if err != nil {
					return err
				}
				it.caption = s
				return nil
			}),
			"expanded": wprop("expanded", func(_ *protocol.BindContext, it *wireItem, v *protocol.Value, f protocol.FlagState) error {
				b, err := protocol.AsBool("expanded", v, f)
				if err != nil {
					return err
				}
				it.expanded = b
				return nil
			}),
		},
		Append: func(parent, child any) error {
			p := parent.(*wireItem)
			c, ok := child.(*wireItem)
			if !ok {
				return fmt.Errorf("item: children must be items, got %T", child)
			}
			p.children = append(p.children, c)
			return nil
		},
	})
}
