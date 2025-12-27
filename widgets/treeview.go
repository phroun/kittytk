// Package widgets provides standard UI widgets for the TUI toolkit.
package widgets

import (
	"time"

	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/style"
)

// TreeItem represents an item in a TreeView.
type TreeItem struct {
	Text     string
	Icon     *style.TextIcon
	Data     interface{} // User data
	Enabled  bool
	Expanded bool
	Parent   *TreeItem
	Children []*TreeItem
}

// NewTreeItem creates a new tree item.
func NewTreeItem(text string) *TreeItem {
	return &TreeItem{
		Text:    text,
		Enabled: true,
	}
}

// AddChild adds a child item.
func (t *TreeItem) AddChild(child *TreeItem) {
	child.Parent = t
	t.Children = append(t.Children, child)
}

// RemoveChild removes a child item.
func (t *TreeItem) RemoveChild(child *TreeItem) {
	for i, c := range t.Children {
		if c == child {
			t.Children = append(t.Children[:i], t.Children[i+1:]...)
			child.Parent = nil
			break
		}
	}
}

// IsLeaf returns whether this item has no children.
func (t *TreeItem) IsLeaf() bool {
	return len(t.Children) == 0
}

// Level returns the nesting level (0 for root items).
func (t *TreeItem) Level() int {
	level := 0
	for p := t.Parent; p != nil; p = p.Parent {
		level++
	}
	return level
}

// TreeView displays a hierarchical tree of items.
type TreeView struct {
	core.WidgetBase
	core.AccessibleWidget

	rootItems    []*TreeItem
	flatList     []*TreeItem // Flattened list of visible items
	currentIndex int
	scrollOffset int

	// Appearance
	indentWidth int // Characters per indent level

	// Double-click detection
	lastClickTime  int64 // Unix nano
	lastClickIndex int

	// Mouse state
	isDragging          bool
	scrollbarDragging   bool // Whether scrollbar thumb is being dragged
	scrollbarDragStart  int  // Y position where drag started
	scrollbarDragOffset int  // Scroll offset when drag started

	// Callbacks
	onCurrentChanged  func(item *TreeItem)
	onItemActivated   func(item *TreeItem)
	onItemExpanded    func(item *TreeItem)
	onItemCollapsed   func(item *TreeItem)
}

// NewTreeView creates a new tree view.
func NewTreeView() *TreeView {
	t := &TreeView{
		currentIndex:   -1,
		indentWidth:    2,
		lastClickIndex: -1,
	}
	t.WidgetBase = *core.NewWidgetBase()
	t.Init(t) // Enable polymorphic focus handling
	t.SetFocusPolicy(core.StrongFocus)
	t.SetAccessibleRole(core.RoleTree)
	return t
}

// AddRootItem adds a root item to the tree.
func (t *TreeView) AddRootItem(item *TreeItem) {
	item.Parent = nil
	t.rootItems = append(t.rootItems, item)
	t.rebuildFlatList()
	if t.currentIndex < 0 && len(t.flatList) > 0 {
		t.SetCurrentIndex(0)
	}
	t.Update()
}

// RemoveRootItem removes a root item from the tree.
func (t *TreeView) RemoveRootItem(item *TreeItem) {
	for i, r := range t.rootItems {
		if r == item {
			t.rootItems = append(t.rootItems[:i], t.rootItems[i+1:]...)
			break
		}
	}
	t.rebuildFlatList()
	if t.currentIndex >= len(t.flatList) {
		t.currentIndex = len(t.flatList) - 1
	}
	t.Update()
}

// Clear removes all items.
func (t *TreeView) Clear() {
	t.rootItems = nil
	t.flatList = nil
	t.currentIndex = -1
	t.scrollOffset = 0
	t.Update()
}

// RootItems returns all root items.
func (t *TreeView) RootItems() []*TreeItem {
	return t.rootItems
}

// CurrentItem returns the currently focused item.
func (t *TreeView) CurrentItem() *TreeItem {
	if t.currentIndex < 0 || t.currentIndex >= len(t.flatList) {
		return nil
	}
	return t.flatList[t.currentIndex]
}

// SetCurrentItem sets the current item.
func (t *TreeView) SetCurrentItem(item *TreeItem) {
	for i, flatItem := range t.flatList {
		if flatItem == item {
			t.SetCurrentIndex(i)
			return
		}
	}
}

// CurrentIndex returns the current index in the flat list.
func (t *TreeView) CurrentIndex() int {
	return t.currentIndex
}

// SetCurrentIndex sets the current index.
func (t *TreeView) SetCurrentIndex(index int) {
	if index < -1 || index >= len(t.flatList) {
		return
	}
	if t.currentIndex == index {
		return
	}

	t.currentIndex = index
	t.ensureVisible(index)
	t.Update()

	// Notify parent scroll containers to scroll this item into view
	if index >= 0 {
		metrics := core.DefaultCellMetrics()
		item := t.flatList[index]
		level := item.Level()

		// Calculate the item's content start position (one space before the expand indicator)
		// For root items (level 0), start at X=0
		contentStartCells := level * t.indentWidth
		if contentStartCells > 0 {
			contentStartCells-- // Show one space of indent
		}

		// Calculate actual content width: expand indicator (2 chars) + text
		expandIndicatorWidth := 2 // "▶ " or "▼ " or "  " (for leaves)
		textWidth := len(item.Text)
		actualContentCells := expandIndicatorWidth + textWidth

		// Calculate the visual Y position of this item (after internal scrolling)
		// This is where the item appears on screen, relative to the TreeView's bounds
		visualRow := index - t.scrollOffset
		itemY := core.Unit(visualRow) * metrics.CellHeight

		itemRect := core.UnitRect{
			X:      core.Unit(contentStartCells) * metrics.CellWidth,
			Y:      itemY,
			Width:  core.Unit(actualContentCells) * metrics.CellWidth,
			Height: metrics.CellHeight,
		}
		t.ScrollRectIntoView(itemRect)
	}

	if t.onCurrentChanged != nil && index >= 0 {
		t.onCurrentChanged(t.flatList[index])
	}
}

// ExpandItem expands an item to show its children.
func (t *TreeView) ExpandItem(item *TreeItem) {
	if item.IsLeaf() || item.Expanded {
		return
	}

	// Save currently selected item
	var selectedItem *TreeItem
	if t.currentIndex >= 0 && t.currentIndex < len(t.flatList) {
		selectedItem = t.flatList[t.currentIndex]
	}

	item.Expanded = true
	t.rebuildFlatList()

	// Restore selection by finding the same item in new flat list
	t.restoreSelectionByItem(selectedItem)
	t.Update()

	if t.onItemExpanded != nil {
		t.onItemExpanded(item)
	}
}

// CollapseItem collapses an item to hide its children.
func (t *TreeView) CollapseItem(item *TreeItem) {
	if !item.Expanded {
		return
	}

	// Save currently selected item
	var selectedItem *TreeItem
	if t.currentIndex >= 0 && t.currentIndex < len(t.flatList) {
		selectedItem = t.flatList[t.currentIndex]
	}

	item.Expanded = false
	t.rebuildFlatList()

	// Restore selection by finding the same item in new flat list
	// If selected item is no longer visible (was in collapsed subtree),
	// select the item that was collapsed
	if !t.restoreSelectionByItem(selectedItem) {
		// Selected item no longer visible, select the collapsed item
		t.restoreSelectionByItem(item)
	}
	t.Update()

	if t.onItemCollapsed != nil {
		t.onItemCollapsed(item)
	}
}

// restoreSelectionByItem finds the given item in the flat list and selects it.
// Returns true if the item was found and selected, false otherwise.
func (t *TreeView) restoreSelectionByItem(item *TreeItem) bool {
	if item == nil {
		return false
	}
	for i, flatItem := range t.flatList {
		if flatItem == item {
			t.currentIndex = i
			return true
		}
	}
	return false
}

// ToggleItem toggles the expanded state of an item.
func (t *TreeView) ToggleItem(item *TreeItem) {
	if item.Expanded {
		t.CollapseItem(item)
	} else {
		t.ExpandItem(item)
	}
}

// ExpandAll expands all items.
func (t *TreeView) ExpandAll() {
	// Save currently selected item
	var selectedItem *TreeItem
	if t.currentIndex >= 0 && t.currentIndex < len(t.flatList) {
		selectedItem = t.flatList[t.currentIndex]
	}

	t.expandRecursive(t.rootItems)
	t.rebuildFlatList()

	// Restore selection by finding the same item
	t.restoreSelectionByItem(selectedItem)
	t.Update()
}

func (t *TreeView) expandRecursive(items []*TreeItem) {
	for _, item := range items {
		item.Expanded = true
		t.expandRecursive(item.Children)
	}
}

// CollapseAll collapses all items.
func (t *TreeView) CollapseAll() {
	// Save currently selected item
	var selectedItem *TreeItem
	if t.currentIndex >= 0 && t.currentIndex < len(t.flatList) {
		selectedItem = t.flatList[t.currentIndex]
	}

	t.collapseRecursive(t.rootItems)
	t.rebuildFlatList()

	// Restore selection - if item is no longer visible, select first root
	if !t.restoreSelectionByItem(selectedItem) && len(t.flatList) > 0 {
		t.currentIndex = 0
	}
	t.Update()
}

func (t *TreeView) collapseRecursive(items []*TreeItem) {
	for _, item := range items {
		item.Expanded = false
		t.collapseRecursive(item.Children)
	}
}

// SetIndentWidth sets the indent width per level.
func (t *TreeView) SetIndentWidth(width int) {
	t.indentWidth = width
	t.Update()
}

// SetOnCurrentChanged sets the current changed callback.
func (t *TreeView) SetOnCurrentChanged(handler func(item *TreeItem)) {
	t.onCurrentChanged = handler
}

// SetOnItemActivated sets the item activated callback.
func (t *TreeView) SetOnItemActivated(handler func(item *TreeItem)) {
	t.onItemActivated = handler
}

// SetOnItemExpanded sets the item expanded callback.
func (t *TreeView) SetOnItemExpanded(handler func(item *TreeItem)) {
	t.onItemExpanded = handler
}

// SetOnItemCollapsed sets the item collapsed callback.
func (t *TreeView) SetOnItemCollapsed(handler func(item *TreeItem)) {
	t.onItemCollapsed = handler
}

// rebuildFlatList rebuilds the flattened list of visible items.
func (t *TreeView) rebuildFlatList() {
	t.flatList = nil
	t.flattenItems(t.rootItems)

	// Clamp scroll offset to valid range after list size changes
	t.clampScrollOffset()
}

// clampScrollOffset ensures scrollOffset is within valid bounds.
func (t *TreeView) clampScrollOffset() {
	if len(t.flatList) == 0 {
		t.scrollOffset = 0
		return
	}

	bounds := t.Bounds()
	metrics := core.DefaultCellMetrics()
	visibleCount := int(bounds.Height / metrics.CellHeight)

	maxScroll := len(t.flatList) - visibleCount
	if maxScroll < 0 {
		maxScroll = 0
	}
	if t.scrollOffset > maxScroll {
		t.scrollOffset = maxScroll
	}
	if t.scrollOffset < 0 {
		t.scrollOffset = 0
	}
}

func (t *TreeView) flattenItems(items []*TreeItem) {
	for _, item := range items {
		t.flatList = append(t.flatList, item)
		if item.Expanded && len(item.Children) > 0 {
			t.flattenItems(item.Children)
		}
	}
}

// ensureVisible ensures the given index is visible.
func (t *TreeView) ensureVisible(index int) {
	if index < 0 {
		return
	}

	bounds := t.Bounds()
	metrics := core.DefaultCellMetrics()
	visibleCount := int(bounds.Height / metrics.CellHeight)

	if index < t.scrollOffset {
		t.scrollOffset = index
	} else if index >= t.scrollOffset+visibleCount {
		t.scrollOffset = index - visibleCount + 1
	}
}

// SizeHint returns the preferred size.
func (t *TreeView) SizeHint() core.UnitSize {
	metrics := core.DefaultCellMetrics()
	return core.UnitSize{
		Width:  metrics.TextWidth(40), // Default width
		Height: metrics.TextHeight(15), // 15 items visible
	}
}

// Paint renders the tree view.
func (t *TreeView) Paint(p *core.Painter) {
	bounds := t.Bounds()
	theme := t.Theme()
	focused := t.HasFocus()
	metrics := p.Metrics()

	// Draw background
	p.FillRect(core.UnitRect{Width: bounds.Width, Height: bounds.Height}, ' ', theme.Normal)

	visibleCount := int(bounds.Height / metrics.CellHeight)

	// Draw items
	for i := 0; i < visibleCount; i++ {
		itemIndex := t.scrollOffset + i
		if itemIndex >= len(t.flatList) {
			break
		}

		item := t.flatList[itemIndex]
		itemY := core.Unit(i) * metrics.CellHeight
		level := item.Level()

		// Determine style
		var s style.CellStyle
		if !item.Enabled {
			s = theme.Disabled
		} else if itemIndex == t.currentIndex {
			if focused {
				s = theme.Selected
			} else {
				// Unfocused selection: silver on dark blue
				s = style.DefaultStyle().WithFg(style.ColorBrightWhite).WithBg(style.ColorBlue)
			}
		} else {
			// Unselected items: white on black
			s = style.DefaultStyle().WithFg(style.ColorWhite).WithBg(style.ColorBlack)
		}

		// Draw row background
		p.FillRect(core.UnitRect{
			X:      0,
			Y:      itemY,
			Width:  bounds.Width,
			Height: metrics.CellHeight,
		}, ' ', s)

		// Calculate x position with indent
		x := core.Unit(level*t.indentWidth) * metrics.CellWidth

		// Draw expand/collapse indicator
		if !item.IsLeaf() {
			if item.Expanded {
				p.DrawCell(x, itemY, '▼', s)
			} else {
				p.DrawCell(x, itemY, '▸', s)
			}
		} else {
			p.DrawCell(x, itemY, ' ', s)
		}
		x += metrics.CellWidth

		// Draw icon if present
		if item.Icon != nil && len(item.Icon.Cells) > 0 {
			cell := item.Icon.Cells[0]
			p.DrawCell(x, itemY, cell.Char, cell.Style)
			x += metrics.CellWidth * 2
		}

		// Draw text
		for _, ch := range item.Text {
			if x >= bounds.Width {
				break
			}
			p.DrawCell(x, itemY, ch, s)
			x += metrics.CellWidth
		}
	}

	// Draw scrollbar if needed
	if len(t.flatList) > visibleCount {
		t.paintScrollbar(p, visibleCount)
	}
}

// visibleCount returns the number of visible rows.
func (t *TreeView) visibleCount() int {
	bounds := t.Bounds()
	metrics := core.DefaultCellMetrics()
	return int(bounds.Height / metrics.CellHeight)
}

// scrollbarGeometry returns scrollbar dimensions and thumb position.
// Returns: scrollbarX, thumbStart, thumbHeight, trackHeight (all in rows)
func (t *TreeView) scrollbarGeometry(visibleCount int) (scrollbarX core.Unit, thumbStart, thumbHeight, trackHeight int) {
	bounds := t.Bounds()
	metrics := core.DefaultCellMetrics()
	totalItems := len(t.flatList)

	scrollbarX = bounds.Width - metrics.CellWidth
	trackHeight = visibleCount

	if totalItems <= visibleCount {
		// No scrolling needed - thumb fills track
		thumbStart = 0
		thumbHeight = trackHeight
		return
	}

	// Calculate thumb height - proportional to visible/total, minimum 1 row
	thumbHeight = visibleCount * visibleCount / totalItems
	if thumbHeight < 1 {
		thumbHeight = 1
	}

	// Calculate thumb position
	// The thumb should only be at position 0 when scrollOffset is 0
	// The thumb should only be at the bottom when scrollOffset is at max
	maxScroll := totalItems - visibleCount
	scrollableTrack := trackHeight - thumbHeight

	if maxScroll > 0 && scrollableTrack > 0 {
		// Map scroll position to thumb position, ensuring extremes are only at extremes
		thumbStart = t.scrollOffset * scrollableTrack / maxScroll

		// Ensure thumb doesn't go to extremes unless scroll is at extremes
		if t.scrollOffset > 0 && thumbStart == 0 {
			thumbStart = 1
		}
		if t.scrollOffset < maxScroll && thumbStart >= scrollableTrack {
			thumbStart = scrollableTrack - 1
		}
	}

	return
}

// paintScrollbar draws a vertical scrollbar.
func (t *TreeView) paintScrollbar(p *core.Painter, visibleCount int) {
	theme := t.Theme()
	metrics := p.Metrics()

	scrollbarX, thumbStart, thumbHeight, trackHeight := t.scrollbarGeometry(visibleCount)

	// Draw scrollbar track
	for i := 0; i < trackHeight; i++ {
		y := core.Unit(i) * metrics.CellHeight
		p.DrawCell(scrollbarX, y, '│', theme.Disabled)
	}

	// Draw scrollbar thumb
	for i := 0; i < thumbHeight; i++ {
		y := core.Unit(thumbStart+i) * metrics.CellHeight
		p.DrawCell(scrollbarX, y, '█', theme.Normal)
	}
}

// HandleKeyPress handles keyboard input.
func (t *TreeView) HandleKeyPress(event core.KeyPressEvent) bool {
	current := t.CurrentItem()

	switch event.Key {
	case "Up":
		if t.currentIndex > 0 {
			t.SetCurrentIndex(t.currentIndex - 1)
		}
		return true

	case "M-Up", "C-Up", "A-Up":
		// Jump by 5 items, scrolling to maintain relative position
		if t.currentIndex > 0 {
			delta := 5
			newIndex := t.currentIndex - delta
			if newIndex < 0 {
				newIndex = 0
			}
			actualDelta := t.currentIndex - newIndex
			// Scroll by same amount to maintain relative position
			newScroll := t.scrollOffset - actualDelta
			if newScroll < 0 {
				newScroll = 0
			}
			t.scrollOffset = newScroll
			t.SetCurrentIndex(newIndex)
		}
		return true

	case "Down":
		if t.currentIndex < len(t.flatList)-1 {
			t.SetCurrentIndex(t.currentIndex + 1)
		}
		return true

	case "M-Down", "C-Down", "A-Down":
		// Jump by 5 items, scrolling to maintain relative position
		if t.currentIndex < len(t.flatList)-1 {
			delta := 5
			newIndex := t.currentIndex + delta
			if newIndex >= len(t.flatList) {
				newIndex = len(t.flatList) - 1
			}
			actualDelta := newIndex - t.currentIndex
			// Scroll by same amount to maintain relative position
			visibleCount := t.visibleCount()
			maxScroll := len(t.flatList) - visibleCount
			if maxScroll < 0 {
				maxScroll = 0
			}
			newScroll := t.scrollOffset + actualDelta
			if newScroll > maxScroll {
				newScroll = maxScroll
			}
			t.scrollOffset = newScroll
			t.SetCurrentIndex(newIndex)
		}
		return true

	case "Left":
		if current != nil {
			if current.Expanded && !current.IsLeaf() {
				t.CollapseItem(current)
			} else if current.Parent != nil {
				t.SetCurrentItem(current.Parent)
			}
		}
		return true

	case "Right":
		if current != nil {
			if !current.Expanded && !current.IsLeaf() {
				t.ExpandItem(current)
			} else if current.Expanded && len(current.Children) > 0 {
				t.SetCurrentItem(current.Children[0])
			}
		}
		return true

	case "Home":
		if len(t.flatList) > 0 {
			t.SetCurrentIndex(0)
		}
		return true

	case "End":
		if len(t.flatList) > 0 {
			t.SetCurrentIndex(len(t.flatList) - 1)
		}
		return true

	case "PageUp":
		bounds := t.Bounds()
		metrics := core.DefaultCellMetrics()
		pageSize := int(bounds.Height / metrics.CellHeight)
		newIndex := t.currentIndex - pageSize
		if newIndex < 0 {
			newIndex = 0
		}
		t.SetCurrentIndex(newIndex)
		return true

	case "PageDown":
		bounds := t.Bounds()
		metrics := core.DefaultCellMetrics()
		pageSize := int(bounds.Height / metrics.CellHeight)
		newIndex := t.currentIndex + pageSize
		if newIndex >= len(t.flatList) {
			newIndex = len(t.flatList) - 1
		}
		t.SetCurrentIndex(newIndex)
		return true

	case "Enter", " ", "Space":
		if current != nil {
			if !current.IsLeaf() {
				t.ToggleItem(current)
			}
			if t.onItemActivated != nil {
				t.onItemActivated(current)
			}
		}
		return true

	case "*":
		// Expand all
		t.ExpandAll()
		return true

	case "-":
		// Collapse current
		if current != nil && !current.IsLeaf() {
			t.CollapseItem(current)
		}
		return true

	case "+":
		// Expand current
		if current != nil && !current.IsLeaf() {
			t.ExpandItem(current)
		}
		return true
	}

	return false
}

// HandleMousePress handles mouse clicks.
func (t *TreeView) HandleMousePress(event core.MousePressEvent) bool {
	if event.Button != core.LeftButton {
		return false
	}

	// Clear any stale drag state from previous incomplete drags
	t.isDragging = false
	t.scrollbarDragging = false

	// Check if click is within our bounds
	bounds := t.Bounds()
	if event.X < 0 || event.Y < 0 || event.X >= bounds.Width || event.Y >= bounds.Height {
		return false
	}

	t.SetFocus()
	metrics := core.DefaultCellMetrics()

	// Check if click is on scrollbar
	scrollbarX, thumbStart, thumbHeight, _ := t.scrollbarGeometry(t.visibleCount())
	if event.X >= scrollbarX && len(t.flatList) > t.visibleCount() {
		clickedRow := int(event.Y / metrics.CellHeight)

		// Check if on thumb
		if clickedRow >= thumbStart && clickedRow < thumbStart+thumbHeight {
			// Start scrollbar drag - clear content drag flag
			t.scrollbarDragging = true
			t.isDragging = false
			t.scrollbarDragStart = clickedRow
			t.scrollbarDragOffset = t.scrollOffset
			return true
		}

		// Click on track - page up or page down
		visibleCount := t.visibleCount()
		if clickedRow < thumbStart {
			// Page up
			t.scrollOffset -= visibleCount
			if t.scrollOffset < 0 {
				t.scrollOffset = 0
			}
		} else {
			// Page down
			maxScroll := len(t.flatList) - visibleCount
			t.scrollOffset += visibleCount
			if t.scrollOffset > maxScroll {
				t.scrollOffset = maxScroll
			}
		}
		t.Update()
		return true
	}

	// Click on tree content (before scrollbar)
	if event.X >= scrollbarX {
		return false // Click is past the content area
	}

	// Calculate which item was clicked
	clickedRow := int(event.Y / metrics.CellHeight)
	clickedIndex := t.scrollOffset + clickedRow

	// Only process if click is on a valid item
	contentWidth := bounds.Width - metrics.CellWidth
	if event.X >= 0 && event.X < contentWidth && clickedIndex >= 0 && clickedIndex < len(t.flatList) {
		item := t.flatList[clickedIndex]
		level := item.Level()

		// Check if clicked on expand/collapse indicator
		indicatorX := core.Unit(level*t.indentWidth) * metrics.CellWidth
		if event.X >= indicatorX && event.X < indicatorX+metrics.CellWidth {
			if !item.IsLeaf() {
				t.ToggleItem(item)
				return true
			}
		}

		// Start content drag - clear scrollbar drag flag
		t.isDragging = true
		t.scrollbarDragging = false

		// Check for double-click (400ms threshold)
		now := time.Now().UnixNano()
		isDoubleClick := t.lastClickIndex == clickedIndex &&
			(now-t.lastClickTime) < int64(400*time.Millisecond)

		// Update click tracking
		t.lastClickTime = now
		t.lastClickIndex = clickedIndex

		if isDoubleClick {
			// Double-click: toggle expand/collapse if not leaf, then activate
			if !item.IsLeaf() {
				t.ToggleItem(item)
			}
			if t.onItemActivated != nil {
				t.onItemActivated(item)
			}
			// Reset double-click state
			t.lastClickIndex = -1
			return true
		}

		t.SetCurrentIndex(clickedIndex)
		return true
	}

	// Click is in content area but not on a valid item
	return false
}

// HandleMouseMove handles mouse drag to sweep selection.
func (t *TreeView) HandleMouseMove(event core.MouseMoveEvent) bool {
	// If we don't have focus, we shouldn't be processing drags
	// (another widget got the click and we have stale drag state)
	if !t.HasFocus() {
		t.isDragging = false
		t.scrollbarDragging = false
		return false
	}

	metrics := core.DefaultCellMetrics()

	// Handle scrollbar thumb drag
	// Note: Once drag is captured on press, we don't check horizontal bounds during drag
	if t.scrollbarDragging {
		currentRow := int(event.Y / metrics.CellHeight)
		rowDelta := currentRow - t.scrollbarDragStart

		visibleCount := t.visibleCount()
		totalItems := len(t.flatList)
		maxScroll := totalItems - visibleCount

		if maxScroll > 0 {
			_, _, thumbHeight, trackHeight := t.scrollbarGeometry(visibleCount)
			scrollableTrack := trackHeight - thumbHeight

			if scrollableTrack > 0 {
				// Convert row delta to scroll offset delta
				scrollDelta := rowDelta * maxScroll / scrollableTrack
				newOffset := t.scrollbarDragOffset + scrollDelta

				// Clamp
				if newOffset < 0 {
					newOffset = 0
				} else if newOffset > maxScroll {
					newOffset = maxScroll
				}

				if newOffset != t.scrollOffset {
					t.scrollOffset = newOffset
					t.Update()
				}
			}
		}
		return true
	}

	// Handle tree item drag
	// Note: Once drag is captured on press, we don't check horizontal bounds during drag
	if !t.isDragging {
		return false
	}

	row := int(event.Y / metrics.CellHeight)
	index := t.scrollOffset + row

	// Clamp to valid range
	if index < 0 {
		index = 0
	} else if index >= len(t.flatList) {
		index = len(t.flatList) - 1
	}

	if index >= 0 && index != t.currentIndex {
		t.SetCurrentIndex(index)
	}

	return true
}

// HandleMouseRelease handles mouse release.
func (t *TreeView) HandleMouseRelease(event core.MouseReleaseEvent) bool {
	t.isDragging = false
	t.scrollbarDragging = false
	return true
}

// HandleMouseWheel handles mouse wheel scrolling.
func (t *TreeView) HandleMouseWheel(event core.MouseWheelEvent) bool {
	if len(t.flatList) == 0 {
		return false
	}

	visibleCount := t.visibleCount()
	maxScroll := len(t.flatList) - visibleCount
	if maxScroll <= 0 {
		return false
	}

	// Scroll by 3 lines per wheel click
	scrollAmount := 3
	if event.DeltaY < 0 {
		// Scroll up
		t.scrollOffset -= scrollAmount
		if t.scrollOffset < 0 {
			t.scrollOffset = 0
		}
	} else if event.DeltaY > 0 {
		// Scroll down
		t.scrollOffset += scrollAmount
		if t.scrollOffset > maxScroll {
			t.scrollOffset = maxScroll
		}
	}

	t.Update()
	return true
}

// HandleFocusIn is called when focus is gained.
func (t *TreeView) HandleFocusIn() {
	// Auto-select first item if nothing is selected
	if t.currentIndex < 0 && len(t.flatList) > 0 {
		t.SetCurrentIndex(0)
	}
	t.Update()
}

// HandleFocusOut is called when focus is lost.
func (t *TreeView) HandleFocusOut() {
	// Clear any active drag state when focus is lost
	t.isDragging = false
	t.scrollbarDragging = false
	t.Update()
}

// AccessibleInfo returns accessibility information.
func (t *TreeView) AccessibleInfo() core.AccessibleInfo {
	info := t.AccessibleWidget.AccessibleInfo()
	info.Role = core.RoleTree
	info.SetSize = len(t.flatList)

	if t.currentIndex >= 0 && t.currentIndex < len(t.flatList) {
		item := t.flatList[t.currentIndex]
		info.PositionInSet = t.currentIndex + 1
		info.Value = item.Text
		info.Level = item.Level() + 1

		if item.Expanded {
			info.State |= core.StateExpanded
		} else if !item.IsLeaf() {
			info.State |= core.StateCollapsed
		}
	}

	if !t.IsEnabled() {
		info.State |= core.StateDisabled
	}

	return info
}
