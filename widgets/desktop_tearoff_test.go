package widgets

import (
	"testing"
	"time"

	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/platform"
	"github.com/phroun/tuitk/raster"
	"github.com/phroun/tuitk/window"
)

// msSurface is a fake native surface: an OS window's worth of state.
type msSurface struct {
	size    core.UnitSize
	handler platform.SurfaceHandler
	x, y    int
	closed  bool
	opts    platform.SurfaceOptions
}

func (s *msSurface) Size() core.UnitSize                  { return s.size }
func (s *msSurface) Metrics() core.CellMetrics            { return core.DefaultCellMetrics() }
func (s *msSurface) SetHandler(h platform.SurfaceHandler) { s.handler = h }
func (s *msSurface) Invalidate(core.UnitRect)             {}
func (s *msSurface) SetCursorVisible(bool)                {}
func (s *msSurface) SetCursorPosition(x, y core.Unit)     {}
func (s *msSurface) ScreenPositionPx() (int, int)         { return s.x, s.y }
func (s *msSurface) SetScreenPositionPx(x, y int)         { s.x, s.y = x, y }
func (s *msSurface) Close()                               { s.closed = true }
func (s *msSurface) WorkAreaPx() (int, int, int, int)     { return 0, 0, 1600, 1000 }

// SetScreenSizePx mimics the real platform: the size change reports
// back through Resized (scale 1: pixels are units).
func (s *msSurface) SetScreenSizePx(w, h int) {
	s.size = core.UnitSize{Width: core.Unit(w), Height: core.Unit(h)}
	if s.handler != nil {
		s.handler.Resized(s.size)
	}
}

// msPlatform is a fake multi-surface platform: the desktop surface
// plus any torn-off windows, with a scriptable global pointer.
type msPlatform struct {
	surfaces []*msSurface
	script   func()
	gx, gy   int
}

func (p *msPlatform) Run(init func(platform.Platform)) int {
	init(p)
	if p.script != nil {
		p.script()
	}
	return 0
}
func (p *msPlatform) Post(fn func())                  { fn() }
func (p *msPlatform) PostAfter(time.Duration, func()) {}
func (p *msPlatform) Quit(int)                        {}
func (p *msPlatform) Clipboard() string               { return "" }
func (p *msPlatform) SetClipboard(string)             {}
func (p *msPlatform) Beep()                           {}
func (p *msPlatform) SupportsMultipleSurfaces() bool  { return true }
func (p *msPlatform) GlobalPointerPx() (int, int)     { return p.gx, p.gy }
func (p *msPlatform) CreateSurface(o platform.SurfaceOptions) (platform.Surface, error) {
	s := &msSurface{opts: o, x: o.XPx, y: o.YPx}
	if len(p.surfaces) == 0 {
		// The desktop window: 800x480 units at 50,60 px, scale 1.
		s.size = core.UnitSize{Width: 800, Height: 480}
		s.x, s.y = 50, 60
	} else {
		s.size = core.UnitSize{Width: core.Unit(o.WidthPx), Height: core.Unit(o.HeightPx)}
	}
	p.surfaces = append(p.surfaces, s)
	return s, nil
}

func containsWindow(wm *window.WindowManager, win *window.Window) bool {
	for _, w := range wm.Windows() {
		if w == win {
			return true
		}
	}
	return false
}

// Dragging a desktop window past the surface edge tears it off into
// its own surface; the live gesture keeps driving that surface from
// the desktop's captured pointer stream, and crossing back over the
// desktop re-docks it with the drag still armed.
func TestTearOffAndRedockDuringDrag(t *testing.T) {
	t.Cleanup(func() { core.SetTextMeasurer(nil) })
	px, err := raster.New(800, 480) // scale 1: pixels are units
	if err != nil {
		t.Fatal(err)
	}
	d := NewDesktop()
	d.SetBackend(px)

	win := window.NewWindow("tearme")
	d.SetOnStartup(func() {
		d.WindowManager().AddWindow(win)
		win.SetBounds(core.UnitRect{X: 100, Y: 100, Width: 200, Height: 100})
		win.Layout()
	})

	plat := &msPlatform{}
	plat.script = func() {
		desk := plat.surfaces[0]
		wm := d.WindowManager()
		send := func(ev core.Event) { desk.handler.Event(ev) }

		// Grab the title bar (offset 120,8 into the window, clear of
		// the title buttons) and move.
		send(core.MousePressEvent{X: 220, Y: 108, Button: core.LeftButton})
		send(core.MouseMoveEvent{X: 230, Y: 110, Buttons: core.LeftButton})
		if b := win.Bounds(); b.X != 110 || b.Y != 102 {
			t.Fatalf("in-surface drag not working: window at %d,%d, want 110,102", b.X, b.Y)
		}

		// Cross the left edge: the window must tear off.
		send(core.MouseMoveEvent{X: -30, Y: 150, Buttons: core.LeftButton})
		if len(plat.surfaces) != 2 {
			t.Fatalf("expected a second surface after tear-off, have %d", len(plat.surfaces))
		}
		torn := plat.surfaces[1]
		if !torn.opts.Borderless {
			t.Error("torn surface should be borderless (tuitk chrome kept)")
		}
		if containsWindow(wm, win) {
			t.Error("window still managed by the desktop after tear-off")
		}
		if torn.x != 50+(-30-120) || torn.y != 60+(150-8) {
			t.Errorf("torn surface at %d,%d; want %d,%d", torn.x, torn.y, 50+(-30-120), 60+(150-8))
		}

		// Still dragging outside: the torn surface follows.
		send(core.MouseMoveEvent{X: -60, Y: 150, Buttons: core.LeftButton})
		if torn.x != 50+(-60-120) {
			t.Errorf("torn surface did not follow the pointer: x=%d", torn.x)
		}

		// Back over the desktop: re-dock, drag continues in the WM.
		send(core.MouseMoveEvent{X: 200, Y: 150, Buttons: core.LeftButton})
		if !torn.closed {
			t.Error("torn surface not closed on re-dock")
		}
		if !containsWindow(wm, win) {
			t.Fatal("window not re-adopted on re-dock")
		}
		if b := win.Bounds(); b.X != 80 || b.Y != 142 {
			t.Errorf("re-docked at %d,%d; want 80,142", b.X, b.Y)
		}
		send(core.MouseMoveEvent{X: 210, Y: 160, Buttons: core.LeftButton})
		if b := win.Bounds(); b.X != 90 || b.Y != 152 {
			t.Errorf("drag not armed after re-dock: window at %d,%d, want 90,152", b.X, b.Y)
		}
		send(core.MouseReleaseEvent{X: 210, Y: 160, Button: core.LeftButton})

		d.QuitWithCode(0)
	}

	d.RunOn(plat)
}

// A window dropped outside stays torn off; a later drag on its own
// title bar moves the OS window via the global pointer and re-docks
// when the pointer crosses back over the desktop.
func TestTornWindowTitleDragAndRedock(t *testing.T) {
	t.Cleanup(func() { core.SetTextMeasurer(nil) })
	px, err := raster.New(800, 480)
	if err != nil {
		t.Fatal(err)
	}
	d := NewDesktop()
	d.SetBackend(px)

	win := window.NewWindow("tearme")
	d.SetOnStartup(func() {
		d.WindowManager().AddWindow(win)
		win.SetBounds(core.UnitRect{X: 100, Y: 100, Width: 200, Height: 100})
		win.Layout()
	})

	plat := &msPlatform{}
	plat.script = func() {
		desk := plat.surfaces[0]
		wm := d.WindowManager()
		send := func(ev core.Event) { desk.handler.Event(ev) }

		// Tear off and RELEASE outside: the window stays torn.
		send(core.MousePressEvent{X: 220, Y: 108, Button: core.LeftButton})
		send(core.MouseMoveEvent{X: -30, Y: 150, Buttons: core.LeftButton})
		send(core.MouseReleaseEvent{X: -30, Y: 150, Button: core.LeftButton})
		if len(plat.surfaces) != 2 {
			t.Fatalf("expected a torn surface, have %d", len(plat.surfaces))
		}
		torn := plat.surfaces[1]
		if torn.closed || containsWindow(wm, win) {
			t.Fatal("window should remain torn off after release outside")
		}

		// Drag the torn window by its own title bar, far from the
		// desktop: the OS window follows the global pointer.
		plat.gx, plat.gy = 1200, 700
		torn.handler.Event(core.MousePressEvent{X: 120, Y: 8, Button: core.LeftButton})
		torn.handler.Event(core.MouseMoveEvent{X: 125, Y: 9, Buttons: core.LeftButton})
		if torn.x != 1200-120 || torn.y != 700-8 {
			t.Errorf("torn window at %d,%d; want %d,%d", torn.x, torn.y, 1200-120, 700-8)
		}

		// Pointer over the desktop mid-drag: re-dock there.
		plat.gx, plat.gy = 400, 300
		torn.handler.Event(core.MouseMoveEvent{X: 25, Y: 9, Buttons: core.LeftButton})
		if !torn.closed {
			t.Error("torn surface not closed on host-driven re-dock")
		}
		if !containsWindow(wm, win) {
			t.Fatal("window not re-adopted on host-driven re-dock")
		}
		if b := win.Bounds(); b.X != (400-50)-120 || b.Y != (300-60)-8 {
			t.Errorf("re-docked at %d,%d; want %d,%d", b.X, b.Y, (400-50)-120, (300-60)-8)
		}
		send(core.MouseReleaseEvent{X: 330, Y: 232, Button: core.LeftButton})

		d.QuitWithCode(0)
	}

	d.RunOn(plat)
}

// The platform may hand the tail of the tearing gesture (motion and
// the release) to the torn window once it appears under the held
// pointer. The desktop must not treat its stale tear state as a live
// drag: a hover back over the desktop re-docks nothing.
func TestMissedReleaseDoesNotStealTornWindow(t *testing.T) {
	t.Cleanup(func() { core.SetTextMeasurer(nil) })
	px, err := raster.New(800, 480)
	if err != nil {
		t.Fatal(err)
	}
	d := NewDesktop()
	d.SetBackend(px)

	win := window.NewWindow("tearme")
	d.SetOnStartup(func() {
		d.WindowManager().AddWindow(win)
		win.SetBounds(core.UnitRect{X: 100, Y: 100, Width: 200, Height: 100})
		win.Layout()
	})

	plat := &msPlatform{}
	plat.script = func() {
		desk := plat.surfaces[0]
		wm := d.WindowManager()
		send := func(ev core.Event) { desk.handler.Event(ev) }

		// Tear off; the gesture continues at the TORN window.
		send(core.MousePressEvent{X: 220, Y: 108, Button: core.LeftButton})
		send(core.MouseMoveEvent{X: -30, Y: 150, Buttons: core.LeftButton})
		if len(plat.surfaces) != 2 {
			t.Fatalf("expected a torn surface, have %d", len(plat.surfaces))
		}
		torn := plat.surfaces[1]

		// The armed host keeps driving the drag from its own stream.
		plat.gx, plat.gy = 900, 500
		torn.handler.Event(core.MouseMoveEvent{X: 40, Y: 9, Buttons: core.LeftButton})
		if torn.x != 900-120 || torn.y != 500-8 {
			t.Errorf("armed host did not follow pointer: %d,%d want %d,%d",
				torn.x, torn.y, 900-120, 500-8)
		}

		// The RELEASE also lands on the torn window - the desktop
		// never sees it.
		torn.handler.Event(core.MouseReleaseEvent{X: 40, Y: 9, Button: core.LeftButton})

		// Hovering the desktop later (button up) must not re-dock.
		send(core.MouseMoveEvent{X: 400, Y: 300})
		send(core.MouseMoveEvent{X: 410, Y: 310})
		if torn.closed || containsWindow(wm, win) {
			t.Fatal("hover over the desktop stole the torn window back")
		}

		// A drag inside the torn window's CONTENT must not move the
		// OS window either.
		wasX := torn.x
		torn.handler.Event(core.MousePressEvent{X: 50, Y: 50, Button: core.LeftButton})
		torn.handler.Event(core.MouseMoveEvent{X: 60, Y: 60, Buttons: core.LeftButton})
		torn.handler.Event(core.MouseReleaseEvent{X: 60, Y: 60, Button: core.LeftButton})
		if torn.x != wasX {
			t.Errorf("content drag moved the OS window: %d -> %d", wasX, torn.x)
		}

		// And a fresh press on the desktop with stale-free state
		// behaves normally.
		send(core.MousePressEvent{X: 400, Y: 300, Button: core.LeftButton})
		send(core.MouseReleaseEvent{X: 400, Y: 300, Button: core.LeftButton})
		if torn.closed || containsWindow(wm, win) {
			t.Fatal("desktop click stole the torn window back")
		}

		d.QuitWithCode(0)
	}

	d.RunOn(plat)
}
