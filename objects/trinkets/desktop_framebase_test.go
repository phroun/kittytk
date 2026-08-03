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
