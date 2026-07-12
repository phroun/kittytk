package trinkets

import (
	"testing"

	"github.com/phroun/kittytk/core"
)

// Growing the list while scrolled down pulls the content back into the
// freed space (the scrollbar must not vanish leaving a stale blank) -
// the TreeView's resize re-clamp, for ListView.
func TestListResizeReclampsScroll(t *testing.T) {
	lv := NewListView()
	for i := 0; i < 30; i++ {
		lv.AddItem(NewListItem(fmtItem(i)))
	}
	lv.SetBounds(core.UnitRect{Width: 200, Height: 160})
	lv.scrollOffset = 30 - lv.visibleCount() // scrolled to the bottom
	if lv.scrollOffset <= 0 {
		t.Fatal("precondition: view smaller than the list")
	}
	lv.SetBounds(core.UnitRect{Width: 200, Height: 400})
	if want := 30 - lv.visibleCount(); lv.scrollOffset != want {
		t.Errorf("scrollOffset after grow = %d, want %d", lv.scrollOffset, want)
	}
	// Tall enough for everything: the offset snaps to 0.
	lv.SetBounds(core.UnitRect{Width: 200, Height: 640})
	if lv.scrollOffset != 0 {
		t.Errorf("scrollOffset with everything visible = %d, want 0", lv.scrollOffset)
	}
}
