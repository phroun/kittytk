package window

import (
	"testing"

	"github.com/phroun/kittytk/core"
)

// Hovering the titlebar close button records it so the frame can draw the
// hovered style; moving into the content clears it.
func TestTitleBarButtonHover(t *testing.T) {
	w := NewWindow("child")
	w.SetBounds(core.UnitRect{X: 0, Y: 0, Width: 320, Height: 240})
	w.SetActive(true)

	metrics := w.frameCellMetrics()
	inset := core.FindFrameBorderUnits(w)
	// Center of the close button ([x]): it starts one cell in from the
	// border on a normal frame.
	x := inset + metrics.CellWidth + metrics.CellWidth/2
	y := inset + metrics.CellHeight/2
	w.HandleMouseMove(core.MouseMoveEvent{X: x, Y: y})

	if w.hoveredButton != TitleButtonClose {
		t.Errorf("hovered button = %v, want close", w.hoveredButton)
	}

	// Move down into the content: no titlebar button hovered.
	w.HandleMouseMove(core.MouseMoveEvent{X: x, Y: 200})
	if w.hoveredButton != TitleButtonNone {
		t.Errorf("hovered button = %v after leaving titlebar, want none", w.hoveredButton)
	}
}
