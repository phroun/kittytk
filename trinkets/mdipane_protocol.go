package trinkets

import (
	"fmt"

	"github.com/phroun/kittytk/core"
	"github.com/phroun/kittytk/protocol"
	"github.com/phroun/kittytk/window"
)

// Wire registration for MDIPane. Child windows are ordinary protocol
// windows appended as children (spawning later is `set pane
// children={new window …}` - D19's append); a non-window child
// becomes the pane's background content. Window-management verbs are
// action properties:
//
//	set pane tile            # flag actions
//	set pane restore=1042    # id-directed actions
//
// Events (all carry window= and, where useful, title=): minimize,
// restore, remove, active (window=0 means none). Note: an MDI child's
// window_closed emission is superseded by the pane's remove event
// (the pane owns the close-complete hook of hosted windows).
func init() {
	// mdiWindowAction adapts an id-directed pane method.
	mdiWindowAction := func(name string, act func(m *MDIPane, w *window.Window)) protocol.PropertyApplier {
		return wprop(name, func(_ *protocol.BindContext, m *MDIPane, v *protocol.Value, f protocol.FlagState) error {
			n, err := protocol.AsInt(name, v, f)
			if err != nil {
				return err
			}
			w := findMDIWindow(m, uint64(n))
			if w == nil {
				return fmt.Errorf("%s: no window %d in this pane", name, n)
			}
			act(m, w)
			return nil
		})
	}
	// mdiFlagAction adapts a no-argument pane method.
	mdiFlagAction := func(name string, act func(m *MDIPane)) protocol.PropertyApplier {
		return wprop(name, func(_ *protocol.BindContext, m *MDIPane, v *protocol.Value, f protocol.FlagState) error {
			b, err := protocol.AsBool(name, v, f)
			if err != nil {
				return err
			}
			if b {
				act(m)
			}
			return nil
		})
	}

	protocol.RegisterType("mdipane", &protocol.TypeSpec{
		New: func() any { return NewMDIPane() },
		ID: func(t any) uint64 {
			return uint64(t.(*MDIPane).ObjectID())
		},
		Bind: func(ctx *protocol.BindContext, target any) {
			m := target.(*MDIPane)
			id := uint64(m.ObjectID())
			winEvent := func(evType string, win *window.Window) {
				if win == nil {
					return
				}
				ctx.EmitEvent(protocol.NewEvent(evType).
					WithUint("trinket", id).
					WithUint("window", uint64(win.ObjectID())).
					WithString("title", win.Title()))
			}
			m.SetOnWindowMinimized(func(w *window.Window) { winEvent("minimize", w) })
			m.SetOnWindowRestored(func(w *window.Window) { winEvent("restore", w) })
			m.SetOnWindowRemoved(func(w *window.Window) { winEvent("remove", w) })
			m.SetOnActiveWindowChanged(func(w *window.Window) {
				ev := protocol.NewEvent("active").WithUint("trinket", id)
				if w != nil {
					ev = ev.WithUint("window", uint64(w.ObjectID())).
						WithString("title", w.Title())
				} else {
					ev = ev.WithUint("window", 0)
				}
				ctx.EmitEvent(ev)
			})
		},
		Props: map[string]protocol.PropertyApplier{
			"fill": wprop("fill", func(_ *protocol.BindContext, m *MDIPane, v *protocol.Value, f protocol.FlagState) error {
				s, err := protocol.AsString("fill", v, f)
				if err != nil {
					return err
				}
				runes := []rune(s)
				if len(runes) != 1 {
					return fmt.Errorf("fill: expected exactly one character")
				}
				m.SetBackgroundChar(runes[0])
				return nil
			}),
			"pattern":  boolProp("pattern", (*MDIPane).SetDrawPattern),
			"tile":     mdiFlagAction("tile", (*MDIPane).TileWindows),
			"cascade":  mdiFlagAction("cascade", (*MDIPane).CascadeWindows),
			"next":     mdiFlagAction("next", (*MDIPane).NextWindow),
			"prev":     mdiFlagAction("prev", (*MDIPane).PrevWindow),
			"restore":  mdiWindowAction("restore", (*MDIPane).RestoreWindow),
			"minimize": mdiWindowAction("minimize", (*MDIPane).MinimizeWindow),
			"remove":   mdiWindowAction("remove", (*MDIPane).RemoveWindow),
		},
		Append: func(parent, child any) error {
			m := parent.(*MDIPane)
			switch c := child.(type) {
			case *window.Window:
				m.AddWindow(c)
				return nil
			case core.Trinket:
				if m.Content() != nil {
					return fmt.Errorf("mdipane: background content already set")
				}
				m.SetContent(c)
				return nil
			default:
				return fmt.Errorf("mdipane: children must be windows or a content trinket, got %T", child)
			}
		},
		Destroy: func(t any) error {
			return destroyTrinket(t.(*MDIPane))
		},
	})
}

// findMDIWindow locates a hosted window by its wire identity.
func findMDIWindow(m *MDIPane, id uint64) *window.Window {
	for _, w := range m.Windows() {
		if uint64(w.ObjectID()) == id {
			return w
		}
	}
	return nil
}
