package widgets

import (
	"testing"

	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/layout"
	"github.com/phroun/tuitk/window"
)

// Replicates the demo's Selection-tab hierarchy: window → tabwidget →
// panel(label + splitter). Toggling the window's denomination must not
// move anything in device space.
func TestWindowDenominationLayoutInvariance(t *testing.T) {
	win := window.NewWindow("t")

	tabs := NewTabWidget()

	split := NewVSplitter()
	split.SetPosition(0.4)
	split.SetSizePolicy(core.NewSizePolicy(core.SizeExpanding, core.SizeExpanding))
	top := NewPanel()
	top.SetLayoutManager(layout.NewBoxLayout(core.Vertical))
	bottom := NewPanel()
	bottom.SetLayoutManager(layout.NewBoxLayout(core.Vertical))
	split.SetFirst(top)
	split.SetSecond(bottom)

	outer := NewPanel()
	ol := layout.NewBoxLayout(core.Vertical)
	lbl := NewLabel("a wrapped label occupying the top row of the page contents area")
	lbl.SetWordWrap(true)
	outer.AddChild(lbl)
	outer.AddChild(split)
	outer.SetLayoutManager(ol)
	ol.ItemAt(0).WithAlign(core.AlignFill)
	ol.ItemAt(1).WithAlign(core.AlignFill)

	tabs.AddTab("Sel", outer)
	win.SetContent(tabs)
	win.SetBounds(core.UnitRect{Width: 8 * 100, Height: 16 * 40})

	// Device-space geometry of the splitter's top pane: rows, in the
	// currency the widgets actually live in.
	paneRows := func() (topRows, splitRows core.Unit) {
		split.Layout()
		m := core.FindEffectiveCellMetrics(split)
		return top.Bounds().Height / m.CellHeight,
			split.Bounds().Height / m.CellHeight
	}

	topBefore, splitBefore := paneRows()
	if splitBefore == 0 || topBefore == 0 {
		t.Fatalf("degenerate baseline: top=%d split=%d rows", topBefore, splitBefore)
	}

	dense := core.CellMetrics{CellWidth: 8, CellHeight: 32}
	win.SetCellMetrics(&dense)
	topAfter, splitAfter := paneRows()

	if splitAfter != splitBefore {
		t.Errorf("splitter height changed: %d -> %d rows", splitBefore, splitAfter)
	}
	if topAfter != topBefore {
		t.Errorf("top pane height changed: %d -> %d rows", topBefore, topAfter)
	}

	// And back again: exact restoration.
	win.SetCellMetrics(nil)
	topBack, splitBack := paneRows()
	if topBack != topBefore || splitBack != splitBefore {
		t.Errorf("toggle-off did not restore: top %d->%d, split %d->%d",
			topBefore, topBack, splitBefore, splitBack)
	}
}
