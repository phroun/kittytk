package window

import (
	"fmt"

	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/protocol"
)

// Wire registration for Window. Per D12, behavior flags are
// individual named flags, never bitsets:
//
//	win=new window title="Tools" x=64 y=64 width=448 height=256 no_resize children={
//	    new panel layout=vbox children={...}
//	}
//
// Coordinates and sizes are in the desktop denomination (D8). The
// single child is the window content; wrap several in a panel.
func init() {
	windowFlagProps := map[string]WindowFlags{
		"frameless":    WindowFlagFrameless,
		"no_title":     WindowFlagNoTitle,
		"no_resize":    WindowFlagNoResize,
		"no_move":      WindowFlagNoMove,
		"no_close":     WindowFlagNoClose,
		"no_minimize":  WindowFlagNoMinimize,
		"no_maximize":  WindowFlagNoMaximize,
		"modal":        WindowFlagModal,
		"stays_on_top": WindowFlagStaysOnTop,
		"tool":         WindowFlagToolWindow,
	}

	props := map[string]protocol.PropertyApplier{
		"title": func(_ *protocol.BindContext, target any, v *protocol.Value, f protocol.FlagState) error {
			s, err := protocol.AsString("title", v, f)
			if err != nil {
				return err
			}
			target.(*Window).SetTitle(s)
			return nil
		},
		// native requests an OS window when the platform can create
		// surfaces (G4 dual mode); single-surface platforms keep the
		// window in-surface.
		"native": func(_ *protocol.BindContext, target any, v *protocol.Value, f protocol.FlagState) error {
			b, err := protocol.AsBool("native", v, f)
			if err != nil {
				return err
			}
			target.(*Window).SetNativeRequested(b)
			return nil
		},
	}

	for _, dim := range []string{"x", "y", "width", "height"} {
		dim := dim
		props[dim] = func(_ *protocol.BindContext, target any, v *protocol.Value, f protocol.FlagState) error {
			n, err := protocol.AsInt(dim, v, f)
			if err != nil {
				return err
			}
			w := target.(*Window)
			b := w.Bounds()
			switch dim {
			case "x":
				b.X = core.Unit(n)
			case "y":
				b.Y = core.Unit(n)
			case "width":
				b.Width = core.Unit(n)
			case "height":
				b.Height = core.Unit(n)
			}
			w.SetBounds(b)
			return nil
		}
	}

	for name, flag := range windowFlagProps {
		name, flag := name, flag
		props[name] = func(_ *protocol.BindContext, target any, v *protocol.Value, f protocol.FlagState) error {
			b, err := protocol.AsBool(name, v, f)
			if err != nil {
				return err
			}
			w := target.(*Window)
			if b {
				w.SetFlags(w.Flags() | flag)
			} else {
				w.SetFlags(w.Flags() &^ flag)
			}
			return nil
		}
	}

	protocol.RegisterType("window", &protocol.TypeSpec{
		New: func() any { return NewWindow("") },
		ID: func(t any) uint64 {
			return uint64(t.(*Window).ObjectID())
		},
		Bind: func(ctx *protocol.BindContext, target any) {
			w := target.(*Window)
			id := uint64(w.ObjectID())
			w.SetOnCloseComplete(func() {
				ctx.EmitEvent(protocol.NewEvent("window_closed").
					WithUint("window", id))
			})
		},
		Props: props,
		Append: func(parent, child any) error {
			w, ok := parent.(*Window)
			if !ok {
				return fmt.Errorf("window: wrong parent type %T", parent)
			}
			cw, ok := child.(core.Widget)
			if !ok {
				return fmt.Errorf("window: content must be a widget, got %T", child)
			}
			if w.Content() != nil {
				return fmt.Errorf("window: only one content widget (wrap several in a panel)")
			}
			w.SetContent(cw)
			return nil
		},
		Destroy: func(t any) error {
			t.(*Window).Close()
			return nil
		},
	})
}
