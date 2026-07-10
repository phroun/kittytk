package trinkets

import (
	"testing"

	"github.com/phroun/kittytk/core"
	"github.com/phroun/kittytk/raster"
)

// Drag-selecting past either edge autoscrolls that way (the horizontal
// analogue of a list view's edge autoscroll). With no desktop timer in the
// test each move past the edge steps once, so repeated moves stand in for
// the timer's ticks.
func TestTextInputDragAutoScrollBothDirections(t *testing.T) {
	t.Cleanup(func() { core.SetTextMeasurer(nil) })
	b, err := raster.New(400, 40)
	if err != nil {
		t.Fatal(err)
	}
	core.SetTextMeasurer(b)

	ti := NewTextInput()
	ti.SetText("The quick brown fox jumps over the lazy dog, twice over.")
	m := ti.EffectiveCellMetrics()
	ti.SetBounds(core.UnitRect{Width: m.CellWidth * 10, Height: m.CellHeight})
	ti.SetFocus()

	// Scroll to the end, then arm a drag selection anchored there.
	ti.SetCursorPosition(len(ti.Text()))
	startOffset := ti.scrollOffset
	if startOffset == 0 {
		t.Fatal("expected a long string to be scrolled right of the origin")
	}
	ti.selStart = ti.cursorPos
	ti.selEnd = ti.cursorPos
	ti.selecting = true

	// Drag past the LEFT edge: the caret walks left and the field scrolls left.
	for i := 0; i < 25; i++ {
		ti.HandleMouseMove(core.MouseMoveEvent{X: -4, Buttons: core.LeftButton})
	}
	if ti.scrollOffset >= startOffset {
		t.Errorf("left autoscroll did not scroll left: offset=%d, started %d", ti.scrollOffset, startOffset)
	}
	if !ti.HasSelection() {
		t.Error("left autoscroll should extend the selection")
	}
	leftOffset := ti.scrollOffset

	// Drag past the RIGHT edge: the caret walks right and the field scrolls right.
	for i := 0; i < 25; i++ {
		ti.HandleMouseMove(core.MouseMoveEvent{X: ti.Bounds().Width + 4, Buttons: core.LeftButton})
	}
	if ti.scrollOffset <= leftOffset {
		t.Errorf("right autoscroll did not scroll right: offset=%d, was %d", ti.scrollOffset, leftOffset)
	}

	// Releasing ends the autoscroll.
	ti.HandleMouseRelease(core.MouseReleaseEvent{Button: core.LeftButton})
	if ti.scrollDir != 0 || ti.scrollTimer != nil {
		t.Error("release should stop the autoscroll")
	}
}
