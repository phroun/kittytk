package trinkets

import (
	"testing"

	"github.com/phroun/kittytk/core"
	"github.com/phroun/kittytk/raster"
	"github.com/phroun/kittytk/window"
)

// The client-area contract per frame mode: cell frames reserve a full
// cell on every side; graphical frames reserve only the titlebar row,
// with content extending to the left, right, and bottom edges.
func TestClientAreaContractPerFrameMode(t *testing.T) {
	t.Cleanup(func() { core.SetTextMeasurer(nil) })

	newWin := func(d *Desktop) *window.Window {
		win := window.NewWindow("client")
		win.SetBounds(core.UnitRect{X: 32, Y: 32, Width: 240, Height: 160})
		d.WindowManager().AddWindow(win)
		return win
	}

	// Cell backend: one-cell border on every side.
	cellDesk := NewDesktop()
	cellDesk.SetBackend(&nullBackend{})
	cellWin := newWin(cellDesk)
	if off := cellWin.ClientAreaOffset(); off.X != 8 || off.Y != 16 {
		t.Errorf("cell frame client offset = %+v, want (8,16)", off)
	}

	// Pixel backend: edge-to-edge below the titlebar.
	pixel, err := raster.New(640, 480)
	if err != nil {
		t.Fatal(err)
	}
	pixDesk := NewDesktop()
	pixDesk.SetBackend(pixel)
	pixWin := newWin(pixDesk)
	if off := pixWin.ClientAreaOffset(); off.X != 0 || off.Y != 16 {
		t.Errorf("graphical frame client offset = %+v, want (0,16)", off)
	}

	// Content width tracks the full window width in graphical mode.
	content := NewPanel()
	pixWin.SetContent(content)
	pixWin.Layout()
	if got := content.Bounds().Width; got != 240 {
		t.Errorf("graphical content width = %d, want full window width 240", got)
	}
	if got := content.Bounds().Height; got != 160-16 {
		t.Errorf("graphical content height = %d, want %d (only titlebar reserved)", got, 160-16)
	}
}

// A window squeezed below its chrome still exposes a 1-unit client
// sliver: content paints clipped instead of spilling.
func TestClientAreaNeverEmpty(t *testing.T) {
	t.Cleanup(func() { core.SetTextMeasurer(nil) })

	pixel, err := raster.New(640, 480)
	if err != nil {
		t.Fatal(err)
	}
	d := NewDesktop()
	d.SetBackend(pixel)

	win := window.NewWindow("tiny")
	content := NewPanel()
	win.SetContent(content)
	d.WindowManager().AddWindow(win)
	win.SetBounds(core.UnitRect{X: 0, Y: 0, Width: 100, Height: 16}) // titlebar only
	win.Layout()

	if h := content.Bounds().Height; h < 1 {
		t.Errorf("client height = %d; must be clamped to >= 1", h)
	}
	if w := content.Bounds().Width; w < 1 {
		t.Errorf("client width = %d; must be clamped to >= 1", w)
	}
}
