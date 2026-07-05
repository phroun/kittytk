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
// v1 limitation (tracked in docs/property-vocabulary.md): tree items
// are not addressable objects yet, so selection events carry the
// item's caption (text=) and visible-row index (selected=); giving
// items wire identity joins the set-verb follow-ups.
func init() {
	protocol.RegisterType("treeview", &protocol.TypeSpec{
		New: func() any { return NewTreeView() },
		ID: func(t any) uint64 {
			return uint64(t.(*TreeView).ObjectID())
		},
		Bind: func(ctx *protocol.BindContext, target any) {
			tv := target.(*TreeView)
			id := uint64(tv.ObjectID())
			tv.SetOnCurrentChanged(func(item *TreeItem) {
				if item == nil {
					return
				}
				ctx.EmitEvent(protocol.NewEvent("change").
					WithUint("widget", id).
					WithInt("selected", tv.CurrentIndex()).
					WithString("text", item.Text))
			})
			tv.SetOnItemActivated(func(item *TreeItem) {
				if item == nil {
					return
				}
				ctx.EmitEvent(protocol.NewEvent("activate").
					WithUint("widget", id).
					WithInt("selected", tv.CurrentIndex()).
					WithString("text", item.Text))
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
			tv.AddRootItem(buildTreeItem(it))
			return nil
		},
		Destroy: func(t any) error {
			return destroyWidget(t.(*TreeView))
		},
	})
}

// buildTreeItem converts a wire item subtree into TreeItems.
func buildTreeItem(it *wireItem) *TreeItem {
	node := NewTreeItem(it.caption)
	node.Expanded = it.expanded
	for _, c := range it.children {
		node.AddChild(buildTreeItem(c))
	}
	return node
}
