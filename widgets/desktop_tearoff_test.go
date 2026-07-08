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
	size        core.UnitSize
	handler     platform.SurfaceHandler
	x, y        int
	closed      bool
	invalidated bool
	opacity     float64
	minimized   bool
	raised      bool
	opts        platform.SurfaceOptions
}

func (s *msSurface) Size() core.UnitSize                  { return s.size }
func (s *msSurface) Metrics() core.CellMetrics            { return core.DefaultCellMetrics() }
func (s *msSurface) SetHandler(h platform.SurfaceHandler) { s.handler = h }
func (s *msSurface) Invalidate(core.UnitRect)             { s.invalidated = true }
func (s *msSurface) SetCursorVisible(bool)                {}
func (s *msSurface) SetCursorPosition(x, y core.Unit)     {}
func (s *msSurface) ScreenPositionPx() (int, int)         { return s.x, s.y }
func (s *msSurface) SetScreenPositionPx(x, y int)         { s.x, s.y = x, y }
func (s *msSurface) Close()                               { s.closed = true }
func (s *msSurface) SetOpacity(o float64)                 { s.opacity = o }
func (s *msSurface) Raise()                               { s.raised = true }
func (s *msSurface) Minimized() bool                      { return s.minimized }
func (s *msSurface) Minimize()                            { s.minimized = true }
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
	afters   []func() // PostAfter callbacks, fired by the script
	gx, gy   int
}

func (p *msPlatform) Run(init func(platform.Platform)) int {
	init(p)
	if p.script != nil {
		p.script()
	}
	return 0
}
func (p *msPlatform) Post(fn func())                       { fn() }
func (p *msPlatform) PostAfter(_ time.Duration, fn func()) { p.afters = append(p.afters, fn) }
func (p *msPlatform) Quit(int)                             {}
func (p *msPlatform) Clipboard() string                    { return "" }
func (p *msPlatform) SetClipboard(string)                  {}
func (p *msPlatform) Beep()                                {}
func (p *msPlatform) SupportsMultipleSurfaces() bool       { return true }
func (p *msPlatform) GlobalPointerPx() (int, int)          { return p.gx, p.gy }
func (p *msPlatform) CreateSurface(o platform.SurfaceOptions) (platform.Surface, error) {
	s := &msSurface{opts: o, x: o.XPx, y: o.YPx, opacity: 1}
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

// The tear handle sits in a button-width slot after [x][.][^]; at
// DefaultCellMetrics (8px cells) that is local x in [80,104). Grab
// its center.
const tearHandleLocalX = core.Unit(88)

// A non-tearable window dragged by the title past the surface edge
// does NOT tear off - tear-off is opt-in.
func TestNonTearableWindowStaysDocked(t *testing.T) {
	t.Cleanup(func() { core.SetTextMeasurer(nil) })
	px, _ := raster.New(800, 480)
	d := NewDesktop()
	d.SetBackend(px)
	win := window.NewWindow("plain")
	d.SetOnStartup(func() {
		d.WindowManager().AddWindow(win)
		win.SetBounds(core.UnitRect{X: 100, Y: 100, Width: 200, Height: 100})
		win.Layout()
	})
	plat := &msPlatform{}
	plat.script = func() {
		desk := plat.surfaces[0]
		send := func(ev core.Event) { desk.handler.Event(ev) }
		send(core.MousePressEvent{X: 220, Y: 108, Button: core.LeftButton})
		send(core.MouseMoveEvent{X: -30, Y: 150, Buttons: core.LeftButton})
		if len(plat.surfaces) != 1 {
			t.Errorf("non-tearable window tore off: %d surfaces", len(plat.surfaces))
		}
		if !containsWindow(d.WindowManager(), win) {
			t.Error("non-tearable window left the desktop")
		}
		d.QuitWithCode(0)
	}
	d.RunOn(plat)
}

// A plain title drag on a tearable window moves it in-surface but does
// NOT tear it off - only a drag by the handle tears.
func TestTearableTitleDragDoesNotTear(t *testing.T) {
	t.Cleanup(func() { core.SetTextMeasurer(nil) })
	px, _ := raster.New(800, 480)
	d := NewDesktop()
	d.SetBackend(px)
	win := window.NewWindow("tearme")
	win.SetTearable(true)
	d.SetOnStartup(func() {
		d.WindowManager().AddWindow(win)
		win.SetBounds(core.UnitRect{X: 100, Y: 100, Width: 200, Height: 100})
		win.Layout()
	})
	plat := &msPlatform{}
	plat.script = func() {
		desk := plat.surfaces[0]
		send := func(ev core.Event) { desk.handler.Event(ev) }
		// Grab the title well right of the handle slot.
		send(core.MousePressEvent{X: 250, Y: 108, Button: core.LeftButton})
		send(core.MouseMoveEvent{X: -30, Y: 150, Buttons: core.LeftButton})
		if len(plat.surfaces) != 1 {
			t.Errorf("title drag tore the window off: %d surfaces", len(plat.surfaces))
		}
		d.QuitWithCode(0)
	}
	d.RunOn(plat)
}

// Dragging a tearable window BY its handle past the surface edge tears
// it off; the live gesture keeps driving the torn surface, and
// crossing back over the desktop re-docks it.
func TestTearByHandleDragAndLiveRedock(t *testing.T) {
	t.Cleanup(func() { core.SetTextMeasurer(nil) })
	px, _ := raster.New(800, 480)
	d := NewDesktop()
	d.SetBackend(px)
	win := window.NewWindow("tearme")
	win.SetTearable(true)
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

		// Press the handle (local 88 -> screen 188), then cross the
		// left edge to tear off.
		send(core.MousePressEvent{X: 188, Y: 108, Button: core.LeftButton})
		send(core.MouseMoveEvent{X: -30, Y: 150, Buttons: core.LeftButton})
		if len(plat.surfaces) != 2 {
			t.Fatalf("handle drag did not tear off: %d surfaces", len(plat.surfaces))
		}
		torn := plat.surfaces[1]
		if !torn.opts.Borderless {
			t.Error("torn surface not borderless")
		}
		if containsWindow(wm, win) {
			t.Error("window still docked after tear-off")
		}
		if !win.IsDetached() {
			t.Error("window not marked detached")
		}

		// Back over the desktop while held: re-dock (live tornDrag).
		plat.gx, plat.gy = 50+200, 60+150
		send(core.MouseMoveEvent{X: 200, Y: 150, Buttons: core.LeftButton})
		if !containsWindow(wm, win) {
			t.Fatal("live drag back did not re-dock")
		}
		if win.IsDetached() {
			t.Error("re-docked window still marked detached")
		}
		send(core.MouseReleaseEvent{X: 200, Y: 150, Button: core.LeftButton})
		d.QuitWithCode(0)
	}
	d.RunOn(plat)
}

// Clicking a tearable window's handle (no drag) detaches it in place;
// clicking the detached '#' handle re-docks it where it sits.
func TestHandleClickDetachAndRedock(t *testing.T) {
	t.Cleanup(func() { core.SetTextMeasurer(nil) })
	px, _ := raster.New(800, 480)
	d := NewDesktop()
	d.SetBackend(px)
	win := window.NewWindow("tearme")
	win.SetTearable(true)
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

		// Press and release the handle in place: detach.
		send(core.MousePressEvent{X: 188, Y: 108, Button: core.LeftButton})
		send(core.MouseReleaseEvent{X: 188, Y: 108, Button: core.LeftButton})
		if len(plat.surfaces) != 2 {
			t.Fatalf("handle click did not detach: %d surfaces", len(plat.surfaces))
		}
		torn := plat.surfaces[1]
		if !win.IsDetached() || containsWindow(wm, win) {
			t.Fatal("window not detached after handle click")
		}

		// Click the '#' handle in the torn window: re-dock in place.
		torn.handler.Event(core.MousePressEvent{X: 88, Y: 8, Button: core.LeftButton})
		torn.handler.Event(core.MouseReleaseEvent{X: 88, Y: 8, Button: core.LeftButton})
		if !containsWindow(wm, win) {
			t.Fatal("'#' click did not re-dock")
		}
		if win.IsDetached() {
			t.Error("re-docked window still detached")
		}
		d.QuitWithCode(0)
	}
	d.RunOn(plat)
}

// The platform may hand the tail of the tearing gesture (motion and
// release) to the torn window once it appears under the held pointer.
// The desktop must not treat its stale tear state as a live drag.
func TestMissedReleaseDoesNotStealTornWindow(t *testing.T) {
	t.Cleanup(func() { core.SetTextMeasurer(nil) })
	px, _ := raster.New(800, 480)
	d := NewDesktop()
	d.SetBackend(px)
	win := window.NewWindow("tearme")
	win.SetTearable(true)
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

		// Tear off by the handle; the gesture continues at the torn window.
		send(core.MousePressEvent{X: 188, Y: 108, Button: core.LeftButton})
		send(core.MouseMoveEvent{X: -30, Y: 150, Buttons: core.LeftButton})
		if len(plat.surfaces) != 2 {
			t.Fatalf("handle drag did not tear off: %d surfaces", len(plat.surfaces))
		}
		torn := plat.surfaces[1]

		// The release lands on the torn window - the desktop never sees it.
		torn.handler.Event(core.MouseReleaseEvent{X: 40, Y: 9, Button: core.LeftButton})

		// Hovering the desktop later (button up) must not re-dock.
		send(core.MouseMoveEvent{X: 400, Y: 300})
		send(core.MouseMoveEvent{X: 410, Y: 310})
		if torn.closed || containsWindow(wm, win) {
			t.Fatal("hover over the desktop stole the torn window back")
		}

		// The repaint tick still drives the torn surface.
		torn.invalidated = false
		if len(plat.afters) > 0 {
			plat.afters[len(plat.afters)-1]()
		}
		if !torn.invalidated {
			t.Error("repaint tick did not invalidate the torn surface")
		}
		d.QuitWithCode(0)
	}
	d.RunOn(plat)
}

// Closing a torn window with its [x] button disposes of its OS
// window immediately - it must not linger until a re-dock.
func TestClosingTornWindowDisposesSurface(t *testing.T) {
	t.Cleanup(func() { core.SetTextMeasurer(nil) })
	px, err := raster.New(800, 480)
	if err != nil {
		t.Fatal(err)
	}
	d := NewDesktop()
	d.SetBackend(px)

	win := window.NewWindow("tearme")
	win.SetTearable(true)
	d.SetOnStartup(func() {
		d.WindowManager().AddWindow(win)
		win.SetBounds(core.UnitRect{X: 100, Y: 100, Width: 200, Height: 100})
		win.Layout()
	})

	plat := &msPlatform{}
	plat.script = func() {
		desk := plat.surfaces[0]
		send := func(ev core.Event) { desk.handler.Event(ev) }

		// Tear off by the handle and release outside.
		send(core.MousePressEvent{X: 188, Y: 108, Button: core.LeftButton})
		send(core.MouseMoveEvent{X: -30, Y: 150, Buttons: core.LeftButton})
		send(core.MouseReleaseEvent{X: -30, Y: 150, Button: core.LeftButton})
		torn := plat.surfaces[1]

		// The window closes itself (the [x] button path).
		win.Close()
		if !torn.closed {
			t.Error("torn surface still open after the window closed")
		}

		// The repaint tick must not resurrect the dead host.
		if len(plat.afters) > 0 {
			plat.afters[len(plat.afters)-1]()
		}

		d.QuitWithCode(0)
	}

	d.RunOn(plat)
}

// When the desktop's own OS window loses focus, its active window's
// chrome dims; re-focusing lights the same window back up.
func TestDesktopBlurDimsActiveWindow(t *testing.T) {
	t.Cleanup(func() { core.SetTextMeasurer(nil) })
	px, err := raster.New(800, 480)
	if err != nil {
		t.Fatal(err)
	}
	d := NewDesktop()
	d.SetBackend(px)

	win := window.NewWindow("focused")
	d.SetOnStartup(func() {
		d.WindowManager().AddWindow(win)
		win.SetBounds(core.UnitRect{X: 100, Y: 100, Width: 200, Height: 100})
		win.Layout()
	})

	plat := &msPlatform{}
	plat.script = func() {
		desk := plat.surfaces[0]
		if !win.IsActive() {
			t.Fatal("window not active after AddWindow")
		}
		desk.handler.Event(core.FocusEvent{Focused: false})
		if win.IsActive() {
			t.Error("active window still lit while the desktop is blurred")
		}
		desk.handler.Event(core.FocusEvent{Focused: true})
		if !win.IsActive() {
			t.Error("active window not re-lit when the desktop re-focused")
		}
		d.QuitWithCode(0)
	}

	d.RunOn(plat)
}

// Tearing off an app's main window carries its non-tearable children
// onto their own surfaces centered over it, raises the main window above
// those children, and gives it focus so the tear ends with it on top.
func TestMainWindowTearOffCascadeCenterRaise(t *testing.T) {
	t.Cleanup(func() { core.SetTextMeasurer(nil) })
	px, _ := raster.New(800, 480)
	d := NewDesktop()
	d.SetBackend(px)

	main := window.NewWindow("main")
	main.SetTearable(true)
	child := window.NewWindow("child") // non-tearable dialog-style child

	app := &mockApp{name: "App", main: main, windows: []*window.Window{main, child}}
	d.AddApplication(app)

	d.SetOnStartup(func() {
		wm := d.WindowManager()
		wm.AddWindow(main)
		main.SetBounds(core.UnitRect{X: 100, Y: 100, Width: 300, Height: 200})
		main.Layout()
		wm.AddWindow(child)
		child.SetBounds(core.UnitRect{X: 0, Y: 0, Width: 100, Height: 80})
		child.Layout()
	})

	plat := &msPlatform{}
	plat.script = func() {
		d.tearOffInPlace(main)

		if len(plat.surfaces) != 3 {
			t.Fatalf("want 3 surfaces (desktop + main + child), got %d", len(plat.surfaces))
		}
		mainSurf := plat.surfaces[1]
		childSurf := plat.surfaces[2]

		if !mainSurf.raised {
			t.Error("main window surface was not raised above its children")
		}
		if !main.IsDetached() {
			t.Error("main window not marked detached")
		}
		if child.IsTearable() || !child.IsDetached() {
			t.Error("non-tearable child should have followed the main window off")
		}

		// Desktop origin is (50,60) px, scale 1. Main torn at unit (100,100)
		// so its surface sits at (150,160). The child (100x80) centers over
		// the 300x200 main: unit (100+100, 100+60) = (200,160) -> px (250,220).
		if childSurf.x != 250 || childSurf.y != 220 {
			t.Errorf("child not centered over main: surface at (%d,%d), want (250,220)", childSurf.x, childSurf.y)
		}

		if d.tornFocusOwner != main {
			t.Error("torn main window did not take focus")
		}
		d.QuitWithCode(0)
	}

	d.RunOn(plat)
}
