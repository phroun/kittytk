package raster

import (
	"testing"

	"github.com/phroun/kittytk/core"
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
