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

	// Tab scrolling (when tabs don't fit)
	tabScrollOffset int  // First visible tab index
	scrollLeftHovered bool  // Mouse over [<] button while pressed
	scrollRightHovered bool // Mouse over [>] button while pressed
	scrollButtonPressed int // 0=none, -1=left, 1=right

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
	t.Init(t)
	// TabWidget can receive focus for tab bar keyboard navigation
	t.SetFocusPolicy(core.TabFocus)
	t.SetAccessibleRole(core.RoleTabList)
	return t
}

// SetBounds sets the widget's bounds and updates content layout.
func (t *TabWidget) SetBounds(bounds core.UnitRect) {
	oldSize := t.Bounds().Size()
	t.WidgetBase.SetBounds(bounds)
	newSize := bounds.Size()

	// Manually call our HandleResize since embedded SetBounds won't do it
	if oldSize != newSize {
		t.HandleResize(oldSize, newSize)
	}
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

// calculateTotalTabsWidth returns the total width needed to display all tabs.
// Format: [prefix][tab1 text][sep][tab2 text][sep]...
// - Prefix: 4 chars if first tab selected (" _/ "), else 2 ("  ")
// - Separator: 4 chars if adjacent to selected (" \_ " or " _/ "), else 2 ("  ")
func (t *TabWidget) calculateTotalTabsWidth() core.Unit {
	metrics := core.DefaultCellMetrics()
	if len(t.tabs) == 0 {
		return 0
	}

	// Prefix: 4 if first tab selected, else 2
	prefixWidth := 2
	if t.currentIndex == 0 {
		prefixWidth = 4
	}
	total := core.Unit(prefixWidth) * metrics.CellWidth

	for i, tab := range t.tabs {
		// Tab text
		total += core.Unit(len(tab.Text)) * metrics.CellWidth

		// Separator after tab: 4 if this or next tab is selected, else 2
		sepWidth := 2
		if i == t.currentIndex || (i+1 < len(t.tabs) && i+1 == t.currentIndex) {
			sepWidth = 4
		}
		total += core.Unit(sepWidth) * metrics.CellWidth
	}
	return total
}

// tabsNeedScrolling returns true if tabs don't fit and need scroll buttons.
func (t *TabWidget) tabsNeedScrolling() bool {
	bounds := t.Bounds()
	// Check if tabs fit the full width - don't pre-subtract scroll button space
	// since scroll buttons only appear when scrolling is actually needed
	return t.calculateTotalTabsWidth() > bounds.Width
}

// scrollButtonWidth returns the width of each scroll button.
func (t *TabWidget) scrollButtonWidth() core.Unit {
	return core.DefaultCellMetrics().TextWidth(3) // [<] or [>]
}

// ensureCurrentTabVisible adjusts scroll offset to make current tab visible.
func (t *TabWidget) ensureCurrentTabVisible() {
	if t.currentIndex < 0 || !t.tabsNeedScrolling() {
		t.tabScrollOffset = 0
		return
	}
	if t.currentIndex < t.tabScrollOffset {
		t.tabScrollOffset = t.currentIndex
	}
	// Check if current tab is past the visible area
	// (This is a simplified check - could be improved)
	maxVisible := t.tabScrollOffset + 3 // Rough estimate
	if t.currentIndex >= maxVisible && maxVisible < len(t.tabs) {
		t.tabScrollOffset = t.currentIndex - 2
		if t.tabScrollOffset < 0 {
			t.tabScrollOffset = 0
		}
	}
}

// canScrollLeft returns true if there are tabs to the left.
func (t *TabWidget) canScrollLeft() bool {
	return t.tabScrollOffset > 0
}

// canScrollRight returns true if there are more tabs to show on the right.
// This checks if the last tab is fully visible, not just if there are more tabs.
func (t *TabWidget) canScrollRight() bool {
	if t.tabScrollOffset >= len(t.tabs)-1 {
		return false
	}
	// Check if the last tab is fully visible
	return !t.isLastTabFullyVisible()
}

// isLastTabFullyVisible returns true if the last tab is completely visible.
func (t *TabWidget) isLastTabFullyVisible() bool {
	bounds := t.Bounds()
	metrics := core.DefaultCellMetrics()

	scrollButtonsWidth := core.Unit(0)
	if t.tabsNeedScrolling() {
		scrollButtonsWidth = metrics.TextWidth(6)
	}
	leftEllipseWidth := core.Unit(0)
	if t.tabScrollOffset > 0 {
		leftEllipseWidth = metrics.TextWidth(3)
	}
	availableWidth := bounds.Width - scrollButtonsWidth - leftEllipseWidth

	// Calculate width needed for visible tabs
	x := core.Unit(0)
	for i := t.tabScrollOffset; i < len(t.tabs); i++ {
		tab := t.tabs[i]
		isFirstVisible := i == t.tabScrollOffset
		isSelected := i == t.currentIndex
		isLastVisible := i == len(t.tabs)-1
		nextIsSelected := !isLastVisible && i+1 == t.currentIndex

		prefixWidth := 0
		if isFirstVisible {
			if isSelected {
				prefixWidth = 4
			} else {
				prefixWidth = 2
			}
		}
		sepWidth := 2
		if isSelected || nextIsSelected {
			sepWidth = 4
		}
		tabSlotWidth := core.Unit(prefixWidth+len(tab.Text)+sepWidth) * metrics.CellWidth
		x += tabSlotWidth

		if x > availableWidth {
			return false
		}
	}
	return true
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

	// Draw background using TabWidget's background color if set
	bgStyle := style.DefaultStyle()
	if bg := t.BackgroundColor(); bg != nil {
		bgStyle = bgStyle.WithBg(*bg)
	}
	p.FillRect(core.UnitRect{Width: bounds.Width, Height: bounds.Height}, ' ', bgStyle)

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
	hasFocus := t.HasFocus()

	// Tab bar style: silver on blue
	tabBarStyle := style.DefaultStyle().WithFg(style.ColorBrightWhite).WithBg(style.ColorBlue)
	// Selected tab style when unfocused: uses page control's background color
	selectedStyle := style.DefaultStyle().WithFg(style.ColorBrightYellow).Bold()
	if bg := t.BackgroundColor(); bg != nil {
		selectedStyle = selectedStyle.WithBg(*bg)
	} else {
		selectedStyle = selectedStyle.WithBg(style.ColorDefault)
	}
	// Focused selected tab style: yellow on teal (for angle brackets and title)
	focusedSelectedStyle := style.DefaultStyle().WithFg(style.ColorBrightYellow).WithBg(style.ColorCyan).Bold()
	// Pressed button style (inverted)
	pressedStyle := tabBarStyle.WithFg(tabBarStyle.Bg).WithBg(tabBarStyle.Fg)

	// Draw tab bar background
	p.FillRect(core.UnitRect{Width: bounds.Width, Height: tabHeight}, ' ', tabBarStyle)

	// Calculate if we need scroll buttons
	needsScrolling := t.tabsNeedScrolling()
	scrollButtonsWidth := core.Unit(0)
	if needsScrolling {
		scrollButtonsWidth = metrics.TextWidth(6) // [<][>] = 6 chars
	}

	// If scrolled right, show left ellipse indicator (clickable to scroll left)
	leftEllipseWidth := core.Unit(0)
	if t.tabScrollOffset > 0 {
		leftEllipseWidth = metrics.TextWidth(3) // "..."
		// Draw the left ellipse
		for i := 0; i < 3; i++ {
			p.DrawCell(metrics.CellToUnitsX(i), 0, '.', tabBarStyle)
		}
	}

	// Reserve space for right ellipsis when scrolling is needed (drawn later, right before buttons)
	rightEllipseWidth := core.Unit(0)
	if needsScrolling {
		rightEllipseWidth = metrics.TextWidth(3) // "..."
	}
	availableWidth := bounds.Width - scrollButtonsWidth - leftEllipseWidth - rightEllipseWidth

	// New tab format: [prefix][tab1 text][sep][tab2 text][sep]...
	// - Prefix: " _/ " (4 chars) if first visible tab is selected, else "  " (2 chars)
	// - Separator after each tab:
	//   - " \_ " (4 chars) if current tab is selected
	//   - " _/ " (4 chars) if next tab is selected
	//   - "  " (2 chars) otherwise
	x := leftEllipseWidth

	visibleTabs := t.tabs[t.tabScrollOffset:]
	for i := 0; i < len(visibleTabs); i++ {
		tabIndex := t.tabScrollOffset + i
		tab := visibleTabs[i]
		isSelected := tabIndex == t.currentIndex
		isFirstVisible := i == 0
		isLastVisible := tabIndex == len(t.tabs)-1
		nextIsSelected := !isLastVisible && tabIndex+1 == t.currentIndex

		// Calculate this tab's width
		prefixWidth := 0
		if isFirstVisible {
			prefixWidth = 4 // " _/ " if selected
			if !isSelected {
				prefixWidth = 2 // "  " if not selected
			}
		}
		sepWidth := 2 // Default "  "
		if isSelected || nextIsSelected {
			sepWidth = 4 // " \_ " or " _/ "
		}
		tabSlotWidth := core.Unit(prefixWidth+len(tab.Text)+sepWidth) * metrics.CellWidth

		// Check if this tab fits
		if x+tabSlotWidth > availableWidth {
			// Try to draw partial tab (ellipsis is drawn separately after the loop)
			remainingSpace := availableWidth - x
			minPartialWidth := metrics.TextWidth(prefixWidth) // just prefix needed
			if remainingSpace >= minPartialWidth {
				var s style.CellStyle
				if !tab.Enabled {
					s = theme.Disabled
				} else if isSelected {
					if hasFocus {
						s = focusedSelectedStyle
					} else {
						s = selectedStyle
					}
				} else {
					s = tabBarStyle
				}

				// Draw prefix if first visible
				if isFirstVisible {
					if isSelected {
						p.DrawCell(x, 0, ' ', tabBarStyle)
						p.DrawCell(x+metrics.CellWidth, 0, '_', tabBarStyle)
						p.DrawCell(x+metrics.CellWidth*2, 0, '/', tabBarStyle)
						if hasFocus {
							p.DrawCell(x+metrics.CellWidth*3, 0, '<', focusedSelectedStyle)
						} else {
							p.DrawCell(x+metrics.CellWidth*3, 0, ' ', tabBarStyle)
						}
						x += metrics.CellWidth * 4
					} else {
						p.DrawCell(x, 0, ' ', tabBarStyle)
						p.DrawCell(x+metrics.CellWidth, 0, ' ', tabBarStyle)
						x += metrics.CellWidth * 2
					}
				}

				// Calculate how much text we can show
				textSpace := int((availableWidth - x) / metrics.CellWidth)
				if textSpace < 0 {
					textSpace = 0
				}
				textRunes := []rune(tab.Text)
				charsToShow := textSpace
				if charsToShow > len(textRunes) {
					charsToShow = len(textRunes)
				}

				// Draw partial text
				for j := 0; j < charsToShow; j++ {
					p.DrawCell(x, 0, textRunes[j], s)
					x += metrics.CellWidth
				}
			}
			break
		}

		var s style.CellStyle
		if !tab.Enabled {
			s = theme.Disabled
		} else if isSelected {
			if hasFocus {
				s = focusedSelectedStyle
			} else {
				s = selectedStyle
			}
		} else {
			s = tabBarStyle
		}

		// Draw prefix if first visible tab
		if isFirstVisible {
			if isSelected {
				// " _/<" (4 chars) when focused, " _/ " when not focused
				p.DrawCell(x, 0, ' ', tabBarStyle)
				p.DrawCell(x+metrics.CellWidth, 0, '_', tabBarStyle)
				p.DrawCell(x+metrics.CellWidth*2, 0, '/', tabBarStyle)
				if hasFocus {
					p.DrawCell(x+metrics.CellWidth*3, 0, '<', focusedSelectedStyle)
				} else {
					p.DrawCell(x+metrics.CellWidth*3, 0, ' ', s)
				}
				x += metrics.CellWidth * 4
			} else {
				// "  " (2 chars)
				p.DrawCell(x, 0, ' ', tabBarStyle)
				p.DrawCell(x+metrics.CellWidth, 0, ' ', tabBarStyle)
				x += metrics.CellWidth * 2
			}
		}

		// Draw tab text
		textStartX := x
		for _, ch := range tab.Text {
			p.DrawCell(x, 0, ch, s)
			x += metrics.CellWidth
		}

		// Draw close button if closable (at end of text, before separator)
		if t.closable || tab.Closable {
			closeX := x - metrics.CellWidth
			p.DrawCell(closeX, 0, '×', s)
		}
		_ = textStartX // May use later for close button positioning

		// Draw separator after tab
		if isSelected {
			// ">\_ " (4 chars) when focused, " \_ " when not focused
			if hasFocus {
				p.DrawCell(x, 0, '>', focusedSelectedStyle)
			} else {
				p.DrawCell(x, 0, ' ', s)
			}
			p.DrawCell(x+metrics.CellWidth, 0, '\\', tabBarStyle)
			p.DrawCell(x+metrics.CellWidth*2, 0, '_', tabBarStyle)
			p.DrawCell(x+metrics.CellWidth*3, 0, ' ', tabBarStyle)
			x += metrics.CellWidth * 4
		} else if nextIsSelected {
			// " _/<" (4 chars) when focused, " _/ " when not focused
			// The trailing space/bracket is part of the selected tab's text area
			p.DrawCell(x, 0, ' ', tabBarStyle)
			p.DrawCell(x+metrics.CellWidth, 0, '_', tabBarStyle)
			p.DrawCell(x+metrics.CellWidth*2, 0, '/', tabBarStyle)
			if hasFocus {
				p.DrawCell(x+metrics.CellWidth*3, 0, '<', focusedSelectedStyle)
			} else {
				p.DrawCell(x+metrics.CellWidth*3, 0, ' ', selectedStyle)
			}
			x += metrics.CellWidth * 4
		} else {
			// "  " (2 chars) regular separator
			p.DrawCell(x, 0, ' ', tabBarStyle)
			p.DrawCell(x+metrics.CellWidth, 0, ' ', tabBarStyle)
			x += metrics.CellWidth * 2
		}
	}

	// Draw right ellipsis if tabs are truncated (right before scroll buttons)
	if needsScrolling && !t.isLastTabFullyVisible() {
		ellipsisX := bounds.Width - scrollButtonsWidth - metrics.TextWidth(3)
		for i := 0; i < 3; i++ {
			p.DrawCell(ellipsisX+core.Unit(i)*metrics.CellWidth, 0, '.', tabBarStyle)
		}
	}

	// Draw scroll buttons if needed
	if needsScrolling {
		buttonX := bounds.Width - scrollButtonsWidth
		disabledStyle := tabBarStyle.WithFg(style.ColorBrightBlack)

		// [<] button - disabled when can't scroll left
		canLeft := t.canScrollLeft()
		if canLeft {
			leftStyle := tabBarStyle
			if t.scrollButtonPressed == -1 && t.scrollLeftHovered {
				leftStyle = pressedStyle
			}
			p.DrawCell(buttonX, 0, '[', leftStyle)
			p.DrawCell(buttonX+metrics.CellWidth, 0, '<', leftStyle)
			p.DrawCell(buttonX+metrics.CellWidth*2, 0, ']', leftStyle)
		} else {
			// Disabled: " < " (no brackets, grayed out)
			p.DrawCell(buttonX, 0, ' ', disabledStyle)
			p.DrawCell(buttonX+metrics.CellWidth, 0, '<', disabledStyle)
			p.DrawCell(buttonX+metrics.CellWidth*2, 0, ' ', disabledStyle)
		}

		// [>] button - disabled when can't scroll right
		canRight := t.canScrollRight()
		if canRight {
			rightStyle := tabBarStyle
			if t.scrollButtonPressed == 1 && t.scrollRightHovered {
				rightStyle = pressedStyle
			}
			p.DrawCell(buttonX+metrics.CellWidth*3, 0, '[', rightStyle)
			p.DrawCell(buttonX+metrics.CellWidth*4, 0, '>', rightStyle)
			p.DrawCell(buttonX+metrics.CellWidth*5, 0, ']', rightStyle)
		} else {
			// Disabled: " > " (no brackets, grayed out)
			p.DrawCell(buttonX+metrics.CellWidth*3, 0, ' ', disabledStyle)
			p.DrawCell(buttonX+metrics.CellWidth*4, 0, '>', disabledStyle)
			p.DrawCell(buttonX+metrics.CellWidth*5, 0, ' ', disabledStyle)
		}
	}
}

func (t *TabWidget) paintBottomTabs(p *core.Painter, bounds core.UnitRect, theme *style.Theme, metrics core.CellMetrics) {
	tabHeight := t.tabBarHeight()
	tabY := bounds.Height - tabHeight
	hasFocus := t.HasFocus()

	// Tab bar style: silver on blue
	tabBarStyle := style.DefaultStyle().WithFg(style.ColorBrightWhite).WithBg(style.ColorBlue)
	// Selected tab style when unfocused: uses page control's background color
	selectedStyle := style.DefaultStyle().WithFg(style.ColorBrightYellow).Bold()
	if bg := t.BackgroundColor(); bg != nil {
		selectedStyle = selectedStyle.WithBg(*bg)
	} else {
		selectedStyle = selectedStyle.WithBg(style.ColorDefault)
	}
	// Focused selected tab style: yellow on teal with underline (underline compensates for missing underscores)
	focusedSelectedStyle := style.DefaultStyle().WithFg(style.ColorBrightYellow).WithBg(style.ColorCyan).Bold().Underline()

	// Draw tab bar background
	p.FillRect(core.UnitRect{Y: tabY, Width: bounds.Width, Height: tabHeight}, ' ', tabBarStyle)

	// New tab format: [prefix][tab1 text][sep][tab2 text][sep]...
	// For bottom tabs, connectors are inverted:
	// - Prefix: " \_ " (4 chars) if first tab is selected, else "  " (2 chars)
	// - Separator after each tab:
	//   - " _/ " (4 chars) if current tab is selected
	//   - " \_ " (4 chars) if next tab is selected
	//   - "  " (2 chars) otherwise
	x := core.Unit(0)

	for i, tab := range t.tabs {
		isSelected := i == t.currentIndex
		isFirstVisible := i == 0
		isLastVisible := i == len(t.tabs)-1
		nextIsSelected := !isLastVisible && i+1 == t.currentIndex

		var s style.CellStyle
		if !tab.Enabled {
			s = theme.Disabled
		} else if isSelected {
			if hasFocus {
				s = focusedSelectedStyle
			} else {
				s = selectedStyle
			}
		} else {
			s = tabBarStyle
		}

		// Draw prefix if first tab (inverted for bottom: " \_" or " \<" when focused)
		// Note: 3 chars for selected (underscore replaces trailing space), 2 chars for unselected
		if isFirstVisible {
			if isSelected {
				p.DrawCell(x, tabY, ' ', tabBarStyle)
				p.DrawCell(x+metrics.CellWidth, tabY, '\\', tabBarStyle)
				if hasFocus {
					// Use < instead of _ when focused, with focusedSelectedStyle
					p.DrawCell(x+metrics.CellWidth*2, tabY, '<', focusedSelectedStyle)
				} else {
					p.DrawCell(x+metrics.CellWidth*2, tabY, '_', s)
				}
				x += metrics.CellWidth * 3
			} else {
				// "  " (2 chars)
				p.DrawCell(x, tabY, ' ', tabBarStyle)
				p.DrawCell(x+metrics.CellWidth, tabY, ' ', tabBarStyle)
				x += metrics.CellWidth * 2
			}
		}

		// Draw tab text
		for _, ch := range tab.Text {
			p.DrawCell(x, tabY, ch, s)
			x += metrics.CellWidth
		}

		// Draw separator after tab (inverted for bottom)
		// Note: 3 chars for selected (underscore replaces leading space), 2 chars for regular
		if isSelected {
			// "_/ " after selected tab - use > instead of _ when focused
			if hasFocus {
				p.DrawCell(x, tabY, '>', focusedSelectedStyle)
			} else {
				p.DrawCell(x, tabY, '_', s)
			}
			p.DrawCell(x+metrics.CellWidth, tabY, '/', tabBarStyle)
			p.DrawCell(x+metrics.CellWidth*2, tabY, ' ', tabBarStyle)
			x += metrics.CellWidth * 3
		} else if nextIsSelected {
			// " \_" (3 chars) before selected tab - use < instead of _ when focused
			p.DrawCell(x, tabY, ' ', tabBarStyle)
			p.DrawCell(x+metrics.CellWidth, tabY, '\\', tabBarStyle)
			if hasFocus {
				p.DrawCell(x+metrics.CellWidth*2, tabY, '<', focusedSelectedStyle)
			} else {
				p.DrawCell(x+metrics.CellWidth*2, tabY, '_', selectedStyle)
			}
			x += metrics.CellWidth * 3
		} else {
			// "  " (2 chars) regular separator
			p.DrawCell(x, tabY, ' ', tabBarStyle)
			p.DrawCell(x+metrics.CellWidth, tabY, ' ', tabBarStyle)
			x += metrics.CellWidth * 2
		}
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
	contentBounds := t.contentBounds()

	// Fill content area with TabWidget's background color if set
	// Use ColorDefault (ANSI 49) when no explicit background is set
	if bg := t.BackgroundColor(); bg != nil {
		contentStyle := style.DefaultStyle().WithBg(*bg)
		p.FillRect(core.UnitRect{
			X:      contentBounds.X,
			Y:      contentBounds.Y,
			Width:  contentBounds.Width,
			Height: contentBounds.Height,
		}, ' ', contentStyle)
	} else {
		// Use terminal default background (ANSI 49)
		contentStyle := style.DefaultStyle()
		p.FillRect(core.UnitRect{
			X:      contentBounds.X,
			Y:      contentBounds.Y,
			Width:  contentBounds.Width,
			Height: contentBounds.Height,
		}, ' ', contentStyle)
	}

	if t.currentIndex < 0 || t.currentIndex >= len(t.tabs) {
		return
	}

	content := t.tabs[t.currentIndex].Content
	if content == nil {
		return
	}

	// Set content bounds without X,Y offset - painter handles positioning
	content.SetBounds(core.UnitRect{
		X:      0,
		Y:      0,
		Width:  contentBounds.Width,
		Height: contentBounds.Height,
	})

	// Create clipped painter for content at the content area position
	contentPainter := p.WithOffset(contentBounds.X, contentBounds.Y).
		WithClip(core.UnitRect{Width: contentBounds.Width, Height: contentBounds.Height})
	content.Paint(contentPainter)
}

// HandleKeyPress handles keyboard input.
func (t *TabWidget) HandleKeyPress(event core.KeyPressEvent) bool {
	// When TabWidget has focus, handle tab bar navigation
	if t.HasFocus() {
		switch event.Key {
		case "Left":
			t.prevTabAndEnsureVisible()
			return true
		case "Right":
			t.nextTabAndEnsureVisible()
			return true
		case "C-Left", "M-Left", "A-Left":
			// Jump to first tab
			t.firstTab()
			return true
		case "C-Right", "M-Right", "A-Right":
			// Jump to last tab
			t.lastTab()
			return true
		}
	}

	// Pass to current content
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

// nextTabAndEnsureVisible moves to next tab and ensures it's fully visible.
func (t *TabWidget) nextTabAndEnsureVisible() {
	if len(t.tabs) == 0 {
		return
	}

	for i := 1; i <= len(t.tabs); i++ {
		idx := (t.currentIndex + i) % len(t.tabs)
		if t.tabs[idx].Enabled {
			t.SetCurrentIndex(idx)
			t.ensureTabFullyVisible(idx)
			return
		}
	}
}

// prevTabAndEnsureVisible moves to previous tab and ensures it's fully visible.
func (t *TabWidget) prevTabAndEnsureVisible() {
	if len(t.tabs) == 0 {
		return
	}

	for i := 1; i <= len(t.tabs); i++ {
		idx := (t.currentIndex - i + len(t.tabs)) % len(t.tabs)
		if t.tabs[idx].Enabled {
			t.SetCurrentIndex(idx)
			t.ensureTabFullyVisible(idx)
			return
		}
	}
}

// firstTab jumps to the first enabled tab.
func (t *TabWidget) firstTab() {
	for i := 0; i < len(t.tabs); i++ {
		if t.tabs[i].Enabled {
			t.SetCurrentIndex(i)
			t.ensureTabFullyVisible(i)
			return
		}
	}
}

// lastTab jumps to the last enabled tab.
func (t *TabWidget) lastTab() {
	for i := len(t.tabs) - 1; i >= 0; i-- {
		if t.tabs[i].Enabled {
			t.SetCurrentIndex(i)
			t.ensureTabFullyVisible(i)
			return
		}
	}
}

// HandleMousePress handles mouse clicks.
func (t *TabWidget) HandleMousePress(event core.MousePressEvent) bool {
	if event.Button != core.LeftButton {
		return false
	}

	// Note: We intentionally don't call SetFocus() here.
	// TabWidget focus is for keyboard tab order navigation only,
	// not for mouse interaction.

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
	bounds := t.Bounds()

	// Check if clicking on left ellipse (scroll left by one)
	if t.tabScrollOffset > 0 {
		leftEllipseWidth := metrics.TextWidth(3)
		if x < leftEllipseWidth {
			t.tabScrollOffset--
			t.Update()
			return
		}
	}

	// Check if clicking on scroll buttons
	if t.tabsNeedScrolling() {
		scrollButtonsWidth := metrics.TextWidth(6)
		buttonX := bounds.Width - scrollButtonsWidth

		// [<] button (3 chars wide) - only active if can scroll left
		if x >= buttonX && x < buttonX+metrics.TextWidth(3) {
			if t.canScrollLeft() {
				t.scrollButtonPressed = -1
				t.scrollLeftHovered = true
				t.Update()
			}
			return
		}

		// [>] button (3 chars wide) - only active if can scroll right
		if x >= buttonX+metrics.TextWidth(3) && x < buttonX+scrollButtonsWidth {
			if t.canScrollRight() {
				t.scrollButtonPressed = 1
				t.scrollRightHovered = true
				t.Update()
			}
			return
		}
	}

	// Calculate available width for tabs
	scrollButtonsWidth := core.Unit(0)
	if t.tabsNeedScrolling() {
		scrollButtonsWidth = metrics.TextWidth(6)
	}
	leftEllipseWidth := core.Unit(0)
	if t.tabScrollOffset > 0 {
		leftEllipseWidth = metrics.TextWidth(3)
	}
	availableWidth := bounds.Width - scrollButtonsWidth - leftEllipseWidth

	// New tab format: [prefix][tab1 text][sep][tab2 text][sep]...
	// - Prefix: 4 chars if first visible tab is selected, else 2 chars
	// - Separator: 4 chars if adjacent to selected, else 2 chars
	tabX := leftEllipseWidth
	for i := t.tabScrollOffset; i < len(t.tabs); i++ {
		tab := t.tabs[i]
		isFirstVisible := i == t.tabScrollOffset
		isSelected := i == t.currentIndex
		isLastVisible := i == len(t.tabs)-1
		nextIsSelected := !isLastVisible && i+1 == t.currentIndex

		// Calculate this tab's width
		prefixWidth := 0
		if isFirstVisible {
			prefixWidth = 4 // " _/ " if selected
			if !isSelected {
				prefixWidth = 2 // "  " if not selected
			}
		}
		sepWidth := 2 // Default "  "
		if isSelected || nextIsSelected {
			sepWidth = 4 // " \_ " or " _/ "
		}
		tabSlotWidth := core.Unit(prefixWidth+len(tab.Text)+sepWidth) * metrics.CellWidth

		// Check if this tab doesn't fully fit (partial tab with ellipsis)
		if tabX+tabSlotWidth > availableWidth {
			// Click is in the partial tab area - select this tab and scroll to show it
			if x >= tabX && x < availableWidth {
				if tab.Enabled {
					t.SetCurrentIndex(i)
					t.ensureTabFullyVisible(i)
				}
			}
			return
		}

		if x >= tabX && x < tabX+tabSlotWidth {
			// Calculate where text starts and ends
			textStartX := tabX + core.Unit(prefixWidth)*metrics.CellWidth
			textEndX := textStartX + core.Unit(len(tab.Text))*metrics.CellWidth

			// Check for close button (at end of text)
			if (t.closable || tab.Closable) && x >= textEndX-metrics.CellWidth && x < textEndX {
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

		tabX += tabSlotWidth
	}
}

// ensureTabFullyVisible scrolls to make the given tab fully visible.
func (t *TabWidget) ensureTabFullyVisible(index int) {
	if index < 0 || index >= len(t.tabs) {
		return
	}

	// If tab is before visible area, scroll left to show it
	if index < t.tabScrollOffset {
		t.tabScrollOffset = index
		t.Update()
		return
	}

	// Check if tab is fully visible
	bounds := t.Bounds()
	metrics := core.DefaultCellMetrics()

	scrollButtonsWidth := core.Unit(0)
	if t.tabsNeedScrolling() {
		scrollButtonsWidth = metrics.TextWidth(6)
	}

	// Try scrolling right until the tab is fully visible
	for t.tabScrollOffset < index {
		leftEllipseWidth := core.Unit(0)
		if t.tabScrollOffset > 0 {
			leftEllipseWidth = metrics.TextWidth(3)
		}
		availableWidth := bounds.Width - scrollButtonsWidth - leftEllipseWidth

		// Calculate if tab at index fits
		x := core.Unit(0)
		fits := true
		for i := t.tabScrollOffset; i <= index; i++ {
			tab := t.tabs[i]
			isFirstVisible := i == t.tabScrollOffset
			isSelected := i == t.currentIndex
			isLastVisible := i == len(t.tabs)-1
			nextIsSelected := !isLastVisible && i+1 == t.currentIndex

			prefixWidth := 0
			if isFirstVisible {
				if isSelected {
					prefixWidth = 4
				} else {
					prefixWidth = 2
				}
			}
			sepWidth := 2
			if isSelected || nextIsSelected {
				sepWidth = 4
			}
			tabSlotWidth := core.Unit(prefixWidth+len(tab.Text)+sepWidth) * metrics.CellWidth
			x += tabSlotWidth

			if i == index && x > availableWidth {
				fits = false
				break
			}
		}

		if fits {
			break
		}
		t.tabScrollOffset++
	}
	t.Update()
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
	// If tracking scroll button press, update hover state
	if t.scrollButtonPressed != 0 {
		metrics := core.DefaultCellMetrics()
		bounds := t.Bounds()
		scrollButtonsWidth := metrics.TextWidth(6)
		buttonX := bounds.Width - scrollButtonsWidth
		tabHeight := t.tabBarHeight()

		// Must be in tab bar
		inTabBar := event.Y >= 0 && event.Y < tabHeight

		if t.scrollButtonPressed == -1 {
			// Tracking [<] button
			newHovered := inTabBar && event.X >= buttonX && event.X < buttonX+metrics.TextWidth(3)
			if newHovered != t.scrollLeftHovered {
				t.scrollLeftHovered = newHovered
				t.Update()
			}
		} else if t.scrollButtonPressed == 1 {
			// Tracking [>] button
			newHovered := inTabBar && event.X >= buttonX+metrics.TextWidth(3) && event.X < buttonX+scrollButtonsWidth
			if newHovered != t.scrollRightHovered {
				t.scrollRightHovered = newHovered
				t.Update()
			}
		}
		return true
	}

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
	// If tracking scroll button press, handle release
	if t.scrollButtonPressed != 0 {
		pressedButton := t.scrollButtonPressed
		wasLeftHovered := t.scrollLeftHovered
		wasRightHovered := t.scrollRightHovered

		// Clear press state
		t.scrollButtonPressed = 0
		t.scrollLeftHovered = false
		t.scrollRightHovered = false
		t.Update()

		// Only trigger action if still hovering
		if pressedButton == -1 && wasLeftHovered {
			// Scroll left
			if t.tabScrollOffset > 0 {
				t.tabScrollOffset--
				t.Update()
			}
		} else if pressedButton == 1 && wasRightHovered {
			// Scroll right
			if t.tabScrollOffset < len(t.tabs)-1 {
				t.tabScrollOffset++
				t.Update()
			}
		}
		return true
	}

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
			// Set bounds with 0,0 for content-relative coordinates
			// (painter handles positioning with WithOffset)
			content.SetBounds(core.UnitRect{
				X:      0,
				Y:      0,
				Width:  contentBounds.Width,
				Height: contentBounds.Height,
			})
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
