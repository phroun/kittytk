package widgets

import (
	"testing"

	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/raster"
	"github.com/phroun/tuitk/window"
)

// The desktop derives the graphical resize grip: a quarter of a
// layout column, floored at 4 device pixels; zero on cell frames.
func TestDesktopResizeGripDerivation(t *testing.T) {
	t.Cleanup(func() { core.SetTextMeasurer(nil) })

	// Scale 2: quarter-column = 2 units = 4 px - exactly the floor.
	px2, err := raster.NewScaled(640, 480, 2)
	if err != nil {
		t.Fatal(err)
	}
	d := NewDesktop()
	d.SetBackend(px2)
	if got := d.GraphicalResizeGrip(); got != 2 {
		t.Errorf("scale 2 grip = %d units, want 2", got)
	}

	// Scale 1: quarter-column = 2 units = 2 px < 4 px floor -> 4 units.
	px1, err := raster.New(640, 480)
	if err != nil {
		t.Fatal(err)
	}
	d1 := NewDesktop()
	d1.SetBackend(px1)
	if got := d1.GraphicalResizeGrip(); got != 4 {
		t.Errorf("scale 1 grip = %d units, want 4 (4px floor)", got)
	}

	// Cell backend: zero (the whole border cell is the grip there).
	dc := NewDesktop()
	dc.SetBackend(&nullBackend{})
	if got := dc.GraphicalResizeGrip(); got != 0 {
		t.Errorf("cell frame grip = %d, want 0", got)
	}
}

// MDI panes inherit the grip through their ancestry.
func TestMDIPaneInheritsResizeGrip(t *testing.T) {
	t.Cleanup(func() { core.SetTextMeasurer(nil) })
	px, err := raster.NewScaled(640, 480, 2)
	if err != nil {
		t.Fatal(err)
	}
	d := NewDesktop()
	d.SetBackend(px)

	pane := NewMDIPane()
	win := window.NewWindow("host")
	win.SetContent(pane)
	d.WindowManager().AddWindow(win)
	if got := core.FindResizeGrip(pane.Self()); got != 2 {
		t.Errorf("MDI pane grip = %d, want 2 (inherited from desktop)", got)
	}
}
