package window

import (
	"testing"

	"github.com/phroun/kittytk/core"
)

// scrollMock is a container that scrolls its single child by a unit
// offset, like a ScrollArea (implements core.ScrollOffsetUnitsProvider).
type scrollMock struct {
	*core.TrinketBase
	child      core.Trinket
	offX, offY core.Unit
}

func (m *scrollMock) Children() []core.Trinket            { return []core.Trinket{m.child} }
func (m *scrollMock) AddChild(core.Trinket)               {}
func (m *scrollMock) RemoveChild(core.Trinket)            {}
func (m *scrollMock) ChildAt(core.UnitPoint) core.Trinket { return m.child }
func (m *scrollMock) Layout()                             {}
func (m *scrollMock) LayoutManager() core.LayoutManager   { return nil }
func (m *scrollMock) SetLayoutManager(core.LayoutManager) {}
func (m *scrollMock) ScrollOffsetUnits() (core.Unit, core.Unit) {
	return m.offX, m.offY
}

// ibeamMock reports an I-beam cursor when hovered within its content
// coordinates (a text field only wants the I-beam over its glyph row).
type ibeamMock struct {
	*core.TrinketBase
	rect core.UnitRect
}

func (m *ibeamMock) CursorShape() core.CursorShape {
	return core.CursorText
}

// The cursor-shape descent must add a scroll container's offset exactly
// as the mouse-event descent does, so the I-beam region tracks the
// content as it scrolls instead of drifting.
func TestCursorShapeFollowsScroll(t *testing.T) {
	ib := &ibeamMock{TrinketBase: core.NewTrinketBase(), rect: core.UnitRect{Width: 200, Height: 16}}
	sc := &scrollMock{TrinketBase: core.NewTrinketBase(), child: ib}

	// Unscrolled: a hover at viewport y=0 lands on the child's top row.
	if got := cursorShapeAtTrinket(sc, core.UnitPoint{X: 10, Y: 0}); got != core.CursorText {
		t.Fatalf("unscrolled: cursor = %v, want I-beam", got)
	}

	// Scroll the content down by 40 units: the same viewport hover now
	// maps to content y=40. The descent must add the offset (mirroring
	// ScrollArea.HandleMouseMove), so the child still receives the hover.
	sc.offX, sc.offY = 0, 40
	if got := cursorShapeAtTrinket(sc, core.UnitPoint{X: 10, Y: 0}); got != core.CursorText {
		t.Errorf("scrolled: cursor = %v, want I-beam (offset must be added)", got)
	}
}
