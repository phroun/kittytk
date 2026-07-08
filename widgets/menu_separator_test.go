package widgets

import (
	"testing"

	"github.com/phroun/tuitk/core"
)

// gfxFrameParent is a minimal Container that reports graphical window
// frames, so a child Menu takes the pixel-surface separator path.
type gfxFrameParent struct{ core.WidgetBase }

func (g *gfxFrameParent) GraphicalWindowFrames() bool         { return true }
func (g *gfxFrameParent) Children() []core.Widget             { return nil }
func (g *gfxFrameParent) AddChild(core.Widget)                {}
func (g *gfxFrameParent) RemoveChild(core.Widget)             {}
func (g *gfxFrameParent) ChildAt(core.UnitPoint) core.Widget  { return nil }
func (g *gfxFrameParent) Layout()                             {}
func (g *gfxFrameParent) LayoutManager() core.LayoutManager   { return nil }
func (g *gfxFrameParent) SetLayoutManager(core.LayoutManager) {}

func newGraphicalMenu() *Menu {
	gp := &gfxFrameParent{}
	gp.WidgetBase = *core.NewWidgetBase()
	gp.Init(gp)
	m := NewMenu("t")
	m.SetParent(gp)
	m.AddItem(NewMenuItem("New"))
	m.AddItem(NewMenuItem("Open"))
	m.AddSeparator()
	m.AddItem(NewMenuItem("Quit"))
	m.Show(0, 0)
	return m
}

// On a graphical surface a separator row is a thin band, so the menu is
// shorter than four full text rows, and Y coordinates still map to the
// right items across the thin separator.
func TestMenuGraphicalSeparatorLayout(t *testing.T) {
	m := newGraphicalMenu()
	cellH := m.EffectiveCellMetrics().CellHeight

	// Height = 3 text rows + 1 thin separator band, not 4 full rows.
	if got, full := m.calculateSize().Height, cellH*4; got >= full {
		t.Errorf("graphical menu height = %d, want < %d (thin separator)", got, full)
	}
	if got, want := m.calculateSize().Height, cellH*3+separatorBandUnits; got != want {
		t.Errorf("graphical menu height = %d, want %d", got, want)
	}

	// hitRow maps Y to the right item across the thin separator band.
	// Rows: New [0,cellH), Open [cellH,2cellH), sep [2cellH,2cellH+band),
	// Quit [2cellH+band, ...).
	quitTop := cellH*2 + separatorBandUnits
	if kind, idx := m.hitRow(quitTop + 2); kind != 3 || idx != 3 {
		t.Errorf("hitRow at Quit = (%d,%d), want (3,3)", kind, idx)
	}
	if kind, idx := m.hitRow(cellH*2 + 1); kind != 3 || idx != 2 {
		t.Errorf("hitRow in separator band = (%d,%d), want (3,2 separator)", kind, idx)
	}
	if kind, idx := m.hitRow(cellH + 1); kind != 3 || idx != 1 {
		t.Errorf("hitRow at Open = (%d,%d), want (3,1)", kind, idx)
	}
}
