package widgets

import (
	"fmt"

	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/protocol"
)

// Wire registration for ComboBox and the virtual `item` type
// (see docs/property-vocabulary.md). Per D13's unification, combobox
// entries are children of type item:
//
//	new combobox children={new item caption="A"; new item caption="B"} selected=1
//
// Note: selected must follow the items that make it valid (properties
// apply in order).

// comboItem is the virtual item target: a record, not a widget.
type comboItem struct {
	caption string
}

func init() {
	protocol.RegisterType("item", &protocol.TypeSpec{
		Virtual: true,
		New:     func() any { return &comboItem{} },
		Props: map[string]protocol.PropertyApplier{
			"caption": wprop("caption", func(_ *protocol.BindContext, it *comboItem, v *protocol.Value, f protocol.FlagState) error {
				s, err := protocol.AsString("caption", v, f)
				if err != nil {
					return err
				}
				it.caption = s
				return nil
			}),
		},
	})

	protocol.RegisterType("combobox", &protocol.TypeSpec{
		New: func() any { return NewComboBox() },
		ID: func(t any) uint64 {
			return uint64(t.(*ComboBox).ObjectID())
		},
		Bind: func(ctx *protocol.BindContext, target any) {
			c := target.(*ComboBox)
			id := uint64(c.ObjectID())
			c.SetOnCurrentIndexChanged(func(index int) {
				ctx.EmitEvent(protocol.NewEvent("change").
					WithUint("widget", id).WithInt("selected", index))
			})
		},
		Props: map[string]protocol.PropertyApplier{
			"selected":    intProp("selected", (*ComboBox).SetCurrentIndex),
			"editable":    boolProp("editable", (*ComboBox).SetEditable),
			"placeholder": stringProp("placeholder", (*ComboBox).SetPlaceholder),
			"max_visible": intProp("max_visible", (*ComboBox).SetMaxVisibleItems),
		},
		Append: func(parent, child any) error {
			c, ok := parent.(*ComboBox)
			if !ok {
				return fmt.Errorf("combobox: wrong parent type %T", parent)
			}
			it, ok := child.(*comboItem)
			if !ok {
				return fmt.Errorf("combobox: children must be items, got %T", child)
			}
			c.AddItem(it.caption)
			return nil
		},
	})
}

// ensure ComboBox still satisfies core.Widget for regWidget users
var _ core.Widget = (*ComboBox)(nil)
