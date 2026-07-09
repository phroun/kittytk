package core

import "testing"

// MeasureRunes scales its per-character advance with the font size, so a
// "30 characters wide" sizing hint is ~30 cells at any font_size instead
// of a fixed 240 units. The default 12pt keeps the historical 8 units
// (16 for the double-width Tuesday face) so existing layouts are
// unchanged.
func TestMeasureRunesScalesWithSize(t *testing.T) {
	cases := []struct {
		name    string
		size    int
		perRune Unit // expected width of a single rune
	}{
		{"ui-text", 12, 8}, // backward-compatible default
		{"ui-text", 6, 4},  // half size -> half width
		{"ui-text", 18, 12},
		{"ui-text", 24, 16},
		{"Tuesday", 12, 16}, // double-width demo face, unchanged at 12pt
		{"Tuesday", 6, 8},
	}
	for _, tc := range cases {
		f := &Font{Name: tc.name, Size: tc.size}
		if got := f.MeasureRunes(1); got != tc.perRune {
			t.Errorf("%s@%dpt: per-rune = %d, want %d", tc.name, tc.size, got, tc.perRune)
		}
		// Linear in the rune count.
		if got := f.MeasureRunes(30); got != 30*tc.perRune {
			t.Errorf("%s@%dpt: MeasureRunes(30) = %d, want %d", tc.name, tc.size, got, 30*tc.perRune)
		}
	}
}
