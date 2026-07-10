package trinkets

import (
	"testing"

	"github.com/phroun/kittytk/core"
	"github.com/phroun/kittytk/window"
)

// graphicalFrameStub is a container parent that reports graphical window
// frames with a nonzero border, so a child window's titlebar sits inside
// the top border (the window.go client-area contract).
type graphicalFrameStub struct {
	*Panel
	border core.Unit
}

func (g *graphicalFrameStub) GraphicalWindowFrames() bool       { return true }
func (g *graphicalFrameStub) WindowFrameBorderUnits() core.Unit { return g.border }

// On a graphical frame the titlebar is drawn inside the top border, so the
// draggable row runs from the border down a full cell. The MDI press path
// used to cut it off a border-thickness short of the titlebar's bottom, so
// grabbing the lower strip of the visible title did nothing. A press there
// must now begin a window drag.
func TestMDITitleBarDragCoversBorderOffset(t *testing.T) {
	stub := &graphicalFrameStub{Panel: NewPanel(), border: 2}

	pane := NewMDIPane()
	pane.SetParent(stub)
	pane.SetBounds(core.UnitRect{Width: 800, Height: 600})

	win := window.NewWindow("child")
	pane.AddWindow(win)
	win.SetBounds(core.UnitRect{X: 40, Y: 40, Width: 320, Height: 240})

	metrics := pane.EffectiveCellMetrics()
	border := core.FindFrameBorderUnits(win)
	if border <= 0 {
		t.Fatalf("expected a nonzero graphical frame border, got %d", border)
	}

	// A point in the bottom strip of the titlebar: below the old cutoff
	// (bounds.Y + CellHeight) but within the real titlebar
	// (bounds.Y + border + CellHeight). Mid-width so it is not a resize
	// grip.
	y := 40 + metrics.CellHeight + border - 1
	pane.HandleMousePress(core.MousePressEvent{X: 40 + 160, Y: y, Button: core.LeftButton})

	if pane.dragging != win {
		t.Errorf("press at titlebar-bottom (y=%d) did not start a drag; dragging=%v", y, pane.dragging)
	}
}
