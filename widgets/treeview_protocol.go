package widgets

import (
	"fmt"

	"github.com/phroun/tuitk/protocol"
)

// Wire registration for TreeView. Nodes are shared virtual items
// (items_protocol.go), nested with children={} blocks and expanded
// flags:
//
//	new treeview children={
//	    new item caption="Fruit" expanded children={
//	        new item caption="Apple"
//	        new item caption="Pear"
//	    }
//	    new item caption="Roots"
//	}
//
// Items are first-class wire objects: each carries an ObjectID, and
// correlation keys name them (`fruit=new item …` → `tree.fruit`, then
// `set tree.fruit caption="…"`, `destroy tree.fruit`). Selection
// events report the item's identity as item=<id> alongside the
// visible-row index.
func init() {
	protocol.RegisterType("treeview", &protocol.TypeSpec{
		New: func() any { return NewTreeView() },
		ID: func(t any) uint64 {
			return uint64(t.(*TreeView).ObjectID())
		},
		Bind: func(ctx *protocol.BindContext, target any) {
			tv := target.(*TreeView)
			id := uint64(tv.ObjectID())
			emit := func(evType string, item *TreeItem) {
				if item == nil {
					return
				}
				ctx.EmitEvent(protocol.NewEvent(evType).
					WithUint("widget", id).
					WithUint("item", uint64(item.ID)).
					WithInt("selected", tv.CurrentIndex()))
			}
			tv.SetOnCurrentChanged(func(item *TreeItem) { emit("change", item) })
			tv.SetOnItemActivated(func(item *TreeItem) { emit("activate", item) })
			tv.SetOnItemExpanded(func(item *TreeItem) {
				if item == nil {
					return
				}
				ctx.EmitEvent(protocol.NewEvent("expand").
					WithUint("widget", id).
					WithUint("item", uint64(item.ID)).
					WithFlag("expanded", protocol.FlagTrue))
			})
			tv.SetOnItemCollapsed(func(item *TreeItem) {
				if item == nil {
					return
				}
				ctx.EmitEvent(protocol.NewEvent("expand").
					WithUint("widget", id).
					WithUint("item", uint64(item.ID)).
					WithFlag("expanded", protocol.FlagFalse))
			})
		},
		Props: map[string]protocol.PropertyApplier{
			"selected":     intProp("selected", (*TreeView).SetCurrentIndex),
			"indent_width": intProp("indent_width", (*TreeView).SetIndentWidth),
		},
		Append: func(parent, child any) error {
			tv, ok := parent.(*TreeView)
			if !ok {
				return fmt.Errorf("treeview: wrong parent type %T", parent)
			}
			it, ok := child.(*wireItem)
			if !ok {
				return fmt.Errorf("treeview: children must be items, got %T", child)
			}
			tv.AddRootItem(it.bind(tv))
			return nil
		},
		Destroy: func(t any) error {
			return destroyWidget(t.(*TreeView))
		},
	})
}
