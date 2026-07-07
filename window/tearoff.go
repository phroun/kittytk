package window

import (
	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/platform"
)

// TearOffHost runs one desktop window as the entire content of its
// own OS surface with the tuitk chrome intact - the torn-off half of
// G4's granting. The surface is borderless: the window's own title
// bar stays the drag handle, but here a title drag moves the OS
// window itself (via the platform's global pointer), and the host's
// redock callback lets the desktop reclaim the window when the
// pointer crosses back over it mid-drag.
type TearOffHost struct {
	win    *Window
	surf   platform.Surface
	native platform.NativeSurface
	scale  int // device pixels per unit
	global func() (int, int)

	// onRedock runs during a title drag with the pointer at the given
	// global pixel position and the grab point in window units.
	// Returning true means the desktop took the window back; the host
	// must go quiet (its surface is closed by the callback).
	onRedock func(globalX, globalY int, grabX, grabY core.Unit) bool

	savedFlags WindowFlags

	dragging bool
	grabX    core.Unit
	grabY    core.Unit
}

// NewTearOffHost attaches the window to its own surface. Unlike
// SurfaceHost no chrome is suppressed; maximize/minimize make no
// sense without a managing desktop and are masked until re-dock.
// Call on the platform thread.
func NewTearOffHost(win *Window, surf platform.Surface, scale int,
	global func() (int, int),
	onRedock func(globalX, globalY int, grabX, grabY core.Unit) bool) *TearOffHost {
	h := &TearOffHost{win: win, surf: surf, scale: scale, global: global, onRedock: onRedock}
	h.native, _ = surf.(platform.NativeSurface)
	if h.scale < 1 {
		h.scale = 1
	}

	h.savedFlags = win.Flags()
	win.SetFlags(h.savedFlags | WindowFlagNoMaximize | WindowFlagNoMinimize | WindowFlagNoResize)

	size := surf.Size()
	win.SetBounds(core.UnitRect{Width: size.Width, Height: size.Height})
	win.Layout()
	win.SetActive(true)

	surf.SetHandler(h)
	surf.Invalidate(core.UnitRect{})
	return h
}

// Window returns the hosted window.
func (h *TearOffHost) Window() *Window { return h.win }

// Surface returns the hosted surface.
func (h *TearOffHost) Surface() platform.Surface { return h.surf }

// SavedFlags returns the window's flags from before the tear-off,
// for the desktop to restore on re-dock.
func (h *TearOffHost) SavedFlags() WindowFlags { return h.savedFlags }

// BeginDrag arms the OS-window drag as if the user had pressed the
// title bar at the given window-unit grab point. The tear-off
// choreography uses it so the gesture that tore the window continues
// seamlessly in the new surface.
func (h *TearOffHost) BeginDrag(grabX, grabY core.Unit) {
	h.dragging = true
	h.grabX, h.grabY = grabX, grabY
}

// Dragging reports whether a title drag is moving the OS window.
func (h *TearOffHost) Dragging() bool { return h.dragging }

// Frame implements platform.SurfaceHandler.
func (h *TearOffHost) Frame(p *core.Painter) {
	h.win.Paint(p)
}

// Event implements platform.SurfaceHandler: surface coordinates ARE
// window coordinates. A title-bar press the window doesn't consume
// starts an OS-window drag, mirroring the WindowManager's in-surface
// title drag.
func (h *TearOffHost) Event(ev core.Event) bool {
	var handled bool
	switch e := ev.(type) {
	case core.KeyPressEvent:
		handled = h.win.HandleKeyPress(e)
	case core.KeyReleaseEvent:
		handled = h.win.HandleKeyRelease(e)
	case core.MousePressEvent:
		handled = h.win.HandleMousePress(e)
		if !handled && e.Button == core.LeftButton && h.inTitleBar(e.X, e.Y) {
			h.BeginDrag(e.X, e.Y)
			handled = true
		}
	case core.MouseMoveEvent:
		if h.dragging {
			handled = h.dragMove()
		} else {
			handled = h.win.HandleMouseMove(e)
		}
	case core.MouseReleaseEvent:
		if h.dragging {
			h.dragging = false
			handled = true
		} else {
			handled = h.win.HandleMouseRelease(e)
		}
	case core.MouseWheelEvent:
		handled = h.win.HandleMouseWheel(e)
	}
	// Parity contract: repaint after input until widgets migrate to
	// precise invalidation.
	h.surf.Invalidate(core.UnitRect{})
	return handled
}

// dragMove follows the global pointer: first the desktop gets a
// chance to reclaim the window (pointer back over the desktop
// surface), otherwise the OS window moves to keep the grab point
// under the pointer.
func (h *TearOffHost) dragMove() bool {
	if h.global == nil || h.native == nil {
		return true
	}
	gx, gy := h.global()
	if h.onRedock != nil && h.onRedock(gx, gy, h.grabX, h.grabY) {
		h.dragging = false
		return true
	}
	h.native.SetScreenPositionPx(gx-int(h.grabX)*h.scale, gy-int(h.grabY)*h.scale)
	return true
}

// inTitleBar reports whether the point sits in the window's title
// row (the drag handle), matching the WindowManager's notion: the
// top cell row, excluding nothing else - button clicks were already
// offered to the window and declined.
func (h *TearOffHost) inTitleBar(x, y core.Unit) bool {
	b := h.win.Bounds()
	th := core.DefaultCellMetrics().CellHeight
	return x >= 0 && x < b.Width && y >= 0 && y < th
}

// Resized implements platform.SurfaceHandler: the window tracks the
// surface.
func (h *TearOffHost) Resized(size core.UnitSize) {
	h.win.SetBounds(core.UnitRect{Width: size.Width, Height: size.Height})
	h.win.Layout()
	h.surf.Invalidate(core.UnitRect{})
}
