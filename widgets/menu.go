// Package widgets provides standard UI widgets for the TUI toolkit.
package widgets

import (
	"strings"
	"time"

	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/style"
)

// MenuItem represents an item in a menu.
type MenuItem struct {
	Text            string          // Display text (with & removed, && converted to &)
	rawText         string          // Original text with & markup
	acceleratorChar rune            // The accelerator character (lowercase), 0 if none
	acceleratorPos  int             // Position in display text where accelerator appears, -1 if none
	Shortcut        core.Shortcut
	Icon            *style.TextIcon
	Enabled         bool
	Checkable       bool
	Checked         bool
	Separator       bool // If true, this is a separator line

	// Submenu
	SubMenu *Menu

	// Callbacks
	OnTriggered func()
}

// NewMenuItem creates a new menu item.
func NewMenuItem(text string) *MenuItem {
	displayText, accel, pos := parseAcceleratorTitle(text)
	return &MenuItem{
		Text:            displayText,
		rawText:         text,
		acceleratorChar: accel,
		acceleratorPos:  pos,
		Enabled:         true,
	}
}

// SetText sets the menu item text with accelerator parsing.
func (m *MenuItem) SetText(text string) {
	displayText, accel, pos := parseAcceleratorTitle(text)
	m.rawText = text
	m.Text = displayText
	m.acceleratorChar = accel
	m.acceleratorPos = pos
}

// AcceleratorChar returns the accelerator character (lowercase) or 0 if none.
func (m *MenuItem) AcceleratorChar() rune {
	return m.acceleratorChar
}

// AcceleratorPos returns the position in the display text where the accelerator
// character appears, or -1 if none.
func (m *MenuItem) AcceleratorPos() int {
	return m.acceleratorPos
}

// NewSeparator creates a separator menu item.
func NewSeparator() *MenuItem {
	return &MenuItem{
		Separator: true,
	}
}

// SetShortcut sets the keyboard shortcut.
func (m *MenuItem) SetShortcut(shortcut core.Shortcut) *MenuItem {
	m.Shortcut = shortcut
	return m
}

// SetIcon sets the icon.
func (m *MenuItem) SetIcon(icon *style.TextIcon) *MenuItem {
	m.Icon = icon
	return m
}

// SetCheckable sets whether the item is checkable.
func (m *MenuItem) SetCheckable(checkable bool) *MenuItem {
	m.Checkable = checkable
	return m
}

// SetChecked sets the checked state.
func (m *MenuItem) SetChecked(checked bool) *MenuItem {
	m.Checked = checked
	return m
}

// SetEnabled sets whether the item is enabled.
func (m *MenuItem) SetEnabled(enabled bool) *MenuItem {
	m.Enabled = enabled
	return m
}

// SetSubMenu sets the submenu.
func (m *MenuItem) SetSubMenu(menu *Menu) *MenuItem {
	m.SubMenu = menu
	return m
}

// SetOnTriggered sets the triggered callback.
func (m *MenuItem) SetOnTriggered(handler func()) *MenuItem {
	m.OnTriggered = handler
	return m
}

// Trigger triggers the menu item action.
func (m *MenuItem) Trigger() {
	if m.Checkable {
		m.Checked = !m.Checked
	}
	if m.OnTriggered != nil {
		m.OnTriggered()
	}
}

// Menu represents a dropdown menu.
type Menu struct {
	core.WidgetBase
	core.AccessibleWidget

	title           string // Display title (with & removed, && converted to &)
	rawTitle        string // Original title with & markup
	acceleratorChar rune   // The accelerator character (lowercase), 0 if none
	acceleratorPos  int    // Position in display title where accelerator appears, -1 if none
	items           []*MenuItem
	currentIndex    int
	visible         bool

	// Position when shown as popup
	popupX, popupY core.Unit

	// Parent menu (for submenus)
	parentMenu *Menu
	parentItem *MenuItem

	// Currently open submenu
	activeSubMenu *Menu

	// Scroll state
	scrollOffset    int       // First visible item index
	maxVisible      int       // Max items to show (0 = unlimited)
	scrollHoverTime time.Time // When drag started hovering over scroll indicator
	scrollHoverZone int       // -1 = top indicator, 1 = bottom indicator, 0 = none
	clickedMode     bool      // If true, was opened via click (not drag), release won't dismiss

	// Callbacks
	onAboutToShow func()
	onAboutToHide func()
}

// parseAcceleratorTitle parses a title with & markup.
// Returns: display title, accelerator character (lowercase), position in display title
// Examples: "&File" -> "File", 'f', 0
//
//	"E&xit" -> "Exit", 'x', 1
//	"Save && Exit" -> "Save & Exit", 0, -1
func parseAcceleratorTitle(raw string) (display string, accel rune, pos int) {
	pos = -1
	runes := []rune(raw)
	var result []rune

	for i := 0; i < len(runes); i++ {
		if runes[i] == '&' {
			if i+1 < len(runes) && runes[i+1] == '&' {
				// Escaped ampersand
				result = append(result, '&')
				i++ // Skip next &
			} else if i+1 < len(runes) {
				// Accelerator - next char is the accelerator
				if pos < 0 { // Only use first accelerator
					pos = len(result)
					accel = rune(strings.ToLower(string(runes[i+1]))[0])
				}
				result = append(result, runes[i+1])
				i++ // Skip the accelerator char (we already added it)
			}
			// else: trailing & is just dropped
		} else {
			result = append(result, runes[i])
		}
	}

	display = string(result)
	return
}

// NewMenu creates a new menu.
func NewMenu(title string) *Menu {
	displayTitle, accel, pos := parseAcceleratorTitle(title)
	m := &Menu{
		rawTitle:        title,
		title:           displayTitle,
		acceleratorChar: accel,
		acceleratorPos:  pos,
		currentIndex:    -1,
		maxVisible:      12, // Default max visible items
	}
	m.WidgetBase = *core.NewWidgetBase()
	// Note: Menu doesn't call Init because it has a Show(x,y) method
	// with different signature than Widget.Show()
	m.SetFocusPolicy(core.StrongFocus)
	m.SetAccessibleRole(core.RoleMenu)
	m.SetAccessibleName(displayTitle)
	return m
}

// SetMaxVisible sets the maximum number of visible items (0 = unlimited).
func (m *Menu) SetMaxVisible(max int) {
	m.maxVisible = max
}

// Title returns the menu title.
func (m *Menu) Title() string {
	return m.title
}

// SetTitle sets the menu title.
func (m *Menu) SetTitle(title string) {
	displayTitle, accel, pos := parseAcceleratorTitle(title)
	m.rawTitle = title
	m.title = displayTitle
	m.acceleratorChar = accel
	m.acceleratorPos = pos
	m.SetAccessibleName(displayTitle)
}

// AcceleratorChar returns the accelerator character (lowercase) or 0 if none.
func (m *Menu) AcceleratorChar() rune {
	return m.acceleratorChar
}

// AcceleratorPos returns the position in the display title where the accelerator
// character appears, or -1 if none.
func (m *Menu) AcceleratorPos() int {
	return m.acceleratorPos
}

// AddItem adds an item to the menu.
func (m *Menu) AddItem(item *MenuItem) {
	m.items = append(m.items, item)
}

// AddAction adds an action as a menu item.
func (m *Menu) AddAction(action *core.Action) *MenuItem {
	item := NewMenuItem(action.Text)
	item.Shortcut = action.Shortcut
	item.Enabled = action.Enabled
	item.OnTriggered = action.OnTriggered
	m.AddItem(item)
	return item
}

// AddSeparator adds a separator.
func (m *Menu) AddSeparator() {
	m.AddItem(NewSeparator())
}

// AddMenu adds a submenu.
func (m *Menu) AddMenu(submenu *Menu) *MenuItem {
	item := NewMenuItem(submenu.title)
	item.SubMenu = submenu
	submenu.parentMenu = m
	submenu.parentItem = item
	m.AddItem(item)
	return item
}

// InsertItem inserts an item at the given index.
func (m *Menu) InsertItem(index int, item *MenuItem) {
	if index < 0 {
		index = 0
	}
	if index > len(m.items) {
		index = len(m.items)
	}
	m.items = append(m.items[:index], append([]*MenuItem{item}, m.items[index:]...)...)
}

// RemoveItem removes an item.
func (m *Menu) RemoveItem(item *MenuItem) {
	for i, it := range m.items {
		if it == item {
			m.items = append(m.items[:i], m.items[i+1:]...)
			break
		}
	}
}

// Clear removes all items.
func (m *Menu) Clear() {
	m.items = nil
	m.currentIndex = -1
}

// Items returns all items.
func (m *Menu) Items() []*MenuItem {
	return m.items
}

// ItemAt returns the item at the given index.
func (m *Menu) ItemAt(index int) *MenuItem {
	if index < 0 || index >= len(m.items) {
		return nil
	}
	return m.items[index]
}

// CurrentItem returns the currently highlighted item.
func (m *Menu) CurrentItem() *MenuItem {
	return m.ItemAt(m.currentIndex)
}

// IsVisible returns whether the menu is visible.
func (m *Menu) IsVisible() bool {
	return m.visible
}

// Show shows the menu at the given position.
func (m *Menu) Show(x, y core.Unit) {
	if m.onAboutToShow != nil {
		m.onAboutToShow()
	}

	m.popupX = x
	m.popupY = y
	m.visible = true
	m.currentIndex = -1 // No item selected until user hovers over one
	m.scrollOffset = 0
	m.scrollHoverZone = 0
	m.scrollHoverTime = time.Time{}
	// Note: Don't call SetFocus() here - the MenuBar retains focus and forwards
	// key events to the active menu. Taking focus would trigger HandleFocusOut
	// on the MenuBar which would close the menu we just opened.
	m.Update()
}

// SetClickedMode sets whether the menu is in clicked mode (release won't dismiss).
func (m *Menu) SetClickedMode(clicked bool) {
	m.clickedMode = clicked
}

// IsClickedMode returns whether the menu is in clicked mode.
func (m *Menu) IsClickedMode() bool {
	return m.clickedMode
}

// Hide hides the menu.
func (m *Menu) Hide() {
	if m.activeSubMenu != nil {
		m.activeSubMenu.Hide()
		m.activeSubMenu = nil
	}

	if m.onAboutToHide != nil {
		m.onAboutToHide()
	}

	m.visible = false
	m.currentIndex = -1
	m.Update()
}

// SetOnAboutToShow sets the about to show callback.
func (m *Menu) SetOnAboutToShow(handler func()) {
	m.onAboutToShow = handler
}

// SetOnAboutToHide sets the about to hide callback.
func (m *Menu) SetOnAboutToHide(handler func()) {
	m.onAboutToHide = handler
}

// findNextEnabled finds the next enabled item.
func (m *Menu) findNextEnabled(from int) int {
	for i := 1; i <= len(m.items); i++ {
		idx := (from + i) % len(m.items)
		if idx < 0 {
			idx = len(m.items) + idx
		}
		item := m.items[idx]
		if !item.Separator && item.Enabled {
			return idx
		}
	}
	return -1
}

// findPrevEnabled finds the previous enabled item.
func (m *Menu) findPrevEnabled(from int) int {
	for i := 1; i <= len(m.items); i++ {
		idx := from - i
		if idx < 0 {
			idx = len(m.items) + idx
		}
		item := m.items[idx]
		if !item.Separator && item.Enabled {
			return idx
		}
	}
	return -1
}

// calculateSize calculates the menu size.
func (m *Menu) calculateSize() core.UnitSize {
	metrics := core.DefaultCellMetrics()

	maxWidth := 0
	for _, item := range m.items {
		width := len(item.Text)
		if item.Shortcut != "" {
			width += 3 + len(item.Shortcut.DisplayString())
		}
		if item.SubMenu != nil {
			width += 3 // For submenu arrow
		}
		if width > maxWidth {
			maxWidth = width
		}
	}

	// Add padding (gutter: 3 cells, content space: 1 cell, right border: 1 cell)
	maxWidth += 5

	// Calculate visible item count
	visibleItems := len(m.items)
	if m.maxVisible > 0 && visibleItems > m.maxVisible {
		visibleItems = m.maxVisible
	}

	// Add space for scroll indicators if needed
	height := visibleItems
	if m.needsScrolling() {
		height += 2 // One row for each scroll indicator
	}

	return core.UnitSize{
		Width:  core.Unit(maxWidth) * metrics.CellWidth,
		Height: core.Unit(height) * metrics.CellHeight,
	}
}

// needsScrolling returns true if the menu has more items than maxVisible.
func (m *Menu) needsScrolling() bool {
	return m.maxVisible > 0 && len(m.items) > m.maxVisible
}

// visibleItemCount returns the number of items that can be shown at once.
func (m *Menu) visibleItemCount() int {
	if m.maxVisible <= 0 || len(m.items) <= m.maxVisible {
		return len(m.items)
	}
	return m.maxVisible
}

// canScrollUp returns true if there are items above the visible area.
func (m *Menu) canScrollUp() bool {
	return m.scrollOffset > 0
}

// canScrollDown returns true if there are items below the visible area.
func (m *Menu) canScrollDown() bool {
	return m.scrollOffset+m.visibleItemCount() < len(m.items)
}

// scrollUp scrolls the menu up by the given number of items.
func (m *Menu) scrollUp(count int) {
	m.scrollOffset -= count
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
	}
	m.Update()
}

// scrollDown scrolls the menu down by the given number of items.
func (m *Menu) scrollDown(count int) {
	maxOffset := len(m.items) - m.visibleItemCount()
	m.scrollOffset += count
	if m.scrollOffset > maxOffset {
		m.scrollOffset = maxOffset
	}
	m.Update()
}

// scrollPageUp scrolls up by one page.
func (m *Menu) scrollPageUp() {
	m.scrollUp(m.visibleItemCount())
}

// scrollPageDown scrolls down by one page.
func (m *Menu) scrollPageDown() {
	m.scrollDown(m.visibleItemCount())
}

// ensureVisible ensures the given item index is visible.
func (m *Menu) ensureVisible(index int) {
	if index < 0 || !m.needsScrolling() {
		return
	}

	// If item is above visible area, scroll up
	if index < m.scrollOffset {
		m.scrollOffset = index
	}

	// If item is below visible area, scroll down
	visibleEnd := m.scrollOffset + m.visibleItemCount() - 1
	if index > visibleEnd {
		m.scrollOffset = index - m.visibleItemCount() + 1
	}
}

// SizeHint returns the preferred size.
func (m *Menu) SizeHint() core.UnitSize {
	return m.calculateSize()
}

// DropdownBounds returns the bounds of the visible dropdown menu.
// Returns an empty rect if the menu is not visible.
func (m *Menu) DropdownBounds() core.UnitRect {
	if !m.visible {
		return core.UnitRect{}
	}
	size := m.calculateSize()
	return core.UnitRect{
		X:      m.popupX,
		Y:      m.popupY,
		Width:  size.Width,
		Height: size.Height,
	}
}

// Paint renders the menu.
func (m *Menu) Paint(p *core.Painter) {
	if !m.visible {
		return
	}

	theme := m.Theme()
	metrics := p.Metrics()
	size := m.calculateSize()
	needsScroll := m.needsScrolling()

	// Draw menu background with border
	menuBounds := core.UnitRect{
		X:      m.popupX,
		Y:      m.popupY,
		Width:  size.Width,
		Height: size.Height,
	}
	p.FillRect(menuBounds, ' ', theme.MenuItem)
	p.DrawRect(menuBounds, theme.DefaultBorder, theme.MenuItem)

	// Track Y offset for drawing
	currentY := m.popupY

	// Draw top scroll indicator if needed
	if needsScroll {
		indicatorStyle := theme.MenuItem
		if m.canScrollUp() {
			// Draw "^ ^ ^" centered
			centerX := m.popupX + size.Width/2
			p.DrawCell(centerX-metrics.CellWidth*2, currentY, '^', indicatorStyle)
			p.DrawCell(centerX, currentY, '^', indicatorStyle)
			p.DrawCell(centerX+metrics.CellWidth*2, currentY, '^', indicatorStyle)
		}
		currentY += metrics.CellHeight
	}

	// Draw visible items
	visibleCount := m.visibleItemCount()
	for i := 0; i < visibleCount; i++ {
		itemIndex := m.scrollOffset + i
		if itemIndex >= len(m.items) {
			break
		}
		item := m.items[itemIndex]
		itemY := currentY

		// Determine style
		var s style.CellStyle
		if item.Separator {
			s = theme.MenuSeparator
		} else if !item.Enabled {
			s = theme.MenuItemDisabled
		} else if itemIndex == m.currentIndex {
			s = theme.MenuItemSelected
		} else {
			s = theme.MenuItem
		}

		// Gutter area: 3 cells (border + checkmark + 1 space) in regular white
		// Content area: rest of the item in bright white (for non-selected items)
		gutterWidth := metrics.CellWidth * 3
		gutterStyle := s
		contentStyle := s
		if !item.Separator && item.Enabled && itemIndex != m.currentIndex {
			// Use bright white background for content area of enabled, non-selected items
			contentStyle = s.WithBg(style.ColorBrightWhite)
		}

		// Draw gutter background (regular white)
		p.FillRect(core.UnitRect{
			X:      m.popupX,
			Y:      itemY,
			Width:  gutterWidth,
			Height: metrics.CellHeight,
		}, ' ', gutterStyle)

		// Draw content background (bright white for enabled items)
		contentBgStyle := contentStyle
		if item.Separator {
			// Separators use bright white for content area too
			contentBgStyle = s.WithBg(style.ColorBrightWhite)
		}
		p.FillRect(core.UnitRect{
			X:      m.popupX + gutterWidth,
			Y:      itemY,
			Width:  size.Width - gutterWidth,
			Height: metrics.CellHeight,
		}, ' ', contentBgStyle)

		if item.Separator {
			// Draw separator line - gutter portion in regular style, content in bright white
			separatorContentStyle := s.WithBg(style.ColorBrightWhite)
			for x := m.popupX + metrics.CellWidth; x < m.popupX+size.Width-metrics.CellWidth; x += metrics.CellWidth {
				if x < m.popupX+gutterWidth {
					p.DrawCell(x, itemY, '─', gutterStyle)
				} else {
					p.DrawCell(x, itemY, '─', separatorContentStyle)
				}
			}
			currentY += metrics.CellHeight
			continue
		}

		x := m.popupX + metrics.CellWidth

		// Draw checkmark or icon in gutter area
		if item.Checkable {
			if item.Checked {
				p.DrawCell(x, itemY, '✓', gutterStyle)
			}
		} else if item.Icon != nil && len(item.Icon.Cells) > 0 {
			cell := item.Icon.Cells[0]
			p.DrawCell(x, itemY, cell.Char, cell.Style)
		}
		x += metrics.CellWidth * 2 // Move past checkmark + 1 gutter space

		// Draw a space in content area before text
		p.DrawCell(x, itemY, ' ', contentStyle)
		x += metrics.CellWidth

		// Now draw text with accelerator highlighting
		var accelStyle style.CellStyle
		if itemIndex == m.currentIndex {
			// Selected item: dark magenta on cyan
			accelStyle = style.DefaultStyle().WithFg(style.ColorMagenta).WithBg(style.ColorCyan)
		} else {
			// Normal item: red on bright white
			accelStyle = style.DefaultStyle().WithFg(style.ColorRed).WithBg(style.ColorBrightWhite)
		}
		for idx, ch := range item.Text {
			charStyle := contentStyle
			// Highlight accelerator for enabled items
			if item.Enabled && idx == item.acceleratorPos {
				charStyle = accelStyle
			}
			p.DrawCell(x, itemY, ch, charStyle)
			x += metrics.CellWidth
		}

		// Draw shortcut or submenu arrow at the right (in content area)
		if item.SubMenu != nil {
			arrowX := m.popupX + size.Width - metrics.CellWidth*2
			p.DrawCell(arrowX, itemY, '▸', contentStyle)
		} else if item.Shortcut != "" {
			shortcutStr := item.Shortcut.DisplayString()
			shortcutX := m.popupX + size.Width - core.Unit(len(shortcutStr)+2)*metrics.CellWidth
			shortcutStyle := contentStyle
			if item.Enabled {
				shortcutStyle = contentStyle.WithAttrs(style.StyleDim)
			}
			for _, ch := range shortcutStr {
				p.DrawCell(shortcutX, itemY, ch, shortcutStyle)
				shortcutX += metrics.CellWidth
			}
		}

		currentY += metrics.CellHeight
	}

	// Draw bottom scroll indicator if needed
	if needsScroll {
		indicatorStyle := theme.MenuItem
		if m.canScrollDown() {
			// Draw "v v v" centered
			centerX := m.popupX + size.Width/2
			p.DrawCell(centerX-metrics.CellWidth*2, currentY, 'v', indicatorStyle)
			p.DrawCell(centerX, currentY, 'v', indicatorStyle)
			p.DrawCell(centerX+metrics.CellWidth*2, currentY, 'v', indicatorStyle)
		}
	}

	// Draw active submenu
	if m.activeSubMenu != nil {
		m.activeSubMenu.Paint(p)
	}
}

// HandleKeyPress handles keyboard input.
func (m *Menu) HandleKeyPress(event core.KeyPressEvent) bool {
	// Handle submenu first
	if m.activeSubMenu != nil {
		if m.activeSubMenu.HandleKeyPress(event) {
			return true
		}
	}

	switch event.Key {
	case "Up":
		m.currentIndex = m.findPrevEnabled(m.currentIndex)
		m.ensureVisible(m.currentIndex)
		m.closeSubMenu()
		m.Update()
		return true

	case "Down":
		m.currentIndex = m.findNextEnabled(m.currentIndex)
		m.ensureVisible(m.currentIndex)
		m.closeSubMenu()
		m.Update()
		return true

	case "Left":
		if m.parentMenu != nil {
			m.Hide()
			return true
		}
		return false // Let menu bar handle it

	case "Right":
		item := m.CurrentItem()
		if item != nil && item.SubMenu != nil {
			m.openSubMenu(item)
			return true
		}
		return false // Let menu bar handle it

	case "Enter", " ", "Space":
		item := m.CurrentItem()
		if item != nil {
			if item.SubMenu != nil {
				m.openSubMenu(item)
			} else {
				m.triggerItem(item)
			}
			return true
		}

	case "Escape":
		if m.parentMenu != nil {
			// Submenu - hide it and return to parent menu
			m.Hide()
			return true
		}
		// Top-level menu - let menu bar handle closing for proper cleanup
		// (MenuBar.CloseMenu will call Hide on us)
		return false

	case "Home":
		m.currentIndex = m.findNextEnabled(-1)
		m.scrollOffset = 0
		m.closeSubMenu()
		m.Update()
		return true

	case "End":
		m.currentIndex = m.findPrevEnabled(0)
		m.ensureVisible(m.currentIndex)
		m.closeSubMenu()
		m.Update()
		return true

	case "PageUp":
		m.scrollPageUp()
		// Move current index to top of visible area
		if m.currentIndex >= 0 {
			m.currentIndex = m.scrollOffset
			for m.currentIndex < len(m.items) && (m.items[m.currentIndex].Separator || !m.items[m.currentIndex].Enabled) {
				m.currentIndex++
			}
		}
		m.closeSubMenu()
		m.Update()
		return true

	case "PageDown":
		m.scrollPageDown()
		// Move current index to bottom of visible area
		if m.currentIndex >= 0 {
			m.currentIndex = m.scrollOffset + m.visibleItemCount() - 1
			if m.currentIndex >= len(m.items) {
				m.currentIndex = len(m.items) - 1
			}
			for m.currentIndex >= 0 && (m.items[m.currentIndex].Separator || !m.items[m.currentIndex].Enabled) {
				m.currentIndex--
			}
		}
		m.closeSubMenu()
		m.Update()
		return true
	}

	// Check for accelerator keys (single character, case insensitive, no modifiers)
	// These work when a menu is dropped down
	if len(event.Key) == 1 {
		letter := event.Key[0]
		// Match letters and digits without any modifier prefix
		if (letter >= 'a' && letter <= 'z') || (letter >= 'A' && letter <= 'Z') ||
			(letter >= '0' && letter <= '9') {
			key := rune(strings.ToLower(string(letter))[0])
			for i, item := range m.items {
				if !item.Separator && item.acceleratorChar == key {
					m.currentIndex = i
					if !item.Enabled {
						// Disabled items with matching accelerator: do nothing but consume the key
						m.Update()
						return true
					}
					if item.SubMenu != nil {
						m.openSubMenu(item)
					} else {
						m.triggerItem(item)
					}
					return true
				}
			}
		}
	}

	return false
}

// openSubMenu opens a submenu.
func (m *Menu) openSubMenu(item *MenuItem) {
	if item.SubMenu == nil {
		return
	}

	m.closeSubMenu()

	metrics := core.DefaultCellMetrics()
	size := m.calculateSize()
	needsScroll := m.needsScrolling()

	// Position submenu to the right of current item
	itemIndex := -1
	for i, it := range m.items {
		if it == item {
			itemIndex = i
			break
		}
	}

	// Calculate Y position accounting for scroll offset and indicators
	visibleIndex := itemIndex - m.scrollOffset
	subY := m.popupY + core.Unit(visibleIndex)*metrics.CellHeight
	if needsScroll {
		subY += metrics.CellHeight // Account for top scroll indicator row
	}

	subX := m.popupX + size.Width

	m.activeSubMenu = item.SubMenu
	item.SubMenu.Show(subX, subY)
}

// closeSubMenu closes the active submenu.
func (m *Menu) closeSubMenu() {
	if m.activeSubMenu != nil {
		m.activeSubMenu.Hide()
		m.activeSubMenu = nil
	}
}

// triggerItem triggers a menu item and closes the menu.
func (m *Menu) triggerItem(item *MenuItem) {
	// Close all menus up to the menu bar
	menu := m
	for menu != nil {
		menu.Hide()
		menu = menu.parentMenu
	}

	// Trigger the action
	item.Trigger()
}

// HandleMousePress handles mouse clicks.
func (m *Menu) HandleMousePress(event core.MousePressEvent) bool {
	if !m.visible {
		return false
	}

	// Check submenu first
	if m.activeSubMenu != nil && m.activeSubMenu.HandleMousePress(event) {
		return true
	}

	metrics := core.DefaultCellMetrics()
	size := m.calculateSize()
	needsScroll := m.needsScrolling()

	// Check if click is in menu bounds
	if event.X >= m.popupX && event.X < m.popupX+size.Width &&
		event.Y >= m.popupY && event.Y < m.popupY+size.Height {

		// Calculate which row was clicked
		rowIndex := int((event.Y - m.popupY) / metrics.CellHeight)

		// Check if clicking on scroll indicators
		if needsScroll {
			// Top scroll indicator (row 0)
			if rowIndex == 0 && m.canScrollUp() {
				// Click on scroll indicator - transition to clicked mode and scroll
				m.clickedMode = true
				m.scrollUp(1)
				return true
			}

			// Bottom scroll indicator (last row)
			lastRow := m.visibleItemCount() + 1 // +1 for top indicator
			if rowIndex == lastRow && m.canScrollDown() {
				// Click on scroll indicator - transition to clicked mode and scroll
				m.clickedMode = true
				m.scrollDown(1)
				return true
			}

			// Adjust row index for items (subtract 1 for top indicator)
			rowIndex--
		}

		// Convert row to item index
		itemIndex := m.scrollOffset + rowIndex
		if itemIndex >= 0 && itemIndex < len(m.items) {
			item := m.items[itemIndex]
			if !item.Separator && item.Enabled {
				if item.SubMenu != nil {
					m.currentIndex = itemIndex
					m.openSubMenu(item)
				} else {
					m.triggerItem(item)
				}
			}
		}
		return true
	}

	// Click outside - close menu
	m.Hide()
	return false
}

// HandleMouseMove handles mouse movement for hover-scrolling.
func (m *Menu) HandleMouseMove(event core.MouseMoveEvent) bool {
	if !m.visible || !m.needsScrolling() {
		m.scrollHoverZone = 0
		return false
	}

	metrics := core.DefaultCellMetrics()
	size := m.calculateSize()

	// Check if mouse is in menu bounds
	if event.X < m.popupX || event.X >= m.popupX+size.Width ||
		event.Y < m.popupY || event.Y >= m.popupY+size.Height {
		m.scrollHoverZone = 0
		return false
	}

	// Calculate which row the mouse is in
	rowIndex := int((event.Y - m.popupY) / metrics.CellHeight)
	lastRow := m.visibleItemCount() + 1 // +1 for top indicator

	// Check if on top scroll indicator
	if rowIndex == 0 && m.canScrollUp() {
		if m.scrollHoverZone != -1 {
			m.scrollHoverZone = -1
			m.scrollHoverTime = time.Now()
		} else {
			// Check if we've been hovering for 1 second
			if time.Since(m.scrollHoverTime) >= time.Second {
				m.scrollPageUp()
				m.scrollHoverTime = time.Now() // Reset for next page scroll
			}
		}
		return true
	}

	// Check if on bottom scroll indicator
	if rowIndex == lastRow && m.canScrollDown() {
		if m.scrollHoverZone != 1 {
			m.scrollHoverZone = 1
			m.scrollHoverTime = time.Now()
		} else {
			// Check if we've been hovering for 1 second
			if time.Since(m.scrollHoverTime) >= time.Second {
				m.scrollPageDown()
				m.scrollHoverTime = time.Now() // Reset for next page scroll
			}
		}
		return true
	}

	// Not on a scroll indicator
	m.scrollHoverZone = 0

	// Update highlighted item
	if rowIndex >= 0 {
		adjustedRow := rowIndex - 1 // Subtract 1 for top indicator
		itemIndex := m.scrollOffset + adjustedRow
		if itemIndex >= 0 && itemIndex < len(m.items) {
			item := m.items[itemIndex]
			if !item.Separator && item.Enabled {
				m.currentIndex = itemIndex
				m.Update()
			}
		}
	}

	return true
}

// HandleFocusOut is called when focus is lost.
func (m *Menu) HandleFocusOut() {
	// Only hide if focus didn't go to a submenu
	if m.activeSubMenu == nil || !m.activeSubMenu.HasFocus() {
		m.Hide()
	}
}

// AccessibleInfo returns accessibility information.
func (m *Menu) AccessibleInfo() core.AccessibleInfo {
	info := m.AccessibleWidget.AccessibleInfo()
	info.Role = core.RoleMenu
	info.Name = m.title
	info.SetSize = len(m.items)

	if m.currentIndex >= 0 && m.currentIndex < len(m.items) {
		item := m.items[m.currentIndex]
		info.PositionInSet = m.currentIndex + 1
		info.Value = item.Text
	}

	return info
}

// MenuBar is a horizontal bar of menus.
type MenuBar struct {
	core.WidgetBase
	core.AccessibleWidget

	menus        []*Menu
	currentIndex int
	activeMenu   *Menu

	// Appearance
	showShortcuts bool

	// Scroll state for overflow handling
	scrollOffset int // Index of first visible menu

	// Accelerator display state
	// Accelerators are shown when:
	// - Menu bar has focus and no menu is down, OR
	// - No keybinding conflict exists for the accelerator key
	acceleratorsActive bool // True when menu bar focused with no menu down

	// Drag tracking for click-and-drag menu navigation
	mouseDown  bool // Mouse button is held down
	dragging   bool // Actually dragging (mouse moved while down)
	mouseDownX core.Unit
	mouseDownY core.Unit

	// Callback when a menu is opened
	onMenuOpen func()

	// Callback when menu bar is dismissed without action (e.g., Escape)
	onMenuDismiss func()
}

// NewMenuBar creates a new menu bar.
func NewMenuBar() *MenuBar {
	m := &MenuBar{
		currentIndex:  -1,
		showShortcuts: true,
	}
	m.WidgetBase = *core.NewWidgetBase()
	m.Init(m)
	m.SetFocusPolicy(core.StrongFocus)
	m.SetAccessibleRole(core.RoleMenuBar)
	return m
}

// SetOnMenuOpen sets a callback that is called when a menu is opened.
func (m *MenuBar) SetOnMenuOpen(callback func()) {
	m.onMenuOpen = callback
}

// SetOnMenuDismiss sets a callback that is called when the menu bar is dismissed without action.
func (m *MenuBar) SetOnMenuDismiss(callback func()) {
	m.onMenuDismiss = callback
}

// calculateTotalMenusWidth returns the total width needed for all menus.
func (m *MenuBar) calculateTotalMenusWidth() core.Unit {
	metrics := core.DefaultCellMetrics()
	total := core.Unit(0)
	for _, menu := range m.menus {
		total += core.Unit(len(menu.title)+2) * metrics.CellWidth
	}
	return total
}

// dateTimeWidth returns the width reserved for the date/time display.
func (m *MenuBar) dateTimeWidth() core.Unit {
	metrics := core.DefaultCellMetrics()
	// " Mon Jan 02 15:04 " = 18 chars
	return 18 * metrics.CellWidth
}

// scrollButtonWidth returns the width of each scroll button.
func (m *MenuBar) scrollButtonWidth() core.Unit {
	return core.DefaultCellMetrics().TextWidth(3) // [<] or [>]
}

// menusNeedScrolling returns true if menus don't fit and need scroll buttons.
func (m *MenuBar) menusNeedScrolling() bool {
	bounds := m.Bounds()
	availableWidth := bounds.Width - m.dateTimeWidth()
	return m.calculateTotalMenusWidth() > availableWidth
}

// canScrollLeft returns true if there are menus to the left.
func (m *MenuBar) canScrollLeft() bool {
	return m.scrollOffset > 0
}

// canScrollRight returns true if there are more menus to show on the right.
func (m *MenuBar) canScrollRight() bool {
	if m.scrollOffset >= len(m.menus)-1 {
		return false
	}
	return !m.isLastMenuFullyVisible()
}

// isLastMenuFullyVisible returns true if the last menu is completely visible.
func (m *MenuBar) isLastMenuFullyVisible() bool {
	bounds := m.Bounds()
	metrics := core.DefaultCellMetrics()

	scrollButtonsWidth := core.Unit(0)
	if m.menusNeedScrolling() {
		scrollButtonsWidth = m.scrollButtonWidth() * 2 // [<][>]
	}
	leftEllipseWidth := core.Unit(0)
	if m.scrollOffset > 0 {
		leftEllipseWidth = metrics.TextWidth(3) // "..."
	}

	availableWidth := bounds.Width - m.dateTimeWidth() - scrollButtonsWidth

	x := leftEllipseWidth
	for i := m.scrollOffset; i < len(m.menus); i++ {
		menuWidth := core.Unit(len(m.menus[i].title)+2) * metrics.CellWidth
		x += menuWidth
		if x > availableWidth {
			return false
		}
	}
	return true
}

// ensureMenuVisible adjusts scroll offset to make the given menu index visible.
func (m *MenuBar) ensureMenuVisible(index int) {
	if index < 0 || index >= len(m.menus) || !m.menusNeedScrolling() {
		return
	}

	// If menu is to the left of visible area, scroll left
	if index < m.scrollOffset {
		m.scrollOffset = index
		return
	}

	// Check if menu is visible from current scroll position
	bounds := m.Bounds()
	metrics := core.DefaultCellMetrics()

	scrollButtonsWidth := m.scrollButtonWidth() * 2
	leftEllipseWidth := core.Unit(0)
	if m.scrollOffset > 0 {
		leftEllipseWidth = metrics.TextWidth(3) // "..."
	}

	availableWidth := bounds.Width - m.dateTimeWidth() - scrollButtonsWidth

	// Calculate position of the target menu
	x := leftEllipseWidth
	for i := m.scrollOffset; i <= index; i++ {
		menuWidth := core.Unit(len(m.menus[i].title)+2) * metrics.CellWidth
		if i == index {
			// Check if this menu fits
			if x+menuWidth > availableWidth {
				// Need to scroll right - increment scroll offset until it fits
				for m.scrollOffset < index {
					m.scrollOffset++
					// Recalculate with new scroll offset
					leftEllipseWidth = metrics.TextWidth(3) // "..." (always present when scrolled)
					x = leftEllipseWidth
					for j := m.scrollOffset; j <= index; j++ {
						mw := core.Unit(len(m.menus[j].title)+2) * metrics.CellWidth
						if j == index && x+mw <= availableWidth {
							return
						}
						x += mw
					}
				}
			}
		}
		x += menuWidth
	}
}

// clampScrollOffset adjusts the scroll offset when the container is resized.
// It ensures we don't have unnecessary empty space on the right when we could
// show more menus, and resets to 0 when scrolling is no longer needed.
func (m *MenuBar) clampScrollOffset() {
	// If no menus or scrolling not needed, reset to 0
	if len(m.menus) == 0 || !m.menusNeedScrolling() {
		m.scrollOffset = 0
		return
	}

	// Calculate how much space we have for menus
	bounds := m.Bounds()
	metrics := core.DefaultCellMetrics()
	scrollButtonsWidth := m.scrollButtonWidth() * 2
	availableWidth := bounds.Width - m.dateTimeWidth() - scrollButtonsWidth

	// Try to reduce scroll offset while still fitting all visible menus
	for m.scrollOffset > 0 {
		// Calculate width needed if we show one more menu on the left
		testOffset := m.scrollOffset - 1
		leftEllipseWidth := core.Unit(0)
		if testOffset > 0 {
			leftEllipseWidth = metrics.TextWidth(3) // "..."
		}

		x := leftEllipseWidth
		fitsWithMoreMenus := true
		for i := testOffset; i < len(m.menus); i++ {
			menuWidth := core.Unit(len(m.menus[i].title)+2) * metrics.CellWidth
			// Reserve space for right ellipsis if not the last menu
			rightEllipsisWidth := core.Unit(0)
			if i < len(m.menus)-1 {
				rightEllipsisWidth = metrics.TextWidth(3)
			}
			if x+menuWidth+rightEllipsisWidth > availableWidth {
				fitsWithMoreMenus = false
				break
			}
			x += menuWidth
		}

		if fitsWithMoreMenus {
			m.scrollOffset = testOffset
		} else {
			break
		}
	}
}

// hasAcceleratorConflict checks if a menu accelerator key conflicts with any
// registered keybinding (e.g., Alt+key is used for something else).
func (m *MenuBar) hasAcceleratorConflict(accel rune) bool {
	if accel == 0 {
		return false
	}
	// Check if M-<letter> is bound to any action
	key := "M-" + string(accel)
	action := core.DefaultKeyBindings.FindAction(key)
	return action != ""
}

// ShouldShowAccelerator returns whether the accelerator for a menu should be
// highlighted in red. Returns true if:
// - The menu bar has focus and no menu is dropped down, OR
// - There is no keybinding conflict for this accelerator
func (m *MenuBar) ShouldShowAccelerator(menu *Menu) bool {
	if menu.acceleratorChar == 0 {
		return false
	}
	// Always show when menu bar is focused with no menu down
	if m.acceleratorsActive {
		return true
	}
	// Otherwise, only show if there's no keybinding conflict
	return !m.hasAcceleratorConflict(menu.acceleratorChar)
}

// AcceleratorsActive returns whether accelerator highlighting is currently active.
func (m *MenuBar) AcceleratorsActive() bool {
	return m.acceleratorsActive
}

// setAcceleratorsActive updates the accelerators active state.
func (m *MenuBar) setAcceleratorsActive(active bool) {
	if m.acceleratorsActive != active {
		m.acceleratorsActive = active
		m.Update()
	}
}

// AddMenu adds a menu to the bar.
func (m *MenuBar) AddMenu(menu *Menu) {
	m.menus = append(m.menus, menu)
	m.Update()
}

// InsertMenu inserts a menu at the given index.
func (m *MenuBar) InsertMenu(index int, menu *Menu) {
	if index < 0 {
		index = 0
	}
	if index > len(m.menus) {
		index = len(m.menus)
	}
	m.menus = append(m.menus[:index], append([]*Menu{menu}, m.menus[index:]...)...)
	m.Update()
}

// RemoveMenu removes a menu.
func (m *MenuBar) RemoveMenu(menu *Menu) {
	for i, mm := range m.menus {
		if mm == menu {
			m.menus = append(m.menus[:i], m.menus[i+1:]...)
			break
		}
	}
	m.Update()
}

// Clear removes all menus.
func (m *MenuBar) Clear() {
	m.menus = nil
	m.currentIndex = -1
	m.activeMenu = nil
	m.Update()
}

// Menus returns all menus.
func (m *MenuBar) Menus() []*Menu {
	return m.menus
}

// MenuAt returns the menu at the given index.
func (m *MenuBar) MenuAt(index int) *Menu {
	if index < 0 || index >= len(m.menus) {
		return nil
	}
	return m.menus[index]
}

// ActiveMenu returns the currently open menu.
func (m *MenuBar) ActiveMenu() *Menu {
	return m.activeMenu
}

// OpenMenu opens a menu by index.
func (m *MenuBar) OpenMenu(index int) {
	if index < 0 || index >= len(m.menus) {
		return
	}

	m.CloseMenu()
	m.currentIndex = index
	m.activeMenu = m.menus[index]
	m.acceleratorsActive = false // Disable bar accelerators when menu is down

	// Ensure the menu is visible before opening (scroll if needed)
	m.ensureMenuVisible(index)

	// Notify that a menu is opening
	if m.onMenuOpen != nil {
		m.onMenuOpen()
	}

	// Calculate position (after scrolling so position is correct)
	metrics := core.DefaultCellMetrics()
	x := m.calculateMenuX(index)
	y := metrics.CellHeight

	m.activeMenu.Show(x, y)
	m.Update()
}

// CloseMenu closes the active menu but keeps the menu bar focused.
func (m *MenuBar) CloseMenu() {
	wasOpen := m.activeMenu != nil
	if m.activeMenu != nil {
		m.activeMenu.Hide()
		m.activeMenu = nil
	}
	// Re-enable accelerators if focused (menu bar retains focus while menu is open)
	if m.HasFocus() {
		m.acceleratorsActive = true
		// Keep currentIndex if we just closed a menu (for continued navigation)
		if !wasOpen {
			m.currentIndex = -1
		}
	} else {
		m.currentIndex = -1
	}
	m.Update()
}

// CloseMenuAndUnfocus closes the active menu and unfocuses the menu bar.
func (m *MenuBar) CloseMenuAndUnfocus() {
	if m.activeMenu != nil {
		m.activeMenu.Hide()
		m.activeMenu = nil
	}
	m.currentIndex = -1
	m.acceleratorsActive = false
	m.ClearFocus()
	m.Update()

	// Notify that the menu bar was dismissed
	if m.onMenuDismiss != nil {
		m.onMenuDismiss()
	}
}

// calculateMenuX calculates the x position of a menu (accounting for scroll offset).
func (m *MenuBar) calculateMenuX(index int) core.Unit {
	metrics := core.DefaultCellMetrics()

	// Start after left ellipsis if scrolled
	x := core.Unit(0)
	if m.scrollOffset > 0 {
		x = metrics.TextWidth(3) // "..."
	}

	// Calculate position from scroll offset
	for i := m.scrollOffset; i < index; i++ {
		x += core.Unit(len(m.menus[i].title)+2) * metrics.CellWidth
	}
	return x
}

// SizeHint returns the preferred size.
func (m *MenuBar) SizeHint() core.UnitSize {
	metrics := core.DefaultCellMetrics()

	width := core.Unit(0)
	for _, menu := range m.menus {
		width += core.Unit(len(menu.title)+2) * metrics.CellWidth
	}

	return core.UnitSize{
		Width:  width,
		Height: metrics.CellHeight,
	}
}

// Paint renders the menu bar (without dropdown - use PaintDropdown for that).
func (m *MenuBar) Paint(p *core.Painter) {
	bounds := m.Bounds()
	theme := m.Theme()
	metrics := p.Metrics()

	// Clamp scroll offset if container was resized and more menus can now fit
	m.clampScrollOffset()

	// Draw background
	p.FillRect(core.UnitRect{Width: bounds.Width, Height: bounds.Height}, ' ', theme.MenuBar)

	// Calculate if we need scroll buttons
	needsScrolling := m.menusNeedScrolling()

	// Draw date/time on the far right edge first (to know where menus must stop)
	now := time.Now()
	dateTimeStr := now.Format(" Mon Jan 02 15:04 ")
	dateTimeStyle := style.DefaultStyle().WithFg(style.ColorBrightYellow).WithBg(style.ColorYellow)
	dateTimeWidth := core.Unit(len(dateTimeStr)) * metrics.CellWidth
	dateTimeX := bounds.Width - dateTimeWidth

	// Draw scroll buttons just left of date/time if needed
	scrollButtonsWidth := core.Unit(0)
	if needsScrolling {
		scrollButtonsWidth = m.scrollButtonWidth() * 2 // [<][>] or  <  >

		// Button styles: blue on white for active, bright white on white for inactive
		activeButtonStyle := style.DefaultStyle().WithFg(style.ColorBlue).WithBg(style.ColorWhite)
		inactiveButtonStyle := style.DefaultStyle().WithFg(style.ColorBrightWhite).WithBg(style.ColorWhite)

		// Draw left button: [<] when active, " < " when inactive
		leftButtonX := dateTimeX - scrollButtonsWidth
		if m.canScrollLeft() {
			p.DrawCell(leftButtonX, 0, '[', activeButtonStyle)
			p.DrawCell(leftButtonX+metrics.CellWidth, 0, '<', activeButtonStyle)
			p.DrawCell(leftButtonX+2*metrics.CellWidth, 0, ']', activeButtonStyle)
		} else {
			p.DrawCell(leftButtonX, 0, ' ', inactiveButtonStyle)
			p.DrawCell(leftButtonX+metrics.CellWidth, 0, '<', inactiveButtonStyle)
			p.DrawCell(leftButtonX+2*metrics.CellWidth, 0, ' ', inactiveButtonStyle)
		}

		// Draw right button: [>] when active, " > " when inactive
		rightButtonX := leftButtonX + 3*metrics.CellWidth
		if m.canScrollRight() {
			p.DrawCell(rightButtonX, 0, '[', activeButtonStyle)
			p.DrawCell(rightButtonX+metrics.CellWidth, 0, '>', activeButtonStyle)
			p.DrawCell(rightButtonX+2*metrics.CellWidth, 0, ']', activeButtonStyle)
		} else {
			p.DrawCell(rightButtonX, 0, ' ', inactiveButtonStyle)
			p.DrawCell(rightButtonX+metrics.CellWidth, 0, '>', inactiveButtonStyle)
			p.DrawCell(rightButtonX+2*metrics.CellWidth, 0, ' ', inactiveButtonStyle)
		}
	}

	// Available width for menus
	availableWidth := dateTimeX - scrollButtonsWidth

	// Draw left ellipsis if scrolled
	x := core.Unit(0)
	if m.scrollOffset > 0 {
		ellipsisStr := "..."
		for i, ch := range ellipsisStr {
			p.DrawCell(core.Unit(i)*metrics.CellWidth, 0, ch, theme.MenuBar)
		}
		x = core.Unit(len(ellipsisStr)) * metrics.CellWidth
	}

	// Draw visible menus
	for i := m.scrollOffset; i < len(m.menus); i++ {
		menu := m.menus[i]
		menuWidth := core.Unit(len(menu.title)+2) * metrics.CellWidth

		// Reserve space for right ellipsis if there are more menus after this one
		rightEllipsisWidth := core.Unit(0)
		if i < len(m.menus)-1 {
			rightEllipsisWidth = metrics.TextWidth(3) // "..."
		}

		// Check if this menu fits (with room for right ellipsis if needed)
		if x+menuWidth+rightEllipsisWidth > availableWidth {
			// Menu doesn't fit fully
			remainingWidth := availableWidth - x

			// Determine style for this menu
			var s style.CellStyle
			isSelected := i == m.currentIndex
			if isSelected {
				s = theme.MenuBarSelected
			} else {
				s = theme.MenuBar
			}

			// Calculate accelerator style for this menu
			var accelStyle style.CellStyle
			if isSelected {
				accelStyle = style.DefaultStyle().WithFg(style.ColorBrightCyan).WithBg(style.ColorBlue)
			} else {
				accelStyle = style.DefaultStyle().WithFg(style.ColorRed).WithBg(style.ColorWhite)
			}
			showAccel := m.ShouldShowAccelerator(menu)

			// If this is the selected menu, try to show the full menu text
			// with ellipsis OUTSIDE the selected area
			if isSelected && remainingWidth >= menuWidth {
				// We can fit the full menu, just not the ellipsis after it
				// Draw the full menu in selected style
				p.FillRect(core.UnitRect{
					X:      x,
					Y:      0,
					Width:  menuWidth,
					Height: metrics.CellHeight,
				}, ' ', s)

				textX := x + metrics.CellWidth
				for idx, ch := range menu.title {
					charStyle := s
					if showAccel && idx == menu.acceleratorPos {
						charStyle = accelStyle
					}
					p.DrawCell(textX, 0, ch, charStyle)
					textX += metrics.CellWidth
				}

				// Draw as much ellipsis as fits in remaining space (in normal style)
				ellipsisX := x + menuWidth
				for _, ch := range "..." {
					if ellipsisX < availableWidth {
						p.DrawCell(ellipsisX, 0, ch, theme.MenuBar)
						ellipsisX += metrics.CellWidth
					}
				}
			} else {
				// Not selected, or not enough room for full menu - show partial with ellipsis
				ellipsisWidth := metrics.TextWidth(3) // "..."

				// Calculate how many chars we can show: space + chars + "..."
				// Need at least 4 chars width for " X..." (space, one char, ellipsis)
				if remainingWidth >= 4*metrics.CellWidth {
					// Draw space before text
					p.DrawCell(x, 0, ' ', s)
					textX := x + metrics.CellWidth

					// Calculate how many title chars we can show
					charsAvailable := int((remainingWidth-metrics.CellWidth-ellipsisWidth) / metrics.CellWidth)
					titleRunes := []rune(menu.title)
					for idx := 0; idx < charsAvailable && idx < len(titleRunes); idx++ {
						charStyle := s
						if showAccel && idx == menu.acceleratorPos {
							charStyle = accelStyle
						}
						p.DrawCell(textX, 0, titleRunes[idx], charStyle)
						textX += metrics.CellWidth
					}
					// Draw ellipsis in the menu style (never accelerator color)
					for _, ch := range "..." {
						p.DrawCell(textX, 0, ch, s)
						textX += metrics.CellWidth
					}
				} else if remainingWidth >= ellipsisWidth {
					// Just show "..." to indicate more menus
					ellipsisX := x
					for _, ch := range "..." {
						if ellipsisX < availableWidth {
							p.DrawCell(ellipsisX, 0, ch, theme.MenuBar)
							ellipsisX += metrics.CellWidth
						}
					}
				}
			}
			break
		}

		// Determine style
		var s style.CellStyle
		if i == m.currentIndex {
			s = theme.MenuBarSelected
		} else {
			s = theme.MenuBar
		}

		// Draw background
		p.FillRect(core.UnitRect{
			X:      x,
			Y:      0,
			Width:  menuWidth,
			Height: metrics.CellHeight,
		}, ' ', s)

		// Draw title with accelerator highlighting
		textX := x + metrics.CellWidth
		// Accelerator style depends on whether menu is selected
		var accelStyle style.CellStyle
		if i == m.currentIndex {
			// Selected menu: bright cyan on blue
			accelStyle = style.DefaultStyle().WithFg(style.ColorBrightCyan).WithBg(style.ColorBlue)
		} else {
			// Normal menu: red on white
			accelStyle = style.DefaultStyle().WithFg(style.ColorRed).WithBg(style.ColorWhite)
		}
		showAccel := m.ShouldShowAccelerator(menu)
		for idx, ch := range menu.title {
			charStyle := s
			if showAccel && idx == menu.acceleratorPos {
				charStyle = accelStyle
			}
			p.DrawCell(textX, 0, ch, charStyle)
			textX += metrics.CellWidth
		}

		x += menuWidth
	}

	// Draw date/time background and text
	p.FillRect(core.UnitRect{
		X:      dateTimeX,
		Y:      0,
		Width:  dateTimeWidth,
		Height: metrics.CellHeight,
	}, ' ', dateTimeStyle)

	for i, ch := range dateTimeStr {
		p.DrawCell(dateTimeX+core.Unit(i)*metrics.CellWidth, 0, ch, dateTimeStyle)
	}
}

// PaintDropdown renders the active menu dropdown (call after windows for correct z-order).
func (m *MenuBar) PaintDropdown(p *core.Painter) {
	if m.activeMenu != nil {
		m.activeMenu.Paint(p)
	}
}

// ActiveMenuBounds returns the bounds of the active dropdown menu.
// Returns an empty rect if no menu is open.
func (m *MenuBar) ActiveMenuBounds() core.UnitRect {
	if m.activeMenu == nil {
		return core.UnitRect{}
	}
	return m.activeMenu.DropdownBounds()
}

// HandleKeyPress handles keyboard input.
func (m *MenuBar) HandleKeyPress(event core.KeyPressEvent) bool {
	// Handle active menu first
	if m.activeMenu != nil {
		if m.activeMenu.HandleKeyPress(event) {
			return true
		}
	}

	switch event.Key {
	case "Left":
		if len(m.menus) > 0 {
			newIndex := m.currentIndex - 1
			if newIndex < 0 {
				newIndex = len(m.menus) - 1
			}
			if m.activeMenu != nil {
				m.OpenMenu(newIndex)
			} else {
				m.currentIndex = newIndex
				m.ensureMenuVisible(newIndex)
				m.Update()
			}
		}
		return true

	case "Right":
		if len(m.menus) > 0 {
			newIndex := m.currentIndex + 1
			if newIndex >= len(m.menus) {
				newIndex = 0
			}
			if m.activeMenu != nil {
				m.OpenMenu(newIndex)
			} else {
				m.currentIndex = newIndex
				m.ensureMenuVisible(newIndex)
				m.Update()
			}
		}
		return true

	case "Enter", " ", "Space", "Down":
		if m.currentIndex >= 0 {
			if m.activeMenu != nil {
				m.CloseMenu()
			} else {
				m.OpenMenu(m.currentIndex)
			}
		}
		return true

	case "Escape":
		if m.activeMenu != nil {
			// First escape: close menu but keep menu bar focused
			m.CloseMenu()
		} else {
			// Second escape: unfocus menu bar
			m.CloseMenuAndUnfocus()
		}
		return true

	case "F10":
		// Toggle menu bar focus
		if m.HasFocus() {
			m.CloseMenuAndUnfocus()
		} else {
			m.SetFocus()
			if m.currentIndex < 0 && len(m.menus) > 0 {
				m.currentIndex = 0
			}
		}
		m.Update()
		return true
	}

	// Check Alt+key shortcuts (M-<letter> format, lowercase only - no shift)
	if strings.HasPrefix(event.Key, "M-") && len(event.Key) == 3 {
		letter := event.Key[2]
		// Only match lowercase (M-f not M-F) to avoid shift combinations
		if letter >= 'a' && letter <= 'z' {
			key := rune(letter)
			for i, menu := range m.menus {
				if menu.acceleratorChar == key {
					m.SetFocus()
					m.OpenMenu(i)
					return true
				}
			}
		}
	}

	// Check accessibility keys: when menu bar is focused with accelerators active,
	// single letter keys (no modifiers) activate menus
	if m.HasFocus() && m.activeMenu == nil && m.acceleratorsActive && len(event.Key) == 1 {
		letter := event.Key[0]
		// Accept both uppercase and lowercase single letters (no modifier prefix)
		if (letter >= 'a' && letter <= 'z') || (letter >= 'A' && letter <= 'Z') {
			key := rune(strings.ToLower(event.Key)[0])
			for i, menu := range m.menus {
				if menu.acceleratorChar == key {
					m.OpenMenu(i)
					return true
				}
			}
		}
	}

	return false
}

// findMenuByAccelerator finds a menu by its accelerator character.
func (m *MenuBar) findMenuByAccelerator(key rune) int {
	key = rune(strings.ToLower(string(key))[0])
	for i, menu := range m.menus {
		if menu.acceleratorChar == key {
			return i
		}
	}
	return -1
}

// HandleMousePress handles mouse clicks.
func (m *MenuBar) HandleMousePress(event core.MousePressEvent) bool {
	metrics := core.DefaultCellMetrics()
	bounds := m.Bounds()

	// Check active menu first - if clicking on an item in the dropdown
	if m.activeMenu != nil && !m.mouseDown {
		if m.activeMenu.HandleMousePress(event) {
			return true
		}
	}

	// Check if click is in menu bar
	if event.Y < metrics.CellHeight {
		// Check for scroll button clicks if scrolling is needed
		needsScrolling := m.menusNeedScrolling()
		if needsScrolling {
			dateTimeWidth := m.dateTimeWidth()
			scrollButtonsWidth := m.scrollButtonWidth() * 2
			dateTimeX := bounds.Width - dateTimeWidth
			leftButtonX := dateTimeX - scrollButtonsWidth

			// Check [<] button
			if event.X >= leftButtonX && event.X < leftButtonX+3*metrics.CellWidth {
				if m.canScrollLeft() {
					m.scrollOffset--
					m.Update()
				}
				return true
			}

			// Check [>] button
			rightButtonX := leftButtonX + 3*metrics.CellWidth
			if event.X >= rightButtonX && event.X < rightButtonX+3*metrics.CellWidth {
				if m.canScrollRight() {
					m.scrollOffset++
					m.Update()
				}
				return true
			}
		}

		// Check for click on left ellipsis ("...") to scroll left and open that menu
		if m.scrollOffset > 0 {
			ellipsisWidth := metrics.TextWidth(3) // "..."
			if event.X >= 0 && event.X < ellipsisWidth {
				// Track mouse down for potential drag (same as clicking a menu)
				m.mouseDown = true
				m.mouseDownX = event.X
				m.mouseDownY = event.Y
				m.dragging = false

				m.scrollOffset--
				// Open the menu that was just scrolled into view
				m.OpenMenu(m.scrollOffset)
				return true
			}
		}

		// Find which menu was clicked (accounting for scroll offset)
		x := core.Unit(0)
		if m.scrollOffset > 0 {
			x = metrics.TextWidth(3) // "..."
		}

		for i := m.scrollOffset; i < len(m.menus); i++ {
			menu := m.menus[i]
			menuWidth := core.Unit(len(menu.title)+2) * metrics.CellWidth
			if event.X >= x && event.X < x+menuWidth {
				// Track mouse down for potential drag
				m.mouseDown = true
				m.mouseDownX = event.X
				m.mouseDownY = event.Y
				m.dragging = false

				if m.activeMenu == menu {
					// Toggle - close if same menu clicked
					m.CloseMenu()
				} else {
					m.OpenMenu(i)
				}
				return true
			}
			x += menuWidth
		}

		// Clicked on empty part of menu bar
		m.CloseMenu()
		m.mouseDown = false
		m.dragging = false
		return true
	}

	// Click below menu bar
	if event.Y >= 0 && event.Y < bounds.Height && m.activeMenu == nil {
		return true
	}

	// Click outside - if menu was open, dismiss and unfocus completely
	if m.activeMenu != nil {
		m.CloseMenuAndUnfocus()
		m.mouseDown = false
		m.dragging = false
		return true
	}

	return false
}

// HandleFocusIn is called when focus is gained.
func (m *MenuBar) HandleFocusIn() {
	if m.currentIndex < 0 && len(m.menus) > 0 {
		m.currentIndex = 0
	}
	// Enable accelerator display when focused with no menu down
	if m.activeMenu == nil {
		m.acceleratorsActive = true
	}
	m.Update()
}

// HandleFocusOut is called when focus is lost.
func (m *MenuBar) HandleFocusOut() {
	m.CloseMenu()
	m.dragging = false
	m.currentIndex = -1
	m.acceleratorsActive = false
	m.Update()
}

// HandleMouseMove handles mouse movement during drag.
func (m *MenuBar) HandleMouseMove(event core.MouseMoveEvent) bool {
	if !m.mouseDown || m.activeMenu == nil {
		return false
	}

	metrics := core.DefaultCellMetrics()

	// Detect if we've started dragging (moved enough from initial click)
	if !m.dragging {
		dx := event.X - m.mouseDownX
		dy := event.Y - m.mouseDownY
		if dx < 0 {
			dx = -dx
		}
		if dy < 0 {
			dy = -dy
		}
		// Only start dragging if moved at least half a cell
		if dx >= metrics.CellWidth/2 || dy >= metrics.CellHeight/2 {
			m.dragging = true
		} else {
			return true // Not dragging yet, consume but don't act
		}
	}

	// Check if mouse is in menu bar - switch menus and deselect dropdown item
	if event.Y < metrics.CellHeight {
		// Deselect current item in dropdown since we're back on the menu bar
		if m.activeMenu != nil && m.activeMenu.currentIndex != -1 {
			m.activeMenu.currentIndex = -1
			m.activeMenu.Update()
		}

		// Find which menu the mouse is over (accounting for scroll offset)
		x := core.Unit(0)
		if m.scrollOffset > 0 {
			x = metrics.TextWidth(3) // "..."
		}

		for i := m.scrollOffset; i < len(m.menus); i++ {
			menu := m.menus[i]
			menuWidth := core.Unit(len(menu.title)+2) * metrics.CellWidth
			if event.X >= x && event.X < x+menuWidth {
				if m.activeMenu != menu {
					m.OpenMenu(i)
				}
				return true
			}
			x += menuWidth
		}
		return true
	}

	// Check if mouse is in dropdown menu - highlight item
	if m.activeMenu != nil && m.activeMenu.visible {
		size := m.activeMenu.calculateSize()
		if event.X >= m.activeMenu.popupX && event.X < m.activeMenu.popupX+size.Width &&
			event.Y >= m.activeMenu.popupY && event.Y < m.activeMenu.popupY+size.Height {
			itemIndex := int((event.Y - m.activeMenu.popupY) / metrics.CellHeight)
			if itemIndex >= 0 && itemIndex < len(m.activeMenu.items) {
				item := m.activeMenu.items[itemIndex]
				if !item.Separator && item.Enabled {
					m.activeMenu.currentIndex = itemIndex
					m.activeMenu.Update()
				}
			}
			return true
		}
		// Mouse is outside both menu bar and dropdown - deselect item
		if m.activeMenu.currentIndex != -1 {
			m.activeMenu.currentIndex = -1
			m.activeMenu.Update()
		}
	}

	return true
}

// HandleMouseRelease handles mouse release during drag.
func (m *MenuBar) HandleMouseRelease(event core.MouseReleaseEvent) bool {
	wasMouseDown := m.mouseDown
	wasDragging := m.dragging

	// Always clear mouse state
	m.mouseDown = false
	m.dragging = false

	// If we weren't in mouse-down mode, nothing to do
	if !wasMouseDown {
		return false
	}

	// If not dragging (just a click), leave menu open for further clicks
	if !wasDragging {
		return true // Consume the release event but don't dismiss
	}

	metrics := core.DefaultCellMetrics()

	// Check if release is on a dropdown menu item - trigger it
	if m.activeMenu != nil && m.activeMenu.visible {
		size := m.activeMenu.calculateSize()
		if event.X >= m.activeMenu.popupX && event.X < m.activeMenu.popupX+size.Width &&
			event.Y >= m.activeMenu.popupY && event.Y < m.activeMenu.popupY+size.Height {
			itemIndex := int((event.Y - m.activeMenu.popupY) / metrics.CellHeight)
			if itemIndex >= 0 && itemIndex < len(m.activeMenu.items) {
				item := m.activeMenu.items[itemIndex]
				if !item.Separator && item.Enabled {
					if item.SubMenu != nil {
						m.activeMenu.currentIndex = itemIndex
						m.activeMenu.openSubMenu(item)
					} else {
						m.activeMenu.triggerItem(item)
					}
					return true
				}
			}
		}
	}

	// Release not on a menu item - dismiss menu
	m.CloseMenu()
	return true
}

// IsDragging returns whether a menu drag is in progress.
func (m *MenuBar) IsDragging() bool {
	return m.dragging
}

// AccessibleInfo returns accessibility information.
func (m *MenuBar) AccessibleInfo() core.AccessibleInfo {
	info := m.AccessibleWidget.AccessibleInfo()
	info.Role = core.RoleMenuBar
	info.SetSize = len(m.menus)

	if m.currentIndex >= 0 && m.currentIndex < len(m.menus) {
		info.PositionInSet = m.currentIndex + 1
		info.Value = m.menus[m.currentIndex].title
	}

	return info
}
