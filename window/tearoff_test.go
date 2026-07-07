package window

import (
	"testing"

	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/platform"
)

// nativeFakeSurface is an OS window's worth of fake: unit size, px
// position, and a size setter that reports back through Resized like
// the real platform (scale 1: pixels are units).
type nativeFakeSurface struct {
	size    core.UnitSize
	handler platform.SurfaceHandler
	x, y    int
	closed  bool
}

func (s *nativeFakeSurface) Size() core.UnitSize                  { return s.size }
func (s *nativeFakeSurface) Metrics() core.CellMetrics            { return core.DefaultCellMetrics() }
func (s *nativeFakeSurface) SetHandler(h platform.SurfaceHandler) { s.handler = h }
func (s *nativeFakeSurface) Invalidate(core.UnitRect)             {}
func (s *nativeFakeSurface) SetCursorVisible(bool)                {}
func (s *nativeFakeSurface) SetCursorPosition(x, y core.Unit)     {}
func (s *nativeFakeSurface) ScreenPositionPx() (int, int)         { return s.x, s.y }
func (s *nativeFakeSurface) SetScreenPositionPx(x, y int)         { s.x, s.y = x, y }
func (s *nativeFakeSurface) WorkAreaPx() (int, int, int, int)     { return 0, 0, 1600, 1000 }
func (s *nativeFakeSurface) Close()                               { s.closed = true }

func (s *nativeFakeSurface) SetScreenSizePx(w, h int) {
	s.size = core.UnitSize{Width: core.Unit(w), Height: core.Unit(h)}
	if s.handler != nil {
		s.handler.Resized(s.size)
	}
}

// Edge presses on a torn window resize its OS window with the
// pointer: the right and bottom edges grow it, the left edge moves
// the origin while pinning the right edge.
func TestTearOffHostEdgeResize(t *testing.T) {
	surf := &nativeFakeSurface{size: core.UnitSize{Width: 200, Height: 100}, x: 500, y: 300}
	gx, gy := 700, 380
	win := NewWindow("torn")
	h := NewTearOffHost(win, surf, 1, func() (int, int) { return gx, gy }, nil)

	// Right edge: press within the grip, drag 40 px right.
	h.Event(core.MousePressEvent{X: 197, Y: 50, Button: core.LeftButton})
	gx = 740
	h.Event(core.MouseMoveEvent{X: 237, Y: 50, Buttons: core.LeftButton})
	if surf.size.Width != 240 {
		t.Errorf("right-edge resize: width %d, want 240", surf.size.Width)
	}
	if b := win.Bounds(); b.Width != 240 {
		t.Errorf("window did not track the resized surface: %d", b.Width)
	}
	h.Event(core.MouseReleaseEvent{X: 237, Y: 50, Button: core.LeftButton})

	// Bottom edge: drag 30 px down.
	gx, gy = 700, 380
	h.Event(core.MousePressEvent{X: 100, Y: 97, Button: core.LeftButton})
	gy = 410
	h.Event(core.MouseMoveEvent{X: 100, Y: 127, Buttons: core.LeftButton})
	if surf.size.Height != 130 {
		t.Errorf("bottom-edge resize: height %d, want 130", surf.size.Height)
	}
	h.Event(core.MouseReleaseEvent{X: 100, Y: 127, Button: core.LeftButton})

	// Left edge: drag 20 px left - width grows, origin follows, the
	// right edge stays pinned.
	gx, gy = 700, 380
	rightEdge := surf.x + int(surf.size.Width)
	h.Event(core.MousePressEvent{X: 3, Y: 50, Button: core.LeftButton})
	gx = 680
	h.Event(core.MouseMoveEvent{X: -17, Y: 50, Buttons: core.LeftButton})
	if surf.size.Width != 260 {
		t.Errorf("left-edge resize: width %d, want 260", surf.size.Width)
	}
	if surf.x+int(surf.size.Width) != rightEdge {
		t.Errorf("left-edge resize moved the right edge: %d, want %d",
			surf.x+int(surf.size.Width), rightEdge)
	}
	h.Event(core.MouseReleaseEvent{X: -17, Y: 50, Button: core.LeftButton})

	// A press in the title row near the left edge drags, not resizes.
	h.Event(core.MousePressEvent{X: 120, Y: 8, Button: core.LeftButton})
	if h.resizing {
		t.Error("title-row press armed a resize")
	}
	if !h.Dragging() {
		t.Error("title-row press did not arm the drag")
	}
	h.Event(core.MouseReleaseEvent{X: 120, Y: 8, Button: core.LeftButton})
}

// The maximize button on a torn window zooms it to the display's
// work area (option-zoom, not a fullscreen space); a second press
// restores the saved rect.
func TestTearOffHostZoom(t *testing.T) {
	surf := &nativeFakeSurface{size: core.UnitSize{Width: 200, Height: 100}, x: 500, y: 300}
	win := NewWindow("torn")
	h := NewTearOffHost(win, surf, 1, func() (int, int) { return 0, 0 }, nil)

	h.ToggleZoom()
	if surf.x != 0 || surf.y != 0 || surf.size.Width != 1600 || surf.size.Height != 1000 {
		t.Errorf("zoom did not fill the work area: %d,%d %dx%d",
			surf.x, surf.y, surf.size.Width, surf.size.Height)
	}
	if !win.IsMaximized() {
		t.Error("window chrome not in maximized state while zoomed")
	}

	h.ToggleZoom()
	if surf.x != 500 || surf.y != 300 || surf.size.Width != 200 || surf.size.Height != 100 {
		t.Errorf("zoom did not restore the saved rect: %d,%d %dx%d",
			surf.x, surf.y, surf.size.Width, surf.size.Height)
	}
	if win.IsMaximized() {
		t.Error("window chrome still maximized after restore")
	}
	if b := win.Bounds(); b.Width != 200 || b.Height != 100 {
		t.Errorf("window did not track the restored surface: %dx%d", b.Width, b.Height)
	}
}

// Title-focus keyboard geometry (arrow moves, Shift-arrow resizes)
// maps onto the OS window while torn: the same keys that walk an
// in-surface window around the tuitk desktop walk the torn window
// around the real one.
func TestTearOffHostKeyboardGeometry(t *testing.T) {
	surf := &nativeFakeSurface{size: core.UnitSize{Width: 200, Height: 100}, x: 500, y: 300}
	win := NewWindow("torn")
	h := NewTearOffHost(win, surf, 2, func() (int, int) { return 0, 0 }, nil)

	// Arrow move: -8 units at scale 2 = -16 px.
	if !h.applyKeyboardBounds(core.UnitRect{X: -8, Y: 0, Width: 200, Height: 100}) {
		t.Fatal("bounds delegate not taken")
	}
	if surf.x != 500-16 || surf.y != 300 {
		t.Errorf("keyboard move: window at %d,%d, want %d,%d", surf.x, surf.y, 500-16, 300)
	}

	// Shift-arrow resize: +16 units wide at scale 2 = +32 px.
	h.applyKeyboardBounds(core.UnitRect{X: 0, Y: 0, Width: 216, Height: 100})
	if surf.size.Width != 200*2+32 {
		t.Errorf("keyboard resize: width %d px, want %d", surf.size.Width, 200*2+32)
	}
}
