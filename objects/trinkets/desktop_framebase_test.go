package trinkets

import (
	"testing"

	"github.com/phroun/kittytk/backend/raster"
	"github.com/phroun/kittytk/core"
	"github.com/phroun/kittytk/objects/window"
	"github.com/phroun/kittytk/platform"
)

// paintProbe records whether a window's content painted.
type paintProbe struct {
	core.TrinketBase
	painted bool
}

func newPaintProbe() *paintProbe {
	p := &paintProbe{}
	p.TrinketBase = *core.NewTrinketBase()
	return p
}

func (p *paintProbe) Paint(*core.Painter) { p.painted = true }

// Frame paints the COMPLETE scene — child windows included — for every
// non-compositing present (software renderer, resize frames). FrameBase
// paints only desktop chrome: the GPU compositor renders windows on
// their own layers. Regression coverage for the desync where Frame
// painted chrome-only whenever windows existed, which blanked all open
// windows on the software path and during live resizes.
func TestDesktopFramePaintsWindowsFrameBaseDoesNot(t *testing.T) {
	t.Cleanup(func() { core.SetTextMeasurer(nil) })
	px, _ := raster.New(800, 240)
	d := NewDesktop()
	d.SetBackend(px)
	surf := &msSurface{size: core.UnitSize{Width: 800, Height: 240}}
	d.surface = surf
	h := &desktopSurfaceHandler{d: d}

	probe := newPaintProbe()
	win := window.NewWindow("W")
	win.SetContent(probe)
	d.WindowManager().SetScreenBounds(core.UnitRect{Width: 800, Height: 240})
	d.WindowManager().AddWindow(win)
	win.SetBounds(core.UnitRect{X: 0, Y: 0, Width: 400, Height: 200})
	win.Layout()

	h.Frame(core.NewPainter(px))
	if !probe.painted {
		t.Error("Frame must paint child windows (software/non-compositing path)")
	}

	probe.painted = false
	h.FrameBase(core.NewPainter(px))
	if probe.painted {
		t.Error("FrameBase must NOT paint child windows (the compositor layers them)")
	}

	// The desktop advertises its windows to the compositor.
	var provider platform.WindowProvider = h
	list := provider.GetChildWindows()
	if list == nil || len(list.Windows) != 1 {
		t.Fatalf("GetChildWindows = %+v, want exactly the one open window", list)
	}
}

// A compositor layer paints itself, so a minimized window still listed
// for the compositor keeps drawing — and casting a shadow — over the
// desktop after it was sent to the dock. The software paint loop has
// always applied this filter; the compositor list must apply it too.
func TestDesktopCompositorSkipsMinimizedWindows(t *testing.T) {
	t.Cleanup(func() { core.SetTextMeasurer(nil) })
	px, _ := raster.New(800, 240)
	d := NewDesktop()
	d.SetBackend(px)
	d.surface = &msSurface{size: core.UnitSize{Width: 800, Height: 240}}
	h := &desktopSurfaceHandler{d: d}

	d.WindowManager().SetScreenBounds(core.UnitRect{Width: 800, Height: 240})
	open := window.NewWindow("Open")
	hidden := window.NewWindow("Hidden")
	minimized := window.NewWindow("Minimized")
	for _, w := range []*window.Window{open, hidden, minimized} {
		d.WindowManager().AddWindow(w)
		w.SetBounds(core.UnitRect{Width: 400, Height: 200})
		w.Layout()
	}
	hidden.SetVisible(false)
	minimized.Minimize()

	var provider platform.WindowProvider = h
	list := provider.GetChildWindows()
	if list == nil {
		t.Fatal("GetChildWindows returned nil with a window manager present")
	}
	if len(list.Windows) != 1 {
		t.Fatalf("compositor got %d windows, want only the one that is open and not minimized",
			len(list.Windows))
	}
	if list.Windows[0] != open {
		t.Errorf("compositor got %v, want the open window", list.Windows[0])
	}
}

// The base layer's revision must hold still while only the windows above
// it change. Windows are the desktop's own trinket children, so a
// keystroke inside one bumps the desktop's subtree counter too — and a
// base-layer cache keyed on that raw counter would repaint the whole
// full-surface texture on every keystroke, which is exactly the cost it
// exists to avoid.
func TestDesktopBaseRevisionIgnoresWindowActivity(t *testing.T) {
	t.Cleanup(func() { core.SetTextMeasurer(nil) })
	px, _ := raster.New(800, 240)
	d := NewDesktop()
	d.SetBackend(px)
	d.surface = &msSurface{size: core.UnitSize{Width: 800, Height: 240}}
	h := &desktopSurfaceHandler{d: d}
	var provider platform.WindowProvider = h

	d.WindowManager().SetScreenBounds(core.UnitRect{Width: 800, Height: 240})
	win := window.NewWindow("W")
	content := core.NewTrinketBase()
	content.Init(content)
	win.SetContent(content)
	d.WindowManager().AddWindow(win)
	win.SetBounds(core.UnitRect{Width: 400, Height: 200})
	win.Layout()

	baseRev := func() uint64 {
		list := provider.GetChildWindows()
		if list == nil || !list.HasBaseRevision {
			t.Fatal("desktop did not report a base revision")
		}
		return list.BaseRevision
	}

	before := baseRev()

	// Anything inside the window: its own layer, not the base's.
	content.Update()
	content.SetVisible(false)
	win.SetActive(true)
	if got := baseRev(); got != before {
		t.Errorf("base revision moved from %d to %d for window-only activity; "+
			"the base layer would repaint on every keystroke in any window", before, got)
	}

	// Chrome IS the base layer, so it must move the revision.
	d.SetWallpaperChunk(4)
	if got := baseRev(); got == before {
		t.Error("base revision did not move when the wallpaper changed")
	}
}
