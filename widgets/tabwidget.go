// Package widgets provides standard UI widgets for the TUI toolkit.
package widgets

import (
	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/style"
)

// TabWidget displays multiple pages with tabs.
type TabWidget struct {
	core.WidgetBase
	core.AccessibleWidget

	tabs         []*Tab
	currentIndex int

	// Tab bar position
	tabPosition TabPosition

	// Appearance
	movable   bool // Can tabs be reordered
	closable  bool // Can tabs be closed

	// Callbacks
	onCurrentChanged func(index int)
	onTabCloseRequested func(index int)
}

// Tab represents a single tab in a TabWidget.
type Tab struct {
	Text    string
	Icon    *style.TextIcon
	Content core.Widget
	Enabled bool
	Closable bool // Per-tab closable setting
	Data    interface{}
}

// TabPosition determines where the tab bar is displayed.
type TabPosition int

const (
	TabsTop TabPosition = iota
	TabsBottom
	TabsLeft
	TabsRight
)

// NewTabWidget creates a new tab widget.
func NewTabWidget() *TabWidget {
	t := &TabWidget{
		currentIndex: -1,
		tabPosition:  TabsTop,
	}
	t.WidgetBase = *core.NewWidgetBase()
	// TabWidget is a container - let children inside get focus
	t.SetFocusPolicy(core.NoFocus)
	t.SetAccessibleRole(core.RoleTabList)
	return t
}

// AddTab adds a tab with the given text and content widget.
func (t *TabWidget) AddTab(text string, content core.Widget) int {
	tab := &Tab{
		Text:    text,
		Content: content,
		Enabled: true,
	}
	t.tabs = append(t.tabs, tab)

	if content != nil {
		content.SetParent(t)
	}

	if t.currentIndex < 0 {
		t.currentIndex = 0
	}

	t.Update()
	return len(t.tabs) - 1
}

// InsertTab inserts a tab at the given index.
func (t *TabWidget) InsertTab(index int, text string, content core.Widget) int {
	if index < 0 {
		index = 0
	}
	if index > len(t.tabs) {
		index = len(t.tabs)
	}

	tab := &Tab{
		Text:    text,
		Content: content,
		Enabled: true,
	}

	t.tabs = append(t.tabs[:index], append([]*Tab{tab}, t.tabs[index:]...)...)

	if content != nil {
		content.SetParent(t)
	}

	if t.currentIndex >= index {
		t.currentIndex++
	}

	if t.currentIndex < 0 {
		t.currentIndex = 0
	}

	t.Update()
	return index
}

// RemoveTab removes the tab at the given index.
func (t *TabWidget) RemoveTab(index int) {
	if index < 0 || index >= len(t.tabs) {
		return
	}

	tab := t.tabs[index]
	if tab.Content != nil {
		tab.Content.SetParent(nil)
	}

	t.tabs = append(t.tabs[:index], t.tabs[index+1:]...)

	// Adjust current index
	if t.currentIndex == index {
		if t.currentIndex >= len(t.tabs) {
			t.currentIndex = len(t.tabs) - 1
		}
		if t.onCurrentChanged != nil && t.currentIndex >= 0 {
			t.onCurrentChanged(t.currentIndex)
		}
	} else if t.currentIndex > index {
		t.currentIndex--
	}

	t.Update()
}

// Clear removes all tabs.
func (t *TabWidget) Clear() {
	for _, tab := range t.tabs {
		if tab.Content != nil {
			tab.Content.SetParent(nil)
		}
	}
	t.tabs = nil
	t.currentIndex = -1
	t.Update()
}

// Children returns the content of the current active tab only.
// This ensures focus navigation only includes visible widgets.
func (t *TabWidget) Children() []core.Widget {
	if t.currentIndex >= 0 && t.currentIndex < len(t.tabs) {
		if content := t.tabs[t.currentIndex].Content; content != nil {
			return []core.Widget{content}
		}
	}
	return nil
}

// AllChildren returns content widgets from all tabs (for layout/painting).
func (t *TabWidget) AllChildren() []core.Widget {
	var children []core.Widget
	for _, tab := range t.tabs {
		if tab.Content != nil {
			children = append(children, tab.Content)
		}
	}
	return children
}

// AddChild adds a child widget as a new tab.
func (t *TabWidget) AddChild(child core.Widget) {
	t.AddTab("Tab", child)
}

// RemoveChild removes a child widget.
func (t *TabWidget) RemoveChild(child core.Widget) {
	for i, tab := range t.tabs {
		if tab.Content == child {
			t.RemoveTab(i)
			return
		}
	}
}

// ChildAt returns the child at the given position.
func (t *TabWidget) ChildAt(pos core.UnitPoint) core.Widget {
	contentRect := t.contentBounds()
	if pos.X >= contentRect.X && pos.X < contentRect.X+contentRect.Width &&
		pos.Y >= contentRect.Y && pos.Y < contentRect.Y+contentRect.Height {
		if t.currentIndex >= 0 && t.currentIndex < len(t.tabs) {
			return t.tabs[t.currentIndex].Content
		}
	}
	return nil
}

// Layout arranges the current tab content.
func (t *TabWidget) Layout() {
	// Content widgets are laid out based on contentBounds
}

// LayoutManager returns nil (TabWidget manages its own layout).
func (t *TabWidget) LayoutManager() core.LayoutManager {
	return nil
}

// SetLayoutManager is a no-op (TabWidget manages its own layout).
func (t *TabWidget) SetLayoutManager(layout core.LayoutManager) {
	// TabWidget manages its own layout, ignore external layout managers
}

// Count returns the number of tabs.
func (t *TabWidget) Count() int {
	return len(t.tabs)
}

// Tab returns the tab at the given index.
func (t *TabWidget) Tab(index int) *Tab {
	if index < 0 || index >= len(t.tabs) {
		return nil
	}
	return t.tabs[index]
}

// CurrentIndex returns the current tab index.
func (t *TabWidget) CurrentIndex() int {
	return t.currentIndex
}

// SetCurrentIndex sets the current tab index.
func (t *TabWidget) SetCurrentIndex(index int) {
	if index < 0 || index >= len(t.tabs) {
		return
	}
	if t.currentIndex == index {
		return
	}

	// Check if tab is enabled
	if !t.tabs[index].Enabled {
		return
	}

	t.currentIndex = index
	t.Update()

	if t.onCurrentChanged != nil {
		t.onCurrentChanged(index)
	}
}

// CurrentWidget returns the current tab's content widget.
func (t *TabWidget) CurrentWidget() core.Widget {
	if t.currentIndex < 0 || t.currentIndex >= len(t.tabs) {
		return nil
	}
	return t.tabs[t.currentIndex].Content
}

// SetTabText sets the text of a tab.
func (t *TabWidget) SetTabText(index int, text string) {
	if index < 0 || index >= len(t.tabs) {
		return
	}
	t.tabs[index].Text = text
	t.Update()
}

// TabText returns the text of a tab.
func (t *TabWidget) TabText(index int) string {
	if index < 0 || index >= len(t.tabs) {
		return ""
	}
	return t.tabs[index].Text
}

// SetTabIcon sets the icon of a tab.
func (t *TabWidget) SetTabIcon(index int, icon *style.TextIcon) {
	if index < 0 || index >= len(t.tabs) {
		return
	}
	t.tabs[index].Icon = icon
	t.Update()
}

// SetTabEnabled sets whether a tab is enabled.
func (t *TabWidget) SetTabEnabled(index int, enabled bool) {
	if index < 0 || index >= len(t.tabs) {
		return
	}
	t.tabs[index].Enabled = enabled
	t.Update()
}

// IsTabEnabled returns whether a tab is enabled.
func (t *TabWidget) IsTabEnabled(index int) bool {
	if index < 0 || index >= len(t.tabs) {
		return false
	}
	return t.tabs[index].Enabled
}

// TabPosition returns the tab bar position.
func (t *TabWidget) TabPosition() TabPosition {
	return t.tabPosition
}

// SetTabPosition sets the tab bar position.
func (t *TabWidget) SetTabPosition(position TabPosition) {
	t.tabPosition = position
	t.Update()
}

// IsMovable returns whether tabs can be reordered.
func (t *TabWidget) IsMovable() bool {
	return t.movable
}

// SetMovable sets whether tabs can be reordered.
func (t *TabWidget) SetMovable(movable bool) {
	t.movable = movable
}

// IsClosable returns whether tabs have close buttons.
func (t *TabWidget) IsClosable() bool {
	return t.closable
}

// SetClosable sets whether tabs have close buttons.
func (t *TabWidget) SetClosable(closable bool) {
	t.closable = closable
	t.Update()
}

// SetOnCurrentChanged sets the current changed callback.
func (t *TabWidget) SetOnCurrentChanged(handler func(index int)) {
	t.onCurrentChanged = handler
}

// SetOnTabCloseRequested sets the tab close requested callback.
func (t *TabWidget) SetOnTabCloseRequested(handler func(index int)) {
	t.onTabCloseRequested = handler
}

// tabBarHeight returns the height of the tab bar.
func (t *TabWidget) tabBarHeight() core.Unit {
	metrics := core.DefaultCellMetrics()
	return metrics.CellHeight
}

// contentBounds returns the bounds for the content area.
func (t *TabWidget) contentBounds() core.UnitRect {
	bounds := t.Bounds()
	tabHeight := t.tabBarHeight()

	switch t.tabPosition {
	case TabsTop:
		return core.UnitRect{
			X:      0,
			Y:      tabHeight,
			Width:  bounds.Width,
			Height: bounds.Height - tabHeight,
		}
	case TabsBottom:
		return core.UnitRect{
			X:      0,
			Y:      0,
			Width:  bounds.Width,
			Height: bounds.Height - tabHeight,
		}
	case TabsLeft:
		tabWidth := t.calculateTabBarWidth()
		return core.UnitRect{
			X:      tabWidth,
			Y:      0,
			Width:  bounds.Width - tabWidth,
			Height: bounds.Height,
		}
	case TabsRight:
		tabWidth := t.calculateTabBarWidth()
		return core.UnitRect{
			X:      0,
			Y:      0,
			Width:  bounds.Width - tabWidth,
			Height: bounds.Height,
		}
	}
	return bounds
}

func (t *TabWidget) calculateTabBarWidth() core.Unit {
	metrics := core.DefaultCellMetrics()
	maxLen := 10
	for _, tab := range t.tabs {
		if len(tab.Text) > maxLen {
			maxLen = len(tab.Text)
		}
	}
	return core.Unit(maxLen+4) * metrics.CellWidth
}

// SizeHint returns the preferred size.
func (t *TabWidget) SizeHint() core.UnitSize {
	metrics := core.DefaultCellMetrics()
	return core.UnitSize{
		Width:  metrics.TextWidth(40),
		Height: metrics.TextHeight(15),
	}
}

// Paint renders the tab widget.
func (t *TabWidget) Paint(p *core.Painter) {
	bounds := t.Bounds()
	theme := t.Theme()
	metrics := p.Metrics()

	// Draw background
	p.FillRect(core.UnitRect{Width: bounds.Width, Height: bounds.Height}, ' ', theme.Normal)

	// Draw tab bar based on position
	switch t.tabPosition {
	case TabsTop:
		t.paintTopTabs(p, bounds, theme, metrics)
	case TabsBottom:
		t.paintBottomTabs(p, bounds, theme, metrics)
	case TabsLeft:
		t.paintLeftTabs(p, bounds, theme, metrics)
	case TabsRight:
		t.paintRightTabs(p, bounds, theme, metrics)
	}

	// Draw content
	t.paintContent(p)
}

func (t *TabWidget) paintTopTabs(p *core.Painter, bounds core.UnitRect, theme *style.Theme, metrics core.CellMetrics) {
	tabHeight := t.tabBarHeight()

	// Draw tab bar background
	p.FillRect(core.UnitRect{Width: bounds.Width, Height: tabHeight}, ' ', theme.MenuBar)

	// Draw tabs with tab-style decorators:
	// Selected:   _/ TabText \_
	// Unselected:    TabText
	x := core.Unit(0)
	for i, tab := range t.tabs {
		// Each tab: 3 chars prefix + text + 3 chars suffix
		tabWidth := core.Unit(len(tab.Text)+6) * metrics.CellWidth

		var s style.CellStyle
		if !tab.Enabled {
			s = theme.Disabled
		} else if i == t.currentIndex {
			s = theme.WindowTitleFocused
		} else {
			s = theme.MenuBar
		}

		isSelected := i == t.currentIndex

		// Draw prefix: "_/ " for selected, "   " for unselected
		if isSelected {
			p.DrawCell(x, 0, '_', theme.MenuBar)
			p.DrawCell(x+metrics.CellWidth, 0, '/', theme.MenuBar)
			p.DrawCell(x+metrics.CellWidth*2, 0, ' ', s)
		} else {
			p.DrawCell(x, 0, ' ', theme.MenuBar)
			p.DrawCell(x+metrics.CellWidth, 0, ' ', theme.MenuBar)
			p.DrawCell(x+metrics.CellWidth*2, 0, ' ', theme.MenuBar)
		}

		// Draw tab text with tab's style
		textX := x + metrics.CellWidth*3
		for _, ch := range tab.Text {
			p.DrawCell(textX, 0, ch, s)
			textX += metrics.CellWidth
		}

		// Draw suffix: " \_" for selected, "   " for unselected
		suffixX := x + metrics.CellWidth*3 + core.Unit(len(tab.Text))*metrics.CellWidth
		if isSelected {
			p.DrawCell(suffixX, 0, ' ', s)
			p.DrawCell(suffixX+metrics.CellWidth, 0, '\\', theme.MenuBar)
			p.DrawCell(suffixX+metrics.CellWidth*2, 0, '_', theme.MenuBar)
		} else {
			p.DrawCell(suffixX, 0, ' ', theme.MenuBar)
			p.DrawCell(suffixX+metrics.CellWidth, 0, ' ', theme.MenuBar)
			p.DrawCell(suffixX+metrics.CellWidth*2, 0, ' ', theme.MenuBar)
		}

		// Draw close button if closable (before suffix)
		if t.closable || tab.Closable {
			closeX := suffixX - metrics.CellWidth
			p.DrawCell(closeX, 0, '×', s)
		}

		x += tabWidth
	}
}

func (t *TabWidget) paintBottomTabs(p *core.Painter, bounds core.UnitRect, theme *style.Theme, metrics core.CellMetrics) {
	tabHeight := t.tabBarHeight()
	tabY := bounds.Height - tabHeight

	// Draw tab bar background
	p.FillRect(core.UnitRect{Y: tabY, Width: bounds.Width, Height: tabHeight}, ' ', theme.MenuBar)

	// Draw tabs with tab-style decorators (inverted for bottom: \_  and  _/)
	x := core.Unit(0)
	for i, tab := range t.tabs {
		// Each tab: 3 chars prefix + text + 3 chars suffix
		tabWidth := core.Unit(len(tab.Text)+6) * metrics.CellWidth

		var s style.CellStyle
		if !tab.Enabled {
			s = theme.Disabled
		} else if i == t.currentIndex {
			s = theme.WindowTitleFocused
		} else {
			s = theme.MenuBar
		}

		isSelected := i == t.currentIndex

		// Draw prefix: " \_" for selected (inverted), "   " for unselected
		if isSelected {
			p.DrawCell(x, tabY, ' ', theme.MenuBar)
			p.DrawCell(x+metrics.CellWidth, tabY, '\\', theme.MenuBar)
			p.DrawCell(x+metrics.CellWidth*2, tabY, '_', s)
		} else {
			p.DrawCell(x, tabY, ' ', theme.MenuBar)
			p.DrawCell(x+metrics.CellWidth, tabY, ' ', theme.MenuBar)
			p.DrawCell(x+metrics.CellWidth*2, tabY, ' ', theme.MenuBar)
		}

		// Draw tab text
		textX := x + metrics.CellWidth*3
		for _, ch := range tab.Text {
			p.DrawCell(textX, tabY, ch, s)
			textX += metrics.CellWidth
		}

		// Draw suffix: "_/ " for selected (inverted), "   " for unselected
		suffixX := x + metrics.CellWidth*3 + core.Unit(len(tab.Text))*metrics.CellWidth
		if isSelected {
			p.DrawCell(suffixX, tabY, '_', s)
			p.DrawCell(suffixX+metrics.CellWidth, tabY, '/', theme.MenuBar)
			p.DrawCell(suffixX+metrics.CellWidth*2, tabY, ' ', theme.MenuBar)
		} else {
			p.DrawCell(suffixX, tabY, ' ', theme.MenuBar)
			p.DrawCell(suffixX+metrics.CellWidth, tabY, ' ', theme.MenuBar)
			p.DrawCell(suffixX+metrics.CellWidth*2, tabY, ' ', theme.MenuBar)
		}

		x += tabWidth
	}
}

func (t *TabWidget) paintLeftTabs(p *core.Painter, bounds core.UnitRect, theme *style.Theme, metrics core.CellMetrics) {
	tabWidth := t.calculateTabBarWidth()

	// Draw tab bar background
	p.FillRect(core.UnitRect{Width: tabWidth, Height: bounds.Height}, ' ', theme.MenuBar)

	// Draw tabs vertically
	y := core.Unit(0)
	for i, tab := range t.tabs {
		var s style.CellStyle
		if !tab.Enabled {
			s = theme.Disabled
		} else if i == t.currentIndex {
			s = theme.WindowTitleFocused
		} else {
			s = theme.MenuBar
		}

		// Draw tab
		p.FillRect(core.UnitRect{Y: y, Width: tabWidth, Height: metrics.CellHeight}, ' ', s)
		textX := metrics.CellWidth
		for _, ch := range tab.Text {
			if textX >= tabWidth-metrics.CellWidth {
				break
			}
			p.DrawCell(textX, y, ch, s)
			textX += metrics.CellWidth
		}

		y += metrics.CellHeight
	}

	// Draw separator line
	for i := core.Unit(0); i < bounds.Height; i += metrics.CellHeight {
		p.DrawCell(tabWidth-metrics.CellWidth, i, '│', theme.Normal)
	}
}

func (t *TabWidget) paintRightTabs(p *core.Painter, bounds core.UnitRect, theme *style.Theme, metrics core.CellMetrics) {
	tabWidth := t.calculateTabBarWidth()
	tabX := bounds.Width - tabWidth

	// Draw tab bar background
	p.FillRect(core.UnitRect{X: tabX, Width: tabWidth, Height: bounds.Height}, ' ', theme.MenuBar)

	// Draw tabs vertically
	y := core.Unit(0)
	for i, tab := range t.tabs {
		var s style.CellStyle
		if !tab.Enabled {
			s = theme.Disabled
		} else if i == t.currentIndex {
			s = theme.WindowTitleFocused
		} else {
			s = theme.MenuBar
		}

		// Draw tab
		p.FillRect(core.UnitRect{X: tabX, Y: y, Width: tabWidth, Height: metrics.CellHeight}, ' ', s)
		textX := tabX + metrics.CellWidth
		for _, ch := range tab.Text {
			if textX >= bounds.Width-metrics.CellWidth {
				break
			}
			p.DrawCell(textX, y, ch, s)
			textX += metrics.CellWidth
		}

		y += metrics.CellHeight
	}
}

func (t *TabWidget) paintContent(p *core.Painter) {
	if t.currentIndex < 0 || t.currentIndex >= len(t.tabs) {
		return
	}

	content := t.tabs[t.currentIndex].Content
	if content == nil {
		return
	}

	contentBounds := t.contentBounds()
	content.SetBounds(contentBounds)

	// Create clipped painter for content
	contentPainter := p.WithOffset(contentBounds.X, contentBounds.Y).
		WithClip(core.UnitRect{Width: contentBounds.Width, Height: contentBounds.Height})
	content.Paint(contentPainter)
}

// HandleKeyPress handles keyboard input.
func (t *TabWidget) HandleKeyPress(event core.KeyPressEvent) bool {
	// Pass to current content first
	if t.currentIndex >= 0 && t.currentIndex < len(t.tabs) {
		content := t.tabs[t.currentIndex].Content
		if content != nil && content.HandleKeyPress(event) {
			return true
		}
	}

	switch event.Key {
	case "^Tab", "C-Tab":
		// Next tab
		t.nextTab()
		return true

	case "^S-Tab", "C-S-Tab":
		// Previous tab
		t.prevTab()
		return true

	case "^PageDown":
		t.nextTab()
		return true

	case "^PageUp":
		t.prevTab()
		return true
	}

	return false
}

func (t *TabWidget) nextTab() {
	if len(t.tabs) == 0 {
		return
	}

	for i := 1; i <= len(t.tabs); i++ {
		idx := (t.currentIndex + i) % len(t.tabs)
		if t.tabs[idx].Enabled {
			t.SetCurrentIndex(idx)
			return
		}
	}
}

func (t *TabWidget) prevTab() {
	if len(t.tabs) == 0 {
		return
	}

	for i := 1; i <= len(t.tabs); i++ {
		idx := (t.currentIndex - i + len(t.tabs)) % len(t.tabs)
		if t.tabs[idx].Enabled {
			t.SetCurrentIndex(idx)
			return
		}
	}
}

// HandleMousePress handles mouse clicks.
func (t *TabWidget) HandleMousePress(event core.MousePressEvent) bool {
	if event.Button != core.LeftButton {
		return false
	}

	t.SetFocus()

	metrics := core.DefaultCellMetrics()
	tabHeight := t.tabBarHeight()

	// Check if click is in tab bar
	switch t.tabPosition {
	case TabsTop:
		if event.Y < tabHeight {
			t.handleTabBarClick(event.X)
			return true
		}
	case TabsBottom:
		bounds := t.Bounds()
		if event.Y >= bounds.Height-tabHeight {
			t.handleTabBarClick(event.X)
			return true
		}
	case TabsLeft:
		tabWidth := t.calculateTabBarWidth()
		if event.X < tabWidth {
			idx := int(event.Y / metrics.CellHeight)
			if idx >= 0 && idx < len(t.tabs) && t.tabs[idx].Enabled {
				t.SetCurrentIndex(idx)
			}
			return true
		}
	case TabsRight:
		bounds := t.Bounds()
		tabWidth := t.calculateTabBarWidth()
		if event.X >= bounds.Width-tabWidth {
			idx := int(event.Y / metrics.CellHeight)
			if idx >= 0 && idx < len(t.tabs) && t.tabs[idx].Enabled {
				t.SetCurrentIndex(idx)
			}
			return true
		}
	}

	// Pass to content
	if t.currentIndex >= 0 && t.currentIndex < len(t.tabs) {
		content := t.tabs[t.currentIndex].Content
		if content != nil {
			contentBounds := t.contentBounds()
			localEvent := event
			localEvent.X -= contentBounds.X
			localEvent.Y -= contentBounds.Y
			return content.HandleMousePress(localEvent)
		}
	}

	return true
}

func (t *TabWidget) handleTabBarClick(x core.Unit) {
	metrics := core.DefaultCellMetrics()
	tabX := core.Unit(0)

	for i, tab := range t.tabs {
		// Each tab: 3 chars prefix + text + 3 chars suffix
		tabWidth := core.Unit(len(tab.Text)+6) * metrics.CellWidth

		if x >= tabX && x < tabX+tabWidth {
			// Check for close button (in the text area, before suffix)
			textEnd := tabX + metrics.CellWidth*3 + core.Unit(len(tab.Text))*metrics.CellWidth
			if (t.closable || tab.Closable) && x >= textEnd-metrics.CellWidth && x < textEnd {
				if t.onTabCloseRequested != nil {
					t.onTabCloseRequested(i)
				}
				return
			}

			if tab.Enabled {
				t.SetCurrentIndex(i)
			}
			return
		}

		tabX += tabWidth
	}
}

// HandleFocusIn is called when focus is gained.
func (t *TabWidget) HandleFocusIn() {
	t.Update()
}

// HandleFocusOut is called when focus is lost.
func (t *TabWidget) HandleFocusOut() {
	t.Update()
}

// HandleMouseMove handles mouse movement.
func (t *TabWidget) HandleMouseMove(event core.MouseMoveEvent) bool {
	// Forward to current content
	if t.currentIndex >= 0 && t.currentIndex < len(t.tabs) {
		content := t.tabs[t.currentIndex].Content
		if content != nil {
			if handler, ok := content.(interface {
				HandleMouseMove(core.MouseMoveEvent) bool
			}); ok {
				contentBounds := t.contentBounds()
				localEvent := event
				localEvent.X -= contentBounds.X
				localEvent.Y -= contentBounds.Y
				return handler.HandleMouseMove(localEvent)
			}
		}
	}
	return false
}

// HandleMouseRelease handles mouse button release.
func (t *TabWidget) HandleMouseRelease(event core.MouseReleaseEvent) bool {
	// Forward to current content
	if t.currentIndex >= 0 && t.currentIndex < len(t.tabs) {
		content := t.tabs[t.currentIndex].Content
		if content != nil {
			if handler, ok := content.(interface {
				HandleMouseRelease(core.MouseReleaseEvent) bool
			}); ok {
				contentBounds := t.contentBounds()
				localEvent := event
				localEvent.X -= contentBounds.X
				localEvent.Y -= contentBounds.Y
				return handler.HandleMouseRelease(localEvent)
			}
		}
	}
	return false
}

// HandleResize is called when the tab widget is resized.
func (t *TabWidget) HandleResize(oldSize, newSize core.UnitSize) {
	// Update content bounds for the current tab
	if t.currentIndex >= 0 && t.currentIndex < len(t.tabs) {
		content := t.tabs[t.currentIndex].Content
		if content != nil {
			contentBounds := t.contentBounds()
			content.SetBounds(contentBounds)
		}
	}
}

// AccessibleInfo returns accessibility information.
func (t *TabWidget) AccessibleInfo() core.AccessibleInfo {
	info := t.AccessibleWidget.AccessibleInfo()
	info.Role = core.RoleTabList
	info.SetSize = len(t.tabs)

	if t.currentIndex >= 0 && t.currentIndex < len(t.tabs) {
		info.PositionInSet = t.currentIndex + 1
		info.Value = t.tabs[t.currentIndex].Text
	}

	if !t.IsEnabled() {
		info.State |= core.StateDisabled
	}

	return info
}
