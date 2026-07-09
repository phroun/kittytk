package main

import (
	"testing"

	"github.com/phroun/kittytk/core"
	"github.com/phroun/kittytk/layout"
	"github.com/phroun/kittytk/raster"
	"github.com/phroun/kittytk/trinkets"
	"github.com/phroun/kittytk/window"
)

// A fixedWidthBox keeps a fixed cell-count width at every font size (so
// its wrapped content stays a narrow column) and reports a height that
// covers that content wrapped to the pinned width (so the checkbox/radio
// column no longer overflows its border top and bottom).
func TestFixedBoxSizeTracksFontAndWraps(t *testing.T) {
	t.Cleanup(func() { core.SetTextMeasurer(nil) })
	for _, size := range []int{12, 6} {
		m := raster.CellMetricsForFontSize(size)
		b, err := raster.New(1024, 400)
		if err != nil {
			t.Fatal(err)
		}
		b.SetCellMetrics(m)
		d := trinkets.NewDesktop()
		d.SetBackend(b)
		d.SetFont(&core.Font{Name: "ui-text", Size: size})
		d.SetBounds(core.UnitRect{Width: 1024, Height: 400})
		d.WindowManager().SetScreenBounds(core.UnitRect{Width: 1024, Height: 400})

		row := trinkets.NewPanel()
		row.SetLayoutManager(layout.NewBoxLayout(core.Horizontal))
		var boxes []*fixedWidthBox

		lbl := trinkets.NewLabel("The quick brown fox jumps over the lazy dog and then keeps trotting along the whole fence")
		lbl.SetWordWrap(true)
		boxes = append(boxes, newFixedWidthBox(32, lbl))

		// The 3rd box: a vbox of a wrapped checkbox + radio (the one the
		// user reports overflowing top/bottom).
		inner := trinkets.NewPanel()
		inner.SetLayoutManager(layout.NewBoxLayout(core.Vertical))
		cb := trinkets.NewCheckbox("Enable the experimental feature that reticulates splines while the moon is full")
		cb.SetWordWrap(true)
		rb := trinkets.NewRadioButton("Prefer the long-form explanation whenever the assistant answers a question")
		rb.SetWordWrap(true)
		inner.AddChild(cb)
		inner.AddChild(rb)
		boxes = append(boxes, newFixedWidthBox(36, inner))

		for _, fb := range boxes {
			row.AddChild(fb)
		}

		win := window.NewWindow("W")
		win.SetContent(row)
		d.WindowManager().AddWindow(win)
		win.SetBounds(core.UnitRect{X: 0, Y: 0, Width: 1024, Height: 400})
		win.Layout()

		for i, fb := range boxes {
			hint := fb.SizeHint()
			// Width stays a fixed cell count at every font size.
			wantCells := fb.widthCells
			if got := hint.Width / m.CellWidth; got != wantCells {
				t.Errorf("size=%d box[%d]: width %d units = %d cells, want %d", size, i, hint.Width, got, wantCells)
			}
			// Height must cover the content wrapped to the pinned width, so
			// the box grows instead of the content overflowing its border.
			if hfw, ok := any(fb).(core.HeightForWidther); ok && hfw.HasHeightForWidth() {
				if need := hfw.HeightForWidth(hint.Width); hint.Height < need {
					t.Errorf("size=%d box[%d]: SizeHint.Height %d < wrapped height %d (content overflows)",
						size, i, hint.Height, need)
				}
			}
		}
	}
}
