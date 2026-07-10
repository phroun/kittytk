package window

import (
	"fmt"

	"github.com/phroun/kittytk/core"
	"github.com/phroun/kittytk/protocol"
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
		"tearable":     WindowFlagTearable,
	}

	props := map[string]protocol.Property{
		"title": protocol.NewProperty("string", func(_ *protocol.BindContext, target any, v *protocol.Value, f protocol.FlagState) error {
			s, err := protocol.AsString("title", v, f)
			if err != nil {
				return err
			}
			target.(*Window).SetTitle(s)
			return nil
		}).Tip("Window title bar text"),
		// native requests an OS window when the platform can create
		// surfaces (G4 dual mode); single-surface platforms keep the
		// window in-surface.
		"native": protocol.NewProperty("flag", func(_ *protocol.BindContext, target any, v *protocol.Value, f protocol.FlagState) error {
			b, err := protocol.AsBool("native", v, f)
			if err != nil {
				return err
			}
			target.(*Window).SetNativeRequested(b)
			return nil
		}).Tip("Request a native OS window").Def("false"),
		// main marks this window as its application's main window: its
		// menu/status chrome detaches with it on tear-off. The host acts
		// on it when adopting the window.
		"main": protocol.NewProperty("flag", func(_ *protocol.BindContext, target any, v *protocol.Value, f protocol.FlagState) error {
			b, err := protocol.AsBool("main", v, f)
			if err != nil {
				return err
			}
			target.(*Window).SetMainRequested(b)
			return nil
		}).Tip("Mark as the application's main window").Def("false"),
		// font overrides the window's font (its content inherits it);
		// empty / "default" clears the override back to the desktop's.
		"font": protocol.NewProperty("string", func(_ *protocol.BindContext, target any, v *protocol.Value, f protocol.FlagState) error {
			s, err := protocol.AsString("font", v, f)
			if err != nil {
				return err
			}
			target.(*Window).SetFont(namedFont(s))
			return nil
		}).Tip("Window font override (\"default\" clears)"),
		// denomination overrides the window's row height in units (its
		// content re-grids to it); 0 clears the override.
		"denomination": protocol.NewProperty("int", func(_ *protocol.BindContext, target any, v *protocol.Value, f protocol.FlagState) error {
			n, err := protocol.AsInt("denomination", v, f)
			if err != nil {
				return err
			}
			w := target.(*Window)
			if n <= 0 {
				w.SetCellMetrics(nil)
			} else {
				m := core.DefaultCellMetrics()
				m.CellHeight = core.Unit(n)
				w.SetCellMetrics(&m)
			}
			return nil
		}).Tip("Row height override in units (0 clears)"),
	}

	for _, dim := range []string{"x", "y", "width", "height"} {
		dim := dim
		props[dim] = protocol.NewProperty("int", func(_ *protocol.BindContext, target any, v *protocol.Value, f protocol.FlagState) error {
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
		}).Tip("Window " + dim + " in desktop units")
	}

	for name, flag := range windowFlagProps {
		name, flag := name, flag
		props[name] = protocol.NewProperty("flag", func(_ *protocol.BindContext, target any, v *protocol.Value, f protocol.FlagState) error {
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
		}).Tip(name + " behavior flag").Def("false")
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
			cw, ok := child.(core.Trinket)
			if !ok {
				return fmt.Errorf("window: content must be a trinket, got %T", child)
			}
			if w.Content() != nil {
				return fmt.Errorf("window: only one content trinket (wrap several in a panel)")
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

// namedFont maps a protocol font name to a built-in font, or nil (inherit
// from the desktop) for empty / "default".
func namedFont(name string) *core.Font {
	switch name {
	case "monday", "monday12", "mono":
		return core.FontMonday12
	case "tuesday", "tuesday12":
		return core.FontTuesday12
	case "uitext", "uitext12", "ui":
		return core.FontUIText12
	default:
		return nil
	}
}
