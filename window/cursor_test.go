package window

import (
	"testing"

	"github.com/phroun/kittytk/core"
)

// ibeamContent is a minimal content trinket that requests the text cursor.
type ibeamContent struct {
	core.TrinketBase
}

func (c *ibeamContent) CursorShape() core.CursorShape { return core.CursorText }

func TestResizeCursorForEdge(t *testing.T) {
	cases := []struct {
		edge int
		want core.CursorShape
	}{
		{ResizeEdgeNone, core.CursorDefault},
		{ResizeEdgeLeft, core.CursorResizeH},
		{ResizeEdgeRight, core.CursorResizeH},
		{ResizeEdgeBottom, core.CursorResizeV},
		{ResizeEdgeLeft | ResizeEdgeBottom, core.CursorResizeNESW},
		{ResizeEdgeRight | ResizeEdgeBottom, core.CursorResizeNWSE},
		{ResizeEdgeTop, core.CursorResizeV},
		{ResizeEdgeLeft | ResizeEdgeTop, core.CursorResizeNWSE},
		{ResizeEdgeRight | ResizeEdgeTop, core.CursorResizeNESW},
	}
	for _, c := range cases {
		if got := resizeCursorForEdge(c.edge); got != c.want {
			t.Errorf("edge %d: got %v, want %v", c.edge, got, c.want)
		}
	}
}

func TestCursorAtResolvesEdgeAndContent(t *testing.T) {
	m := NewWindowManager()
	w := NewWindow("w")
	content := &ibeamContent{}
	content.TrinketBase = *core.NewTrinketBase()
	w.SetContent(content)
	w.SetBounds(core.UnitRect{X: 100, Y: 100, Width: 200, Height: 120})
	m.AddWindow(w)
	w.Layout()

	// Over the right edge -> horizontal resize cursor.
	if got := m.CursorAt(299, 160); got != core.CursorResizeH {
		t.Errorf("right edge cursor = %v, want CursorResizeH", got)
	}
	// Over the content interior -> the text I-beam from the content trinket.
	if got := m.CursorAt(200, 160); got != core.CursorText {
		t.Errorf("content cursor = %v, want CursorText", got)
	}
	// Off any window -> the default arrow.
	if got := m.CursorAt(10, 10); got != core.CursorDefault {
		t.Errorf("empty-desktop cursor = %v, want CursorDefault", got)
	}
}

func TestWindowCursorShapeAtTitleBarIsDefault(t *testing.T) {
	w := NewWindow("w")
	content := &ibeamContent{}
	content.TrinketBase = *core.NewTrinketBase()
	w.SetContent(content)
	w.SetBounds(core.UnitRect{X: 0, Y: 0, Width: 200, Height: 120})
	w.Layout()

	// The title bar row is not content: arrow, not I-beam.
	if got := w.CursorShapeAt(100, 0); got != core.CursorDefault {
		t.Errorf("title-bar cursor = %v, want CursorDefault", got)
	}
}

func TestTornCursorForEdge(t *testing.T) {
	cases := []struct {
		edges int
		want  core.CursorShape
	}{
		{0, core.CursorDefault},
		{resizeLeft, core.CursorResizeH},
		{resizeRight, core.CursorResizeH},
		{resizeBottom, core.CursorResizeV},
		{resizeLeft | resizeBottom, core.CursorResizeNESW},
		{resizeRight | resizeBottom, core.CursorResizeNWSE},
		{resizeTop, core.CursorResizeV},
		{resizeLeft | resizeTop, core.CursorResizeNWSE},
		{resizeRight | resizeTop, core.CursorResizeNESW},
	}
	for _, c := range cases {
		if got := tornCursorForEdge(c.edges); got != c.want {
			t.Errorf("edges %d: got %v, want %v", c.edges, got, c.want)
		}
	}
}

func TestTornEdgeRects(t *testing.T) {
	b := core.UnitRect{Width: 200, Height: 120}
	g := tearResizeGrip

	// Bottom-right corner -> right band + bottom band.
	got := tornEdgeRects(b, resizeRight|resizeBottom, g)
	if len(got) != 2 {
		t.Fatalf("corner: want 2 rects, got %d", len(got))
	}
	wantRight := core.UnitRect{X: 200 - g, Width: g, Height: 120}
	wantBottom := core.UnitRect{Y: 120 - g, Width: 200, Height: g}
	if got[0] != wantRight {
		t.Errorf("right band = %v, want %v", got[0], wantRight)
	}
	if got[1] != wantBottom {
		t.Errorf("bottom band = %v, want %v", got[1], wantBottom)
	}

	// Top-left corner -> left band + top band.
	got = tornEdgeRects(b, resizeLeft|resizeTop, g)
	if len(got) != 2 {
		t.Fatalf("top corner: want 2 rects, got %d", len(got))
	}
	wantLeft := core.UnitRect{Width: g, Height: 120}
	wantTop := core.UnitRect{Width: 200, Height: g}
	if got[0] != wantLeft {
		t.Errorf("left band = %v, want %v", got[0], wantLeft)
	}
	if got[1] != wantTop {
		t.Errorf("top band = %v, want %v", got[1], wantTop)
	}
}

// A detached window's ClientArea (used to clamp its own menu bar's
// dropdowns) stays within the window surface, so tall menus scroll
// instead of overflowing.
func TestDetachedWindowClientAreaBounded(t *testing.T) {
	w := NewWindow("w")
	w.SetDetached(true)
	mb := &ibeamContent{}
	mb.TrinketBase = *core.NewTrinketBase()
	w.SetWindowMenuBar(mb)
	w.SetBounds(core.UnitRect{Width: 400, Height: 300})
	w.Layout()

	ca := w.ClientArea()
	b := w.Bounds()
	if ca.Height <= 0 {
		t.Fatalf("client area height = %d, want > 0", ca.Height)
	}
	if ca.Y+ca.Height > b.Height {
		t.Errorf("client area bottom %d exceeds window height %d", ca.Y+ca.Height, b.Height)
	}
}
