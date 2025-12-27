// Package widgets provides standard UI widgets for the TUI toolkit.
package widgets

import (
	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/style"
)

// ComboBox is a drop-down selection widget.
type ComboBox struct {
	core.WidgetBase
	core.AccessibleWidget

	items        []string
	currentIndex int
	editable     bool
	editText     string
	placeholer   string

	// Drop-down state
	isOpen bool

	// Scroll state for drop-down
	scrollOffset int
	maxVisible   int

	// Callbacks
	onCurrentIndexChanged func(index int)
	onCurrentTextChanged  func(text string)
	onActivated           func(index int)
}

// NewComboBox creates a new combo box.
func NewComboBox() *ComboBox {
	c := &ComboBox{
		currentIndex: -1,
		maxVisible:   8,
	}
	c.WidgetBase = *core.NewWidgetBase()
	c.Init(c) // Enable polymorphic focus handling
	c.SetFocusPolicy(core.StrongFocus)
	c.SetAccessibleRole(core.RoleComboBox)
	return c
}

// AddItem adds an item to the combo box.
func (c *ComboBox) AddItem(text string) {
	c.items = append(c.items, text)
	if c.currentIndex < 0 && len(c.items) == 1 {
		c.SetCurrentIndex(0)
	}
	c.Update()
}

// AddItems adds multiple items to the combo box.
func (c *ComboBox) AddItems(items []string) {
	for _, item := range items {
		c.AddItem(item)
	}
}

// InsertItem inserts an item at the given index.
func (c *ComboBox) InsertItem(index int, text string) {
	if index < 0 {
		index = 0
	}
	if index > len(c.items) {
		index = len(c.items)
	}

	c.items = append(c.items[:index], append([]string{text}, c.items[index:]...)...)

	// Adjust current index if needed
	if c.currentIndex >= index {
		c.currentIndex++
	}
	c.Update()
}

// RemoveItem removes an item at the given index.
func (c *ComboBox) RemoveItem(index int) {
	if index < 0 || index >= len(c.items) {
		return
	}

	c.items = append(c.items[:index], c.items[index+1:]...)

	// Adjust current index
	if c.currentIndex == index {
		if c.currentIndex >= len(c.items) {
			c.currentIndex = len(c.items) - 1
		}
		c.notifyIndexChanged()
	} else if c.currentIndex > index {
		c.currentIndex--
	}
	c.Update()
}

// Clear removes all items.
func (c *ComboBox) Clear() {
	c.items = nil
	c.currentIndex = -1
	c.editText = ""
	c.Update()
}

// Count returns the number of items.
func (c *ComboBox) Count() int {
	return len(c.items)
}

// ItemText returns the text of the item at the given index.
func (c *ComboBox) ItemText(index int) string {
	if index < 0 || index >= len(c.items) {
		return ""
	}
	return c.items[index]
}

// SetItemText sets the text of the item at the given index.
func (c *ComboBox) SetItemText(index int, text string) {
	if index < 0 || index >= len(c.items) {
		return
	}
	c.items[index] = text
	c.Update()
}

// CurrentIndex returns the current selected index.
func (c *ComboBox) CurrentIndex() int {
	return c.currentIndex
}

// SetCurrentIndex sets the current selected index.
func (c *ComboBox) SetCurrentIndex(index int) {
	if index < -1 || index >= len(c.items) {
		return
	}
	if c.currentIndex == index {
		return
	}

	c.currentIndex = index
	if index >= 0 {
		c.editText = c.items[index]
	}
	c.Update()
	c.notifyIndexChanged()
}

// CurrentText returns the current text.
func (c *ComboBox) CurrentText() string {
	if c.editable {
		return c.editText
	}
	if c.currentIndex >= 0 && c.currentIndex < len(c.items) {
		return c.items[c.currentIndex]
	}
	return ""
}

// SetCurrentText sets the current text (for editable combo boxes).
func (c *ComboBox) SetCurrentText(text string) {
	if !c.editable {
		// Find matching item
		for i, item := range c.items {
			if item == text {
				c.SetCurrentIndex(i)
				return
			}
		}
		return
	}

	c.editText = text
	c.Update()

	if c.onCurrentTextChanged != nil {
		c.onCurrentTextChanged(text)
	}
}

// IsEditable returns whether the combo box is editable.
func (c *ComboBox) IsEditable() bool {
	return c.editable
}

// SetEditable sets whether the combo box is editable.
func (c *ComboBox) SetEditable(editable bool) {
	c.editable = editable
	c.Update()
}

// Placeholder returns the placeholder text.
func (c *ComboBox) Placeholder() string {
	return c.placeholer
}

// SetPlaceholder sets the placeholder text.
func (c *ComboBox) SetPlaceholder(text string) {
	c.placeholer = text
	c.Update()
}

// IsOpen returns whether the drop-down is open.
func (c *ComboBox) IsOpen() bool {
	return c.isOpen
}

// ShowPopup opens the drop-down.
func (c *ComboBox) ShowPopup() {
	if len(c.items) == 0 {
		return
	}
	c.isOpen = true
	c.scrollOffset = 0
	// Ensure current item is visible
	if c.currentIndex >= 0 {
		if c.currentIndex < c.scrollOffset {
			c.scrollOffset = c.currentIndex
		} else if c.currentIndex >= c.scrollOffset+c.maxVisible {
			c.scrollOffset = c.currentIndex - c.maxVisible + 1
		}
	}

	// Register popup overlay - find popup controller by walking parent chain
	if pc := c.findPopupController(); pc != nil {
		c.registerPopupOverlay(pc)
	}

	c.Update()
}

// findPopupController walks up the parent chain to find a popup controller.
func (c *ComboBox) findPopupController() core.PopupController {
	// First check if we have one set directly
	if pc := c.PopupController(); pc != nil {
		return pc
	}

	// Walk up parent chain looking for a widget with a popup controller
	current := c.Parent()
	for current != nil {
		if widget, ok := current.(core.Widget); ok {
			if getter, ok := widget.(interface{ PopupController() core.PopupController }); ok {
				if pc := getter.PopupController(); pc != nil {
					return pc
				}
			}
			current = widget.Parent()
		} else {
			break
		}
	}
	return nil
}

// HidePopup closes the drop-down.
func (c *ComboBox) HidePopup() {
	c.isOpen = false

	// Unregister popup overlay - find popup controller by walking parent chain
	if pc := c.findPopupController(); pc != nil {
		pc.UnregisterPopup(c.popupID())
	}

	c.Update()
}

// popupID returns a unique identifier for this ComboBox's popup.
func (c *ComboBox) popupID() string {
	return "combobox-" + c.Name()
}

// registerPopupOverlay registers the popup with the popup controller.
func (c *ComboBox) registerPopupOverlay(pc core.PopupController) {
	bounds := c.Bounds()
	metrics := core.DefaultCellMetrics()

	// Calculate popup height
	popupHeight := len(c.items)
	if popupHeight > c.maxVisible {
		popupHeight = c.maxVisible
	}
	popupHeightUnits := core.Unit(popupHeight) * metrics.CellHeight

	// Get screen bounds to check if we need to pop up instead of down
	screenBounds := pc.ScreenBounds()

	// Get the widget's top-left corner on screen
	widgetScreenPos := pc.MapToScreen(c, core.UnitPoint{X: 0, Y: 0})

	// Default: pop down (below the widget)
	popupY := widgetScreenPos.Y + metrics.CellHeight

	// Check if popup would go below screen - if so, pop up instead
	if popupY+popupHeightUnits > screenBounds.Y+screenBounds.Height {
		// Pop up: position popup above the widget
		popupY = widgetScreenPos.Y - popupHeightUnits
		// Make sure we don't go above the screen either
		if popupY < screenBounds.Y {
			popupY = screenBounds.Y
		}
	}

	popupBounds := core.UnitRect{
		X:      widgetScreenPos.X,
		Y:      popupY,
		Width:  bounds.Width,
		Height: popupHeightUnits,
	}

	// Create popup request
	request := &core.PopupRequest{
		ID:     c.popupID(),
		Bounds: popupBounds,
		Paint: func(p *core.Painter) {
			c.paintPopupOverlay(p, popupBounds)
		},
		HandleMousePress: func(event core.MousePressEvent) bool {
			return c.handlePopupMousePress(event, popupBounds)
		},
	}

	pc.RegisterPopup(request)
}

// SetMaxVisibleItems sets the maximum number of visible items in the drop-down.
func (c *ComboBox) SetMaxVisibleItems(count int) {
	if count < 1 {
		count = 1
	}
	c.maxVisible = count
}

// SetOnCurrentIndexChanged sets the index changed callback.
func (c *ComboBox) SetOnCurrentIndexChanged(handler func(index int)) {
	c.onCurrentIndexChanged = handler
}

// SetOnCurrentTextChanged sets the text changed callback.
func (c *ComboBox) SetOnCurrentTextChanged(handler func(text string)) {
	c.onCurrentTextChanged = handler
}

// SetOnActivated sets the activated callback (when item is selected).
func (c *ComboBox) SetOnActivated(handler func(index int)) {
	c.onActivated = handler
}

func (c *ComboBox) notifyIndexChanged() {
	if c.onCurrentIndexChanged != nil {
		c.onCurrentIndexChanged(c.currentIndex)
	}
	if c.onCurrentTextChanged != nil {
		c.onCurrentTextChanged(c.CurrentText())
	}
}

// SizeHint returns the preferred size.
func (c *ComboBox) SizeHint() core.UnitSize {
	metrics := core.DefaultCellMetrics()

	// Calculate width based on longest item
	maxLen := 10 // Minimum width
	for _, item := range c.items {
		if len(item) > maxLen {
			maxLen = len(item)
		}
	}

	// Add space for dropdown arrow
	width := metrics.TextWidth(maxLen + 3) // " ▼"

	return core.UnitSize{
		Width:  width,
		Height: metrics.TextHeight(1),
	}
}

// IsInlineWidget returns true to indicate this is a text-style widget
// that should receive horizontal margins when in a vertical box layout.
func (c *ComboBox) IsInlineWidget() bool {
	return true
}

// Paint renders the combo box.
func (c *ComboBox) Paint(p *core.Painter) {
	bounds := c.Bounds()
	theme := c.Theme()
	focused := c.HasFocus()
	metrics := p.Metrics()

	// Determine style
	var s style.CellStyle
	if !c.IsEnabled() {
		s = theme.Disabled
	} else if c.isOpen {
		// When popup is open, style like an open menu
		s = theme.MenuBarSelected
	} else if focused {
		s = theme.InputFocused
	} else {
		s = theme.Input
	}

	// Draw background
	p.FillRect(core.UnitRect{Width: bounds.Width, Height: bounds.Height}, ' ', s)

	// Get current text
	text := c.CurrentText()
	if text == "" && c.placeholer != "" {
		text = c.placeholer
		s = s.WithAttrs(style.StyleDim)
	}

	// Calculate text area width (leave space for arrow)
	textWidth := metrics.CharsForWidth(bounds.Width) - 2

	// Draw text
	x := core.Unit(0)
	for i, ch := range text {
		if i >= textWidth {
			break
		}
		p.DrawCell(x, 0, ch, s)
		x += metrics.CellWidth
	}

	// Draw dropdown arrow at the right
	arrowX := bounds.Width - metrics.CellWidth*2
	p.DrawCell(arrowX, 0, ' ', s)
	p.DrawCell(arrowX+metrics.CellWidth, 0, '▼', s)

	// Draw popup if open - only use fallback if no popup controller found
	if c.isOpen && c.findPopupController() == nil {
		c.paintPopup(p)
	}
}

// paintPopup renders the drop-down popup.
func (c *ComboBox) paintPopup(p *core.Painter) {
	bounds := c.Bounds()
	theme := c.Theme()
	metrics := p.Metrics()

	// Calculate popup bounds
	popupHeight := len(c.items)
	if popupHeight > c.maxVisible {
		popupHeight = c.maxVisible
	}

	popupY := metrics.CellHeight // Below the main widget

	// Draw popup background
	popupBounds := core.UnitRect{
		X:      0,
		Y:      popupY,
		Width:  bounds.Width,
		Height: core.Unit(popupHeight) * metrics.CellHeight,
	}
	p.FillRect(popupBounds, ' ', theme.MenuItem)
	p.DrawRect(popupBounds, theme.DefaultBorder, theme.MenuItem)

	// Draw items
	for i := 0; i < popupHeight; i++ {
		itemIndex := c.scrollOffset + i
		if itemIndex >= len(c.items) {
			break
		}

		item := c.items[itemIndex]
		itemY := popupY + core.Unit(i)*metrics.CellHeight

		// Determine item style
		var itemStyle style.CellStyle
		if itemIndex == c.currentIndex {
			itemStyle = theme.MenuItemSelected
		} else {
			itemStyle = theme.MenuItem
		}

		// Draw item background
		p.FillRect(core.UnitRect{
			X:      0,
			Y:      itemY,
			Width:  bounds.Width,
			Height: metrics.CellHeight,
		}, ' ', itemStyle)

		// Draw item text
		x := metrics.CellWidth
		for _, ch := range item {
			if x >= bounds.Width-metrics.CellWidth {
				break
			}
			p.DrawCell(x, itemY, ch, itemStyle)
			x += metrics.CellWidth
		}
	}

	// Draw scroll indicators if needed
	if c.scrollOffset > 0 {
		p.DrawCell(bounds.Width-metrics.CellWidth*2, popupY, '▲', theme.MenuItem)
	}
	if c.scrollOffset+popupHeight < len(c.items) {
		endY := popupY + core.Unit(popupHeight-1)*metrics.CellHeight
		p.DrawCell(bounds.Width-metrics.CellWidth*2, endY, '▼', theme.MenuItem)
	}
}

// paintPopupOverlay renders the popup for the overlay system.
// The popup is rendered at its screen position.
func (c *ComboBox) paintPopupOverlay(p *core.Painter, popupBounds core.UnitRect) {
	theme := c.Theme()
	metrics := p.Metrics()

	// Calculate popup height
	popupHeight := len(c.items)
	if popupHeight > c.maxVisible {
		popupHeight = c.maxVisible
	}

	// Use a painter offset to the popup position
	popupPainter := p.WithOffset(popupBounds.X, popupBounds.Y)

	// Draw popup background
	localBounds := core.UnitRect{
		X:      0,
		Y:      0,
		Width:  popupBounds.Width,
		Height: popupBounds.Height,
	}
	popupPainter.FillRect(localBounds, ' ', theme.MenuItem)
	popupPainter.DrawRect(localBounds, theme.DefaultBorder, theme.MenuItem)

	// Draw items
	for i := 0; i < popupHeight; i++ {
		itemIndex := c.scrollOffset + i
		if itemIndex >= len(c.items) {
			break
		}

		item := c.items[itemIndex]
		itemY := core.Unit(i) * metrics.CellHeight

		// Determine item style
		var itemStyle style.CellStyle
		if itemIndex == c.currentIndex {
			itemStyle = theme.MenuItemSelected
		} else {
			itemStyle = theme.MenuItem
		}

		// Draw item background
		popupPainter.FillRect(core.UnitRect{
			X:      0,
			Y:      itemY,
			Width:  popupBounds.Width,
			Height: metrics.CellHeight,
		}, ' ', itemStyle)

		// Draw item text
		x := metrics.CellWidth
		for _, ch := range item {
			if x >= popupBounds.Width-metrics.CellWidth {
				break
			}
			popupPainter.DrawCell(x, itemY, ch, itemStyle)
			x += metrics.CellWidth
		}
	}

	// Draw scroll indicators if needed
	if c.scrollOffset > 0 {
		popupPainter.DrawCell(popupBounds.Width-metrics.CellWidth*2, 0, '▲', theme.MenuItem)
	}
	if c.scrollOffset+popupHeight < len(c.items) {
		endY := core.Unit(popupHeight-1) * metrics.CellHeight
		popupPainter.DrawCell(popupBounds.Width-metrics.CellWidth*2, endY, '▼', theme.MenuItem)
	}
}

// handlePopupMousePress handles mouse clicks on the popup overlay.
func (c *ComboBox) handlePopupMousePress(event core.MousePressEvent, popupBounds core.UnitRect) bool {
	if event.Button != core.LeftButton {
		return false
	}

	// Check if the click is within the popup bounds
	if event.X >= popupBounds.X && event.X < popupBounds.X+popupBounds.Width &&
		event.Y >= popupBounds.Y && event.Y < popupBounds.Y+popupBounds.Height {
		// Calculate which item was clicked
		metrics := core.DefaultCellMetrics()
		relY := event.Y - popupBounds.Y
		itemIndex := int(relY / metrics.CellHeight)
		actualIndex := c.scrollOffset + itemIndex

		if actualIndex >= 0 && actualIndex < len(c.items) {
			c.SetCurrentIndex(actualIndex)
			if c.onActivated != nil {
				c.onActivated(actualIndex)
			}
			c.HidePopup()
			return true
		}
	}

	// Click was outside popup - close it
	c.HidePopup()
	return true
}

// HandleKeyPress handles keyboard input.
func (c *ComboBox) HandleKeyPress(event core.KeyPressEvent) bool {
	if c.isOpen {
		return c.handlePopupKeyPress(event)
	}

	switch event.Key {
	case " ", "Space", "Enter":
		c.ShowPopup()
		return true

	case "Up":
		if c.currentIndex > 0 {
			c.SetCurrentIndex(c.currentIndex - 1)
		}
		return true

	case "Down":
		if c.currentIndex < len(c.items)-1 {
			c.SetCurrentIndex(c.currentIndex + 1)
		}
		return true

	case "Home":
		if len(c.items) > 0 {
			c.SetCurrentIndex(0)
		}
		return true

	case "End":
		if len(c.items) > 0 {
			c.SetCurrentIndex(len(c.items) - 1)
		}
		return true

	case "F4", "M-Down":
		c.ShowPopup()
		return true
	}

	return false
}

// handlePopupKeyPress handles key events when popup is open.
func (c *ComboBox) handlePopupKeyPress(event core.KeyPressEvent) bool {
	switch event.Key {
	case "Escape":
		c.HidePopup()
		return true

	case "Enter", " ", "Space":
		if c.currentIndex >= 0 {
			if c.onActivated != nil {
				c.onActivated(c.currentIndex)
			}
		}
		c.HidePopup()
		return true

	case "Up":
		if c.currentIndex > 0 {
			c.SetCurrentIndex(c.currentIndex - 1)
			c.ensureVisible(c.currentIndex)
		}
		return true

	case "Down":
		if c.currentIndex < len(c.items)-1 {
			c.SetCurrentIndex(c.currentIndex + 1)
			c.ensureVisible(c.currentIndex)
		}
		return true

	case "PageUp":
		newIndex := c.currentIndex - c.maxVisible
		if newIndex < 0 {
			newIndex = 0
		}
		c.SetCurrentIndex(newIndex)
		c.ensureVisible(newIndex)
		return true

	case "PageDown":
		newIndex := c.currentIndex + c.maxVisible
		if newIndex >= len(c.items) {
			newIndex = len(c.items) - 1
		}
		c.SetCurrentIndex(newIndex)
		c.ensureVisible(newIndex)
		return true

	case "Home":
		c.SetCurrentIndex(0)
		c.scrollOffset = 0
		return true

	case "End":
		c.SetCurrentIndex(len(c.items) - 1)
		c.ensureVisible(len(c.items) - 1)
		return true
	}

	return false
}

// ensureVisible ensures the given index is visible in the popup.
func (c *ComboBox) ensureVisible(index int) {
	if index < c.scrollOffset {
		c.scrollOffset = index
	} else if index >= c.scrollOffset+c.maxVisible {
		c.scrollOffset = index - c.maxVisible + 1
	}
	c.Update()
}

// HandleMousePress handles mouse clicks.
func (c *ComboBox) HandleMousePress(event core.MousePressEvent) bool {
	if event.Button != core.LeftButton {
		return false
	}

	c.SetFocus()

	if c.isOpen {
		// Check if clicked on an item
		metrics := core.DefaultCellMetrics()
		_ = c.Bounds() // unused but kept for consistency
		popupY := metrics.CellHeight

		if event.Y >= popupY && event.Y < popupY+core.Unit(c.maxVisible)*metrics.CellHeight {
			// Clicked in popup area
			itemIndex := int((event.Y - popupY) / metrics.CellHeight)
			actualIndex := c.scrollOffset + itemIndex

			if actualIndex >= 0 && actualIndex < len(c.items) {
				c.SetCurrentIndex(actualIndex)
				if c.onActivated != nil {
					c.onActivated(actualIndex)
				}
				c.HidePopup()
				return true
			}
		}

		// Clicked outside popup
		c.HidePopup()
		return true
	}

	// Toggle popup
	if c.isOpen {
		c.HidePopup()
	} else {
		c.ShowPopup()
	}
	return true
}

// HandleFocusIn is called when focus is gained.
func (c *ComboBox) HandleFocusIn() {
	c.Update()
}

// HandleFocusOut is called when focus is lost.
func (c *ComboBox) HandleFocusOut() {
	c.HidePopup()
	c.Update()
}

// AccessibleInfo returns accessibility information.
func (c *ComboBox) AccessibleInfo() core.AccessibleInfo {
	info := c.AccessibleWidget.AccessibleInfo()
	info.Role = core.RoleComboBox
	info.Value = c.CurrentText()
	info.SetSize = len(c.items)

	if c.currentIndex >= 0 {
		info.PositionInSet = c.currentIndex + 1
	}

	if c.isOpen {
		info.State |= core.StateExpanded
	} else {
		info.State |= core.StateCollapsed
	}

	if !c.IsEnabled() {
		info.State |= core.StateDisabled
	}

	return info
}
