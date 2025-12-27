// Package widgets provides standard UI widgets for the TUI toolkit.
package widgets

import (
	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/style"
)

// ScrollBar represents a scrollbar widget.
type ScrollBar struct {
	core.WidgetBase
	core.AccessibleWidget

	orientation  core.Orientation
	minimum      int
	maximum      int
	value        int
	pageStep     int
	singleStep   int

	// Appearance
	tracking bool // Update value while dragging

	// Drag state
	dragging   bool
	dragOffset int

	// Callbacks
	onValueChanged func(value int)
}

// NewScrollBar creates a new scrollbar.
func NewScrollBar(orientation core.Orientation) *ScrollBar {
	s := &ScrollBar{
		orientation: orientation,
		minimum:     0,
		maximum:     100,
		value:       0,
		pageStep:    10,
		singleStep:  1,
		tracking:    true,
	}
	s.WidgetBase = *core.NewWidgetBase()
	s.Init(s)
	s.SetFocusPolicy(core.NoFocus)
	s.SetAccessibleRole(core.RoleScrollBar)
	return s
}

// Orientation returns the orientation.
func (s *ScrollBar) Orientation() core.Orientation {
	return s.orientation
}

// Value returns the current value.
func (s *ScrollBar) Value() int {
	return s.value
}

// SetValue sets the current value.
func (s *ScrollBar) SetValue(value int) {
	if value < s.minimum {
		value = s.minimum
	}
	if value > s.maximum {
		value = s.maximum
	}
	if s.value == value {
		return
	}
	s.value = value
	s.Update()
	if s.onValueChanged != nil {
		s.onValueChanged(value)
	}
}

// Minimum returns the minimum value.
func (s *ScrollBar) Minimum() int {
	return s.minimum
}

// SetMinimum sets the minimum value.
func (s *ScrollBar) SetMinimum(min int) {
	s.minimum = min
	if s.value < min {
		s.SetValue(min)
	}
}

// Maximum returns the maximum value.
func (s *ScrollBar) Maximum() int {
	return s.maximum
}

// SetMaximum sets the maximum value.
func (s *ScrollBar) SetMaximum(max int) {
	s.maximum = max
	if s.value > max {
		s.SetValue(max)
	}
}

// SetRange sets the minimum and maximum values.
func (s *ScrollBar) SetRange(min, max int) {
	s.minimum = min
	s.maximum = max
	if s.value < min {
		s.value = min
	}
	if s.value > max {
		s.value = max
	}
	s.Update()
}

// PageStep returns the page step.
func (s *ScrollBar) PageStep() int {
	return s.pageStep
}

// SetPageStep sets the page step.
func (s *ScrollBar) SetPageStep(step int) {
	s.pageStep = step
}

// SingleStep returns the single step.
func (s *ScrollBar) SingleStep() int {
	return s.singleStep
}

// SetSingleStep sets the single step.
func (s *ScrollBar) SetSingleStep(step int) {
	s.singleStep = step
}

// SetOnValueChanged sets the value changed callback.
func (s *ScrollBar) SetOnValueChanged(handler func(value int)) {
	s.onValueChanged = handler
}

// SizeHint returns the preferred size.
func (s *ScrollBar) SizeHint() core.UnitSize {
	metrics := core.DefaultCellMetrics()
	if s.orientation == core.Horizontal {
		return core.UnitSize{
			Width:  metrics.CellWidth * 20,
			Height: metrics.CellHeight,
		}
	}
	return core.UnitSize{
		Width:  metrics.CellWidth,
		Height: metrics.CellHeight * 10,
	}
}

// Paint renders the scrollbar.
func (s *ScrollBar) Paint(p *core.Painter) {
	bounds := s.Bounds()
	theme := s.Theme()
	metrics := p.Metrics()

	if s.orientation == core.Horizontal {
		s.paintHorizontal(p, bounds, theme, metrics)
	} else {
		s.paintVertical(p, bounds, theme, metrics)
	}
}

func (s *ScrollBar) paintHorizontal(p *core.Painter, bounds core.UnitRect, theme *style.Theme, metrics core.CellMetrics) {
	// Draw track
	p.FillRect(core.UnitRect{Width: bounds.Width, Height: bounds.Height}, '░', theme.Disabled)

	// Calculate thumb using ListView-style formula:
	// thumbSize = visibleCount² / totalItems
	// where visibleCount = trackCells, totalItems = maximum + trackCells (when min=0)
	if s.maximum > s.minimum {
		trackCells := metrics.CharsForWidth(bounds.Width)
		// totalItems = scrollRange + visibleCount = (max - min) + trackCells
		totalItems := s.maximum - s.minimum + trackCells
		thumbSize := trackCells * trackCells / totalItems
		if thumbSize < 1 {
			thumbSize = 1
		}
		if thumbSize > trackCells {
			thumbSize = trackCells
		}

		// thumbPos = scrollOffset * scrollableTrack / maxScroll
		scrollableTrack := trackCells - thumbSize
		maxScroll := s.maximum - s.minimum
		thumbPos := 0
		if maxScroll > 0 && scrollableTrack > 0 {
			thumbPos = (s.value - s.minimum) * scrollableTrack / maxScroll
		}
		if thumbPos < 0 {
			thumbPos = 0
		}

		// Draw thumb
		for i := 0; i < thumbSize; i++ {
			x := core.Unit(thumbPos+i) * metrics.CellWidth
			p.DrawCell(x, 0, '█', theme.Normal)
		}
	}
}

func (s *ScrollBar) paintVertical(p *core.Painter, bounds core.UnitRect, theme *style.Theme, metrics core.CellMetrics) {
	// Draw track
	p.FillRect(core.UnitRect{Width: bounds.Width, Height: bounds.Height}, '░', theme.Disabled)

	// Calculate thumb using ListView-style formula:
	// thumbSize = visibleCount² / totalItems
	// where visibleCount = trackCells, totalItems = maximum + trackCells (when min=0)
	if s.maximum > s.minimum {
		trackCells := int(bounds.Height / metrics.CellHeight)
		// totalItems = scrollRange + visibleCount = (max - min) + trackCells
		totalItems := s.maximum - s.minimum + trackCells
		thumbSize := trackCells * trackCells / totalItems
		if thumbSize < 1 {
			thumbSize = 1
		}
		if thumbSize > trackCells {
			thumbSize = trackCells
		}

		// thumbPos = scrollOffset * scrollableTrack / maxScroll
		scrollableTrack := trackCells - thumbSize
		maxScroll := s.maximum - s.minimum
		thumbPos := 0
		if maxScroll > 0 && scrollableTrack > 0 {
			thumbPos = (s.value - s.minimum) * scrollableTrack / maxScroll
		}
		if thumbPos < 0 {
			thumbPos = 0
		}

		// Draw thumb
		for i := 0; i < thumbSize; i++ {
			y := core.Unit(thumbPos+i) * metrics.CellHeight
			p.FillRect(core.UnitRect{Y: y, Width: bounds.Width, Height: metrics.CellHeight}, '█', theme.Normal)
		}
	}
}

// HandleMousePress handles mouse clicks.
func (s *ScrollBar) HandleMousePress(event core.MousePressEvent) bool {
	if event.Button != core.LeftButton {
		return false
	}

	metrics := core.DefaultCellMetrics()
	bounds := s.Bounds()

	if s.orientation == core.Horizontal {
		clickPos := metrics.UnitsToCellX(event.X)
		trackCells := metrics.CharsForWidth(bounds.Width)
		// Use ListView-style formula
		totalItems := s.maximum - s.minimum + trackCells
		thumbSize := trackCells * trackCells / totalItems
		if thumbSize < 1 {
			thumbSize = 1
		}
		scrollableTrack := trackCells - thumbSize
		maxScroll := s.maximum - s.minimum
		thumbPos := 0
		if maxScroll > 0 && scrollableTrack > 0 {
			thumbPos = (s.value - s.minimum) * scrollableTrack / maxScroll
		}

		if clickPos >= thumbPos && clickPos < thumbPos+thumbSize {
			// Start dragging
			s.dragging = true
			s.dragOffset = clickPos - thumbPos
		} else if clickPos < thumbPos {
			// Page up
			s.SetValue(s.value - s.pageStep)
		} else {
			// Page down
			s.SetValue(s.value + s.pageStep)
		}
	} else {
		clickPos := int(event.Y / metrics.CellHeight)
		trackCells := int(bounds.Height / metrics.CellHeight)
		// Use ListView-style formula
		totalItems := s.maximum - s.minimum + trackCells
		thumbSize := trackCells * trackCells / totalItems
		if thumbSize < 1 {
			thumbSize = 1
		}
		scrollableTrack := trackCells - thumbSize
		maxScroll := s.maximum - s.minimum
		thumbPos := 0
		if maxScroll > 0 && scrollableTrack > 0 {
			thumbPos = (s.value - s.minimum) * scrollableTrack / maxScroll
		}

		if clickPos >= thumbPos && clickPos < thumbPos+thumbSize {
			// Start dragging
			s.dragging = true
			s.dragOffset = clickPos - thumbPos
		} else if clickPos < thumbPos {
			// Page up
			s.SetValue(s.value - s.pageStep)
		} else {
			// Page down
			s.SetValue(s.value + s.pageStep)
		}
	}

	return true
}

// HandleMouseMove handles mouse move/drag events.
func (s *ScrollBar) HandleMouseMove(event core.MouseMoveEvent) bool {
	if !s.dragging {
		return false
	}

	metrics := core.DefaultCellMetrics()
	bounds := s.Bounds()

	if s.orientation == core.Horizontal {
		dragPos := metrics.UnitsToCellX(event.X)
		trackCells := metrics.CharsForWidth(bounds.Width)
		// Use ListView-style formula
		totalItems := s.maximum - s.minimum + trackCells
		thumbSize := trackCells * trackCells / totalItems
		if thumbSize < 1 {
			thumbSize = 1
		}

		scrollableTrack := trackCells - thumbSize
		newThumbPos := dragPos - s.dragOffset
		if newThumbPos < 0 {
			newThumbPos = 0
		}
		if newThumbPos > scrollableTrack {
			newThumbPos = scrollableTrack
		}

		// Convert thumb position to scroll value
		maxScroll := s.maximum - s.minimum
		newValue := s.minimum
		if scrollableTrack > 0 {
			newValue = s.minimum + newThumbPos*maxScroll/scrollableTrack
		}
		if s.tracking {
			s.SetValue(newValue)
		}
	} else {
		dragPos := int(event.Y / metrics.CellHeight)
		trackCells := int(bounds.Height / metrics.CellHeight)
		// Use ListView-style formula
		totalItems := s.maximum - s.minimum + trackCells
		thumbSize := trackCells * trackCells / totalItems
		if thumbSize < 1 {
			thumbSize = 1
		}

		scrollableTrack := trackCells - thumbSize
		newThumbPos := dragPos - s.dragOffset
		if newThumbPos < 0 {
			newThumbPos = 0
		}
		if newThumbPos > scrollableTrack {
			newThumbPos = scrollableTrack
		}

		// Convert thumb position to scroll value
		maxScroll := s.maximum - s.minimum
		newValue := s.minimum
		if scrollableTrack > 0 {
			newValue = s.minimum + newThumbPos*maxScroll/scrollableTrack
		}
		if s.tracking {
			s.SetValue(newValue)
		}
	}

	return true
}

// HandleMouseRelease handles mouse release events.
func (s *ScrollBar) HandleMouseRelease(event core.MouseReleaseEvent) bool {
	if s.dragging {
		s.dragging = false
		return true
	}
	return false
}

// AccessibleInfo returns accessibility information.
func (s *ScrollBar) AccessibleInfo() core.AccessibleInfo {
	info := s.AccessibleWidget.AccessibleInfo()
	info.Role = core.RoleScrollBar
	info.ValueMin = string(rune('0' + s.minimum))
	info.ValueMax = string(rune('0' + s.maximum))

	return info
}

// ScrollArea provides a scrollable viewport for a widget.
type ScrollArea struct {
	core.WidgetBase
	core.AccessibleWidget

	content      core.Widget
	scrollX      int
	scrollY      int
	contentWidth  core.Unit
	contentHeight core.Unit

	// Scrollbars
	hScrollBar *ScrollBar
	vScrollBar *ScrollBar

	// Policy
	hScrollBarPolicy ScrollBarPolicy
	vScrollBarPolicy ScrollBarPolicy

	// Appearance
	widgetResizable bool // If true, content widget is resized to viewport
}

// ScrollBarPolicy determines when to show scrollbars.
type ScrollBarPolicy int

const (
	ScrollBarAsNeeded ScrollBarPolicy = iota
	ScrollBarAlwaysOn
	ScrollBarAlwaysOff
)

// NewScrollArea creates a new scroll area.
func NewScrollArea() *ScrollArea {
	s := &ScrollArea{
		hScrollBarPolicy: ScrollBarAsNeeded,
		vScrollBarPolicy: ScrollBarAsNeeded,
	}
	s.WidgetBase = *core.NewWidgetBase()
	s.Init(s)
	s.SetFocusPolicy(core.StrongFocus)
	s.SetAccessibleRole(core.RoleGroup)

	// Create scrollbars
	s.hScrollBar = NewScrollBar(core.Horizontal)
	s.hScrollBar.SetOnValueChanged(func(value int) {
		s.scrollX = value
		s.Update()
	})

	s.vScrollBar = NewScrollBar(core.Vertical)
	s.vScrollBar.SetOnValueChanged(func(value int) {
		s.scrollY = value
		s.Update()
	})

	return s
}

// Content returns the content widget.
func (s *ScrollArea) Content() core.Widget {
	return s.content
}

// SetContent sets the content widget.
func (s *ScrollArea) SetContent(content core.Widget) {
	if s.content != nil {
		s.content.SetParent(nil)
	}
	s.content = content
	if content != nil {
		content.SetParent(s)
	}
	s.updateScrollBars()
	s.Update()
}

// Children returns all child widgets (the content widget if set).
func (s *ScrollArea) Children() []core.Widget {
	if s.content != nil {
		return []core.Widget{s.content}
	}
	return nil
}

// AddChild adds a child widget (sets as content).
func (s *ScrollArea) AddChild(child core.Widget) {
	s.SetContent(child)
}

// RemoveChild removes a child widget.
func (s *ScrollArea) RemoveChild(child core.Widget) {
	if s.content == child {
		s.SetContent(nil)
	}
}

// ChildAt returns the child at the given position.
func (s *ScrollArea) ChildAt(pos core.UnitPoint) core.Widget {
	if s.content != nil {
		viewport := s.viewportBounds()
		if pos.X >= viewport.X && pos.X < viewport.X+viewport.Width &&
			pos.Y >= viewport.Y && pos.Y < viewport.Y+viewport.Height {
			return s.content
		}
	}
	return nil
}

// Layout arranges the content within the viewport.
func (s *ScrollArea) Layout() {
	s.updateScrollBars()
}

// LayoutManager returns nil (ScrollArea manages its own layout).
func (s *ScrollArea) LayoutManager() core.LayoutManager {
	return nil
}

// SetLayoutManager is a no-op (ScrollArea manages its own layout).
func (s *ScrollArea) SetLayoutManager(layout core.LayoutManager) {
	// ScrollArea manages its own layout, ignore external layout managers
}

// ScrollX returns the horizontal scroll position.
func (s *ScrollArea) ScrollX() int {
	return s.scrollX
}

// SetScrollX sets the horizontal scroll position.
func (s *ScrollArea) SetScrollX(x int) {
	s.hScrollBar.SetValue(x)
}

// ScrollY returns the vertical scroll position.
func (s *ScrollArea) ScrollY() int {
	return s.scrollY
}

// SetScrollY sets the vertical scroll position.
func (s *ScrollArea) SetScrollY(y int) {
	s.vScrollBar.SetValue(y)
}

// ScrollTo scrolls to a specific position.
func (s *ScrollArea) ScrollTo(x, y int) {
	s.SetScrollX(x)
	s.SetScrollY(y)
}

// EnsureVisible scrolls to make a point visible.
func (s *ScrollArea) EnsureVisible(x, y core.Unit) {
	s.EnsureRectVisible(core.UnitRect{X: x, Y: y, Width: 1, Height: 1})
}

// EnsureRectVisible scrolls to make a rectangle visible within the viewport.
// Prioritizes showing the left/top edge of the rectangle.
func (s *ScrollArea) EnsureRectVisible(rect core.UnitRect) {
	viewport := s.viewportBounds()
	metrics := core.DefaultCellMetrics()

	// Calculate cell positions
	cellX := metrics.UnitsToCellX(rect.X)
	cellY := int(rect.Y / metrics.CellHeight)
	cellWidth := metrics.CharsForWidth(rect.Width)
	cellHeight := int(rect.Height / metrics.CellHeight)
	if cellWidth < 1 {
		cellWidth = 1
	}
	if cellHeight < 1 {
		cellHeight = 1
	}

	viewCellWidth := metrics.CharsForWidth(viewport.Width)
	viewCellHeight := int(viewport.Height / metrics.CellHeight)

	// Adjust horizontal scroll if needed - prioritize showing left edge
	if cellX < s.scrollX {
		// Left edge is not visible - scroll left to show it
		s.SetScrollX(cellX)
	} else if cellX+cellWidth > s.scrollX+viewCellWidth && cellX >= s.scrollX {
		// Right edge extends past viewport but left edge is visible
		// Scroll right, but never hide the left edge
		newScrollX := cellX + cellWidth - viewCellWidth
		if newScrollX > cellX {
			// Would hide left edge - just show left edge instead
			newScrollX = cellX
		}
		s.SetScrollX(newScrollX)
	}

	// Adjust vertical scroll if needed - prioritize showing top edge
	if cellY < s.scrollY {
		// Top edge is not visible - scroll up to show it
		s.SetScrollY(cellY)
	} else if cellY+cellHeight > s.scrollY+viewCellHeight && cellY >= s.scrollY {
		// Bottom edge extends past viewport but top edge is visible
		// Scroll down, but never hide the top edge
		newScrollY := cellY + cellHeight - viewCellHeight
		if newScrollY > cellY {
			// Would hide top edge - just show top edge instead
			newScrollY = cellY
		}
		s.SetScrollY(newScrollY)
	}
}

// ScrollChildIntoView scrolls to make a descendant widget visible.
// Implements core.ScrollIntoViewHandler for automatic focus scrolling.
func (s *ScrollArea) ScrollChildIntoView(child core.Widget) {
	if s.content == nil {
		return
	}

	// Calculate the child's position relative to our content
	// by walking up from the child to our content widget
	childBounds := child.Bounds()
	offsetX := childBounds.X
	offsetY := childBounds.Y

	// Check if this is a proxy from ScrollRectIntoView - if so, the parent
	// will be the ScrollArea itself and the bounds already contain the
	// content-relative position (no need to accumulate more offsets)
	parent := child.Parent()
	if parent == s {
		// Bounds already contain content-relative coordinates
		s.EnsureRectVisible(core.UnitRect{
			X:      offsetX,
			Y:      offsetY,
			Width:  childBounds.Width,
			Height: childBounds.Height,
		})
		return
	}

	// Walk up the parent chain until we reach our content widget
	current := parent
	for current != nil {
		// Stop if we've reached our content widget
		if widget, ok := current.(core.Widget); ok {
			if widget == s.content {
				break
			}
			// Accumulate the offset from this parent
			parentBounds := widget.Bounds()
			offsetX += parentBounds.X
			offsetY += parentBounds.Y
			current = widget.Parent()
		} else {
			break
		}
	}

	// Ensure the calculated rectangle is visible
	s.EnsureRectVisible(core.UnitRect{
		X:      offsetX,
		Y:      offsetY,
		Width:  childBounds.Width,
		Height: childBounds.Height,
	})
}

// HorizontalScrollBarPolicy returns the horizontal scrollbar policy.
func (s *ScrollArea) HorizontalScrollBarPolicy() ScrollBarPolicy {
	return s.hScrollBarPolicy
}

// SetHorizontalScrollBarPolicy sets the horizontal scrollbar policy.
func (s *ScrollArea) SetHorizontalScrollBarPolicy(policy ScrollBarPolicy) {
	s.hScrollBarPolicy = policy
	s.updateScrollBars()
	s.Update()
}

// VerticalScrollBarPolicy returns the vertical scrollbar policy.
func (s *ScrollArea) VerticalScrollBarPolicy() ScrollBarPolicy {
	return s.vScrollBarPolicy
}

// SetVerticalScrollBarPolicy sets the vertical scrollbar policy.
func (s *ScrollArea) SetVerticalScrollBarPolicy(policy ScrollBarPolicy) {
	s.vScrollBarPolicy = policy
	s.updateScrollBars()
	s.Update()
}

// IsWidgetResizable returns whether the content widget is resized to viewport.
func (s *ScrollArea) IsWidgetResizable() bool {
	return s.widgetResizable
}

// SetWidgetResizable sets whether the content widget is resized to viewport.
func (s *ScrollArea) SetWidgetResizable(resizable bool) {
	s.widgetResizable = resizable
	s.Update()
}

// viewportBounds returns the viewport bounds (excluding scrollbars).
func (s *ScrollArea) viewportBounds() core.UnitRect {
	bounds := s.Bounds()
	metrics := core.DefaultCellMetrics()

	width := bounds.Width
	height := bounds.Height

	// Calculate scrollbar needs based on raw bounds to avoid recursion
	needsV, needsH := s.calculateScrollBarNeeds()

	if needsV {
		width -= metrics.CellWidth
	}
	if needsH {
		height -= metrics.CellHeight
	}

	return core.UnitRect{Width: width, Height: height}
}

// calculateScrollBarNeeds determines if scrollbars are needed without recursion.
// Returns (needsVertical, needsHorizontal).
func (s *ScrollArea) calculateScrollBarNeeds() (bool, bool) {
	bounds := s.Bounds()
	metrics := core.DefaultCellMetrics()

	// First pass: check if scrollbars needed with full bounds
	needsV := false
	needsH := false

	switch s.vScrollBarPolicy {
	case ScrollBarAlwaysOff:
		needsV = false
	case ScrollBarAlwaysOn:
		needsV = true
	default: // ScrollBarAsNeeded
		needsV = s.contentHeight > bounds.Height
	}

	switch s.hScrollBarPolicy {
	case ScrollBarAlwaysOff:
		needsH = false
	case ScrollBarAlwaysOn:
		needsH = true
	default: // ScrollBarAsNeeded
		needsH = s.contentWidth > bounds.Width
	}

	// Second pass: if one scrollbar is shown, it reduces space for the other
	if needsV && s.hScrollBarPolicy == ScrollBarAsNeeded {
		needsH = s.contentWidth > (bounds.Width - metrics.CellWidth)
	}
	if needsH && s.vScrollBarPolicy == ScrollBarAsNeeded {
		needsV = s.contentHeight > (bounds.Height - metrics.CellHeight)
	}

	return needsV, needsH
}

func (s *ScrollArea) needsHScrollBar() bool {
	_, needsH := s.calculateScrollBarNeeds()
	return needsH
}

func (s *ScrollArea) needsVScrollBar() bool {
	needsV, _ := s.calculateScrollBarNeeds()
	return needsV
}

func (s *ScrollArea) updateScrollBars() {
	if s.content == nil {
		return
	}

	hint := s.content.SizeHint()
	s.contentWidth = hint.Width
	s.contentHeight = hint.Height

	viewport := s.viewportBounds()
	metrics := core.DefaultCellMetrics()

	// Update horizontal scrollbar using ListView-style calculation
	// visible = viewport cells, total = content cells
	viewCellWidth := metrics.CharsForWidth(viewport.Width)
	contentCellWidth := metrics.CharsForWidth(s.contentWidth)
	maxScrollX := contentCellWidth - viewCellWidth
	if maxScrollX < 0 {
		maxScrollX = 0
	}
	s.hScrollBar.SetRange(0, maxScrollX)
	s.hScrollBar.SetPageStep(viewCellWidth)

	// Update vertical scrollbar using ListView-style calculation
	viewCellHeight := int(viewport.Height / metrics.CellHeight)
	contentCellHeight := int(s.contentHeight / metrics.CellHeight)
	maxScrollY := contentCellHeight - viewCellHeight
	if maxScrollY < 0 {
		maxScrollY = 0
	}
	s.vScrollBar.SetRange(0, maxScrollY)
	s.vScrollBar.SetPageStep(viewCellHeight)
}

// SizeHint returns the preferred size.
func (s *ScrollArea) SizeHint() core.UnitSize {
	metrics := core.DefaultCellMetrics()
	return core.UnitSize{
		Width:  metrics.TextWidth(30),
		Height: metrics.TextHeight(10),
	}
}

// Paint renders the scroll area.
func (s *ScrollArea) Paint(p *core.Painter) {
	bounds := s.Bounds()
	theme := s.Theme()
	metrics := p.Metrics()

	// Draw background using inherited background color
	bgStyle := theme.Normal.WithBg(s.EffectiveBackgroundColor())
	p.FillRect(core.UnitRect{Width: bounds.Width, Height: bounds.Height}, ' ', bgStyle)

	viewport := s.viewportBounds()

	// Draw content
	if s.content != nil {
		scrollOffsetX := core.Unit(s.scrollX) * metrics.CellWidth
		scrollOffsetY := core.Unit(s.scrollY) * metrics.CellHeight

		contentBounds := core.UnitRect{
			X:      -scrollOffsetX,
			Y:      -scrollOffsetY,
			Width:  s.contentWidth,
			Height: s.contentHeight,
		}

		if s.widgetResizable {
			contentBounds.Width = viewport.Width
			contentBounds.Height = viewport.Height
		}

		s.content.SetBounds(core.UnitRect{
			Width:  contentBounds.Width,
			Height: contentBounds.Height,
		})

		// Create clipped painter
		contentPainter := p.WithOffset(contentBounds.X, contentBounds.Y).
			WithClip(core.UnitRect{
				X:      scrollOffsetX,
				Y:      scrollOffsetY,
				Width:  viewport.Width,
				Height: viewport.Height,
			})
		s.content.Paint(contentPainter)
	}

	// Draw vertical scrollbar (use offset painter since scrollbar paints at 0,0)
	if s.needsVScrollBar() {
		s.vScrollBar.SetBounds(core.UnitRect{
			X:      0,
			Y:      0,
			Width:  metrics.CellWidth,
			Height: viewport.Height,
		})
		s.vScrollBar.Paint(p.WithOffset(viewport.Width, 0))
	}

	// Draw horizontal scrollbar (use offset painter since scrollbar paints at 0,0)
	if s.needsHScrollBar() {
		s.hScrollBar.SetBounds(core.UnitRect{
			X:      0,
			Y:      0,
			Width:  viewport.Width,
			Height: metrics.CellHeight,
		})
		s.hScrollBar.Paint(p.WithOffset(0, viewport.Height))
	}

	// Draw corner if both scrollbars visible
	if s.needsHScrollBar() && s.needsVScrollBar() {
		p.DrawCell(viewport.Width, viewport.Height, ' ', theme.Disabled)
	}
}

// HandleKeyPress handles keyboard input.
func (s *ScrollArea) HandleKeyPress(event core.KeyPressEvent) bool {
	// Pass to content first
	if s.content != nil && s.content.HandleKeyPress(event) {
		return true
	}

	switch event.Key {
	case "Up":
		s.SetScrollY(s.scrollY - s.vScrollBar.SingleStep())
		return true
	case "Down":
		s.SetScrollY(s.scrollY + s.vScrollBar.SingleStep())
		return true
	case "Left":
		s.SetScrollX(s.scrollX - s.hScrollBar.SingleStep())
		return true
	case "Right":
		s.SetScrollX(s.scrollX + s.hScrollBar.SingleStep())
		return true
	case "PageUp":
		s.SetScrollY(s.scrollY - s.vScrollBar.PageStep())
		return true
	case "PageDown":
		s.SetScrollY(s.scrollY + s.vScrollBar.PageStep())
		return true
	case "Home":
		s.SetScrollY(0)
		return true
	case "End":
		s.SetScrollY(s.vScrollBar.Maximum())
		return true
	}

	return false
}

// SetBounds sets the scroll area bounds and triggers layout.
func (s *ScrollArea) SetBounds(bounds core.UnitRect) {
	oldSize := s.Bounds().Size()
	s.WidgetBase.SetBounds(bounds)
	newSize := bounds.Size()
	if oldSize != newSize {
		s.HandleResize(oldSize, newSize)
	}
}

// HandleResize is called when the scroll area is resized.
func (s *ScrollArea) HandleResize(oldSize, newSize core.UnitSize) {
	// Update scrollbar ranges when viewport size changes
	s.updateScrollBars()
}

// HandleMousePress handles mouse clicks.
func (s *ScrollArea) HandleMousePress(event core.MousePressEvent) bool {
	viewport := s.viewportBounds()

	// Check if click is on vertical scrollbar
	if s.needsVScrollBar() && event.X >= viewport.Width {
		// Clear horizontal scrollbar drag state
		s.hScrollBar.dragging = false
		return s.vScrollBar.HandleMousePress(core.MousePressEvent{
			X:      event.X - viewport.Width,
			Y:      event.Y,
			Button: event.Button,
		})
	}

	// Check if click is on horizontal scrollbar
	if s.needsHScrollBar() && event.Y >= viewport.Height {
		// Clear vertical scrollbar drag state
		s.vScrollBar.dragging = false
		return s.hScrollBar.HandleMousePress(core.MousePressEvent{
			X:      event.X,
			Y:      event.Y - viewport.Height,
			Button: event.Button,
		})
	}

	// Click is on content area - clear both scrollbar drag states
	s.vScrollBar.dragging = false
	s.hScrollBar.dragging = false

	// Pass to content
	if s.content != nil {
		metrics := core.DefaultCellMetrics()
		scrollOffsetX := core.Unit(s.scrollX) * metrics.CellWidth
		scrollOffsetY := core.Unit(s.scrollY) * metrics.CellHeight

		return s.content.HandleMousePress(core.MousePressEvent{
			X:      event.X + scrollOffsetX,
			Y:      event.Y + scrollOffsetY,
			Button: event.Button,
		})
	}

	return false
}

// HandleMouseMove handles mouse move/drag events.
func (s *ScrollArea) HandleMouseMove(event core.MouseMoveEvent) bool {
	viewport := s.viewportBounds()

	// Forward to scrollbars if dragging
	if s.vScrollBar.dragging {
		return s.vScrollBar.HandleMouseMove(core.MouseMoveEvent{
			X: event.X - viewport.Width,
			Y: event.Y,
		})
	}

	if s.hScrollBar.dragging {
		return s.hScrollBar.HandleMouseMove(core.MouseMoveEvent{
			X: event.X,
			Y: event.Y - viewport.Height,
		})
	}

	// Forward to content widget
	if s.content != nil {
		metrics := core.DefaultCellMetrics()
		scrollOffsetX := core.Unit(s.scrollX) * metrics.CellWidth
		scrollOffsetY := core.Unit(s.scrollY) * metrics.CellHeight

		return s.content.HandleMouseMove(core.MouseMoveEvent{
			X: event.X + scrollOffsetX,
			Y: event.Y + scrollOffsetY,
		})
	}

	return false
}

// HandleMouseRelease handles mouse release events.
func (s *ScrollArea) HandleMouseRelease(event core.MouseReleaseEvent) bool {
	viewport := s.viewportBounds()

	// Forward to scrollbars
	if s.vScrollBar.dragging {
		return s.vScrollBar.HandleMouseRelease(core.MouseReleaseEvent{
			X:      event.X - viewport.Width,
			Y:      event.Y,
			Button: event.Button,
		})
	}

	if s.hScrollBar.dragging {
		return s.hScrollBar.HandleMouseRelease(core.MouseReleaseEvent{
			X:      event.X,
			Y:      event.Y - viewport.Height,
			Button: event.Button,
		})
	}

	// Forward to content widget
	if s.content != nil {
		metrics := core.DefaultCellMetrics()
		scrollOffsetX := core.Unit(s.scrollX) * metrics.CellWidth
		scrollOffsetY := core.Unit(s.scrollY) * metrics.CellHeight

		return s.content.HandleMouseRelease(core.MouseReleaseEvent{
			X:      event.X + scrollOffsetX,
			Y:      event.Y + scrollOffsetY,
			Button: event.Button,
		})
	}

	return false
}

// HandleFocusIn is called when focus is gained.
func (s *ScrollArea) HandleFocusIn() {
	s.Update()
}

// HandleFocusOut is called when focus is lost.
func (s *ScrollArea) HandleFocusOut() {
	// Clear any active scrollbar drag states when focus is lost
	s.vScrollBar.dragging = false
	s.hScrollBar.dragging = false
	s.Update()
}

// AccessibleInfo returns accessibility information.
func (s *ScrollArea) AccessibleInfo() core.AccessibleInfo {
	info := s.AccessibleWidget.AccessibleInfo()
	info.Role = core.RoleGroup
	return info
}
