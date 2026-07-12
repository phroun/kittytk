package trinkets

import (
	"testing"

	"github.com/phroun/kittytk/core"
)

// newEditableTree: key + Size(editable) + Kind(editable) + Date(not),
// flat rows so index math is simple.
func newEditableTree() *TreeView {
	tv := NewTreeView()
	tv.SetShowHeader(true)
	size := NewTreeColumn("size", "Size", 10)
	size.Editable = true
	kind := NewTreeColumn("kind", "Kind", 12)
	kind.Editable = true
	date := NewTreeColumn("date", "Date", 12)
	tv.AddColumn(size)
	tv.AddColumn(kind)
	tv.AddColumn(date)
	for _, name := range []string{"alpha", "beta", "gamma"} {
		it := NewTreeItem(name)
		it.SetValue("size", "1 KB")
		it.SetValue("kind", "File")
		it.SetValue("date", "Today")
		tv.AddRootItem(it)
	}
	tv.SetBounds(core.UnitRect{Width: 480, Height: 160})
	tv.SetCurrentIndex(0)
	return tv
}

// With no editable columns, Enter keeps its classic Space behavior
// (expand/collapse the current item).
func TestTreeEnterWithoutEditableActsLikeSpace(t *testing.T) {
	tv := newColumnsTree(60, 10) // size/kind are NOT editable here
	tv.SetCurrentIndex(0)        // "Folder", expanded
	folder := tv.CurrentItem()
	if !folder.Expanded {
		t.Fatal("precondition: folder expanded")
	}
	tv.HandleKeyPress(core.KeyPressEvent{Key: "Enter"})
	if tv.rowEditing {
		t.Fatal("row edit began with no editable columns")
	}
	if folder.Expanded {
		t.Error("Enter did not collapse (Space behavior)")
	}
}

// Enter opens the row editor on the first editable column; Tab commits
// and cycles; Enter commits the row and dismisses; re-entering resumes
// the last-edited column.
func TestTreeRowEditLifecycle(t *testing.T) {
	tv := newEditableTree()
	edits := 0
	tv.SetOnCellEdited(func(item *TreeItem, col *TreeColumn, v string) { edits++ })

	tv.HandleKeyPress(core.KeyPressEvent{Key: "Enter"})
	if !tv.rowEditing || tv.editCol != tv.ColumnByID("size") {
		t.Fatalf("edit: rowEditing=%v col=%v, want size", tv.rowEditing, tv.editCol)
	}
	if tv.editBox.Text() != "1 KB" {
		t.Fatalf("editor prefill = %q", tv.editBox.Text())
	}
	// Change the value, Tab to the next editable column (kind - the
	// non-editable date column is skipped by construction of the Tab
	// ring). The size value commits on the way out.
	tv.editBox.SetText("2 KB")
	tv.HandleKeyPress(core.KeyPressEvent{Key: "Tab"})
	if tv.editCol != tv.ColumnByID("kind") {
		t.Fatalf("after Tab: col=%v, want kind", tv.editCol)
	}
	if got := tv.CurrentItem().Value("size"); got != "2 KB" {
		t.Errorf("Tab did not commit the cell: size=%q", got)
	}
	if edits != 1 {
		t.Errorf("edits=%d, want 1", edits)
	}
	// S-Tab wraps back to size.
	tv.HandleKeyPress(core.KeyPressEvent{Key: "S-Tab"})
	if tv.editCol != tv.ColumnByID("size") {
		t.Fatalf("after S-Tab: col=%v, want size", tv.editCol)
	}
	tv.HandleKeyPress(core.KeyPressEvent{Key: "Tab"}) // back onto kind
	tv.editBox.SetText("Document")
	tv.HandleKeyPress(core.KeyPressEvent{Key: "Enter"})
	if tv.rowEditing {
		t.Fatal("Enter did not dismiss the row editor")
	}
	if got := tv.CurrentItem().Value("kind"); got != "Document" {
		t.Errorf("Enter did not commit the row: kind=%q", got)
	}
	// Re-entering resumes the column edited last (kind).
	tv.HandleKeyPress(core.KeyPressEvent{Key: "Enter"})
	if !tv.rowEditing || tv.editCol != tv.ColumnByID("kind") {
		t.Fatalf("re-enter: col=%v, want kind (remembered)", tv.editCol)
	}
	tv.HandleKeyPress(core.KeyPressEvent{Key: "Escape"})
}

// Escape cancels: the cell keeps its original value.
func TestTreeRowEditEscapeCancels(t *testing.T) {
	tv := newEditableTree()
	tv.HandleKeyPress(core.KeyPressEvent{Key: "Enter"})
	tv.editBox.SetText("garbage")
	tv.HandleKeyPress(core.KeyPressEvent{Key: "Escape"})
	if tv.rowEditing {
		t.Fatal("Escape did not dismiss")
	}
	if got := tv.CurrentItem().Value("size"); got != "1 KB" {
		t.Errorf("Escape wrote the value: size=%q", got)
	}
}

// Up/Down accept the edit and continue editing the SAME column on the
// neighboring row.
func TestTreeRowEditUpDownContinues(t *testing.T) {
	tv := newEditableTree()
	tv.HandleKeyPress(core.KeyPressEvent{Key: "Enter"})
	tv.HandleKeyPress(core.KeyPressEvent{Key: "Tab"}) // onto kind
	tv.editBox.SetText("Folder")
	tv.HandleKeyPress(core.KeyPressEvent{Key: "Down"})
	if got := tv.RootItems()[0].Value("kind"); got != "Folder" {
		t.Errorf("Down did not accept the edit: kind=%q", got)
	}
	if tv.CurrentIndex() != 1 {
		t.Fatalf("Down did not move the selection: index=%d", tv.CurrentIndex())
	}
	if !tv.rowEditing || tv.editCol != tv.ColumnByID("kind") {
		t.Fatalf("Down did not continue editing kind on the next row")
	}
	tv.HandleKeyPress(core.KeyPressEvent{Key: "Up"})
	if tv.CurrentIndex() != 0 || !tv.rowEditing || tv.editCol != tv.ColumnByID("kind") {
		t.Errorf("Up did not continue editing kind on the row above")
	}
	tv.HandleKeyPress(core.KeyPressEvent{Key: "Escape"})
}

// Ordinary keys reach the text box while editing (they must not fall
// through to tree navigation).
func TestTreeRowEditForwardsTyping(t *testing.T) {
	tv := newEditableTree()
	tv.HandleKeyPress(core.KeyPressEvent{Key: "Enter"})
	// The prefill is selected; typing replaces it.
	tv.HandleKeyPress(core.KeyPressEvent{Key: "x", Text: "x"})
	tv.HandleKeyPress(core.KeyPressEvent{Key: "y", Text: "y"})
	if got := tv.editBox.Text(); got != "xy" {
		t.Errorf("typed text = %q, want %q", got, "xy")
	}
	// The selection did not move (Up/Down are edit navigation, but a
	// plain letter must never reach the tree).
	if tv.CurrentIndex() != 0 {
		t.Errorf("typing moved the tree selection")
	}
	tv.HandleKeyPress(core.KeyPressEvent{Key: "Escape"})
}

// Clicking off the editor accepts the value; the click then acts
// normally (here: selects the clicked row).
func TestTreeRowEditClickOffAccepts(t *testing.T) {
	tv := newEditableTree()
	tv.HandleKeyPress(core.KeyPressEvent{Key: "Enter"})
	tv.editBox.SetText("42 KB")
	// Row 2 sits at y = header(16) + 2*16 + mid.
	tv.HandleMousePress(core.MousePressEvent{X: 20, Y: 16 + 2*16 + 8, Button: core.LeftButton})
	if tv.rowEditing {
		t.Fatal("click off did not dismiss the editor")
	}
	if got := tv.RootItems()[0].Value("size"); got != "42 KB" {
		t.Errorf("click off did not accept the value: size=%q", got)
	}
	if tv.CurrentIndex() != 2 {
		t.Errorf("the click did not proceed to select row 2: index=%d", tv.CurrentIndex())
	}
}

// Switching edit columns brings the editor into view with the
// ScrollArea's conservative rule: scroll the minimum needed, and not
// at all when the cell is already fully visible.
func TestTreeEditEnsuresColumnVisible(t *testing.T) {
	tv := newColumnsTree(30, 10) // TUI: content 29 cells
	tv.SetFitWidth(false)
	tv.SetKeyWidth(20) // natural: key 20 |1| size 10 |1| kind 12 = 44
	tv.ColumnByID("size").Editable = true
	tv.ColumnByID("kind").Editable = true
	tv.SetCurrentIndex(0)

	// Entering edit on Size (cells 21..31, view 29 wide, hScroll 0):
	// the right edge is 2 cells past the view - scroll exactly 2.
	tv.HandleKeyPress(core.KeyPressEvent{Key: "Enter"})
	if !tv.rowEditing || tv.editCol != tv.ColumnByID("size") {
		t.Fatal("precondition: editing size")
	}
	if tv.hScroll != 2 {
		t.Errorf("hScroll after edit start = %d, want 2", tv.hScroll)
	}
	// Tab to Kind (cells 32..44): right-align to it, clamped to the
	// max scroll (15).
	tv.HandleKeyPress(core.KeyPressEvent{Key: "Tab"})
	if tv.hScroll != 15 {
		t.Errorf("hScroll after Tab to kind = %d, want 15", tv.hScroll)
	}
	// S-Tab back to Size: at hScroll 15 its cells 21..31 already sit
	// inside the view (15..44) - conservative: NO movement.
	tv.HandleKeyPress(core.KeyPressEvent{Key: "S-Tab"})
	if tv.editCol != tv.ColumnByID("size") {
		t.Fatal("S-Tab did not return to size")
	}
	if tv.hScroll != 15 {
		t.Errorf("hScroll moved for an already-visible column: %d, want 15", tv.hScroll)
	}
	tv.HandleKeyPress(core.KeyPressEvent{Key: "Escape"})

	// A pinned column is always in view: editing it never scrolls.
	tv.SetFixedColumns(0, 1) // pin kind
	tv.HandleKeyPress(core.KeyPressEvent{Key: "Enter"})
	tv.HandleKeyPress(core.KeyPressEvent{Key: "Tab"}) // onto pinned kind
	if tv.editCol != tv.ColumnByID("kind") {
		t.Fatal("Tab did not reach kind")
	}
	if tv.hScroll != 15 {
		t.Errorf("editing a pinned column scrolled the region: %d, want 15", tv.hScroll)
	}
	tv.HandleKeyPress(core.KeyPressEvent{Key: "Escape"})
}

// A drag-free click on an editable cell of the ALREADY selected row
// flips straight into edit mode - no settle delay.
func TestTreeClickToEdit(t *testing.T) {
	tv := newEditableTree()
	lay := tv.columnLayout()
	var sizeX core.Unit
	for _, sp := range lay.spans {
		if sp.col == tv.ColumnByID("size") {
			sizeX = sp.x + sp.w/2
		}
	}
	rowY := core.Unit(16 + 8) // row 0, already selected

	press := core.MousePressEvent{X: sizeX, Y: rowY, Button: core.LeftButton}
	release := core.MouseReleaseEvent{X: sizeX, Y: rowY, Button: core.LeftButton}
	tv.HandleMousePress(press)
	if tv.clickEditItem == nil {
		t.Fatal("press on selected editable cell did not become a candidate")
	}
	if tv.rowEditing {
		t.Fatal("edit began on press; it must wait for the drag-free release")
	}
	tv.HandleMouseRelease(release)
	if !tv.rowEditing || tv.editCol != tv.ColumnByID("size") {
		t.Fatal("release did not flip the cell straight into edit mode")
	}
	tv.HandleKeyPress(core.KeyPressEvent{Key: "Escape"})

	// A drag between press and release never triggers it.
	tv.HandleMousePress(press)
	tv.HandleMouseRelease(core.MouseReleaseEvent{X: sizeX + 40, Y: rowY, Button: core.LeftButton})
	if tv.rowEditing {
		t.Error("dragged release entered edit mode")
	}

	// A click on a NOT-yet-selected row never triggers (it selects).
	tv.SetCurrentIndex(0)
	tv.HandleMousePress(core.MousePressEvent{X: sizeX, Y: 16 + 2*16 + 8, Button: core.LeftButton})
	if tv.clickEditItem != nil {
		t.Error("press on an unselected row became a click-to-edit candidate")
	}
	tv.HandleMouseRelease(core.MouseReleaseEvent{X: sizeX, Y: 16 + 2*16 + 8, Button: core.LeftButton})
	if tv.rowEditing {
		t.Error("selecting click entered edit mode")
	}
	// But the NEXT click on that now-selected row does.
	tv.HandleMousePress(core.MousePressEvent{X: sizeX, Y: 16 + 2*16 + 8, Button: core.LeftButton})
	tv.HandleMouseRelease(core.MouseReleaseEvent{X: sizeX, Y: 16 + 2*16 + 8, Button: core.LeftButton})
	if !tv.rowEditing {
		t.Error("second click on the newly selected row did not enter edit mode")
	}
}
