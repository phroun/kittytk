// Package widgets provides standard UI widgets for the TUI toolkit.
package widgets

import (
	"strings"

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

	// Callbacks
	onAboutToShow func()
	onAboutToHide func()
}

// NewMenu creates a new menu.
func NewMenu(title string) *Menu {
	m := &Menu{
		title:        title,
		currentIndex: -1,
	}
	m.WidgetBase = *core.NewWidgetBase()
	m.SetFocusPolicy(core.StrongFocus)
	m.SetAccessibleRole(core.RoleMenu)
	m.SetAccessibleName(title)
	return m
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
	m.currentIndex = m.findNextEnabled(-1)
	m.SetFocus()
	m.Update()
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

	return core.UnitSize{
		Width:  core.Unit(maxWidth) * metrics.CellWidth,
		Height: core.Unit(len(m.items)) * metrics.CellHeight,
	}
}

// SizeHint returns the preferred size.
func (m *Menu) SizeHint() core.UnitSize {
	return m.calculateSize()
}

// Paint renders the menu.
func (m *Menu) Paint(p *core.Painter) {
	if !m.visible {
		return
	}

	theme := m.Theme()
	metrics := p.Metrics()
	size := m.calculateSize()

	// Draw menu background with border
	menuBounds := core.UnitRect{
		X:      m.popupX,
		Y:      m.popupY,
		Width:  size.Width,
		Height: size.Height,
	}
	p.FillRect(menuBounds, ' ', theme.MenuItem)
	p.DrawRect(menuBounds, theme.DefaultBorder, theme.MenuItem)

	// Draw items
	for i, item := range m.items {
		itemY := m.popupY + core.Unit(i)*metrics.CellHeight

		// Determine style
		var s style.CellStyle
		if item.Separator {
			s = theme.MenuSeparator
		} else if !item.Enabled {
			s = theme.MenuItemDisabled
		} else if i == m.currentIndex {
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
		m.closeSubMenu()
		m.Update()
		return true

	case "Down":
		m.currentIndex = m.findNextEnabled(m.currentIndex)
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
		m.closeSubMenu()
		m.Update()
		return true

	case "End":
		m.currentIndex = m.findPrevEnabled(0)
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

	// Position submenu to the right of current item
	itemIndex := -1
	for i, it := range m.items {
		if it == item {
			itemIndex = i
			break
		}
	}

	subX := m.popupX + size.Width
	subY := m.popupY + core.Unit(itemIndex)*metrics.CellHeight

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

	// Check if click is in menu bounds
	if event.X >= m.popupX && event.X < m.popupX+size.Width &&
		event.Y >= m.popupY && event.Y < m.popupY+size.Height {

		itemIndex := int((event.Y - m.popupY) / metrics.CellHeight)
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
}

// NewMenuBar creates a new menu bar.
func NewMenuBar() *MenuBar {
	m := &MenuBar{
		currentIndex:  -1,
		showShortcuts: true,
	}
	m.WidgetBase = *core.NewWidgetBase()
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

// Paint renders the menu bar.
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

	// Draw active menu
	if m.activeMenu != nil {
		m.activeMenu.Paint(p)
	}
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

	// Check active menu first
	if m.activeMenu != nil && m.activeMenu.HandleMousePress(event) {
		return true
	}

	// Check if click is in menu bar
	if event.Y < metrics.CellHeight {
		// Find which menu was clicked
		x := core.Unit(0)
		for i, menu := range m.menus {
			menuWidth := core.Unit(len(menu.title)+2) * metrics.CellWidth
			if event.X >= x && event.X < x+menuWidth {
				if m.activeMenu == menu {
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
		return true
	}

	// Click below menu bar
	if event.Y >= 0 && event.Y < bounds.Height && m.activeMenu == nil {
		return true
	}

	// Click outside
	m.CloseMenu()
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
	m.currentIndex = -1
	m.Update()
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
