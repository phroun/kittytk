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
	isDragging bool

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

	if t.onCurrentChanged != nil && index >= 0 {
		t.onCurrentChanged(t.flatList[index])
	}
}

// ExpandItem expands an item to show its children.
func (t *TreeView) ExpandItem(item *TreeItem) {
	if item.IsLeaf() || item.Expanded {
		return
	}

	item.Expanded = true
	t.rebuildFlatList()
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

	item.Expanded = false
	t.rebuildFlatList()

	// Adjust current index if it was in a collapsed subtree
	if t.currentIndex >= len(t.flatList) {
		t.currentIndex = len(t.flatList) - 1
	}
	t.Update()

	if t.onItemCollapsed != nil {
		t.onItemCollapsed(item)
	}
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
	t.expandRecursive(t.rootItems)
	t.rebuildFlatList()
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
	t.collapseRecursive(t.rootItems)
	t.rebuildFlatList()
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
			s = theme.Normal
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

// paintScrollbar draws a vertical scrollbar.
func (t *TreeView) paintScrollbar(p *core.Painter, visibleCount int) {
	bounds := t.Bounds()
	theme := t.Theme()
	metrics := p.Metrics()

	scrollbarX := bounds.Width - metrics.CellWidth

	// Calculate scrollbar position
	totalItems := len(t.flatList)
	thumbHeight := visibleCount * visibleCount / totalItems
	if thumbHeight < 1 {
		thumbHeight = 1
	}

	thumbPos := t.scrollOffset * visibleCount / totalItems

	// Draw scrollbar track
	for i := 0; i < visibleCount; i++ {
		y := core.Unit(i) * metrics.CellHeight
		p.DrawCell(scrollbarX, y, '│', theme.Disabled)
	}

	// Draw scrollbar thumb
	for i := 0; i < thumbHeight; i++ {
		y := core.Unit(thumbPos+i) * metrics.CellHeight
		if y < bounds.Height {
			p.DrawCell(scrollbarX, y, '█', theme.Normal)
		}
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

	case "Down":
		if t.currentIndex < len(t.flatList)-1 {
			t.SetCurrentIndex(t.currentIndex + 1)
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

	t.SetFocus()
	t.isDragging = true

	metrics := core.DefaultCellMetrics()
	clickedRow := int(event.Y / metrics.CellHeight)
	clickedIndex := t.scrollOffset + clickedRow

	if clickedIndex >= 0 && clickedIndex < len(t.flatList) {
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
	}

	return true
}

// HandleMouseMove handles mouse drag to sweep selection.
func (t *TreeView) HandleMouseMove(event core.MouseMoveEvent) bool {
	if !t.isDragging {
		return false
	}

	metrics := core.DefaultCellMetrics()
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
	return true
}

// HandleFocusIn is called when focus is gained.
func (t *TreeView) HandleFocusIn() {
	t.Update()
}

// HandleFocusOut is called when focus is lost.
func (t *TreeView) HandleFocusOut() {
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
