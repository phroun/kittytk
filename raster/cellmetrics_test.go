package raster

import (
	"testing"

	"github.com/phroun/kittytk/core"
	"github.com/phroun/kittytk/style"
)

// CellMetricsForFontSize reproduces the historical 8x16 grid at the
// default 12pt, so an unset/12pt font_size leaves layout unchanged.
func TestCellMetricsForFontSizeDefaultIs8x16(t *testing.T) {
	m := CellMetricsForFontSize(12)
	if m.CellWidth != 8 || m.CellHeight != 16 {
		t.Fatalf("12pt cell = %dx%d, want 8x16", m.CellWidth, m.CellHeight)
	}
	// A non-positive size is treated as the 12pt default.
	if z := CellMetricsForFontSize(0); z != m {
		t.Fatalf("0pt cell = %+v, want the 12pt default %+v", z, m)
	}
}

// The cell grows with the point size, keeping the height as the font's
// real line box (Size*4/3) and the width scaled from the 8@12 baseline.
func TestCellMetricsForFontSizeScales(t *testing.T) {
	prev := CellMetricsForFontSize(8)
	for _, size := range []int{10, 12, 14, 16, 20, 24} {
		m := CellMetricsForFontSize(size)
		if m.CellWidth < prev.CellWidth || m.CellHeight < prev.CellHeight {
			t.Fatalf("size %d cell %+v shrank vs previous %+v", size, m, prev)
		}
		prev = m
	}
	// 24pt is exactly double 12pt on both axes (line box and advance).
	if m := CellMetricsForFontSize(24); m.CellWidth != 16 || m.CellHeight != 32 {
		t.Fatalf("24pt cell = %dx%d, want 16x32", m.CellWidth, m.CellHeight)
	}
}

// SetCellMetrics re-seeds Metrics() and clamps degenerate values to 1.
func TestSetCellMetrics(t *testing.T) {
	b, err := New(100, 100)
	if err != nil {
		t.Fatal(err)
	}
	if m := b.Metrics(); m.CellWidth != 8 || m.CellHeight != 16 {
		t.Fatalf("default Metrics = %+v, want 8x16", m)
	}
	b.SetCellMetrics(CellMetricsForFontSize(24))
	if m := b.Metrics(); m.CellWidth != 16 || m.CellHeight != 32 {
		t.Fatalf("after set Metrics = %+v, want 16x32", m)
	}
	b.SetCellMetrics(core.CellMetrics{CellWidth: 0, CellHeight: 0})
	if m := b.Metrics(); m.CellWidth != 1 || m.CellHeight != 1 {
		t.Fatalf("degenerate metrics = %+v, want clamped to 1x1", m)
	}
}

// The cell primitive (DrawCell) fills exactly one cell of the backend's
// denomination, so chrome glyphs (checkbox [x], radio, tree arrows)
// grow with font_size instead of staying at the historical 8x16.
func TestDrawCellFillsOneCell(t *testing.T) {
	// A distinctive opaque background so the filled cell is measurable.
	bg := style.DefaultStyle().WithBg(style.RGB(10, 20, 200))
	measure := func(m core.CellMetrics) (w, h int) {
		b, err := New(200, 200)
		if err != nil {
			t.Fatal(err)
		}
		b.SetCellMetrics(m)
		b.Clear(style.DefaultStyle())
		b.DrawCell(0, 0, ' ', bg) // space: background fill only, no glyph
		img := b.Image()
		for x := 0; x < 200; x++ {
			if c := img.RGBAAt(x, 0); c.B > 150 && c.R < 100 {
				w = x + 1
			}
		}
		for y := 0; y < 200; y++ {
			if c := img.RGBAAt(0, y); c.B > 150 && c.R < 100 {
				h = y + 1
			}
		}
		return w, h
	}

	if w, h := measure(CellMetricsForFontSize(12)); w != 8 || h != 16 {
		t.Errorf("12pt cell fill = %dx%d px, want 8x16", w, h)
	}
	if w, h := measure(CellMetricsForFontSize(24)); w != 16 || h != 32 {
		t.Errorf("24pt cell fill = %dx%d px, want 16x32 (double)", w, h)
	}
}
