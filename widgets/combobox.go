// Package widgets provides standard UI widgets for the TUI toolkit.
package widgets

import (
	"time"

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
	isOpen     bool
	hoverIndex int // Index of item currently hovered (-1 for none)

	// Scroll state for drop-down
	scrollOffset int
	maxVisible   int

	// Mouse interaction state
	mouseDown     bool       // Mouse button is held down
	dragging      bool       // Actually dragging (mouse moved while down)
	clickMode     bool       // True = click-to-open mode (popup stays open), False = hold-and-drag mode
	mouseDownX    core.Unit  // Initial mouse X position
	mouseDownY    core.Unit  // Initial mouse Y position
	originalIndex int        // Index before popup opened (for cancel on release outside)
	scrollHoverZone int      // -1 = hovering top scroll, 1 = bottom scroll, 0 = none

	// Scrollbar interaction state (click mode only)
	scrollbarDragging   bool  // Whether scrollbar thumb is being dragged
	scrollbarDragStartY int   // Row where drag started
	scrollbarDragOffset int   // Scroll offset when drag started

	// Timer for scroll repeating
	scrollTimer        *DesktopTimer
	scrollTimerStarter func(interval time.Duration, callback func()) *DesktopTimer
	requestUpdate      func()

	// Callbacks
	onCurrentIndexChanged func(index int)
	onCurrentTextChanged  func(text string)
	onActivated           func(index int)
}

// NewComboBox creates a new combo box.
func NewComboBox() *ComboBox {
	c := &ComboBox{
		currentIndex:    -1,
		hoverIndex:      -1,
		maxVisible:      8,
		originalIndex:   -1,
		scrollHoverZone: 0,
	}
	c.WidgetBase = *core.NewWidgetBase()
	c.Init(c) // Enable polymorphic focus handling
	c.SetFocusPolicy(core.StrongFocus)
	c.SetAccessibleRole(core.RoleComboBox)
	return c
}

// SetScrollTimerStarter sets the function used to create scroll timers.
// This should be called by the parent widget (e.g., Desktop) to enable
// timer-based scrolling.
func (c *ComboBox) SetScrollTimerStarter(starter func(interval time.Duration, callback func()) *DesktopTimer) {
	c.scrollTimerStarter = starter
}

// SetRequestUpdate sets the function to call when the widget needs redrawing.
func (c *ComboBox) SetRequestUpdate(fn func()) {
	c.requestUpdate = fn
}

// findTimerProvider walks up the parent chain to find a timer provider.
func (c *ComboBox) findTimerProvider() interface {
	StartRepeatingTimer(interval time.Duration, callback func()) *DesktopTimer
} {
	// Walk up parent chain looking for a widget with StartRepeatingTimer
	current := c.Parent()
	for current != nil {
		if widget, ok := current.(core.Widget); ok {
			if provider, ok := widget.(interface {
				StartRepeatingTimer(interval time.Duration, callback func()) *DesktopTimer
			}); ok {
				return provider
			}
			current = widget.Parent()
		} else {
			break
		}
	}
	return nil
}

// startScrollTimer starts a repeating timer for scrolling.
func (c *ComboBox) startScrollTimer(direction int) {
	c.stopScrollTimer()

	// Find timer provider through parent chain
	timerProvider := c.findTimerProvider()
	if timerProvider == nil && c.scrollTimerStarter == nil {
		return
	}

	callback := func() {
		// Check if we're still hovering the same scroll zone
		if (direction < 0 && c.scrollHoverZone != -1) ||
			(direction > 0 && c.scrollHoverZone != 1) {
			return
		}
		if direction < 0 && c.canScrollUp() {
			c.scrollUp(1)
		} else if direction > 0 && c.canScrollDown() {
			c.scrollDown(1)
		}
		if c.requestUpdate != nil {
			c.requestUpdate()
		}
	}

	if timerProvider != nil {
		c.scrollTimer = timerProvider.StartRepeatingTimer(50*time.Millisecond, callback)
	} else if c.scrollTimerStarter != nil {
		c.scrollTimer = c.scrollTimerStarter(50*time.Millisecond, callback)
	}
}

// stopScrollTimer stops the scroll timer.
func (c *ComboBox) stopScrollTimer() {
	if c.scrollTimer != nil {
		c.scrollTimer.Stop()
		c.scrollTimer = nil
	}
}

// visibleItemCount returns the number of items currently visible in the popup.
// This accounts for scroll indicator rows in drag mode.
func (c *ComboBox) visibleItemCount() int {
	count := c.maxVisible
	if count > len(c.items) {
		count = len(c.items)
	}
	// In drag mode with scrolling, scroll indicators take up rows
	if len(c.items) > c.maxVisible && !c.clickMode {
		if c.canScrollUp() {
			count--
		}
		if c.canScrollDown() {
			count--
		}
	}
	return count
}

// canScrollUp returns true if there are items above the visible area.
func (c *ComboBox) canScrollUp() bool {
	return c.scrollOffset > 0
}

// canScrollDown returns true if there are items below the visible area.
func (c *ComboBox) canScrollDown() bool {
	if c.clickMode {
		// In click mode, all maxVisible rows are item rows
		return c.scrollOffset+c.maxVisible < len(c.items)
	}
	// In drag mode, account for scroll up indicator if present
	effectiveVisible := c.maxVisible
	if c.scrollOffset > 0 {
		effectiveVisible-- // up indicator takes a row
	}
	return c.scrollOffset+effectiveVisible < len(c.items)
}

// scrollUp scrolls the list up by the given amount.
func (c *ComboBox) scrollUp(amount int) {
	c.scrollOffset -= amount
	if c.scrollOffset < 0 {
		c.scrollOffset = 0
	}
	c.Update()
}

// scrollDown scrolls the list down by the given amount.
func (c *ComboBox) scrollDown(amount int) {
	// Calculate effective visible count
	visibleCount := c.maxVisible
	// In drag mode with scrolling, up indicator takes a row when scrollOffset > 0
	// After scrolling down, scrollOffset will definitely be > 0
	if !c.clickMode && len(c.items) > c.maxVisible {
		visibleCount-- // up indicator will take a row
	}
	maxOffset := len(c.items) - visibleCount
	if maxOffset < 0 {
		maxOffset = 0
	}
	c.scrollOffset += amount
	if c.scrollOffset > maxOffset {
		c.scrollOffset = maxOffset
	}
	c.Update()
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
	c.originalIndex = c.currentIndex // Save for cancel on release outside
	c.hoverIndex = c.currentIndex    // Start with current selection highlighted
	c.scrollHoverZone = 0

	// Ensure current item is visible
	if c.currentIndex >= 0 {
		if c.currentIndex < c.scrollOffset {
			c.scrollOffset = c.currentIndex
		} else if c.currentIndex >= c.scrollOffset+c.maxVisible {
			c.scrollOffset = c.currentIndex - c.maxVisible + 1
		}
	}

	// Set up timer provider and request update for scroll timer
	if timerProvider := c.findTimerProvider(); timerProvider != nil {
		if provider, ok := timerProvider.(interface{ RequestUpdate() }); ok {
			c.requestUpdate = provider.RequestUpdate
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
	c.stopScrollTimer()
	c.mouseDown = false
	c.dragging = false
	c.clickMode = false
	c.scrollHoverZone = 0
	c.scrollbarDragging = false

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

	// Get the widget's bottom-left corner on screen (where popup should start)
	widgetBottomPos := pc.MapToScreen(c, core.UnitPoint{X: 0, Y: metrics.CellHeight})

	// Default: pop down (below the widget)
	popupY := widgetBottomPos.Y

	// Check if popup would go below screen - if so, pop up instead
	if popupY+popupHeightUnits > screenBounds.Y+screenBounds.Height {
		// Pop up: position popup above the widget
		// Get widget's top-left corner
		widgetTopPos := pc.MapToScreen(c, core.UnitPoint{X: 0, Y: 0})
		popupY = widgetTopPos.Y - popupHeightUnits
		// Make sure we don't go above the screen either
		if popupY < screenBounds.Y {
			popupY = screenBounds.Y
		}
	}

	popupBounds := core.UnitRect{
		X:      widgetBottomPos.X,
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
		HandleMouseMove: func(event core.MouseMoveEvent) bool {
			return c.handlePopupMouseMove(event, popupBounds)
		},
		HandleMouseRelease: func(event core.MouseReleaseEvent) bool {
			return c.handlePopupMouseRelease(event, popupBounds)
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
	scheme := c.GetScheme()
	focused := c.HasFocus()
	metrics := p.Metrics()

	// Get inherited background to determine pane type
	inheritedBg := c.EffectiveBackgroundColor()
	paneType := style.GetPaneType(inheritedBg)

	// Determine style
	var s style.CellStyle
	if !c.IsEnabled() {
		s = style.DefaultStyle().WithFg(scheme.GetDisabledComboBoxFG()).WithBg(scheme.GetComboBox(paneType).Bg)
	} else if c.isOpen {
		// When popup is open, show pressed style
		s = scheme.GetPressedComboBox(true)
	} else if focused {
		s = scheme.GetFocusedComboBox()
	} else {
		s = scheme.GetComboBox(paneType)
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
	scheme := c.GetScheme()
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
	itemStyle := scheme.GetDropdownItemText()
	p.FillRect(popupBounds, ' ', itemStyle)
	p.DrawRect(popupBounds, c.Theme().DefaultBorder, itemStyle)

	// Draw items
	for i := 0; i < popupHeight; i++ {
		itemIndex := c.scrollOffset + i
		if itemIndex >= len(c.items) {
			break
		}

		item := c.items[itemIndex]
		itemY := popupY + core.Unit(i)*metrics.CellHeight

		// Determine item style
		var s style.CellStyle
		if itemIndex == c.currentIndex {
			s = scheme.GetFocusedDropdownItemText()
		} else {
			s = scheme.GetDropdownItemText()
		}

		// Draw item background
		p.FillRect(core.UnitRect{
			X:      0,
			Y:      itemY,
			Width:  bounds.Width,
			Height: metrics.CellHeight,
		}, ' ', s)

		// Draw item text
		x := metrics.CellWidth
		for _, ch := range item {
			if x >= bounds.Width-metrics.CellWidth {
				break
			}
			p.DrawCell(x, itemY, ch, s)
			x += metrics.CellWidth
		}
	}

	// Draw scroll indicators if needed
	if c.scrollOffset > 0 {
		p.DrawCell(bounds.Width-metrics.CellWidth*2, popupY, '▲', itemStyle)
	}
	if c.scrollOffset+popupHeight < len(c.items) {
		endY := popupY + core.Unit(popupHeight-1)*metrics.CellHeight
		p.DrawCell(bounds.Width-metrics.CellWidth*2, endY, '▼', itemStyle)
	}
}

// paintPopupOverlay renders the popup for the overlay system.
// The popup is rendered at its screen position.
func (c *ComboBox) paintPopupOverlay(p *core.Painter, popupBounds core.UnitRect) {
	scheme := c.GetScheme()
	metrics := p.Metrics()

	// Use a painter offset to the popup position
	popupPainter := p.WithOffset(popupBounds.X, popupBounds.Y)

	// Draw popup background
	localBounds := core.UnitRect{
		X:      0,
		Y:      0,
		Width:  popupBounds.Width,
		Height: popupBounds.Height,
	}
	itemStyle := scheme.GetDropdownItemText()
	popupPainter.FillRect(localBounds, ' ', itemStyle)
	popupPainter.DrawRect(localBounds, c.Theme().DefaultBorder, itemStyle)

	needsScroll := len(c.items) > c.maxVisible

	// In drag mode with scrolling, first/last rows are scroll indicators
	// In click mode, all rows are items (scrollbar on right side)
	showScrollIndicators := needsScroll && !c.clickMode

	// Calculate visible item count and starting Y position
	visibleItems := c.maxVisible
	if visibleItems > len(c.items) {
		visibleItems = len(c.items)
	}

	startY := core.Unit(0)
	itemCount := visibleItems

	if showScrollIndicators {
		// Reserve first row for scroll up indicator if can scroll up
		if c.canScrollUp() {
			centerX := popupBounds.Width / 2
			popupPainter.DrawCell(centerX-metrics.CellWidth*2, 0, '^', itemStyle)
			popupPainter.DrawCell(centerX, 0, '^', itemStyle)
			popupPainter.DrawCell(centerX+metrics.CellWidth*2, 0, '^', itemStyle)
			startY = metrics.CellHeight
			itemCount--
		}
		// Reserve last row for scroll down indicator if can scroll down
		if c.canScrollDown() {
			itemCount--
		}
	}

	// Draw visible items
	for i := 0; i < itemCount; i++ {
		itemIndex := c.scrollOffset + i
		if itemIndex >= len(c.items) {
			break
		}

		item := c.items[itemIndex]
		itemY := startY + core.Unit(i)*metrics.CellHeight

		// Determine item style - highlight hovered item (or current if no hover)
		var s style.CellStyle
		highlightIndex := c.hoverIndex
		if highlightIndex < 0 {
			highlightIndex = c.currentIndex
		}
		if itemIndex == highlightIndex {
			s = scheme.GetFocusedDropdownItemText()
		} else {
			s = scheme.GetDropdownItemText()
		}

		// Draw item background
		popupPainter.FillRect(core.UnitRect{
			X:      0,
			Y:      itemY,
			Width:  popupBounds.Width,
			Height: metrics.CellHeight,
		}, ' ', s)

		// Draw item text
		x := metrics.CellWidth
		for _, ch := range item {
			if x >= popupBounds.Width-metrics.CellWidth {
				break
			}
			popupPainter.DrawCell(x, itemY, ch, s)
			x += metrics.CellWidth
		}
	}

	// Draw scroll down indicator or scrollbar
	if needsScroll {
		if c.clickMode {
			// In click mode, show a proper scrollbar
			c.paintScrollbar(popupPainter, popupBounds.Width, visibleItems)
		} else if c.canScrollDown() {
			// In drag mode, show scroll down indicator on last row
			endY := core.Unit(visibleItems-1) * metrics.CellHeight
			centerX := popupBounds.Width / 2
			popupPainter.DrawCell(centerX-metrics.CellWidth*2, endY, 'v', itemStyle)
			popupPainter.DrawCell(centerX, endY, 'v', itemStyle)
			popupPainter.DrawCell(centerX+metrics.CellWidth*2, endY, 'v', itemStyle)
		}
	}
}

// scrollbarGeometry returns scrollbar dimensions and thumb position.
// Returns: scrollbarX, thumbStart, thumbHeight, trackHeight (all in rows)
func (c *ComboBox) scrollbarGeometry(popupWidth core.Unit, visibleCount int) (scrollbarX core.Unit, thumbStart, thumbHeight, trackHeight int) {
	metrics := core.DefaultCellMetrics()
	totalItems := len(c.items)

	scrollbarX = popupWidth - metrics.CellWidth
	trackHeight = visibleCount

	if totalItems <= visibleCount {
		// No scrolling needed - thumb fills track
		thumbStart = 0
		thumbHeight = trackHeight
		return
	}

	// Calculate thumb size proportional to visible/total ratio
	thumbHeight = visibleCount * visibleCount / totalItems
	if thumbHeight < 1 {
		thumbHeight = 1
	}

	// Calculate thumb position based on scroll offset
	maxScroll := totalItems - visibleCount
	if maxScroll > 0 {
		scrollableTrack := trackHeight - thumbHeight
		thumbStart = c.scrollOffset * scrollableTrack / maxScroll
		if thumbStart+thumbHeight > trackHeight {
			thumbStart = trackHeight - thumbHeight
		}
	}

	return
}

// paintScrollbar draws a vertical scrollbar for the popup.
func (c *ComboBox) paintScrollbar(p *core.Painter, popupWidth core.Unit, visibleCount int) {
	scheme := c.GetScheme()
	metrics := core.DefaultCellMetrics()

	scrollbarX, thumbStart, thumbHeight, trackHeight := c.scrollbarGeometry(popupWidth, visibleCount)

	// Draw scrollbar track
	trackStyle := scheme.GetDropdownScrollbar()
	for i := 0; i < trackHeight; i++ {
		y := core.Unit(i) * metrics.CellHeight
		p.DrawCell(scrollbarX, y, '│', trackStyle)
	}

	// Draw scrollbar thumb
	thumbStyle := scheme.GetDropdownScrollbarThumb()
	for i := 0; i < thumbHeight; i++ {
		y := core.Unit(thumbStart+i) * metrics.CellHeight
		p.DrawCell(scrollbarX, y, '█', thumbStyle)
	}
}

// handlePopupMousePress handles mouse clicks on the popup overlay.
// This is called when clicking within an already-open popup (click mode).
func (c *ComboBox) handlePopupMousePress(event core.MousePressEvent, popupBounds core.UnitRect) bool {
	if event.Button != core.LeftButton {
		return false
	}

	metrics := core.DefaultCellMetrics()

	// Check if the click is within the popup bounds
	if event.X >= popupBounds.X && event.X < popupBounds.X+popupBounds.Width &&
		event.Y >= popupBounds.Y && event.Y < popupBounds.Y+popupBounds.Height {

		// Check for scrollbar clicks in click mode
		if c.clickMode && len(c.items) > c.maxVisible {
			popupHeight := c.maxVisible
			if popupHeight > len(c.items) {
				popupHeight = len(c.items)
			}

			scrollbarX, thumbStart, thumbHeight, _ := c.scrollbarGeometry(popupBounds.Width, popupHeight)
			if event.X >= popupBounds.X+scrollbarX {
				// Click on scrollbar area
				relY := event.Y - popupBounds.Y
				clickedRow := int(relY / metrics.CellHeight)

				// Check if click is on thumb - start drag
				if clickedRow >= thumbStart && clickedRow < thumbStart+thumbHeight {
					c.scrollbarDragging = true
					c.scrollbarDragStartY = clickedRow
					c.scrollbarDragOffset = c.scrollOffset
					c.mouseDown = true
					return true
				}

				// Click on track - page up or down
				visibleCount := popupHeight
				if clickedRow < thumbStart && c.canScrollUp() {
					// Page up
					newOffset := c.scrollOffset - visibleCount
					if newOffset < 0 {
						newOffset = 0
					}
					c.scrollOffset = newOffset
					c.Update()
				} else if clickedRow >= thumbStart+thumbHeight && c.canScrollDown() {
					// Page down
					maxScroll := len(c.items) - visibleCount
					newOffset := c.scrollOffset + visibleCount
					if newOffset > maxScroll {
						newOffset = maxScroll
					}
					c.scrollOffset = newOffset
					c.Update()
				}
				return true
			}
		}

		// Calculate which item was pressed
		relY := event.Y - popupBounds.Y
		rowIndex := int(relY / metrics.CellHeight)

		// In drag mode with scrolling, adjust for scroll up indicator
		needsScroll := len(c.items) > c.maxVisible
		if !c.clickMode && needsScroll && c.scrollOffset > 0 {
			rowIndex-- // First row is scroll up indicator
		}

		actualIndex := c.scrollOffset + rowIndex

		if actualIndex >= 0 && actualIndex < len(c.items) {
			c.hoverIndex = actualIndex
			c.mouseDown = true
			c.mouseDownX = event.X
			c.mouseDownY = event.Y
			c.dragging = false
			c.Update()
		}
		return true
	}

	// Click was outside popup - close it and cancel
	if !c.clickMode {
		// Drag mode - restore original
		c.SetCurrentIndex(c.originalIndex)
	}
	c.HidePopup()
	return true
}

// handlePopupMouseMove handles mouse movement on the popup overlay.
func (c *ComboBox) handlePopupMouseMove(event core.MouseMoveEvent, popupBounds core.UnitRect) bool {
	metrics := core.DefaultCellMetrics()
	popupHeight := c.maxVisible
	if popupHeight > len(c.items) {
		popupHeight = len(c.items)
	}

	// Handle scrollbar thumb dragging first
	if c.scrollbarDragging {
		relY := event.Y - popupBounds.Y
		currentRow := int(relY / metrics.CellHeight)
		rowDelta := currentRow - c.scrollbarDragStartY

		visibleCount := popupHeight
		totalItems := len(c.items)
		maxScroll := totalItems - visibleCount

		if maxScroll > 0 {
			_, _, thumbHeight, trackHeight := c.scrollbarGeometry(popupBounds.Width, visibleCount)
			scrollableTrack := trackHeight - thumbHeight

			if scrollableTrack > 0 {
				// Convert row delta to scroll offset delta
				scrollDelta := rowDelta * maxScroll / scrollableTrack
				newOffset := c.scrollbarDragOffset + scrollDelta

				// Clamp
				if newOffset < 0 {
					newOffset = 0
				} else if newOffset > maxScroll {
					newOffset = maxScroll
				}

				if c.scrollOffset != newOffset {
					c.scrollOffset = newOffset
					c.Update()
				}
			}
		}
		return true
	}

	// Calculate relative position to popup
	relX := event.X - popupBounds.X
	relY := event.Y - popupBounds.Y

	// Check if we need to start dragging (for click mode)
	if c.mouseDown && !c.dragging && c.clickMode {
		dx := event.X - c.mouseDownX
		dy := event.Y - c.mouseDownY
		if dx < 0 {
			dx = -dx
		}
		if dy < 0 {
			dy = -dy
		}
		threshold := metrics.CellWidth / 2
		if dx > threshold || dy > threshold {
			c.dragging = true
		}
	}

	// Check if mouse is within the popup X bounds (for scroll handling)
	inXBounds := relX >= 0 && relX < popupBounds.Width

	// Handle mouse above popup - scroll up
	if relY < 0 && inXBounds {
		if c.scrollHoverZone != -1 {
			c.scrollHoverZone = -1
			if c.canScrollUp() {
				c.scrollUp(1)
				c.startScrollTimer(-1)
			}
		}
		// Keep the topmost visible item highlighted
		if c.hoverIndex != c.scrollOffset {
			c.hoverIndex = c.scrollOffset
			c.Update()
		}
		return true
	}

	// Handle mouse below popup - scroll down
	if relY >= popupBounds.Height && inXBounds {
		if c.scrollHoverZone != 1 {
			c.scrollHoverZone = 1
			if c.canScrollDown() {
				c.scrollDown(1)
				c.startScrollTimer(1)
			}
		}
		// Keep the bottommost visible item highlighted
		lastVisible := c.scrollOffset + popupHeight - 1
		if lastVisible >= len(c.items) {
			lastVisible = len(c.items) - 1
		}
		if c.hoverIndex != lastVisible {
			c.hoverIndex = lastVisible
			c.Update()
		}
		return true
	}

	// Mouse left or right of popup - keep nearest item selected
	if !inXBounds {
		c.scrollHoverZone = 0
		c.stopScrollTimer()
		// Don't change hoverIndex - keep current selection visible
		return c.mouseDown // Consume if dragging
	}

	// Mouse is within the popup bounds
	needsScroll := len(c.items) > c.maxVisible
	showScrollIndicators := needsScroll && !c.clickMode

	// In drag mode with scrolling, check scroll indicator rows
	if showScrollIndicators {
		// First row is scroll up indicator if can scroll up
		if c.canScrollUp() && relY < metrics.CellHeight {
			if c.scrollHoverZone != -1 {
				c.scrollHoverZone = -1
				c.scrollUp(1)
				c.startScrollTimer(-1)
			}
			// Keep topmost visible item highlighted
			if c.hoverIndex != c.scrollOffset {
				c.hoverIndex = c.scrollOffset
				c.Update()
			}
			return true
		}
		// Last row is scroll down indicator if can scroll down
		if c.canScrollDown() && relY >= core.Unit(popupHeight-1)*metrics.CellHeight {
			if c.scrollHoverZone != 1 {
				c.scrollHoverZone = 1
				c.scrollDown(1)
				c.startScrollTimer(1)
			}
			// Keep bottommost visible item highlighted
			lastVisible := c.scrollOffset + c.visibleItemCount() - 1
			if lastVisible >= len(c.items) {
				lastVisible = len(c.items) - 1
			}
			if c.hoverIndex != lastVisible {
				c.hoverIndex = lastVisible
				c.Update()
			}
			return true
		}
	}

	// Not on a scroll indicator - stop scrolling
	c.scrollHoverZone = 0
	c.stopScrollTimer()

	// Calculate which item row the mouse is on
	// In drag mode with scroll indicators, adjust for the top indicator row
	rowIndex := int(relY / metrics.CellHeight)
	if showScrollIndicators && c.canScrollUp() {
		rowIndex-- // First row is scroll indicator, so subtract 1
	}
	actualIndex := c.scrollOffset + rowIndex

	if actualIndex >= 0 && actualIndex < len(c.items) {
		if c.hoverIndex != actualIndex {
			c.hoverIndex = actualIndex
			c.Update()
		}
	}
	return true
}

// handlePopupMouseRelease handles mouse release on the popup overlay.
func (c *ComboBox) handlePopupMouseRelease(event core.MouseReleaseEvent, popupBounds core.UnitRect) bool {
	if event.Button != core.LeftButton {
		return false
	}

	c.stopScrollTimer()
	wasMouseDown := c.mouseDown
	wasDragging := c.dragging
	wasClickMode := c.clickMode
	wasScrollbarDragging := c.scrollbarDragging

	c.mouseDown = false
	c.dragging = false
	c.scrollbarDragging = false

	// If we were dragging the scrollbar, just stop - don't process as item selection
	if wasScrollbarDragging {
		return true
	}

	metrics := core.DefaultCellMetrics()

	// Check if the release is within the popup bounds
	inPopup := event.X >= popupBounds.X && event.X < popupBounds.X+popupBounds.Width &&
		event.Y >= popupBounds.Y && event.Y < popupBounds.Y+popupBounds.Height

	if inPopup {
		// Calculate which item was released on
		relY := event.Y - popupBounds.Y
		rowIndex := int(relY / metrics.CellHeight)

		// In drag mode with scrolling, adjust for scroll up indicator
		needsScroll := len(c.items) > c.maxVisible
		if !wasClickMode && needsScroll && c.scrollOffset > 0 {
			rowIndex-- // First row is scroll up indicator
		}

		actualIndex := c.scrollOffset + rowIndex

		if actualIndex >= 0 && actualIndex < len(c.items) {
			// In click mode with no drag, this confirms selection
			// In drag mode, this also confirms selection
			if wasClickMode && !wasDragging && wasMouseDown {
				// Click mode: second click confirms
				c.SetCurrentIndex(actualIndex)
				if c.onActivated != nil {
					c.onActivated(actualIndex)
				}
				c.HidePopup()
				return true
			} else if !wasClickMode {
				// Drag mode: release inside confirms
				c.SetCurrentIndex(actualIndex)
				if c.onActivated != nil {
					c.onActivated(actualIndex)
				}
				c.HidePopup()
				return true
			}
		}
		// Click mode but didn't click an item - stay open
		return true
	}

	// Release was outside popup
	if wasDragging {
		// Was dragging (in either mode) and released outside - cancel
		c.SetCurrentIndex(c.originalIndex)
		c.HidePopup()
	} else if wasClickMode {
		// In click mode, released outside without drag - dismiss
		c.HidePopup()
	} else {
		// Not in click mode, released outside without drag - enter click mode
		// This is the "click to open" case - popup should stay open
		c.clickMode = true
	}

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
	// Use hoverIndex for navigation, fall back to currentIndex if not set
	getEffectiveIndex := func() int {
		if c.hoverIndex >= 0 {
			return c.hoverIndex
		}
		return c.currentIndex
	}

	switch event.Key {
	case "Escape":
		// Cancel - restore original and close
		c.SetCurrentIndex(c.originalIndex)
		c.HidePopup()
		return true

	case "Enter", " ", "Space":
		// Confirm selection - use hover index if set
		selectedIndex := getEffectiveIndex()
		if selectedIndex >= 0 {
			c.SetCurrentIndex(selectedIndex)
			if c.onActivated != nil {
				c.onActivated(selectedIndex)
			}
		}
		c.HidePopup()
		return true

	case "Up":
		idx := getEffectiveIndex()
		if idx > 0 {
			c.hoverIndex = idx - 1
			c.ensureVisible(c.hoverIndex)
			c.Update()
		}
		return true

	case "Down":
		idx := getEffectiveIndex()
		if idx < len(c.items)-1 {
			c.hoverIndex = idx + 1
			c.ensureVisible(c.hoverIndex)
			c.Update()
		}
		return true

	case "PageUp":
		idx := getEffectiveIndex()
		newIndex := idx - c.maxVisible
		if newIndex < 0 {
			newIndex = 0
		}
		c.hoverIndex = newIndex
		c.ensureVisible(newIndex)
		c.Update()
		return true

	case "PageDown":
		idx := getEffectiveIndex()
		newIndex := idx + c.maxVisible
		if newIndex >= len(c.items) {
			newIndex = len(c.items) - 1
		}
		c.hoverIndex = newIndex
		c.ensureVisible(newIndex)
		c.Update()
		return true

	case "Home":
		c.hoverIndex = 0
		c.scrollOffset = 0
		c.Update()
		return true

	case "End":
		c.hoverIndex = len(c.items) - 1
		c.ensureVisible(len(c.items) - 1)
		c.Update()
		return true
	}

	return false
}

// ensureVisible ensures the given index is visible in the popup.
func (c *ComboBox) ensureVisible(index int) {
	if index < c.scrollOffset {
		c.scrollOffset = index
	} else {
		// Calculate effective visible count accounting for scroll indicators in drag mode
		effectiveVisible := c.maxVisible
		if !c.clickMode && len(c.items) > c.maxVisible {
			// In drag mode with scrolling, indicators take rows
			if c.scrollOffset > 0 {
				effectiveVisible-- // up indicator takes a row
			}
			// If scrolling down would show more items, down indicator also takes a row
			if c.scrollOffset+effectiveVisible < len(c.items) {
				effectiveVisible--
			}
		}
		if index >= c.scrollOffset+effectiveVisible {
			c.scrollOffset = index - effectiveVisible + 1
		}
	}
	c.Update()
}

// HandleMousePress handles mouse clicks on the combobox widget itself.
func (c *ComboBox) HandleMousePress(event core.MousePressEvent) bool {
	if event.Button != core.LeftButton {
		return false
	}

	c.SetFocus()
	metrics := core.DefaultCellMetrics()
	bounds := c.Bounds()

	// Check if click is on the main combobox area (first row)
	if event.Y < metrics.CellHeight && event.X >= 0 && event.X < bounds.Width {
		if c.isOpen {
			if c.clickMode {
				// Click mode: click on button toggles closed
				c.HidePopup()
				return true
			}
			// Drag mode: click on button while open starts drag from button
			c.mouseDown = true
			c.mouseDownX = event.X
			c.mouseDownY = event.Y
			c.dragging = false
			return true
		}

		// Not open - open popup and start drag mode
		c.mouseDown = true
		c.mouseDownX = event.X
		c.mouseDownY = event.Y
		c.dragging = false
		c.clickMode = false // Will switch to click mode on release without drag
		c.ShowPopup()
		return true
	}

	// If popup is open and click is in popup area, let popup handler deal with it
	if c.isOpen {
		popupY := metrics.CellHeight
		popupHeight := c.maxVisible
		if popupHeight > len(c.items) {
			popupHeight = len(c.items)
		}

		if event.Y >= popupY && event.Y < popupY+core.Unit(popupHeight)*metrics.CellHeight {
			// Clicked in popup area (fallback for non-overlay mode)
			itemIndex := int((event.Y - popupY) / metrics.CellHeight)
			actualIndex := c.scrollOffset + itemIndex

			if actualIndex >= 0 && actualIndex < len(c.items) {
				c.hoverIndex = actualIndex
				c.mouseDown = true
				c.mouseDownX = event.X
				c.mouseDownY = event.Y
				c.dragging = false
				c.Update()
			}
			return true
		}

		// Clicked outside popup - cancel
		if !c.clickMode {
			c.SetCurrentIndex(c.originalIndex)
		}
		c.HidePopup()
		return true
	}

	return true
}

// HandleMouseMove handles mouse movement while button may be held.
func (c *ComboBox) HandleMouseMove(event core.MouseMoveEvent) bool {
	if !c.mouseDown || !c.isOpen {
		return false
	}

	metrics := core.DefaultCellMetrics()
	bounds := c.Bounds()

	// Detect if we've started dragging
	if !c.dragging {
		dx := event.X - c.mouseDownX
		dy := event.Y - c.mouseDownY
		if dx < 0 {
			dx = -dx
		}
		if dy < 0 {
			dy = -dy
		}
		threshold := metrics.CellWidth / 2
		if dx > threshold || dy > threshold {
			c.dragging = true
		} else {
			return true // Not dragging yet
		}
	}

	// Calculate popup bounds for hit testing
	popupY := metrics.CellHeight
	popupHeight := c.maxVisible
	if popupHeight > len(c.items) {
		popupHeight = len(c.items)
	}
	popupEndY := popupY + core.Unit(popupHeight)*metrics.CellHeight

	// Handle scrolling when dragging above/below popup
	if event.Y < popupY && event.X >= 0 && event.X < bounds.Width {
		// Above popup - scroll up
		if c.scrollHoverZone != -1 {
			c.scrollHoverZone = -1
			if c.canScrollUp() {
				c.scrollUp(1)
				c.startScrollTimer(-1)
			}
		}
		c.hoverIndex = c.scrollOffset
		c.Update()
		return true
	}

	if event.Y >= popupEndY && event.X >= 0 && event.X < bounds.Width {
		// Below popup - scroll down
		if c.scrollHoverZone != 1 {
			c.scrollHoverZone = 1
			if c.canScrollDown() {
				c.scrollDown(1)
				c.startScrollTimer(1)
			}
		}
		lastVisible := c.scrollOffset + popupHeight - 1
		if lastVisible >= len(c.items) {
			lastVisible = len(c.items) - 1
		}
		c.hoverIndex = lastVisible
		c.Update()
		return true
	}

	// Within popup area
	if event.Y >= popupY && event.Y < popupEndY && event.X >= 0 && event.X < bounds.Width {
		c.scrollHoverZone = 0
		c.stopScrollTimer()

		itemIndex := int((event.Y - popupY) / metrics.CellHeight)
		actualIndex := c.scrollOffset + itemIndex

		if actualIndex >= 0 && actualIndex < len(c.items) {
			if c.hoverIndex != actualIndex {
				c.hoverIndex = actualIndex
				c.Update()
			}
		}
		return true
	}

	// Off to the side - stop scrolling but keep selection
	c.scrollHoverZone = 0
	c.stopScrollTimer()
	return true
}

// HandleMouseRelease handles mouse button release.
func (c *ComboBox) HandleMouseRelease(event core.MouseReleaseEvent) bool {
	if event.Button != core.LeftButton {
		return false
	}

	if !c.isOpen {
		c.mouseDown = false
		c.dragging = false
		return false
	}

	c.stopScrollTimer()
	wasMouseDown := c.mouseDown
	wasDragging := c.dragging

	c.mouseDown = false
	c.dragging = false

	metrics := core.DefaultCellMetrics()
	bounds := c.Bounds()

	// Calculate popup bounds
	popupY := metrics.CellHeight
	popupHeight := c.maxVisible
	if popupHeight > len(c.items) {
		popupHeight = len(c.items)
	}
	popupEndY := popupY + core.Unit(popupHeight)*metrics.CellHeight

	// Check if release is within popup
	inPopup := event.Y >= popupY && event.Y < popupEndY &&
		event.X >= 0 && event.X < bounds.Width

	if inPopup && wasMouseDown {
		if wasDragging {
			// Drag mode - release inside confirms
			itemIndex := int((event.Y - popupY) / metrics.CellHeight)
			actualIndex := c.scrollOffset + itemIndex
			if actualIndex >= 0 && actualIndex < len(c.items) {
				c.SetCurrentIndex(actualIndex)
				if c.onActivated != nil {
					c.onActivated(actualIndex)
				}
			}
			c.HidePopup()
			return true
		} else {
			// Click without drag on combobox - switch to click mode
			// User clicked and released on combobox without moving
			// Popup should stay open for click-to-select
			c.clickMode = true
			return true
		}
	}

	// Release on the combobox button itself (not popup)
	if event.Y < popupY && event.X >= 0 && event.X < bounds.Width && wasMouseDown && !wasDragging {
		// Quick click on combobox - switch to click mode
		c.clickMode = true
		return true
	}

	// Release outside - cancel if dragging, otherwise switch to click mode
	if wasDragging {
		c.SetCurrentIndex(c.originalIndex)
		c.HidePopup()
	} else if wasMouseDown {
		// Released outside without dragging - just switch to click mode
		c.clickMode = true
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
