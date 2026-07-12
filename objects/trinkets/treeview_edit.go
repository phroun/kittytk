package trinkets

import (
	"github.com/phroun/kittytk/core"
)

// In-place row editing for the multi-column TreeView.
//
// Enter on a row (when any visible column is Editable) opens the ROW
// EDITOR: a spun-into-existence TextInput floating over one cell. The
// tree keeps real focus and forwards input (the popup-menu pattern);
// once the editor is dismissed the plain grid is back.
//
//	Enter   commit the row and dismiss
//	Escape  cancel the current cell (original value stays) and dismiss
//	Tab     commit the cell, edit the next editable column (wraps)
//	S-Tab   commit the cell, edit the previous editable column
//	Up/Down commit the cell, move to that row, keep editing the SAME
//	        column there
//	click elsewhere: the value is accepted, then the click proceeds
//
// Re-entering edit mode returns to the column edited last time. When
// NO column is editable, Enter keeps its classic Space behavior
// (expand/collapse or nothing).
//
// A CLICK on a cell of the already-selected row also flips straight
// into edit mode (on a drag-free release).

// treeClickEditSlop is how far the pointer may travel between press
// and release before the click stops counting as a click.
const treeClickEditSlop = core.Unit(4)

// SetOnCellEdited installs the observer for committed cell edits (only
// fired when the value actually changed).
func (t *TreeView) SetOnCellEdited(fn func(item *TreeItem, column *TreeColumn, value string)) {
	t.onCellEdited = fn
}

// RowEditing reports whether the in-place row editor is open.
func (t *TreeView) RowEditing() bool { return t.rowEditing }

// editableColumns returns the VISIBLE editable columns in display
// order - the row editor's Tab order.
func (t *TreeView) editableColumns() []*TreeColumn {
	var out []*TreeColumn
	for _, c := range t.visibleColumns() {
		if c != nil && c.Editable {
			out = append(out, c)
		}
	}
	return out
}

func (t *TreeView) hasEditableColumns() bool { return len(t.editableColumns()) > 0 }

// startRowEdit enters row-edit mode on the current item, resuming the
// last-edited column when it is still available, else the first
// editable one. Returns false when there is nothing to edit.
func (t *TreeView) startRowEdit() bool {
	item := t.CurrentItem()
	if item == nil || !t.multiColumn() || t.rowEditing {
		return false
	}
	cols := t.editableColumns()
	if len(cols) == 0 {
		return false
	}
	col := cols[0]
	for _, c := range cols {
		if c == t.editLastCol {
			col = c
			break
		}
	}
	t.beginCellEdit(item, col)
	return true
}

// beginCellEdit spins the editor into existence over one cell. The
// editor is not parented into the tree (SetParent wants a Container),
// so the display context is handed down the way popup menus do it.
func (t *TreeView) beginCellEdit(item *TreeItem, col *TreeColumn) {
	t.cancelClickEdit()
	ed := NewTextInput()
	cm := t.EffectiveCellMetrics()
	ed.SetCellMetrics(&cm)
	ed.SetFont(t.EffectiveFont())
	ed.SetText(item.Value(col.ID))
	ed.SelectAll()
	// The editor believes it is focused (live caret); the TREE keeps
	// the real focus and forwards input, like the column chooser.
	ed.SetFocus()
	t.editBox = ed
	t.editItem = item
	t.editCol = col
	t.editLastCol = col
	t.editOrig = item.Value(col.ID)
	t.rowEditing = true
	t.ensureEditColVisible()
	t.Update()
}

// ensureEditColVisible scrolls the horizontal column region the
// MINIMUM needed to reveal the edited column (scroll mode only).
// Same conservative rule as the ScrollArea's EnsureRectVisible: no
// movement at all when the cell is already fully in view; otherwise
// align the nearer edge, prioritizing (never hiding) the left edge.
func (t *TreeView) ensureEditColVisible() {
	if t.fitWidth || !t.rowEditing || t.editCol == nil {
		return
	}
	lay := t.columnLayout()
	cw := t.EffectiveCellMetrics().CellWidth
	for _, sp := range lay.spans {
		if sp.col != t.editCol {
			continue
		}
		if sp.fixed {
			return // pinned columns are always in view
		}
		// The span's NATURAL cell offset within the scrolling region
		// (its painted x has the current scroll already applied).
		leftCells := int(lay.scrollL / cw)
		viewCells := int((lay.scrollR - lay.scrollL) / cw)
		start := int(sp.x/cw) - leftCells + t.hScroll
		end := start + int(sp.w/cw)
		hs := t.hScroll
		if start < hs {
			hs = start
		} else if end > hs+viewCells {
			hs = end - viewCells
			if hs > start {
				hs = start // never hide the left edge
			}
		}
		if hs < 0 {
			hs = 0
		}
		if hs > lay.maxHScroll {
			hs = lay.maxHScroll
		}
		if hs != t.hScroll {
			t.hScroll = hs
			t.Update()
		}
		return
	}
}

// commitCellEdit writes the editor's value onto the cell if it changed
// and reports it; the editor stays up.
func (t *TreeView) commitCellEdit() {
	if !t.rowEditing || t.editBox == nil {
		return
	}
	v := t.editBox.Text()
	if v == t.editOrig {
		return
	}
	t.editItem.SetValue(t.editCol.ID, v)
	if t.onCellEdited != nil {
		t.onCellEdited(t.editItem, t.editCol, v)
	}
	// Under an active visual sort the new value can move rows; the
	// trinket re-sorts itself and the selection tracks the item.
	if t.sorted {
		t.resortKeepingSelection()
	}
}

// endRowEdit dismisses the editor. commit=false is Escape: nothing is
// written, the cell keeps its original value.
func (t *TreeView) endRowEdit(commit bool) {
	if !t.rowEditing {
		return
	}
	if commit {
		t.commitCellEdit()
	}
	if t.editBox != nil {
		t.editBox.ClearFocus()
	}
	t.editBox = nil
	t.editItem = nil
	t.editCol = nil
	t.rowEditing = false
	t.editMouseDown = false
	t.Update()
}

// stepEditColumn commits the current cell and moves the editor to the
// next (+1) or previous (-1) editable column, wrapping around.
func (t *TreeView) stepEditColumn(delta int) {
	cols := t.editableColumns()
	if len(cols) == 0 {
		t.endRowEdit(true)
		return
	}
	idx := 0
	for i, c := range cols {
		if c == t.editCol {
			idx = i
			break
		}
	}
	t.commitCellEdit()
	next := cols[(idx+delta+len(cols))%len(cols)]
	t.editCol = next
	t.editLastCol = next
	t.editOrig = t.editItem.Value(next.ID)
	t.editBox.SetText(t.editOrig)
	t.editBox.SelectAll()
	t.ensureEditColVisible()
	t.Update()
}

// stepEditRow accepts the edit, moves the selection up (-1) or down
// (+1), and immediately resumes editing the SAME column on that row.
func (t *TreeView) stepEditRow(delta int) {
	col := t.editCol
	t.endRowEdit(true)
	if ni := t.CurrentIndex() + delta; ni >= 0 && ni < len(t.flatList) {
		t.SetCurrentIndex(ni)
	}
	if it := t.CurrentItem(); it != nil && col != nil {
		t.beginCellEdit(it, col)
	}
}

// handleEditKey routes keys while the row editor is up. Everything is
// consumed: navigation belongs to the editor session, the rest belongs
// to the text box.
func (t *TreeView) handleEditKey(event core.KeyPressEvent) bool {
	if !t.rowEditing || t.editBox == nil {
		return false
	}
	switch {
	case event.Key == "Enter":
		t.endRowEdit(true)
	case event.Key == "Escape":
		t.endRowEdit(false)
	case isShiftTab(event):
		t.stepEditColumn(-1)
	case event.Key == "Tab":
		t.stepEditColumn(1)
	case event.Key == "Up":
		t.stepEditRow(-1)
	case event.Key == "Down":
		t.stepEditRow(1)
	default:
		t.editBox.HandleKeyPress(event)
	}
	return true
}

// editorRect returns the edited cell's rect in tree-local units.
// ok=false when the cell is currently invisible (scrolled away or the
// column was hidden mid-edit) - the edit stays alive, just unpainted.
func (t *TreeView) editorRect() (core.UnitRect, bool) {
	if !t.rowEditing || t.editCol == nil || t.editItem == nil {
		return core.UnitRect{}, false
	}
	idx := -1
	for i, it := range t.flatList {
		if it == t.editItem {
			idx = i
			break
		}
	}
	row := idx - t.scrollOffset
	if idx < 0 || row < 0 || row >= t.visibleCount() {
		return core.UnitRect{}, false
	}
	metrics := t.EffectiveCellMetrics()
	lay := t.columnLayout()
	for _, sp := range lay.spans {
		if sp.col != t.editCol {
			continue
		}
		clip, ok := lay.spanClip(sp, metrics.CellHeight)
		if !ok {
			return core.UnitRect{}, false
		}
		y := lay.headerH + core.Unit(row)*metrics.CellHeight
		return core.UnitRect{X: clip.X, Y: y, Width: clip.Width, Height: metrics.CellHeight}, true
	}
	return core.UnitRect{}, false
}

// paintRowEditor paints the live cell editor over the grid (called at
// the end of paintMulti, above everything).
func (t *TreeView) paintRowEditor(p *core.Painter) {
	if !t.rowEditing || t.editBox == nil {
		return
	}
	r, ok := t.editorRect()
	if !ok {
		return
	}
	t.editBox.SetBounds(core.UnitRect{Width: r.Width, Height: r.Height})
	t.editBox.Paint(p.WithOffset(r.X, r.Y))
}

// handleEditMousePress routes a press while editing: inside the editor
// it goes to the text box (caret/selection); anywhere else ACCEPTS the
// value and lets the press proceed normally. Returns handled.
func (t *TreeView) handleEditMousePress(event core.MousePressEvent) bool {
	if !t.rowEditing || t.editBox == nil {
		return false
	}
	if r, ok := t.editorRect(); ok {
		pt := core.UnitPoint{X: event.X, Y: event.Y}
		if pt.X >= r.X && pt.X < r.X+r.Width && pt.Y >= r.Y && pt.Y < r.Y+r.Height {
			ev := event
			ev.X -= r.X
			ev.Y -= r.Y
			t.editMouseDown = true
			t.editBox.HandleMousePress(ev)
			return true
		}
	}
	t.endRowEdit(true) // clicking off accepts; the click falls through
	return false
}

// handleEditMouseMove forwards drags that started inside the editor
// (text selection). Returns handled.
func (t *TreeView) handleEditMouseMove(event core.MouseMoveEvent) bool {
	if !t.rowEditing || !t.editMouseDown || t.editBox == nil {
		return false
	}
	if r, ok := t.editorRect(); ok {
		ev := event
		ev.X -= r.X
		ev.Y -= r.Y
		t.editBox.HandleMouseMove(ev)
	}
	return true
}

// handleEditMouseRelease completes an editor-internal drag. Returns
// handled.
func (t *TreeView) handleEditMouseRelease(event core.MouseReleaseEvent) bool {
	if !t.editMouseDown {
		return false
	}
	t.editMouseDown = false
	if t.rowEditing && t.editBox != nil {
		if r, ok := t.editorRect(); ok {
			ev := event
			ev.X -= r.X
			ev.Y -= r.Y
			t.editBox.HandleMouseRelease(ev)
		}
	}
	return true
}

// --- click-to-edit (a click on the already-selected row) ---

// noteClickEditPress records, at press time, whether this click landed
// on an editable cell of the ALREADY selected row.
func (t *TreeView) noteClickEditPress(event core.MousePressEvent) {
	t.clickEditItem = nil
	if !t.multiColumn() || t.rowEditing {
		return
	}
	headerH := t.headerHeight()
	if event.Y < headerH {
		return
	}
	metrics := t.EffectiveCellMetrics()
	row := t.scrollOffset + int((event.Y-headerH)/metrics.CellHeight)
	if row != t.currentIndex || row < 0 || row >= len(t.flatList) {
		return
	}
	lay := t.columnLayout()
	for _, sp := range lay.spans {
		if sp.col == nil || !sp.col.Editable {
			continue
		}
		clip, ok := lay.spanClip(sp, metrics.CellHeight)
		if !ok || event.X < clip.X || event.X >= clip.X+clip.Width {
			continue
		}
		t.clickEditItem = t.flatList[row]
		t.clickEditCol = sp.col
		t.clickEditX, t.clickEditY = event.X, event.Y
		return
	}
}

// armClickEdit begins the edit IMMEDIATELY on a drag-free release over
// the press-time candidate - the second click on an already-selected
// row flips straight into edit mode, no double-click settle delay.
func (t *TreeView) armClickEdit(event core.MouseReleaseEvent) {
	if t.clickEditItem == nil {
		return
	}
	item, col := t.clickEditItem, t.clickEditCol
	t.clickEditItem = nil
	dx, dy := event.X-t.clickEditX, event.Y-t.clickEditY
	if dx < 0 {
		dx = -dx
	}
	if dy < 0 {
		dy = -dy
	}
	if dx > treeClickEditSlop || dy > treeClickEditSlop {
		return // a drag, not a click
	}
	if t.rowEditing || t.CurrentItem() != item || col.Hidden || !col.Editable {
		return
	}
	t.beginCellEdit(item, col)
}

// cancelClickEdit drops any press-time click-to-edit candidate.
func (t *TreeView) cancelClickEdit() {
	t.clickEditItem = nil
}
