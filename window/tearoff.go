package window

import (
	"time"

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

	// Edge-resize drag: the OS window resizes with the pointer.
	resizing    bool
	resizeEdges int // resizeLeft | resizeRight | resizeBottom
	startGX     int // global pointer at resize start, px
	startGY     int
	startX      int // OS window rect at resize start, px
	startY      int
	startW      int
	startH      int

	// Zoom (the maximize button while torn): fill the display's work
	// area, second press restores the saved rect.
	zoomed    bool
	zoomSaved [4]int // x, y, w, h in px
	// dragRestored latches after a title drag un-zooms the window, so
	// the same drag can't snap-zoom right back until the pointer has
	// clearly left the top strip.
	dragRestored bool

	// Double-click tracking for the title bar (zoom toggle), matching
	// the in-surface manager's maximize double-click.
	lastClickAt time.Time
	lastClickX  core.Unit
	lastClickY  core.Unit

	// onClosed runs when the hosted window closes itself (the [x]
	// button): the desktop disposes of the surface. Without it the
	// closed window would keep showing in its orphaned OS window.
	onClosed func()

	// Ghost mode: the desktop has re-adopted the window mid-drag, but
	// THIS window still owns the OS mouse session (the press happened
	// here). The surface goes invisible instead of being destroyed,
	// and the rest of the gesture relays to the desktop; the release
	// finishes it and the desktop then closes the surface. Destroying
	// the session's window mid-gesture loses the release and wedges
	// the platform's button state.
	ghost       bool
	onGhostMove func(gx, gy int)
	onGhostEnd  func()
}

// Resize edge bits. The top edge is the title bar (drag handle), so
// only left/right/bottom resize - matching the in-surface manager.
const (
	resizeLeft = 1 << iota
	resizeRight
	resizeBottom
)

// tearResizeGrip is the edge thickness (units) that starts a resize.
const tearResizeGrip core.Unit = 6

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
	// Minimizing has no meaning without a managing desktop; resize and
	// maximize stay - the host maps them onto the OS window (maximize
	// zooms to the display's work area, macOS option-zoom style).
	win.SetFlags(h.savedFlags | WindowFlagNoMinimize)
	win.SetOnMaximizeRequest(h.ToggleZoom)
	win.SetOnBoundsRequest(h.applyKeyboardBounds)
	win.SetOnCloseComplete(func() {
		if h.onClosed != nil {
			h.onClosed()
		}
	})

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

// Invalidate requests a repaint of the hosted window. The desktop's
// repaint tick calls it so animation (blinking carets, indeterminate
// progress) keeps running in torn-off windows.
func (h *TearOffHost) Invalidate() {
	h.surf.Invalidate(core.UnitRect{})
}

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

// SetOnClosed installs the desktop's disposal for a torn window that
// closes itself.
func (h *TearOffHost) SetOnClosed(fn func()) { h.onClosed = fn }

// SetGhostRelay installs the desktop's continuation for a gesture
// that outlives its window: move relays motion (global px), end
// finishes the drag and disposes of the ghost surface.
func (h *TearOffHost) SetGhostRelay(move func(gx, gy int), end func()) {
	h.onGhostMove = move
	h.onGhostEnd = end
}

// finishGhost ends the relayed gesture.
func (h *TearOffHost) finishGhost() {
	h.ghost = false
	h.dragging = false
	if h.onGhostEnd != nil {
		h.onGhostEnd()
	}
}

// EndDrag disarms the drag and its restore latch. The desktop calls it when the gesture's
// end shows up on its side of the split event stream (release, or a
// move with the button no longer held) - without it a later drag
// inside the torn window's content would move the OS window.
func (h *TearOffHost) EndDrag() {
	h.dragging = false
	h.dragRestored = false
}

// Frame implements platform.SurfaceHandler.
func (h *TearOffHost) Frame(p *core.Painter) {
	if h.ghost {
		// The window lives on the desktop again; this surface only
		// survives (invisibly) to finish its mouse session.
		return
	}
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
		if h.ghost {
			// A press reaching a ghost means its release was lost:
			// finish the relay and swallow the stray press.
			h.finishGhost()
			handled = true
			break
		}
		// A press while a drag/resize is still armed means the
		// gesture's release was lost in the split event stream:
		// disarm and process the press normally.
		h.dragging = false
		h.resizing = false
		if e.Button == core.LeftButton && h.beginResize(e.X, e.Y) {
			handled = true
			break
		}
		handled = h.win.HandleMousePress(e)
		if !handled && e.Button == core.LeftButton && h.inTitleBar(e.X, e.Y) {
			// Double-click on the title bar toggles the zoom, exactly
			// as it toggles maximize in-surface.
			metrics := core.DefaultCellMetrics()
			now := time.Now()
			if now.Sub(h.lastClickAt) < 400*time.Millisecond &&
				e.X-h.lastClickX < metrics.CellWidth && h.lastClickX-e.X < metrics.CellWidth &&
				e.Y-h.lastClickY < metrics.CellHeight && h.lastClickY-e.Y < metrics.CellHeight {
				h.lastClickAt = time.Time{}
				h.ToggleZoom()
			} else {
				h.lastClickAt = now
				h.lastClickX, h.lastClickY = e.X, e.Y
				h.BeginDrag(e.X, e.Y)
			}
			handled = true
		}
	case core.MouseMoveEvent:
		if h.ghost {
			if e.Buttons&core.LeftButton == 0 {
				h.finishGhost()
			} else if h.global != nil && h.onGhostMove != nil {
				gx, gy := h.global()
				h.onGhostMove(gx, gy)
			}
			handled = true
		} else if (h.resizing || h.dragging) && e.Buttons&core.LeftButton == 0 {
			// Button no longer held: the release happened where we
			// couldn't see it. The gesture is over - do not move the
			// window on a mere hover.
			h.resizing = false
			h.dragging = false
			handled = h.win.HandleMouseMove(e)
		} else if h.resizing {
			handled = h.resizeMove()
		} else if h.dragging {
			handled = h.dragMove()
		} else {
			handled = h.win.HandleMouseMove(e)
		}
	case core.MouseReleaseEvent:
		if h.ghost {
			h.finishGhost()
			handled = true
		} else if h.resizing || h.dragging {
			h.resizing = false
			h.dragging = false
			h.dragRestored = false
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
// under the pointer. In-surface parity for the zoom state: dragging
// a zoomed window down restores it (grab kept proportional), and
// dragging the pointer above the work area's top snap-zooms.
func (h *TearOffHost) dragMove() bool {
	if h.global == nil || h.native == nil {
		return true
	}
	gx, gy := h.global()
	if h.onRedock != nil && h.onRedock(gx, gy, h.grabX, h.grabY) {
		// The desktop took the window; this surface stays (invisible)
		// to relay the rest of its live mouse session.
		h.ghost = true
		return true
	}
	_, way, ww, wh := h.native.WorkAreaPx()
	if h.zoomed {
		// A zoomed window doesn't slide; dragging its title below the
		// work area's top restores it, with the grab point staying
		// proportionally placed on the narrower title bar.
		if gy-int(h.grabY)*h.scale >= way {
			if ww > 0 {
				h.grabX = core.Unit(float64(h.grabX) * float64(h.zoomSaved[2]) / float64(ww))
			}
			h.zoomed = false
			h.dragRestored = true
			h.win.Restore()
			h.native.SetScreenSizePx(h.zoomSaved[2], h.zoomSaved[3])
			h.native.SetScreenPositionPx(gx-int(h.grabX)*h.scale, gy-int(h.grabY)*h.scale)
		}
		return true
	}
	if h.dragRestored && gy >= way+int(core.DefaultCellMetrics().CellHeight)*h.scale {
		// Pointer clearly below the top strip: re-arm the snap.
		h.dragRestored = false
	}
	if ww > 0 && wh > 0 && !h.dragRestored &&
		(gy < way || (way <= 0 && gy <= 0)) {
		// Into the strip above the work area (the macOS menu bar):
		// snap-zoom, exactly like dragging into the desktop's menu
		// bar maximizes in-surface. Keep dragging so the user can
		// pull back down to restore.
		h.zoomToWorkArea()
		return true
	}
	h.native.SetScreenPositionPx(gx-int(h.grabX)*h.scale, gy-int(h.grabY)*h.scale)
	return true
}

// beginResize arms an edge resize when the press lands within the
// grip distance of the left, right, or bottom edge (the top edge is
// the title bar). Returns false when the window is not resizable or
// the press is interior.
func (h *TearOffHost) beginResize(x, y core.Unit) bool {
	if h.native == nil || h.global == nil || h.zoomed ||
		h.win.Flags()&WindowFlagNoResize != 0 {
		return false
	}
	b := h.win.Bounds()
	edges := 0
	if x < tearResizeGrip {
		edges |= resizeLeft
	}
	if x >= b.Width-tearResizeGrip {
		edges |= resizeRight
	}
	if y >= b.Height-tearResizeGrip {
		edges |= resizeBottom
	}
	if edges == 0 || y < core.DefaultCellMetrics().CellHeight {
		// Interior press, or within the title row (drag, not resize).
		return false
	}
	h.resizing = true
	h.resizeEdges = edges
	h.startGX, h.startGY = h.global()
	h.startX, h.startY = h.native.ScreenPositionPx()
	size := h.surf.Size()
	h.startW = int(size.Width) * h.scale
	h.startH = int(size.Height) * h.scale
	return true
}

// resizeMove applies the pointer delta to the armed edges, moving and
// resizing the OS window; the size change reports back through
// Resized and the window re-lays out to the surface.
func (h *TearOffHost) resizeMove() bool {
	gx, gy := h.global()
	dx, dy := gx-h.startGX, gy-h.startGY
	metrics := core.DefaultCellMetrics()
	minW := int(metrics.CellWidth) * 12 * h.scale
	minH := int(metrics.CellHeight) * 4 * h.scale

	x, y, w, ht := h.startX, h.startY, h.startW, h.startH
	if h.resizeEdges&resizeLeft != 0 {
		w -= dx
		if w < minW {
			dx -= minW - w
			w = minW
		}
		x += dx
	}
	if h.resizeEdges&resizeRight != 0 {
		w += dx
		if w < minW {
			w = minW
		}
	}
	if h.resizeEdges&resizeBottom != 0 {
		ht += dy
		if ht < minH {
			ht = minH
		}
	}
	if h.resizeEdges&resizeLeft != 0 {
		h.native.SetScreenPositionPx(x, y)
	}
	h.native.SetScreenSizePx(w, ht)
	return true
}

// ToggleZoom fills the display's work area (the maximize button's
// meaning while torn - macOS option-zoom, not a fullscreen space);
// a second toggle restores the saved rect.
func (h *TearOffHost) ToggleZoom() {
	if h.native == nil {
		return
	}
	if h.zoomed {
		h.zoomed = false
		h.win.Restore()
		h.native.SetScreenPositionPx(h.zoomSaved[0], h.zoomSaved[1])
		h.native.SetScreenSizePx(h.zoomSaved[2], h.zoomSaved[3])
		return
	}
	h.zoomToWorkArea()
}

// zoomToWorkArea saves the current rect and fills the display's work
// area.
func (h *TearOffHost) zoomToWorkArea() {
	wx, wy, ww, wh := h.native.WorkAreaPx()
	if ww <= 0 || wh <= 0 {
		return
	}
	x, y := h.native.ScreenPositionPx()
	size := h.surf.Size()
	h.zoomSaved = [4]int{x, y, int(size.Width) * h.scale, int(size.Height) * h.scale}
	h.zoomed = true
	h.win.Maximize()
	h.native.SetScreenPositionPx(wx, wy)
	h.native.SetScreenSizePx(ww, wh)
}

// applyKeyboardBounds maps a title-focus keyboard geometry change
// (arrow move, Shift-arrow resize, Escape revert) onto the OS
// window: position deltas move it across the real desktop, size
// deltas resize it, exactly as the same keys move an in-surface
// window around the tuitk desktop.
func (h *TearOffHost) applyKeyboardBounds(b core.UnitRect) bool {
	if h.native == nil || h.zoomed {
		return h.zoomed // zoomed: swallow, geometry is the work area's
	}
	cur := h.win.Bounds()
	dx := int(b.X-cur.X) * h.scale
	dy := int(b.Y-cur.Y) * h.scale
	dw := int(b.Width-cur.Width) * h.scale
	dh := int(b.Height-cur.Height) * h.scale
	if dx != 0 || dy != 0 {
		x, y := h.native.ScreenPositionPx()
		h.native.SetScreenPositionPx(x+dx, y+dy)
	}
	if (dw != 0 || dh != 0) && h.win.Flags()&WindowFlagNoResize == 0 {
		size := h.surf.Size()
		h.native.SetScreenSizePx(int(size.Width)*h.scale+dw, int(size.Height)*h.scale+dh)
	}
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
