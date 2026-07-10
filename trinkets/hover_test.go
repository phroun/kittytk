package trinkets

import (
	"testing"

	"github.com/phroun/kittytk/core"
)

// A plain pointer move over a button sets its hover state; moving off
// clears it. The button must not consume the move (return false) so
// sibling widgets can still clear their own hover.
func TestButtonPointerHover(t *testing.T) {
	b := NewButton("ok")
	b.SetBounds(core.UnitRect{Width: 200, Height: 40})

	if b.HandleMouseMove(core.MouseMoveEvent{X: 50, Y: 0}) {
		t.Error("hover move should not be consumed")
	}
	if !b.mouseOver {
		t.Error("pointer inside the button did not set hover")
	}

	b.HandleMouseMove(core.MouseMoveEvent{X: -5, Y: 0})
	if b.mouseOver {
		t.Error("pointer outside the button did not clear hover")
	}
}

// The dock highlights the entry under the pointer and clears it when the
// pointer moves to background.
func TestDockItemHover(t *testing.T) {
	d := NewDockRow()
	d.SetBounds(core.UnitRect{Width: 400, Height: 40})
	d.AddEntry(&DockEntry{Title: "one"})
	d.AddEntry(&DockEntry{Title: "two"})

	metrics := d.EffectiveCellMetrics()
	slot := core.Unit(d.entryWidth) * metrics.CellWidth

	// Middle of the second slot.
	d.HandleMouseMove(core.MouseMoveEvent{X: slot + slot/2, Y: 0})
	if d.hoverIndex != 1 {
		t.Errorf("hoverIndex = %d, want 1", d.hoverIndex)
	}

	// Below the row: no entry.
	d.HandleMouseMove(core.MouseMoveEvent{X: slot + slot/2, Y: metrics.CellHeight + 1})
	if d.hoverIndex != -1 {
		t.Errorf("hoverIndex = %d after leaving, want -1", d.hoverIndex)
	}
}

// The splitter tracks hover over its divider band so the grab handle can
// light up before a drag.
func TestSplitterDividerHover(t *testing.T) {
	sp := NewSplitter(core.Horizontal)
	sp.SetBounds(core.UnitRect{Width: 300, Height: 200})

	divider := sp.dividerBounds()
	sp.HandleMouseMove(core.MouseMoveEvent{X: divider.X + divider.Width/2, Y: 100})
	if !sp.hoveringDivider {
		t.Error("pointer over the divider did not set hover")
	}

	// Far to the left, inside the first pane, not the divider.
	sp.HandleMouseMove(core.MouseMoveEvent{X: 0, Y: 100})
	if sp.hoveringDivider {
		t.Error("pointer off the divider did not clear hover")
	}
}

// The menu bar highlights the top-level item under the pointer even when
// no dropdown is open.
func TestMenuBarItemHover(t *testing.T) {
	m := NewMenuBar()
	m.SetBounds(core.UnitRect{Width: 400, Height: 30})
	m.AddMenu(NewMenu("File"))
	m.AddMenu(NewMenu("Edit"))

	// Somewhere well inside the first item's title.
	metrics := m.EffectiveCellMetrics()
	m.HandleMouseMove(core.MouseMoveEvent{X: m.leftInset() + metrics.CellWidth, Y: 0})
	if m.hoverIndex != 0 {
		t.Errorf("hoverIndex = %d, want 0", m.hoverIndex)
	}

	// Below the bar row: cleared.
	m.HandleMouseMove(core.MouseMoveEvent{X: m.leftInset() + metrics.CellWidth, Y: metrics.CellHeight + 1})
	if m.hoverIndex != -1 {
		t.Errorf("hoverIndex = %d after leaving the bar, want -1", m.hoverIndex)
	}
}
