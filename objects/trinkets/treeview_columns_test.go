package trinkets

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/phroun/kittytk/backend/raster"
	"github.com/phroun/kittytk/core"
	"github.com/phroun/kittytk/style"
)

// newColumnsTree builds a 3-column tree (key + Size + Kind) sized to
// width cells x height rows at the base 8x16 metrics, with a few
// items carrying cell values.
func newColumnsTree(widthCells, heightRows int) *TreeView {
	tv := NewTreeView()
	tv.SetShowHeader(true)
	size := NewTreeColumn("size", "Size", 10)
	size.Align = "right"
	kind := NewTreeColumn("kind", "Kind", 12)
	tv.AddColumn(size)
	tv.AddColumn(kind)

	root := NewTreeItem("Folder")
	root.Expanded = true
	root.SetValue("size", "--")
	root.SetValue("kind", "Folder")
	child := NewTreeItem("file.png")
	child.SetValue("size", "311 KB")
	child.SetValue("kind", "PNG image")
	root.AddChild(child)
	tv.AddRootItem(root)
	for i := 0; i < 30; i++ {
		it := NewTreeItem("extra")
		it.SetValue("size", "1 KB")
		it.SetValue("kind", "File")
		tv.AddRootItem(it)
	}

	tv.SetBounds(core.UnitRect{Width: core.Unit(widthCells * 8), Height: core.Unit(heightRows * 16)})
	return tv
}

// Fit mode: spans tile the content area exactly - key column absorbs
// the slack, dividers one cell, no horizontal scrolling.
func TestTreeColumnLayoutFitMode(t *testing.T) {
	tv := newColumnsTree(60, 10)
	lay := tv.columnLayout()

	if len(lay.spans) != 3 {
		t.Fatalf("want 3 spans (key+2), got %d", len(lay.spans))
	}
	if lay.maxHScroll != 0 {
		t.Errorf("fit mode must not scroll horizontally, maxHScroll=%d", lay.maxHScroll)
	}
	// Content = 60 cells minus the scrollbar lane = 59 cells.
	// key + div + 10 + div + 12 = 59 -> key = 35 cells.
	if lay.spans[0].w != 35*8 {
		t.Errorf("key span width = %d units, want %d", lay.spans[0].w, 35*8)
	}
	if lay.spans[1].x != 36*8 || lay.spans[1].w != 10*8 {
		t.Errorf("size span = x%d w%d, want x%d w%d", lay.spans[1].x, lay.spans[1].w, 36*8, 10*8)
	}
	if lay.spans[2].x != 47*8 || lay.spans[2].w != 12*8 {
		t.Errorf("kind span = x%d w%d, want x%d w%d", lay.spans[2].x, lay.spans[2].w, 47*8, 12*8)
	}
	if lay.spans[0].divX != 35*8 || lay.spans[1].divX != 46*8 {
		t.Errorf("dividers at %d,%d; want %d,%d", lay.spans[0].divX, lay.spans[1].divX, 35*8, 46*8)
	}
	// Header consumes one row; no footer in fit mode.
	if tv.headerHeight() != 16 || tv.footerHeight() != 0 {
		t.Errorf("headerH=%d footerH=%d, want 16,0", tv.headerHeight(), tv.footerHeight())
	}
	if tv.visibleCount() != 9 {
		t.Errorf("visibleCount=%d, want 9 (10 rows minus header)", tv.visibleCount())
	}
}

// Fit mode under overflow: data columns shrink toward MinWidth so the
// key column keeps a usable minimum.
func TestTreeColumnLayoutFitShrinks(t *testing.T) {
	tv := newColumnsTree(20, 10) // 19 content cells for key+10+12+2 dividers
	lay := tv.columnLayout()
	total := core.Unit(0)
	for _, sp := range lay.spans {
		total += sp.w
	}
	// key(min 6) + shrunk data columns + 2 dividers must fit 19 cells.
	if lay.spans[0].w < 6*8 {
		t.Errorf("key span %d under minimum", lay.spans[0].w)
	}
	if total+2*8 > 19*8 {
		t.Errorf("fit-mode spans overflow: total %d + dividers > %d", total, 19*8)
	}
}

// Fit mode under pressure reclaims MEASURED slack: a column declared
// far wider than its content (measured with the effective font, not a
// rune count) gives that padding to the key column before anything
// truncates - the key column ends up far wider than its hard minimum.
func TestTreeColumnFitReclaimsMeasuredSlack(t *testing.T) {
	tv := NewTreeView()
	tv.SetShowHeader(true)
	wide := NewTreeColumn("pad", "Pad", 30) // declared 30 cells of mostly padding
	tv.AddColumn(wide)
	it := NewTreeItem("a-rather-long-file-name.png")
	it.SetValue("pad", "x") // content needs ~1 cell
	tv.AddRootItem(it)
	// 40 cells wide: 39 content; declared key(20 desired)+30+1 divider
	// overflows, so the pad column must shrink toward its measured need.
	tv.SetBounds(core.UnitRect{Width: 40 * 8, Height: 10 * 16})

	lay := tv.columnLayout()
	keyW := int(lay.spans[0].w / 8)
	padW := int(lay.spans[1].w / 8)
	// The key reaches its desired width (20 cells) by reclaiming the
	// padded column's measured slack; the pad keeps what is left (a
	// declared width is respected absent pressure). Under the old
	// MinWidth-only shrink the key would have been crushed to 6.
	if keyW < 20 {
		t.Errorf("key column got %d cells; measured reclaim should reach desired 20 (padW=%d)", keyW, padW)
	}
	if padW >= 30 {
		t.Errorf("padded column gave up nothing (%d cells)", padW)
	}
}

// With the key column hidden, the FIRST visible data column hosts the
// tree affordances: nesting indent, a working expander, and forced
// left alignment regardless of its Align setting.
func TestTreeColumnHostWhenKeyHidden(t *testing.T) {
	tv := newColumnsTree(60, 10)
	tv.SetShowKey(false)
	size := tv.ColumnByID("size") // first visible data column, align=right
	if tv.treeHostColumn() != size {
		t.Fatalf("host = %v, want the size column", tv.treeHostColumn())
	}
	// Hiding the first column moves the host to the next one.
	size.Hidden = true
	if tv.treeHostColumn() != tv.ColumnByID("kind") {
		t.Fatalf("host after hiding = %v, want kind", tv.treeHostColumn())
	}
	size.Hidden = false

	// The expander click works in the host span: the expanded root
	// folder (level 0, indicator in the span's first cell) collapses.
	lay := tv.columnLayout()
	if lay.spans[0].col != size {
		t.Fatalf("first span should be the size column, got %v", lay.spans[0].col)
	}
	root := tv.RootItems()[0]
	if !root.Expanded {
		t.Fatal("precondition: root expanded")
	}
	tv.HandleMousePress(core.MousePressEvent{
		X: lay.spans[0].x + 2, Y: tv.headerHeight() + 2, Button: core.LeftButton,
	})
	if root.Expanded {
		t.Error("expander click in the host data column did not collapse")
	}

	// With the key visible, the host is nil (the key hosts the tree).
	tv.SetShowKey(true)
	if tv.treeHostColumn() != nil {
		t.Errorf("host with key visible = %v, want nil", tv.treeHostColumn())
	}
}

// Scroll mode: natural widths, footer row reserved, hScroll pans the
// unfixed spans and clamps to the overflow.
func TestTreeColumnLayoutScrollMode(t *testing.T) {
	tv := newColumnsTree(30, 10)
	tv.SetFitWidth(false)
	tv.SetKeyWidth(20)

	if tv.footerHeight() != 16 {
		t.Fatalf("scroll mode must reserve the footer row")
	}
	if tv.visibleCount() != 8 {
		t.Errorf("visibleCount=%d, want 8 (10 rows minus header and footer)", tv.visibleCount())
	}

	lay := tv.columnLayout()
	// Natural: 20 + 1 + 10 + 1 + 12 = 44 cells in 29 content cells.
	if lay.maxHScroll != 44-29 {
		t.Errorf("maxHScroll=%d, want %d", lay.maxHScroll, 44-29)
	}

	if !tv.scrollHorizontally(5) {
		t.Fatal("scrollHorizontally(5) did nothing")
	}
	lay = tv.columnLayout()
	if lay.spans[0].x != -5*8 {
		t.Errorf("panned key span x=%d, want %d", lay.spans[0].x, -5*8)
	}
	// Clamp at the end.
	tv.scrollHorizontally(1000)
	lay = tv.columnLayout()
	if tv.hScroll != lay.maxHScroll {
		t.Errorf("hScroll=%d, want clamp at %d", tv.hScroll, lay.maxHScroll)
	}
}

// Fixed columns stay pinned while the middle pans.
func TestTreeColumnFixedLeft(t *testing.T) {
	tv := newColumnsTree(30, 10)
	tv.SetFitWidth(false)
	tv.SetKeyWidth(15)
	tv.SetFixedColumns(1, 0)
	tv.scrollHorizontally(4)

	lay := tv.columnLayout()
	if !lay.spans[0].fixed || lay.spans[0].x != 0 {
		t.Errorf("fixed key span moved: fixed=%v x=%d", lay.spans[0].fixed, lay.spans[0].x)
	}
	if lay.spans[1].fixed {
		t.Errorf("span 1 should scroll")
	}
	// Scroll region starts after the fixed key span + divider.
	if lay.scrollL != 16*8 {
		t.Errorf("scrollL=%d, want %d", lay.scrollL, 16*8)
	}
	if lay.spans[1].x != lay.scrollL-4*8 {
		t.Errorf("panned span 1 x=%d, want %d", lay.spans[1].x, lay.scrollL-4*8)
	}
}

// Hidden columns drop out of the layout; the chooser toggles them back.
func TestTreeColumnHiddenAndChooser(t *testing.T) {
	tv := newColumnsTree(60, 10)
	tv.ColumnByID("kind").Hidden = true
	lay := tv.columnLayout()
	if len(lay.spans) != 2 {
		t.Fatalf("hidden column still laid out: %d spans", len(lay.spans))
	}

	// The [=] button exists (optional columns present) and sits in the
	// scrollbar lane's header cell.
	r, ok := tv.chooserButtonRect()
	if !ok {
		t.Fatal("chooser button missing")
	}
	if r.X != tv.Bounds().Width-8 || r.Y != 0 {
		t.Errorf("chooser at %v", r)
	}

	// Press on the button opens the REAL Menu popup on the ancestor's
	// controller (same overlay path as combobox/context menus).
	host := &recordingPopupController{}
	parent := NewPanel()
	parent.SetPopupController(host)
	tv.SetParent(parent)
	if !tv.HandleMousePress(core.MousePressEvent{X: r.X, Y: r.Y, Button: core.LeftButton}) {
		t.Fatal("chooser press not handled")
	}
	if host.popup == nil || tv.chooserMenu == nil {
		t.Fatal("no menu popup registered")
	}
	if len(tv.chooserMenu.Items()) != 2 {
		t.Fatalf("chooser menu items = %d, want 2", len(tv.chooserMenu.Items()))
	}
	// The tree retains focus and forwards keys (the menu bar pattern):
	// Down, Down, Enter toggles the second item - the hidden "kind".
	tv.HandleKeyPress(core.KeyPressEvent{Key: "Down"})
	tv.HandleKeyPress(core.KeyPressEvent{Key: "Down"})
	tv.HandleKeyPress(core.KeyPressEvent{Key: "Enter"})
	if tv.ColumnByID("kind").Hidden {
		t.Error("chooser keyboard toggle did not unhide the column")
	}
	if tv.chooserOpen {
		t.Error("menu should close after triggering an item")
	}
	// Escape path: reopen, dismiss.
	tv.openColumnChooser(true)
	if !tv.chooserOpen {
		t.Fatal("reopen failed")
	}
	tv.HandleKeyPress(core.KeyPressEvent{Key: "Escape"})
	if tv.chooserOpen || host.popup != nil {
		t.Error("Escape did not dismiss the chooser")
	}
}

// Divider drags size the side the slack dictates. Fit mode with the
// auto-fill key column on the left: every divider sizes the column to
// its RIGHT with the delta inverted, so the grabbed boundary follows
// the mouse (sizing the left column would move a DIFFERENT boundary -
// the drag felt like pulling the wrong end). Scroll mode has no slack
// column: the classic left-column sizing applies.
func TestTreeColumnDividerDrag(t *testing.T) {
	tv := newColumnsTree(60, 10)
	lay := tv.columnLayout()
	divX := lay.spans[1].divX // divider between Size and Kind

	if !tv.HandleMousePress(core.MousePressEvent{X: divX + 2, Y: 4, Button: core.LeftButton}) {
		t.Fatal("divider press not handled")
	}
	if !tv.colDragging || !tv.colDragInvert {
		t.Fatalf("fit-mode drag: dragging=%v invert=%v, want true/true", tv.colDragging, tv.colDragInvert)
	}
	// Slack (key) is left: drag RIGHT 4 cells -> Kind (the right column)
	// gets 4 cells NARROWER, the grabbed boundary moves right with the
	// mouse, and Kind's right edge stays put.
	tv.HandleMouseMove(core.MouseMoveEvent{X: divX + 2 + 4*8, Y: 4, Buttons: 1})
	if got := tv.ColumnByID("kind").Width; got != 8 {
		t.Errorf("inverted drag: kind width=%d, want 8", got)
	}
	if got := tv.ColumnByID("size").Width; got != 10 {
		t.Errorf("inverted drag touched the left column: size=%d, want 10", got)
	}
	tv.HandleMouseRelease(core.MouseReleaseEvent{Button: core.LeftButton})
	if tv.colDragging {
		t.Error("release did not end the drag")
	}

	// Scroll mode (no slack column): the divider sizes the column to
	// its LEFT, classic semantics.
	tv.SetFitWidth(false)
	tv.SetKeyWidth(15)
	lay = tv.columnLayout()
	divX = lay.spans[1].divX
	tv.HandleMousePress(core.MousePressEvent{X: divX + 2, Y: 4, Button: core.LeftButton})
	if !tv.colDragging || tv.colDragInvert {
		t.Fatalf("scroll-mode drag: dragging=%v invert=%v, want true/false", tv.colDragging, tv.colDragInvert)
	}
	tv.HandleMouseMove(core.MouseMoveEvent{X: divX + 2 + 4*8, Y: 4, Buttons: 1})
	if got := tv.ColumnByID("size").Width; got != 14 {
		t.Errorf("scroll-mode drag: size width=%d, want 14", got)
	}
	tv.HandleMouseRelease(core.MouseReleaseEvent{Button: core.LeftButton})

	// Non-resizable right column refuses the inverted drag.
	tv.SetFitWidth(true)
	tv.ColumnByID("kind").Resizable = false
	lay = tv.columnLayout()
	tv.HandleMousePress(core.MousePressEvent{X: lay.spans[1].divX + 2, Y: 4, Button: core.LeftButton})
	if tv.colDragging {
		t.Error("divider left of a non-resizable column started a drag in fit mode")
	}
}

// Activating a sortable header cycles ascending -> descending ->
// unsorted and fires the observer; non-sortable data captions do
// nothing; the key column is always sortable (sortedBy -1).
func TestTreeColumnSortClick(t *testing.T) {
	tv := newColumnsTree(60, 10)
	tv.ColumnByID("size").Sortable = true

	var gotSorted, gotDesc bool
	var gotBy int
	calls := 0
	tv.SetOnSortRequested(func(sorted bool, by int, desc bool) {
		gotSorted, gotBy, gotDesc = sorted, by, desc
		calls++
	})

	lay := tv.columnLayout()
	x := lay.spans[1].x + 8 // inside the Size caption (data index 0)
	click := func(cx core.Unit) {
		tv.HandleMousePress(core.MousePressEvent{X: cx, Y: 4, Button: core.LeftButton})
	}
	click(x)
	if calls != 1 || !gotSorted || gotBy != 0 || gotDesc {
		t.Fatalf("first click: calls=%d sorted=%v by=%d desc=%v", calls, gotSorted, gotBy, gotDesc)
	}
	click(x)
	if calls != 2 || !gotSorted || !gotDesc {
		t.Fatalf("second click should reverse: calls=%d sorted=%v desc=%v", calls, gotSorted, gotDesc)
	}
	click(x)
	if calls != 3 || gotSorted {
		t.Fatalf("third click should unsort: calls=%d sorted=%v", calls, gotSorted)
	}
	if sorted, _, _ := tv.Sorted(); sorted {
		t.Errorf("view still sorted after the cycle completed")
	}

	// Kind (data index 1) is not sortable: no callback.
	click(lay.spans[2].x + 8)
	if calls != 3 {
		t.Errorf("non-sortable caption fired the callback")
	}

	// The key column is always sortable: sortedBy -1.
	click(lay.spans[0].x + 8)
	if calls != 4 || !gotSorted || gotBy != -1 || gotDesc {
		t.Errorf("key caption click: calls=%d sorted=%v by=%d desc=%v", calls, gotSorted, gotBy, gotDesc)
	}
}

// Content clicks land on the right rows with the header row present.
func TestTreeColumnHeaderRowOffset(t *testing.T) {
	tv := newColumnsTree(60, 10)
	// Row 0 of content = y in [16,32).
	tv.HandleMousePress(core.MousePressEvent{X: 30 * 8, Y: 20, Button: core.LeftButton})
	if tv.CurrentIndex() != 0 {
		t.Errorf("click on first content row selected %d", tv.CurrentIndex())
	}
	tv.HandleMousePress(core.MousePressEvent{X: 30 * 8, Y: 40, Button: core.LeftButton})
	if tv.CurrentIndex() != 1 {
		t.Errorf("click on second content row selected %d", tv.CurrentIndex())
	}
}

// Rendering smoke + optional visual proof on the pixel path.
func TestTreeColumnPaintSmoke(t *testing.T) {
	b, err := raster.New(640, 240)
	if err != nil {
		t.Fatal(err)
	}
	d := NewDesktop()
	d.SetBackend(b)
	tv := newColumnsTree(60, 10)
	tv.SetParent(d)
	tv.ColumnByID("size").Sortable = true
	tv.SetSorted(true, 0, false)
	b.Clear(style.DefaultStyle())
	tv.Paint(core.NewPainter(b))

	if dir := os.Getenv("KITTYTK_PROOF_DIR"); dir != "" {
		out := filepath.Join(dir, "treeview_columns.png")
		if err := b.WritePNG(out); err != nil {
			t.Fatal(err)
		}
		t.Logf("proof -> %s", out)
	}
}

// recordingPopupController captures the registered popup so tests can
// drive its handlers.
type recordingPopupController struct {
	popup *core.PopupRequest
}

func (h *recordingPopupController) RegisterPopup(r *core.PopupRequest) { h.popup = r }
func (h *recordingPopupController) UnregisterPopup(string)             { h.popup = nil }
func (h *recordingPopupController) MapToScreen(_ core.Trinket, p core.UnitPoint) core.UnitPoint {
	return p
}
func (h *recordingPopupController) ScreenBounds() core.UnitRect {
	return core.UnitRect{Width: 800, Height: 480}
}
