package main

import (
	"testing"

	"github.com/phroun/kittytk/core"
)

// denominate scales absolute-unit dimensions by the font ratio and only
// those - cell counts (cols), ratios (position), and data (value) are
// left alone; the default 8x16 is a byte-identical no-op.
func TestDenominate(t *testing.T) {
	in := `w=new window width=480 height=288 children={
new panel layout=vbox spacing=16
new fixedbox cols=32
new splitter position=0.4
new progress value=25
new mdipane min_width=640 max_height=400
new dockrow entry_width=20}`

	// No-op at the authored 8x16.
	if got := denominate(in, core.CellMetrics{CellWidth: 8, CellHeight: 16}); got != in {
		t.Errorf("8x16 should be a no-op:\n%s", got)
	}

	// At 6pt (4x8) the font ratio is 1/2: unit dimensions halve, cols /
	// position / value untouched.
	got := denominate(in, core.CellMetrics{CellWidth: 4, CellHeight: 8})
	for _, want := range []string{
		"width=240", "height=144", "spacing=8",
		"min_width=320", "max_height=200",
		"entry_width=20", "cols=32", "position=0.4", "value=25", // NOT scaled
	} {
		if !contains(got, want) {
			t.Errorf("6pt denominate missing %q in:\n%s", want, got)
		}
	}
	// value=25 must not have become 12 (it isn't a dimension).
	if contains(got, "value=12") {
		t.Errorf("value= was wrongly scaled:\n%s", got)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
