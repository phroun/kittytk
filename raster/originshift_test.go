package raster

import (
	"testing"

	"github.com/phroun/kittytk/core"
)

// ShiftOriginPx nudges every unit->pixel conversion by whole device
// pixels and is exactly undone by the negated shift - the mechanism the
// menu bar uses to inset its first item by one device pixel, a sub-unit
// offset the integer unit grid can't express.
func TestShiftOriginPxNudgesConversions(t *testing.T) {
	b, err := New(200, 100)
	if err != nil {
		t.Fatal(err)
	}

	x0 := b.UnitToPxX(16)
	y0 := b.UnitToPxY(16)

	b.ShiftOriginPx(1, 2)
	if got := b.UnitToPxX(16); got != x0+1 {
		t.Errorf("after ShiftOriginPx(1,_): UnitToPxX = %d, want %d", got, x0+1)
	}
	if got := b.UnitToPxY(16); got != y0+2 {
		t.Errorf("after ShiftOriginPx(_,2): UnitToPxY = %d, want %d", got, y0+2)
	}

	b.ShiftOriginPx(-1, -2)
	if got := b.UnitToPxX(16); got != x0 {
		t.Errorf("after restore: UnitToPxX = %d, want %d", got, x0)
	}
	if got := b.UnitToPxY(16); got != y0 {
		t.Errorf("after restore: UnitToPxY = %d, want %d", got, y0)
	}
}

// The raster backend advertises the device-origin nudge through the
// painter capability; cell surfaces don't, so callers get a
// graphical-only nudge for free.
func TestPainterShiftDeviceOriginGraphicalOnly(t *testing.T) {
	b, err := New(64, 32)
	if err != nil {
		t.Fatal(err)
	}
	p := core.NewPainter(b)
	if !p.ShiftDeviceOrigin(1, 0) {
		t.Error("raster painter should support a device-origin nudge")
	}
	p.ShiftDeviceOrigin(-1, 0) // restore
}
