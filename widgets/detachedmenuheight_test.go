package widgets

import (
	"fmt"
	"testing"

	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/window"
)

// A tall menu opened from a detached window's own menu bar clamps to the
// window surface (maxVisible set, so scroll bumpers appear) instead of
// overflowing off the bottom.
func TestDetachedMenuClampsToWindowHeight(t *testing.T) {
	w := window.NewWindow("w")
	w.SetDetached(true)

	mb := NewMenuBar()
	menu := NewMenu("Big")
	for i := 0; i < 50; i++ {
		menu.AddItem(NewMenuItem(fmt.Sprintf("Item %d", i)))
	}
	mb.AddMenu(menu)

	// Chrome install points the menu bar's parent at the window.
	w.SetWindowMenuBar(mb)
	w.SetBounds(core.UnitRect{Width: 400, Height: 200})
	w.Layout()

	mb.OpenMenu(0)

	if !menu.needsScrolling() {
		t.Errorf("menu did not clamp to the window height (maxVisible=%d, items=%d)",
			menu.maxVisible, len(menu.items))
	}
}
