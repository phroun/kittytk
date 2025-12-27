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
	Text      string
	Shortcut  core.Shortcut
	Icon      *style.TextIcon
	Enabled   bool
	Checkable bool
	Checked   bool
	Separator bool // If true, this is a separator line

	// Submenu
	SubMenu *Menu

	// Callbacks
	OnTriggered func()
}

// NewMenuItem creates a new menu item.
func NewMenuItem(text string) *MenuItem {
	return &MenuItem{
		Text:    text,
		Enabled: true,
	}
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

	title        string
	items        []*MenuItem
	currentIndex int
	visible      bool

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

// NewMenu creates a new menu.
func NewMenu(title string) *Menu {
	m := &Menu{
		title:        title,
		currentIndex: -1,
		maxVisible:   12, // Default max visible items
	}
	m.WidgetBase = *core.NewWidgetBase()
	// Note: Menu doesn't call Init because it has a Show(x,y) method
	// with different signature than Widget.Show()
	m.SetFocusPolicy(core.StrongFocus)
	m.SetAccessibleRole(core.RoleMenu)
	m.SetAccessibleName(title)
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
	m.title = title
	m.SetAccessibleName(title)
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
	m.SetFocus()
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
			width += 4 + len(item.Shortcut.DisplayString())
		}
		if item.SubMenu != nil {
			width += 3 // For submenu arrow
		}
		if width > maxWidth {
			maxWidth = width
		}
	}

	// Add padding
	maxWidth += 4

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

		// Draw item background
		p.FillRect(core.UnitRect{
			X:      m.popupX,
			Y:      itemY,
			Width:  size.Width,
			Height: metrics.CellHeight,
		}, ' ', s)

		if item.Separator {
			// Draw separator line
			for x := m.popupX + metrics.CellWidth; x < m.popupX+size.Width-metrics.CellWidth; x += metrics.CellWidth {
				p.DrawCell(x, itemY, '─', s)
			}
			currentY += metrics.CellHeight
			continue
		}

		x := m.popupX + metrics.CellWidth

		// Draw checkmark or icon
		if item.Checkable {
			if item.Checked {
				p.DrawCell(x, itemY, '✓', s)
			}
		} else if item.Icon != nil && len(item.Icon.Cells) > 0 {
			cell := item.Icon.Cells[0]
			p.DrawCell(x, itemY, cell.Char, cell.Style)
		}
		x += metrics.CellWidth * 2

		// Draw text
		for _, ch := range item.Text {
			p.DrawCell(x, itemY, ch, s)
			x += metrics.CellWidth
		}

		// Draw shortcut or submenu arrow at the right
		if item.SubMenu != nil {
			arrowX := m.popupX + size.Width - metrics.CellWidth*2
			p.DrawCell(arrowX, itemY, '▸', s)
		} else if item.Shortcut != "" {
			shortcutStr := item.Shortcut.DisplayString()
			shortcutX := m.popupX + size.Width - core.Unit(len(shortcutStr)+2)*metrics.CellWidth
			shortcutStyle := s
			if item.Enabled {
				shortcutStyle = s.WithAttrs(style.StyleDim)
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
		m.Hide()
		return true

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

	// Check for mnemonics (first letter)
	if len(event.Key) == 1 {
		key := strings.ToLower(event.Key)
		for i, item := range m.items {
			if item.Enabled && !item.Separator && len(item.Text) > 0 {
				if strings.ToLower(string(item.Text[0])) == key {
					m.currentIndex = i
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

	// Drag tracking for click-and-drag menu navigation
	mouseDown      bool  // Mouse button is held down
	dragging       bool  // Actually dragging (mouse moved while down)
	mouseDownX     core.Unit
	mouseDownY     core.Unit
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

	// Calculate position
	metrics := core.DefaultCellMetrics()
	x := m.calculateMenuX(index)
	y := metrics.CellHeight

	m.activeMenu.Show(x, y)
	m.Update()
}

// CloseMenu closes the active menu.
func (m *MenuBar) CloseMenu() {
	if m.activeMenu != nil {
		m.activeMenu.Hide()
		m.activeMenu = nil
	}
	m.currentIndex = -1
	m.Update()
}

// calculateMenuX calculates the x position of a menu.
func (m *MenuBar) calculateMenuX(index int) core.Unit {
	metrics := core.DefaultCellMetrics()
	x := core.Unit(0)
	for i := 0; i < index; i++ {
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

	// Draw background
	p.FillRect(core.UnitRect{Width: bounds.Width, Height: bounds.Height}, ' ', theme.MenuBar)

	// Draw menus
	x := core.Unit(0)
	for i, menu := range m.menus {
		menuWidth := core.Unit(len(menu.title)+2) * metrics.CellWidth

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

		// Draw title
		textX := x + metrics.CellWidth
		for _, ch := range menu.title {
			p.DrawCell(textX, 0, ch, s)
			textX += metrics.CellWidth
		}

		x += menuWidth
	}

	// Draw date/time on the far right edge
	now := time.Now()
	dateTimeStr := now.Format(" Mon Jan 02 15:04 ")
	dateTimeStyle := style.DefaultStyle().WithFg(style.ColorBrightYellow).WithBg(style.ColorYellow)

	// Calculate position for right-aligned date/time
	dateTimeWidth := core.Unit(len(dateTimeStr)) * metrics.CellWidth
	dateTimeX := bounds.Width - dateTimeWidth

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
			m.CloseMenu()
			return true
		}
		return false

	case "F10":
		// Toggle menu bar focus
		if m.HasFocus() {
			m.CloseMenu()
			m.currentIndex = -1
		} else {
			m.SetFocus()
			m.currentIndex = 0
		}
		m.Update()
		return true
	}

	// Check Alt+key shortcuts
	if event.Modifiers&core.AltModifier != 0 && len(event.Key) == 1 {
		key := strings.ToLower(event.Key)
		for i, menu := range m.menus {
			if len(menu.title) > 0 && strings.ToLower(string(menu.title[0])) == key {
				m.OpenMenu(i)
				return true
			}
		}
	}

	return false
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
		// Find which menu was clicked
		x := core.Unit(0)
		for i, menu := range m.menus {
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

	// Click outside - if menu was open, consume the event to dismiss without activating anything
	if m.activeMenu != nil {
		m.CloseMenu()
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
	m.Update()
}

// HandleFocusOut is called when focus is lost.
func (m *MenuBar) HandleFocusOut() {
	m.CloseMenu()
	m.dragging = false
	m.currentIndex = -1
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

		x := core.Unit(0)
		for i, menu := range m.menus {
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
