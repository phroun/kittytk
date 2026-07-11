package trinkets

import (
	"testing"

	"github.com/phroun/kittytk/core"
)

// The hover/drag hit box must be the button's full bounds - the same region
// the click path routes by - not just the face row. A button sizes itself two
// rows tall (face + drop shadow), so the shadow row must hover and must keep a
// press alive during a drag.
func TestButtonHoverCoversFullBounds(t *testing.T) {
	b := NewButton("OK")
	// Two rows tall, a few cells wide: row 0 is the face, row 1 the shadow.
	b.SetBounds(core.UnitRect{X: 0, Y: 0, Width: 60, Height: 32})

	// Hover on the shadow row (Y in the lower half) must light the button.
	b.HandleMouseMove(core.MouseMoveEvent{X: 20, Y: 24, Buttons: 0})
	if !b.mouseOver {
		t.Error("hover over the shadow row did not set mouseOver")
	}
	// Fully outside clears it.
	b.HandleMouseMove(core.MouseMoveEvent{X: 20, Y: 40, Buttons: 0})
	if b.mouseOver {
		t.Error("hover below the button did not clear mouseOver")
	}

	// Press on the face, then drag down onto the shadow row: still pressed.
	b.HandleMousePress(core.MousePressEvent{X: 20, Y: 4, Button: core.LeftButton})
	if !b.hovered {
		t.Fatal("button not pressed-hovered after the press")
	}
	b.HandleMouseMove(core.MouseMoveEvent{X: 20, Y: 24, Buttons: 1})
	if !b.hovered {
		t.Error("dragging onto the shadow row dropped the pressed look")
	}
	// Drag fully off: now it drops.
	b.HandleMouseMove(core.MouseMoveEvent{X: 20, Y: 40, Buttons: 1})
	if b.hovered {
		t.Error("dragging off the button kept the pressed look")
	}
}
