package window

import (
	"testing"

	"github.com/phroun/tuitk/core"
)

// Graphical frames narrow the resize grip to the outer edge sliver
// so widgets at the window edge stay clickable; cell frames keep the
// classic full-cell zones.
func TestResizeGripNarrowsEdgeZones(t *testing.T) {
	m, win := newPositioningManager(true)
	m.SetResizeGrip(2) // quarter-column at 2x scale

	// 3 units inside the right edge (x=397 of 80..400): outside the
	// 2-unit grip - NOT a resize; the press reaches the content.
	m.HandleMousePress(core.MousePressEvent{X: 397, Y: 160, Button: core.LeftButton})
	m.HandleMouseMove(core.MouseMoveEvent{X: 410, Y: 160})
	m.HandleMouseRelease(core.MouseReleaseEvent{X: 410, Y: 160, Button: core.LeftButton})
	if b := win.Bounds(); b.Width != 320 {
		t.Errorf("press outside the grip resized the window: width %d", b.Width)
	}

	// 1 unit inside the right edge: within the grip - resizes.
	m.HandleMousePress(core.MousePressEvent{X: 399, Y: 160, Button: core.LeftButton})
	m.HandleMouseMove(core.MouseMoveEvent{X: 412, Y: 160})
	m.HandleMouseRelease(core.MouseReleaseEvent{X: 412, Y: 160, Button: core.LeftButton})
	if b := win.Bounds(); b.Width != 333 {
		t.Errorf("press inside the grip did not resize: width %d, want 333", b.Width)
	}
}

// The bottom band narrows too: on cell frames a whole row grabbed
// the bottom edge; with a grip only the outer sliver does.
func TestResizeGripNarrowsBottomBand(t *testing.T) {
	m, win := newPositioningManager(true)
	m.SetResizeGrip(2)

	// 5 units above the bottom edge (y=235 of 80..240): outside the
	// grip - not a resize.
	m.HandleMousePress(core.MousePressEvent{X: 200, Y: 235, Button: core.LeftButton})
	m.HandleMouseMove(core.MouseMoveEvent{X: 200, Y: 250})
	m.HandleMouseRelease(core.MouseReleaseEvent{X: 200, Y: 250, Button: core.LeftButton})
	if b := win.Bounds(); b.Height != 160 {
		t.Errorf("press outside the bottom grip resized: height %d", b.Height)
	}

	// 1 unit above the bottom edge: resizes.
	m.HandleMousePress(core.MousePressEvent{X: 200, Y: 239, Button: core.LeftButton})
	m.HandleMouseMove(core.MouseMoveEvent{X: 200, Y: 252})
	m.HandleMouseRelease(core.MouseReleaseEvent{X: 200, Y: 252, Button: core.LeftButton})
	if b := win.Bounds(); b.Height != 173 {
		t.Errorf("press inside the bottom grip did not resize: height %d, want 173", b.Height)
	}
}

// Zero grip preserves the classic cell-frame zones untouched.
func TestZeroGripKeepsCellZones(t *testing.T) {
	m, win := newPositioningManager(false)

	// 5 units inside the right edge: within the classic one-cell zone.
	m.HandleMousePress(core.MousePressEvent{X: 395, Y: 160, Button: core.LeftButton})
	m.HandleMouseMove(core.MouseMoveEvent{X: 403, Y: 160})
	m.HandleMouseRelease(core.MouseReleaseEvent{X: 403, Y: 160, Button: core.LeftButton})
	if b := win.Bounds(); b.Width == 320 {
		t.Error("cell-frame edge zone should still be one cell wide")
	}
}
