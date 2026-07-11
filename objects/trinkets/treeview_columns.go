package trinkets

import (
	"fmt"

	"github.com/phroun/kittytk/core"
	"github.com/phroun/kittytk/style"
)

// Multi-column ("details" / "as List") support for TreeView. The tree
// itself - indent, arrows, icons, captions - is the KEY column; data
// columns render one cell value per item beside it. A header row
// (optional) shows captions, draggable dividers resize, and a [=]
// button above the scrollbar drops a show/hide checklist of the
// optional columns. The same layout drives the TUI (cell dividers,
// '=' button) and the pixel path (hairline dividers) - Finder's
// list view / Explorer's Details mode.

// TreeColumn describes one data column.
type TreeColumn struct {
	// ID is the stable key cell values are stored under (TreeItem.SetValue)
	// and the wire addresses.
	ID      string
	Caption string

	// Width is the current width in text cells; Min/MaxWidth bound
	// drag-resizing (MaxWidth 0 = unbounded).
	Width    int
	MinWidth int
	MaxWidth int

	// Align is "left" (default), "center", or "right".
	Align string

	// Resizable allows drag-resizing via the header divider.
	Resizable bool
	// Hidden removes the column from display (toggled by the chooser).
	Hidden bool
	// Optional lists the column in the [=] show/hide chooser.
	Optional bool
	// Sortable makes the header caption clickable: clicks toggle the
	// view's sort indicator on this column and fire the sort-request
	// callback. The VIEW only indicates and requests - reordering the
	// items is the application's job (it owns the data).
	Sortable bool
}

// NewTreeColumn creates a column with sensible defaults (resizable,
// optional, left-aligned, min width 3).
func NewTreeColumn(id, caption string, width int) *TreeColumn {
	if width < 1 {
		width = 8
	}
	return &TreeColumn{
		ID: id, Caption: caption, Width: width,
		MinWidth: 3, Align: "left", Resizable: true, Optional: true,
	}
}

// clampWidth bounds w to the column's Min/MaxWidth.
func (c *TreeColumn) clampWidth(w int) int {
	if w < c.MinWidth {
		w = c.MinWidth
	}
	if c.MaxWidth > 0 && w > c.MaxWidth {
		w = c.MaxWidth
	}
	if w < 1 {
		w = 1
	}
	return w
}

// --- TreeItem cell values ---

// SetValue sets this item's cell text for the given column ID.
func (t *TreeItem) SetValue(columnID, text string) {
	if t.Values == nil {
		t.Values = make(map[string]string)
	}
	t.Values[columnID] = text
}

// Value returns this item's cell text for the given column ID.
func (t *TreeItem) Value(columnID string) string {
	return t.Values[columnID]
}

// --- TreeView column API ---

// AddColumn appends a data column.
func (t *TreeView) AddColumn(c *TreeColumn) {
	t.columns = append(t.columns, c)
	t.Update()
}

// Columns returns the data columns (declared order, including hidden).
func (t *TreeView) Columns() []*TreeColumn { return t.columns }

// ColumnByID finds a column by its stable ID (nil if absent).
func (t *TreeView) ColumnByID(id string) *TreeColumn {
	for _, c := range t.columns {
		if c.ID == id {
			return c
		}
	}
	return nil
}

// SetShowHeader shows or hides the header row (captions, dividers,
// the [=] chooser button).
func (t *TreeView) SetShowHeader(on bool) { t.showHeader = on; t.Update() }

// SetOnSortRequested installs the sort-request callback: a click on a
// sortable column's header reports the requested sort here (the
// application reorders its items and updates the tree). sortedBy is
// -1 for the key column, else the declared data-column index.
func (t *TreeView) SetOnSortRequested(fn func(sortedBy int, descending bool)) {
	t.onSortRequested = fn
}

// Sorted returns the view's sort state: whether a sort indicator is
// shown, which column it is on (-1 = the key column, else a declared
// data-column index), and the direction.
func (t *TreeView) Sorted() (sorted bool, sortedBy int, descending bool) {
	return t.sorted, t.sortedBy, t.sortDescending
}

// SetSorted sets the sort state for header display. Display-only: it
// does not reorder items (the application owns the data).
func (t *TreeView) SetSorted(sorted bool, sortedBy int, descending bool) {
	t.sorted, t.sortedBy, t.sortDescending = sorted, sortedBy, descending
	t.Update()
}

// columnIndex returns c's declared index (-1 for nil = the key column).
func (t *TreeView) columnIndex(c *TreeColumn) int {
	if c == nil {
		return -1
	}
	for i, tc := range t.columns {
		if tc == c {
			return i
		}
	}
	return -1
}

// sortIndicatorFor reports whether the indicator sits on this span's
// column (nil col = the key column).
func (t *TreeView) sortIndicatorFor(col *TreeColumn) bool {
	return t.sorted && t.sortedBy == t.columnIndex(col)
}

// headerSortClick handles a press on a column caption: the key column
// is always sortable; data columns opt in via Sortable. Clicking the
// active column toggles direction, a new column starts ascending.
func (t *TreeView) headerSortClick(col *TreeColumn) {
	if col != nil && !col.Sortable {
		return
	}
	by := t.columnIndex(col)
	descending := false
	if t.sorted && t.sortedBy == by {
		descending = !t.sortDescending
	}
	t.SetSorted(true, by, descending)
	if t.onSortRequested != nil {
		t.onSortRequested(by, descending)
	}
}

// SetShowKey controls whether the tree (key) column - the original
// single-column tree - is shown as the first visible column.
func (t *TreeView) SetShowKey(on bool) { t.showKey = on; t.Update() }

// SetKeyCaption sets the header caption over the key (tree) column -
// "Name" in a file listing.
func (t *TreeView) SetKeyCaption(s string) { t.keyCaption = s; t.Update() }

// KeyCaption returns the key column's header caption.
func (t *TreeView) KeyCaption() string { return t.keyCaption }

// SetFitWidth selects fit mode: true squeezes columns into the width
// (no horizontal scrolling, the key column absorbs slack); false uses
// natural widths and scrolls horizontally as needed.
func (t *TreeView) SetFitWidth(on bool) {
	t.fitWidth = on
	if on {
		t.hScroll = 0
	}
	t.Update()
}

// SetFixedColumns pins the first left / last right visible columns
// outside the horizontal scrolling region.
func (t *TreeView) SetFixedColumns(left, right int) {
	if left < 0 {
		left = 0
	}
	if right < 0 {
		right = 0
	}
	t.fixedLeft, t.fixedRight = left, right
	t.Update()
}

// SetKeyWidth sets the tree column's width in text cells for scroll
// mode (fit mode sizes it to the leftover space automatically).
func (t *TreeView) SetKeyWidth(cells int) {
	if cells < treeKeyMinCells {
		cells = treeKeyMinCells
	}
	t.keyWidth = cells
	t.Update()
}

const (
	treeKeyMinCells     = 6  // narrowest useful tree column
	treeKeyDefaultCells = 20 // scroll-mode default tree column width
)

// multiColumn reports whether the multi-column presentation is active
// (any data column declared, or an explicit header request).
func (t *TreeView) multiColumn() bool {
	return len(t.columns) > 0 || t.showHeader
}

// headerHeight is the header row's height (0 when hidden).
func (t *TreeView) headerHeight() core.Unit {
	if !t.multiColumn() || !t.showHeader {
		return 0
	}
	return t.EffectiveCellMetrics().CellHeight
}

// footerHeight is the horizontal scrollbar row's height: the bottom
// display row is reserved whenever horizontal scrolling is enabled
// (scroll mode), so content never sits under the bar.
func (t *TreeView) footerHeight() core.Unit {
	if !t.multiColumn() || t.fitWidth {
		return 0
	}
	return t.EffectiveCellMetrics().CellHeight
}

// colSpan is one visible column's placement for this paint/hit pass.
type colSpan struct {
	col   *TreeColumn // nil = the tree (key) column
	x     core.Unit   // content left edge (post-scroll)
	w     core.Unit   // content width
	divX  core.Unit   // divider column right of the span (-1 = none)
	fixed bool        // pinned outside the horizontal scroll region
}

// treeColLayout is the computed column geometry for one pass.
type treeColLayout struct {
	spans      []colSpan
	headerH    core.Unit
	contentW   core.Unit // width left of the scrollbar lane
	scrollL    core.Unit // horizontal scroll region [scrollL, scrollR)
	scrollR    core.Unit
	maxHScroll int // in cells
}

// visibleColumns returns the visible sequence: key column (nil entry)
// first when showKey, then unhidden data columns in declared order.
// A tree with no visible data columns always shows the key column.
func (t *TreeView) visibleColumns() []*TreeColumn {
	var seq []*TreeColumn
	if t.showKey || !t.anyVisibleData() {
		seq = append(seq, nil)
	}
	for _, c := range t.columns {
		if !c.Hidden {
			seq = append(seq, c)
		}
	}
	return seq
}

func (t *TreeView) anyVisibleData() bool {
	for _, c := range t.columns {
		if !c.Hidden {
			return true
		}
	}
	return false
}

// columnLayout computes the visible spans. Widths are cell-quantized
// (the TUI's natural grid; the pixel path shares it so dividers land
// identically on both). Dividers occupy one cell between spans. In
// fit mode the key column absorbs slack and data columns shrink
// toward MinWidth on overflow; in scroll mode natural widths stand
// and the non-fixed spans pan by hScroll cells.
func (t *TreeView) columnLayout() treeColLayout {
	metrics := t.EffectiveCellMetrics()
	cw := metrics.CellWidth
	bounds := t.Bounds()
	lay := treeColLayout{headerH: t.headerHeight()}
	lay.contentW = bounds.Width - cw // scrollbar lane
	if lay.contentW < cw {
		lay.contentW = cw
	}
	contentCells := int(lay.contentW / cw)

	seq := t.visibleColumns()
	n := len(seq)
	widths := make([]int, n) // cells
	keyIdx := -1
	for i, c := range seq {
		if c == nil {
			keyIdx = i
			continue
		}
		widths[i] = c.clampWidth(c.Width)
	}
	dividers := n - 1
	if dividers < 0 {
		dividers = 0
	}

	if keyIdx >= 0 {
		if t.fitWidth {
			used := dividers
			for i, w := range widths {
				if i != keyIdx {
					used += w
				}
			}
			keyW := contentCells - used
			if keyW < treeKeyMinCells {
				// Shrink data columns (rightmost first) toward MinWidth
				// to make room for a usable key column.
				need := treeKeyMinCells - keyW
				for i := n - 1; i >= 0 && need > 0; i-- {
					if i == keyIdx {
						continue
					}
					c := seq[i]
					give := widths[i] - c.MinWidth
					if give > need {
						give = need
					}
					if give > 0 {
						widths[i] -= give
						need -= give
					}
				}
				keyW = treeKeyMinCells
			}
			widths[keyIdx] = keyW
		} else {
			kw := t.keyWidth
			if kw <= 0 {
				kw = treeKeyDefaultCells
			}
			widths[keyIdx] = kw
		}
	} else if t.fitWidth {
		// No key column: shrink data columns on overflow the same way.
		used := dividers
		for _, w := range widths {
			used += w
		}
		for i := n - 1; i >= 0 && used > contentCells; i-- {
			give := widths[i] - seq[i].MinWidth
			if over := used - contentCells; give > over {
				give = over
			}
			if give > 0 {
				widths[i] -= give
				used -= give
			}
		}
	}

	// Fixed pinning: clamp counts, and in fit mode everything is fixed.
	fl, fr := t.fixedLeft, t.fixedRight
	if fl+fr > n {
		fl = n
		fr = 0
	}

	// Widths of the pinned flanks (with their trailing/leading dividers).
	leftCells := 0
	for i := 0; i < fl; i++ {
		leftCells += widths[i] + 1 // span + divider
	}
	rightCells := 0
	for i := n - fr; i < n; i++ {
		rightCells += 1 + widths[i] // divider + span
	}
	scrollCells := contentCells - leftCells - rightCells
	if scrollCells < 0 {
		scrollCells = 0
	}
	lay.scrollL = core.Unit(leftCells) * cw
	lay.scrollR = lay.scrollL + core.Unit(scrollCells)*cw

	// Natural width of the scrolling region's spans.
	midCells := 0
	for i := fl; i < n-fr; i++ {
		midCells += widths[i]
		if i < n-fr-1 {
			midCells++ // divider between scrolling spans
		}
	}
	if !t.fitWidth && midCells > scrollCells {
		lay.maxHScroll = midCells - scrollCells
	}
	hs := t.hScroll
	if hs > lay.maxHScroll {
		hs = lay.maxHScroll
	}
	if hs < 0 {
		hs = 0
	}

	// Emit spans left to right.
	xCells := 0
	for i := 0; i < n; i++ {
		fixed := i < fl || i >= n-fr
		if i == fl && !fixed || (i == fl && fl < n-fr) {
			// entering the scrolling region
			xCells = leftCells - hs
		}
		if i == n-fr && fr > 0 {
			xCells = contentCells - rightCells + 1 // after the divider
			// The divider left of the right flank sits at its fixed spot;
			// record it on the previous span below via divX handling.
		}
		sp := colSpan{
			col:   seq[i],
			x:     core.Unit(xCells) * cw,
			w:     core.Unit(widths[i]) * cw,
			fixed: fixed,
			divX:  -1,
		}
		xCells += widths[i]
		if i < n-1 {
			sp.divX = core.Unit(xCells) * cw
			xCells++
		}
		lay.spans = append(lay.spans, sp)
	}
	// Pin the divider between the scroll region and the right flank.
	if fr > 0 && n-fr-1 >= 0 {
		lay.spans[n-fr-1].divX = lay.scrollR
	}
	return lay
}

// spanClip returns the clip rect for a span (fixed spans clip to
// themselves; scrolling spans additionally clip to the scroll region).
func (l *treeColLayout) spanClip(sp colSpan, height core.Unit) (core.UnitRect, bool) {
	x0, x1 := sp.x, sp.x+sp.w
	if !sp.fixed {
		if x0 < l.scrollL {
			x0 = l.scrollL
		}
		if x1 > l.scrollR {
			x1 = l.scrollR
		}
	}
	if x1 > l.contentW {
		x1 = l.contentW
	}
	if x0 < 0 {
		x0 = 0
	}
	if x1 <= x0 {
		return core.UnitRect{}, false
	}
	return core.UnitRect{X: x0, Y: 0, Width: x1 - x0, Height: height}, true
}

// divVisible reports whether a divider at divX should paint (dividers
// belonging to the scrolling region hide once panned outside it).
func (l *treeColLayout) divVisible(sp colSpan) bool {
	if sp.divX < 0 {
		return false
	}
	if sp.fixed || sp.divX == l.scrollR {
		return sp.divX < l.contentW
	}
	return sp.divX >= l.scrollL && sp.divX < l.scrollR
}

// --- painting ---

// paintMulti renders the multi-column presentation: header, per-column
// rows, dividers, chooser button, scrollbar.
func (t *TreeView) paintMulti(p *core.Painter) {
	bounds := t.Bounds()
	scheme := t.GetScheme()
	focused := t.HasFocus()
	metrics := t.EffectiveCellMetrics()
	font := t.EffectiveFont()
	lay := t.columnLayout()

	bgStyle := style.DefaultStyle().WithFg(scheme.GetListFG()).WithBg(scheme.GetListBG())
	p.FillRect(core.UnitRect{Width: bounds.Width, Height: bounds.Height}, ' ', bgStyle)

	// Header row.
	headerStyle := bgStyle
	if lay.headerH > 0 {
		if !p.Graphical() {
			headerStyle = headerStyle.Underline()
		}
		p.FillRect(core.UnitRect{Width: bounds.Width, Height: lay.headerH}, ' ', headerStyle)
		for _, sp := range lay.spans {
			clip, ok := lay.spanClip(sp, lay.headerH)
			if !ok {
				continue
			}
			caption := t.keyCaption // the key column's header caption
			if sp.col != nil {
				caption = sp.col.Caption
			}
			cp := p.WithClip(clip)
			// Sort indicator on the active sort column - key included.
			// Same glyph family as the tree's expander ('▼' and its
			// inverse), right-aligned in the span so it reads as the
			// header's affordance, not part of the caption.
			if t.sortIndicatorFor(sp.col) {
				arrow := "▲"
				if t.sortDescending {
					arrow = "▼"
				}
				t.drawAligned(cp, arrow, sp, 0, headerStyle, font, "right")
				// Keep the caption clear of the arrow.
				capSp := sp
				if room := font.MeasureText(arrow) + metrics.CellWidth; capSp.w > room {
					capSp.w -= room
				}
				t.drawAligned(cp, caption, capSp, 0, headerStyle, font, "left")
			} else {
				t.drawAligned(cp, caption, sp, 0, headerStyle, font, "left")
			}
		}
		if p.Graphical() {
			// Hairline under the header (the TUI uses the underline attr).
			fr, fg, fb := scheme.GetListFG().RGBComponents()
			p.FillRectPixelsAlpha(0, lay.headerH, 0, -1,
				p.UnitSpanPxX(0, bounds.Width), 1, fr, fg, fb, 0.6)
		}
		t.paintChooserButton(p, lay, headerStyle)
	}

	// Rows.
	visibleCount := t.visibleCount()
	for i := 0; i < visibleCount; i++ {
		itemIndex := t.scrollOffset + i
		if itemIndex >= len(t.flatList) {
			break
		}
		item := t.flatList[itemIndex]
		itemY := lay.headerH + core.Unit(i)*metrics.CellHeight

		var s style.CellStyle
		switch {
		case !item.Enabled:
			s = style.DefaultStyle().WithFg(scheme.GetDisabledTextFG()).WithBg(scheme.GetListBG())
		case itemIndex == t.currentIndex && focused:
			s = scheme.GetFocusedListItem()
		case itemIndex == t.currentIndex:
			s = scheme.GetSelectedListItem()
		default:
			s = bgStyle
		}
		if itemIndex == t.currentIndex {
			// Selection spans the full row, Finder/Explorer style.
			p.FillRect(core.UnitRect{X: 0, Y: itemY, Width: lay.contentW, Height: metrics.CellHeight}, ' ', s)
		}

		for _, sp := range lay.spans {
			clip, ok := lay.spanClip(sp, bounds.Height)
			if !ok {
				continue
			}
			cp := p.WithClip(clip)
			if sp.col == nil {
				t.paintKeyCell(cp, item, sp, itemY, s, metrics, font)
			} else {
				t.drawAligned(cp, item.Value(sp.col.ID), sp, itemY, s, font, sp.col.Align)
			}
		}
	}

	// Dividers, over the rows, down to the footer row (the reserved
	// horizontal-scrollbar row stays clear).
	divBottom := bounds.Height - t.footerHeight()
	divStyle := style.DefaultStyle().WithFg(scheme.GetScrollbar().Fg).WithBg(scheme.GetListBG())
	for _, sp := range lay.spans {
		if !lay.divVisible(sp) {
			continue
		}
		if p.Graphical() {
			fr, fg, fb := scheme.GetListFG().RGBComponents()
			p.FillRectPixelsAlpha(sp.divX+metrics.CellWidth/2, 0, 0, 0,
				1, p.UnitSpanPxY(0, divBottom), fr, fg, fb, 0.35)
		} else {
			for y := core.Unit(0); y < divBottom; y += metrics.CellHeight {
				st := divStyle
				if y < lay.headerH {
					st = st.Underline()
				}
				p.DrawCell(sp.divX, y, '│', st)
			}
		}
	}

	if t.footerHeight() > 0 {
		t.paintHScrollbar(p, lay)
	}
	if len(t.flatList) > visibleCount {
		t.paintScrollbar(p, visibleCount)
	}
}

// paintKeyCell draws the tree column's content for one row: indent,
// expander, icon, caption (the original single-column rendering,
// constrained to the span).
func (t *TreeView) paintKeyCell(p *core.Painter, item *TreeItem, sp colSpan, itemY core.Unit, s style.CellStyle, metrics core.CellMetrics, font *core.Font) {
	level := item.Level()
	x := sp.x + core.Unit(level*t.indentWidth)*metrics.CellWidth
	if !item.IsLeaf() {
		if item.Expanded {
			p.DrawCell(x, itemY, '▼', s)
		} else {
			p.DrawCell(x, itemY, '▸', s)
		}
	}
	x += metrics.CellWidth
	if item.Icon != nil && len(item.Icon.Cells) > 0 {
		cell := item.Icon.Cells[0]
		p.DrawCell(x, itemY, cell.Char, cell.Style)
		x += metrics.CellWidth * 2
	}
	avail := sp.x + sp.w - x
	text := item.Text
	for font.MeasureText(text) > avail && len(text) > 0 {
		text = text[:len(text)-1]
	}
	p.DrawText(x, itemY, text, s, font)
}

// drawAligned draws one cell value inside a span with the column's
// alignment, truncated to fit.
func (t *TreeView) drawAligned(p *core.Painter, text string, sp colSpan, y core.Unit, s style.CellStyle, font *core.Font, align string) {
	metrics := t.EffectiveCellMetrics()
	pad := metrics.CellWidth / 2
	avail := sp.w - pad
	if avail < 0 {
		avail = 0
	}
	for font.MeasureText(text) > avail && len(text) > 0 {
		text = text[:len(text)-1]
	}
	tw := font.MeasureText(text)
	x := sp.x
	switch align {
	case "right":
		x = sp.x + sp.w - tw - pad/2
	case "center":
		x = sp.x + (sp.w-tw)/2
	}
	if x < sp.x {
		x = sp.x
	}
	p.DrawText(x, y, text, s, font)
}

// chooserButtonRect is the [=] column-chooser button: the scrollbar
// lane's header cell (upper-right corner, above the scrollbar).
func (t *TreeView) chooserButtonRect() (core.UnitRect, bool) {
	if t.headerHeight() == 0 || len(t.optionalColumns()) == 0 {
		return core.UnitRect{}, false
	}
	metrics := t.EffectiveCellMetrics()
	return core.UnitRect{
		X: t.Bounds().Width - metrics.CellWidth, Y: 0,
		Width: metrics.CellWidth, Height: t.headerHeight(),
	}, true
}

func (t *TreeView) paintChooserButton(p *core.Painter, lay treeColLayout, headerStyle style.CellStyle) {
	r, ok := t.chooserButtonRect()
	if !ok {
		return
	}
	if p.Graphical() {
		// Three short lines - a crisp "≡" at any pixel size.
		lineW := r.Width * 3 / 5
		x := r.X + (r.Width-lineW)/2
		fr, fg, fb := t.GetScheme().GetListFG().RGBComponents()
		wPx := p.UnitSpanPxX(x, x+lineW)
		gapPx := p.UnitsToPx(r.Height) / 5
		if gapPx < 2 {
			gapPx = 2
		}
		for i := 0; i < 3; i++ {
			p.FillRectPixelsAlpha(x, r.Y+r.Height/2, 0, (i-1)*gapPx,
				wPx, 1, fr, fg, fb, 1)
		}
		return
	}
	// TUI: ASCII '=' per the project's text-mode conventions.
	p.DrawCell(r.X, r.Y, '=', headerStyle)
}

// optionalColumns lists the columns the chooser can toggle.
func (t *TreeView) optionalColumns() []*TreeColumn {
	var out []*TreeColumn
	for _, c := range t.columns {
		if c.Optional {
			out = append(out, c)
		}
	}
	return out
}

// --- interactions ---

// findTreePopupController resolves the popup host: the view's own
// controller field, else the nearest ancestor's (the same walk the
// text input's context menu uses - inside an MDI child only an
// ancestor carries the controller).
func (t *TreeView) findTreePopupController() core.PopupController {
	if pc := t.PopupController(); pc != nil {
		return pc
	}
	for current := t.Parent(); current != nil; {
		trinket, ok := current.(core.Trinket)
		if !ok {
			break
		}
		if getter, ok := trinket.(interface {
			PopupController() core.PopupController
		}); ok {
			if pc := getter.PopupController(); pc != nil {
				return pc
			}
		}
		current = trinket.Parent()
	}
	return nil
}

func (t *TreeView) chooserPopupID() string {
	return fmt.Sprintf("treeview-columns-%d", t.ObjectID())
}

// openColumnChooser drops the [=] checklist: one row per optional
// column, checked = visible; clicking toggles. Click-away dismisses
// (unhandled press falls through to the controller's dismiss).
func (t *TreeView) openColumnChooser() {
	pc := t.findTreePopupController()
	if pc == nil {
		return
	}
	cols := t.optionalColumns()
	if len(cols) == 0 {
		return
	}
	metrics := t.EffectiveCellMetrics()
	font := t.EffectiveFont()
	rowH := metrics.CellHeight
	w := core.Unit(0)
	for _, c := range cols {
		if cw := font.MeasureText(c.Caption); cw > w {
			w = cw
		}
	}
	w += metrics.TextWidth(4) // "[x] " gutter + padding
	h := rowH * core.Unit(len(cols))

	btn, _ := t.chooserButtonRect()
	at := pc.MapToScreen(t.Self(), core.UnitPoint{X: btn.X, Y: btn.Y + btn.Height})
	screen := pc.ScreenBounds()
	if at.X+w > screen.X+screen.Width {
		at.X = screen.X + screen.Width - w
	}
	if at.Y+h > screen.Y+screen.Height {
		at.Y = screen.Y + screen.Height - h
	}
	bounds := core.UnitRect{X: at.X, Y: at.Y, Width: w, Height: h}
	scheme := t.GetScheme()

	pc.RegisterPopup(&core.PopupRequest{
		ID:     t.chooserPopupID(),
		Bounds: bounds,
		Paint: func(p *core.Painter) {
			bg := style.DefaultStyle().WithFg(scheme.GetListFG()).WithBg(scheme.GetListBG())
			p.FillRect(bounds, ' ', bg)
			for i, c := range cols {
				y := bounds.Y + rowH*core.Unit(i)
				mark := "[ ] "
				if !c.Hidden {
					mark = "[x] "
				}
				p.DrawText(bounds.X+metrics.CellWidth/2, y, mark+c.Caption, bg, font)
			}
		},
		HandleMousePress: func(ev core.MousePressEvent) bool {
			pt := core.UnitPoint{X: ev.X, Y: ev.Y}
			if !bounds.Contains(pt) {
				pc.UnregisterPopup(t.chooserPopupID())
				return false
			}
			i := int((ev.Y - bounds.Y) / rowH)
			if i >= 0 && i < len(cols) {
				cols[i].Hidden = !cols[i].Hidden
				t.Update()
			}
			return true
		},
		HandleMouseMove:    func(core.MouseMoveEvent) bool { return true },
		HandleMouseRelease: func(core.MouseReleaseEvent) bool { return true },
	})
}

// dividerAt returns the column resized by a drag starting at header
// position x: the span left of the divider cell containing x.
func (t *TreeView) dividerAt(x core.Unit, lay treeColLayout) (*TreeColumn, int, bool) {
	cw := t.EffectiveCellMetrics().CellWidth
	for i, sp := range lay.spans {
		if !lay.divVisible(sp) {
			continue
		}
		if x >= sp.divX && x < sp.divX+cw {
			if sp.col == nil {
				// The key column: resizable in scroll mode only (fit
				// mode sizes it automatically from the leftover).
				if t.fitWidth {
					return nil, 0, false
				}
				kw := t.keyWidth
				if kw <= 0 {
					kw = treeKeyDefaultCells
				}
				return nil, kw, true
			}
			if !sp.col.Resizable {
				return nil, 0, false
			}
			return sp.col, sp.col.clampWidth(sp.col.Width), true
		}
		_ = i
	}
	return nil, 0, false
}

// handleMultiPress handles presses in the header band (chooser button,
// divider drags). Returns handled.
func (t *TreeView) handleMultiPress(event core.MousePressEvent) bool {
	if !t.multiColumn() {
		return false
	}
	headerH := t.headerHeight()
	if headerH == 0 || event.Y >= headerH {
		return false
	}
	if r, ok := t.chooserButtonRect(); ok && event.X >= r.X && event.X < r.X+r.Width {
		t.openColumnChooser()
		return true
	}
	lay := t.columnLayout()
	if col, startW, ok := t.dividerAt(event.X, lay); ok {
		t.colDragging = true
		t.colDragCol = col
		t.colDragStartX = event.X
		t.colDragStartW = startW
		return true
	}
	// A click on a column caption requests a sort (the key column is
	// always sortable; data columns opt in via Sortable).
	for _, sp := range lay.spans {
		clip, ok := lay.spanClip(sp, lay.headerH)
		if !ok {
			continue
		}
		if event.X >= clip.X && event.X < clip.X+clip.Width {
			t.headerSortClick(sp.col)
			break
		}
	}
	return true // header clicks never fall through to the rows
}

// handleMultiMove continues a divider drag. Returns handled.
func (t *TreeView) handleMultiMove(event core.MouseMoveEvent) bool {
	if !t.colDragging {
		return false
	}
	cw := t.EffectiveCellMetrics().CellWidth
	deltaCells := int((event.X - t.colDragStartX) / cw)
	w := t.colDragStartW + deltaCells
	if t.colDragCol != nil {
		if nw := t.colDragCol.clampWidth(w); nw != t.colDragCol.Width {
			t.colDragCol.Width = nw
			t.Update()
		}
	} else {
		if w < treeKeyMinCells {
			w = treeKeyMinCells
		}
		if w != t.keyWidth {
			t.keyWidth = w
			t.Update()
		}
	}
	return true
}

// handleMultiRelease ends a divider or footer-scrollbar drag.
// Returns handled.
func (t *TreeView) handleMultiRelease() bool {
	handled := false
	if t.colDragging {
		t.colDragging = false
		t.colDragCol = nil
		handled = true
	}
	if t.hbarDragging {
		t.hbarDragging = false
		handled = true
	}
	return handled
}

// hScrollbarGeometry returns the footer horizontal scrollbar's track
// and thumb in units. The track spans the scrolling region; thumb
// length is proportional to the visible share of the scrollable
// content. ok=false when there is nothing to scroll.
func (t *TreeView) hScrollbarGeometry(lay treeColLayout) (trackX0, trackX1, thumbX0, thumbX1 core.Unit, ok bool) {
	if t.footerHeight() == 0 || lay.maxHScroll <= 0 {
		return 0, 0, 0, 0, false
	}
	cw := t.EffectiveCellMetrics().CellWidth
	trackX0, trackX1 = lay.scrollL, lay.scrollR
	trackCells := int((trackX1 - trackX0) / cw)
	if trackCells <= 0 {
		return 0, 0, 0, 0, false
	}
	totalCells := trackCells + lay.maxHScroll
	thumbCells := trackCells * trackCells / totalCells
	if thumbCells < 1 {
		thumbCells = 1
	}
	scrollable := trackCells - thumbCells
	pos := 0
	if scrollable > 0 {
		pos = t.hScroll * scrollable / lay.maxHScroll
		if t.hScroll > 0 && pos == 0 {
			pos = 1
		}
		if t.hScroll < lay.maxHScroll && pos >= scrollable {
			pos = scrollable - 1
		}
		if pos < 0 {
			pos = 0
		}
	}
	thumbX0 = trackX0 + core.Unit(pos)*cw
	thumbX1 = thumbX0 + core.Unit(thumbCells)*cw
	return trackX0, trackX1, thumbX0, thumbX1, true
}

// paintHScrollbar renders the reserved footer row's horizontal
// scrollbar across the scrolling region.
func (t *TreeView) paintHScrollbar(p *core.Painter, lay treeColLayout) {
	trackX0, trackX1, thumbX0, thumbX1, ok := t.hScrollbarGeometry(lay)
	if !ok {
		return
	}
	scheme := t.GetScheme()
	metrics := t.EffectiveCellMetrics()
	y := t.Bounds().Height - metrics.CellHeight
	trackStyle := scheme.GetScrollbar()
	thumbStyle := scheme.GetScrollbarThumbState(false)

	if p.Graphical() {
		stripeY := y + metrics.CellHeight/2
		p.FillRect(core.UnitRect{X: trackX0, Y: stripeY, Width: trackX1 - trackX0, Height: 1},
			'▒', trackStyle.WithBg(style.ColorTransparent))
		p.FillRect(core.UnitRect{X: thumbX0, Y: y + 1, Width: thumbX1 - thumbX0, Height: metrics.CellHeight - 2},
			' ', thumbStyle.WithBg(thumbStyle.Fg))
		return
	}
	for x := trackX0; x < trackX1; x += metrics.CellWidth {
		p.DrawCell(x, y, '─', trackStyle)
	}
	for x := thumbX0; x < thumbX1; x += metrics.CellWidth {
		p.DrawCell(x, y, '█', thumbStyle)
	}
}

// handleHBarPress starts a thumb drag or pages on a track click in
// the footer row. Returns handled.
func (t *TreeView) handleHBarPress(event core.MousePressEvent) bool {
	footerH := t.footerHeight()
	if footerH == 0 {
		return false
	}
	bounds := t.Bounds()
	if event.Y < bounds.Height-footerH {
		return false
	}
	lay := t.columnLayout()
	trackX0, trackX1, thumbX0, thumbX1, ok := t.hScrollbarGeometry(lay)
	if !ok {
		return true // reserved row, nothing to scroll: swallow
	}
	switch {
	case event.X >= thumbX0 && event.X < thumbX1:
		t.hbarDragging = true
		t.hbarDragStartX = event.X
		t.hbarDragStartHS = t.hScroll
	case event.X >= trackX0 && event.X < thumbX0:
		t.scrollHorizontally(-int((trackX1 - trackX0) / t.EffectiveCellMetrics().CellWidth))
	case event.X >= thumbX1 && event.X < trackX1:
		t.scrollHorizontally(int((trackX1 - trackX0) / t.EffectiveCellMetrics().CellWidth))
	}
	return true
}

// handleHBarMove continues a footer thumb drag. Returns handled.
func (t *TreeView) handleHBarMove(event core.MouseMoveEvent) bool {
	if !t.hbarDragging {
		return false
	}
	lay := t.columnLayout()
	trackX0, trackX1, thumbX0, thumbX1, ok := t.hScrollbarGeometry(lay)
	if !ok {
		t.hbarDragging = false
		return true
	}
	cw := t.EffectiveCellMetrics().CellWidth
	trackCells := int((trackX1 - trackX0) / cw)
	thumbCells := int((thumbX1 - thumbX0) / cw)
	scrollable := trackCells - thumbCells
	if scrollable <= 0 {
		return true
	}
	deltaCells := int((event.X - t.hbarDragStartX) / cw)
	hs := t.hbarDragStartHS + deltaCells*lay.maxHScroll/scrollable
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
	return true
}

// scrollHorizontally pans the scroll region by delta cells (scroll
// mode only), clamped to the content.
func (t *TreeView) scrollHorizontally(deltaCells int) bool {
	if t.fitWidth || !t.multiColumn() {
		return false
	}
	lay := t.columnLayout()
	hs := t.hScroll + deltaCells
	if hs < 0 {
		hs = 0
	}
	if hs > lay.maxHScroll {
		hs = lay.maxHScroll
	}
	if hs == t.hScroll {
		return false
	}
	t.hScroll = hs
	t.Update()
	return true
}
