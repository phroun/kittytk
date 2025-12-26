// Package widgets provides standard UI widgets for the TUI toolkit.
package widgets

import (
	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/style"
)

// ListItem represents an item in a ListView.
type ListItem struct {
	Text    string
	Icon    *style.TextIcon
	Data    interface{} // User data
	Enabled bool
}

// NewListItem creates a new list item.
func NewListItem(text string) *ListItem {
	return &ListItem{
		Text:    text,
		Enabled: true,
	}
}

// ListView displays a scrollable list of items.
type ListView struct {
	core.WidgetBase
	core.AccessibleWidget

	items        []*ListItem
	currentIndex int
	scrollOffset int

	// Selection mode
	selectionMode SelectionMode
	selectedItems map[int]bool

	// Appearance
	alternateRowColors bool
	showIcons          bool

	// Callbacks
	onCurrentChanged  func(index int)
	onItemActivated   func(index int)
	onSelectionChanged func()
}

// SelectionMode determines how items can be selected.
type SelectionMode int

const (
	SingleSelection SelectionMode = iota
	MultiSelection
	ExtendedSelection
	NoSelection
)

// NewListView creates a new list view.
func NewListView() *ListView {
	l := &ListView{
		currentIndex:  -1,
		selectionMode: SingleSelection,
		selectedItems: make(map[int]bool),
	}
	l.WidgetBase = *core.NewWidgetBase()
	l.SetFocusPolicy(core.StrongFocus)
	l.SetAccessibleRole(core.RoleList)
	return l
}

// AddItem adds an item to the list.
func (l *ListView) AddItem(item *ListItem) {
	l.items = append(l.items, item)
	if l.currentIndex < 0 && len(l.items) == 1 {
		l.SetCurrentIndex(0)
	}
	l.Update()
}

// AddTextItem adds a text item to the list.
func (l *ListView) AddTextItem(text string) {
	l.AddItem(NewListItem(text))
}

// InsertItem inserts an item at the given index.
func (l *ListView) InsertItem(index int, item *ListItem) {
	if index < 0 {
		index = 0
	}
	if index > len(l.items) {
		index = len(l.items)
	}

	l.items = append(l.items[:index], append([]*ListItem{item}, l.items[index:]...)...)

	// Adjust selection
	newSelected := make(map[int]bool)
	for idx := range l.selectedItems {
		if idx >= index {
			newSelected[idx+1] = true
		} else {
			newSelected[idx] = true
		}
	}
	l.selectedItems = newSelected

	if l.currentIndex >= index {
		l.currentIndex++
	}
	l.Update()
}

// RemoveItem removes an item at the given index.
func (l *ListView) RemoveItem(index int) {
	if index < 0 || index >= len(l.items) {
		return
	}

	l.items = append(l.items[:index], l.items[index+1:]...)

	// Adjust selection
	newSelected := make(map[int]bool)
	for idx := range l.selectedItems {
		if idx < index {
			newSelected[idx] = true
		} else if idx > index {
			newSelected[idx-1] = true
		}
	}
	l.selectedItems = newSelected

	// Adjust current index
	if l.currentIndex == index {
		if l.currentIndex >= len(l.items) {
			l.currentIndex = len(l.items) - 1
		}
		if l.onCurrentChanged != nil {
			l.onCurrentChanged(l.currentIndex)
		}
	} else if l.currentIndex > index {
		l.currentIndex--
	}
	l.Update()
}

// Clear removes all items.
func (l *ListView) Clear() {
	l.items = nil
	l.currentIndex = -1
	l.scrollOffset = 0
	l.selectedItems = make(map[int]bool)
	l.Update()
}

// Count returns the number of items.
func (l *ListView) Count() int {
	return len(l.items)
}

// Item returns the item at the given index.
func (l *ListView) Item(index int) *ListItem {
	if index < 0 || index >= len(l.items) {
		return nil
	}
	return l.items[index]
}

// Items returns all items.
func (l *ListView) Items() []*ListItem {
	return l.items
}

// CurrentIndex returns the current item index.
func (l *ListView) CurrentIndex() int {
	return l.currentIndex
}

// SetCurrentIndex sets the current item index.
func (l *ListView) SetCurrentIndex(index int) {
	if index < -1 || index >= len(l.items) {
		return
	}
	if l.currentIndex == index {
		return
	}

	l.currentIndex = index
	l.ensureVisible(index)
	l.Update()

	if l.selectionMode == SingleSelection {
		l.selectedItems = make(map[int]bool)
		if index >= 0 {
			l.selectedItems[index] = true
		}
		if l.onSelectionChanged != nil {
			l.onSelectionChanged()
		}
	}

	if l.onCurrentChanged != nil {
		l.onCurrentChanged(index)
	}
}

// CurrentItem returns the current item.
func (l *ListView) CurrentItem() *ListItem {
	return l.Item(l.currentIndex)
}

// SelectionMode returns the selection mode.
func (l *ListView) SelectionMode() SelectionMode {
	return l.selectionMode
}

// SetSelectionMode sets the selection mode.
func (l *ListView) SetSelectionMode(mode SelectionMode) {
	l.selectionMode = mode
	if mode == NoSelection {
		l.selectedItems = make(map[int]bool)
	}
	l.Update()
}

// IsSelected returns whether the item at index is selected.
func (l *ListView) IsSelected(index int) bool {
	return l.selectedItems[index]
}

// SetSelected sets the selection state of an item.
func (l *ListView) SetSelected(index int, selected bool) {
	if index < 0 || index >= len(l.items) {
		return
	}
	if l.selectionMode == NoSelection {
		return
	}

	if l.selectionMode == SingleSelection && selected {
		l.selectedItems = make(map[int]bool)
	}

	if selected {
		l.selectedItems[index] = true
	} else {
		delete(l.selectedItems, index)
	}
	l.Update()

	if l.onSelectionChanged != nil {
		l.onSelectionChanged()
	}
}

// SelectedIndexes returns all selected item indexes.
func (l *ListView) SelectedIndexes() []int {
	var result []int
	for idx := range l.selectedItems {
		result = append(result, idx)
	}
	return result
}

// SelectAll selects all items.
func (l *ListView) SelectAll() {
	if l.selectionMode == SingleSelection || l.selectionMode == NoSelection {
		return
	}

	for i := range l.items {
		l.selectedItems[i] = true
	}
	l.Update()

	if l.onSelectionChanged != nil {
		l.onSelectionChanged()
	}
}

// ClearSelection clears all selections.
func (l *ListView) ClearSelection() {
	l.selectedItems = make(map[int]bool)
	l.Update()

	if l.onSelectionChanged != nil {
		l.onSelectionChanged()
	}
}

// SetAlternateRowColors sets whether to use alternate row colors.
func (l *ListView) SetAlternateRowColors(alternate bool) {
	l.alternateRowColors = alternate
	l.Update()
}

// SetShowIcons sets whether to show icons.
func (l *ListView) SetShowIcons(show bool) {
	l.showIcons = show
	l.Update()
}

// SetOnCurrentChanged sets the current changed callback.
func (l *ListView) SetOnCurrentChanged(handler func(index int)) {
	l.onCurrentChanged = handler
}

// SetOnItemActivated sets the item activated callback (double-click or Enter).
func (l *ListView) SetOnItemActivated(handler func(index int)) {
	l.onItemActivated = handler
}

// SetOnSelectionChanged sets the selection changed callback.
func (l *ListView) SetOnSelectionChanged(handler func()) {
	l.onSelectionChanged = handler
}

// ensureVisible ensures the given index is visible.
func (l *ListView) ensureVisible(index int) {
	if index < 0 {
		return
	}

	bounds := l.Bounds()
	metrics := core.DefaultCellMetrics()
	visibleCount := int(bounds.Height / metrics.CellHeight)

	if index < l.scrollOffset {
		l.scrollOffset = index
	} else if index >= l.scrollOffset+visibleCount {
		l.scrollOffset = index - visibleCount + 1
	}
}

// SizeHint returns the preferred size.
func (l *ListView) SizeHint() core.UnitSize {
	metrics := core.DefaultCellMetrics()
	return core.UnitSize{
		Width:  metrics.TextWidth(30), // Default width
		Height: metrics.TextHeight(10), // 10 items visible
	}
}

// Paint renders the list view.
func (l *ListView) Paint(p *core.Painter) {
	bounds := l.Bounds()
	theme := l.Theme()
	focused := l.HasFocus()
	metrics := p.Metrics()

	// Draw background
	p.FillRect(core.UnitRect{Width: bounds.Width, Height: bounds.Height}, ' ', theme.Normal)

	visibleCount := int(bounds.Height / metrics.CellHeight)

	// Draw items
	for i := 0; i < visibleCount; i++ {
		itemIndex := l.scrollOffset + i
		if itemIndex >= len(l.items) {
			break
		}

		item := l.items[itemIndex]
		itemY := core.Unit(i) * metrics.CellHeight

		// Determine style
		var s style.CellStyle
		if !item.Enabled {
			s = theme.Disabled
		} else if l.selectedItems[itemIndex] {
			if focused {
				s = theme.Selected
			} else {
				// Unfocused selection: silver on dark blue
				s = style.DefaultStyle().WithFg(style.ColorBrightWhite).WithBg(style.ColorBlue)
			}
		} else if l.alternateRowColors && itemIndex%2 == 1 {
			s = theme.Normal.WithAttrs(style.StyleDim)
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

		// Draw current indicator
		x := core.Unit(0)
		if itemIndex == l.currentIndex && focused {
			p.DrawCell(x, itemY, '▸', s)
		}
		x += metrics.CellWidth

		// Draw icon if present
		if l.showIcons && item.Icon != nil {
			// Draw icon (simplified - just first char for now)
			if len(item.Icon.Cells) > 0 {
				cell := item.Icon.Cells[0]
				p.DrawCell(x, itemY, cell.Char, cell.Style)
			}
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
	if len(l.items) > visibleCount {
		l.paintScrollbar(p, visibleCount)
	}
}

// paintScrollbar draws a vertical scrollbar.
func (l *ListView) paintScrollbar(p *core.Painter, visibleCount int) {
	bounds := l.Bounds()
	theme := l.Theme()
	metrics := p.Metrics()

	scrollbarX := bounds.Width - metrics.CellWidth

	// Calculate scrollbar position
	totalItems := len(l.items)
	thumbHeight := visibleCount * visibleCount / totalItems
	if thumbHeight < 1 {
		thumbHeight = 1
	}

	thumbPos := l.scrollOffset * visibleCount / totalItems

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
func (l *ListView) HandleKeyPress(event core.KeyPressEvent) bool {
	switch event.Key {
	case "Up":
		if l.currentIndex > 0 {
			l.SetCurrentIndex(l.currentIndex - 1)
		}
		return true

	case "Down":
		if l.currentIndex < len(l.items)-1 {
			l.SetCurrentIndex(l.currentIndex + 1)
		}
		return true

	case "Home":
		if len(l.items) > 0 {
			l.SetCurrentIndex(0)
		}
		return true

	case "End":
		if len(l.items) > 0 {
			l.SetCurrentIndex(len(l.items) - 1)
		}
		return true

	case "PageUp":
		bounds := l.Bounds()
		metrics := core.DefaultCellMetrics()
		pageSize := int(bounds.Height / metrics.CellHeight)
		newIndex := l.currentIndex - pageSize
		if newIndex < 0 {
			newIndex = 0
		}
		l.SetCurrentIndex(newIndex)
		return true

	case "PageDown":
		bounds := l.Bounds()
		metrics := core.DefaultCellMetrics()
		pageSize := int(bounds.Height / metrics.CellHeight)
		newIndex := l.currentIndex + pageSize
		if newIndex >= len(l.items) {
			newIndex = len(l.items) - 1
		}
		l.SetCurrentIndex(newIndex)
		return true

	case "Enter", " ", "Space":
		if l.currentIndex >= 0 && l.onItemActivated != nil {
			l.onItemActivated(l.currentIndex)
		}
		return true

	case "^A":
		l.SelectAll()
		return true
	}

	return false
}

// HandleMousePress handles mouse clicks.
func (l *ListView) HandleMousePress(event core.MousePressEvent) bool {
	if event.Button != core.LeftButton {
		return false
	}

	l.SetFocus()

	metrics := core.DefaultCellMetrics()
	clickedRow := int(event.Y / metrics.CellHeight)
	clickedIndex := l.scrollOffset + clickedRow

	if clickedIndex >= 0 && clickedIndex < len(l.items) {
		l.SetCurrentIndex(clickedIndex)
	}

	return true
}

// HandleFocusIn is called when focus is gained.
func (l *ListView) HandleFocusIn() {
	l.Update()
}

// HandleFocusOut is called when focus is lost.
func (l *ListView) HandleFocusOut() {
	l.Update()
}

// AccessibleInfo returns accessibility information.
func (l *ListView) AccessibleInfo() core.AccessibleInfo {
	info := l.AccessibleWidget.AccessibleInfo()
	info.Role = core.RoleList
	info.SetSize = len(l.items)

	if l.currentIndex >= 0 {
		info.PositionInSet = l.currentIndex + 1
		if l.currentIndex < len(l.items) {
			info.Value = l.items[l.currentIndex].Text
		}
	}

	if l.selectionMode == MultiSelection || l.selectionMode == ExtendedSelection {
		info.State |= core.StateMultiSelectable
	}

	if !l.IsEnabled() {
		info.State |= core.StateDisabled
	}

	return info
}
