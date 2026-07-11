package trinkets

import (
	"testing"

	"github.com/phroun/kittytk/core"
)

// On a cell surface the hover/drag hit box is the button's full bounds - the
// same region the click path routes by - not just the face row. A button sizes
// itself two rows tall (face + drop shadow), so the shadow row must hover and
// must keep a press alive during a drag.
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

// On a graphical surface the hit box excludes the bottom half-row of the
// bounds (dead space below the drop shadow), uniformly for hover, drag, and
// click.
func TestButtonGraphicalHitBoxExcludesBottomHalfRow(t *testing.T) {
	stub := &graphicalFrameStub{Panel: NewPanel(), border: 0}
	b := NewButton("OK")
	b.SetParent(stub)
	b.SetBounds(core.UnitRect{X: 0, Y: 0, Width: 60, Height: 32})

	half := b.EffectiveCellMetrics().CellHeight / 2
	bottom := core.Unit(32) - half // last row minus the excluded half-row
	inside := bottom - 1           // just inside the hit box
	dead := bottom + 1             // in the excluded bottom half-row

	// Hover just inside the lower edge lights it; hover in the dead zone doesn't.
	b.HandleMouseMove(core.MouseMoveEvent{X: 20, Y: inside, Buttons: 0})
	if !b.mouseOver {
		t.Errorf("hover at Y=%d (inside hit box) did not light the button", inside)
	}
	b.HandleMouseMove(core.MouseMoveEvent{X: 20, Y: dead, Buttons: 0})
	if b.mouseOver {
		t.Errorf("hover at Y=%d (dead bottom half-row) lit the button", dead)
	}

	// A press in the dead zone isn't on the button (returns false, not pressed).
	if b.HandleMousePress(core.MousePressEvent{X: 20, Y: dead, Button: core.LeftButton}) {
		t.Error("press in the dead bottom half-row was consumed by the button")
	}
	if b.pressed {
		t.Error("press in the dead bottom half-row set pressed")
	}
	// A press just inside does press.
	if !b.HandleMousePress(core.MousePressEvent{X: 20, Y: inside, Button: core.LeftButton}) {
		t.Error("press inside the hit box was not consumed")
	}
	if !b.pressed {
		t.Error("press inside the hit box did not set pressed")
	}
	// While pressed, dragging into the dead zone drops the pressed look.
	b.HandleMouseMove(core.MouseMoveEvent{X: 20, Y: dead, Buttons: 1})
	if b.hovered {
		t.Error("dragging into the dead bottom half-row kept the pressed look")
	}
}
