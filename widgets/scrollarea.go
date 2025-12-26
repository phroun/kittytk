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

	// Calculate thumb
	if s.maximum > s.minimum {
		trackCells := metrics.CharsForWidth(bounds.Width)
		thumbSize := trackCells * s.pageStep / (s.maximum - s.minimum + s.pageStep)
		if thumbSize < 1 {
			thumbSize = 1
		}
		if thumbSize > trackCells {
			thumbSize = trackCells
		}

		thumbPos := (s.value - s.minimum) * (trackCells - thumbSize) / (s.maximum - s.minimum)
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

	// Calculate thumb
	if s.maximum > s.minimum {
		trackCells := int(bounds.Height / metrics.CellHeight)
		thumbSize := trackCells * s.pageStep / (s.maximum - s.minimum + s.pageStep)
		if thumbSize < 1 {
			thumbSize = 1
		}
		if thumbSize > trackCells {
			thumbSize = trackCells
		}

		thumbPos := (s.value - s.minimum) * (trackCells - thumbSize) / (s.maximum - s.minimum)
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
		thumbSize := trackCells * s.pageStep / (s.maximum - s.minimum + s.pageStep)
		if thumbSize < 1 {
			thumbSize = 1
		}
		thumbPos := (s.value - s.minimum) * (trackCells - thumbSize) / (s.maximum - s.minimum)

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
		thumbSize := trackCells * s.pageStep / (s.maximum - s.minimum + s.pageStep)
		if thumbSize < 1 {
			thumbSize = 1
		}
		thumbPos := (s.value - s.minimum) * (trackCells - thumbSize) / (s.maximum - s.minimum)

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
	viewport := s.viewportBounds()
	metrics := core.DefaultCellMetrics()

	// Calculate cell positions
	cellX := metrics.UnitsToCellX(x)
	cellY := int(y / metrics.CellHeight)

	viewCellWidth := metrics.CharsForWidth(viewport.Width)
	viewCellHeight := int(viewport.Height / metrics.CellHeight)

	// Adjust scroll if needed
	if cellX < s.scrollX {
		s.SetScrollX(cellX)
	} else if cellX >= s.scrollX+viewCellWidth {
		s.SetScrollX(cellX - viewCellWidth + 1)
	}

	if cellY < s.scrollY {
		s.SetScrollY(cellY)
	} else if cellY >= s.scrollY+viewCellHeight {
		s.SetScrollY(cellY - viewCellHeight + 1)
	}
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

	if s.needsVScrollBar() {
		width -= metrics.CellWidth
	}
	if s.needsHScrollBar() {
		height -= metrics.CellHeight
	}

	return core.UnitRect{Width: width, Height: height}
}

func (s *ScrollArea) needsHScrollBar() bool {
	if s.hScrollBarPolicy == ScrollBarAlwaysOff {
		return false
	}
	if s.hScrollBarPolicy == ScrollBarAlwaysOn {
		return true
	}
	return s.contentWidth > s.viewportBounds().Width
}

func (s *ScrollArea) needsVScrollBar() bool {
	if s.vScrollBarPolicy == ScrollBarAlwaysOff {
		return false
	}
	if s.vScrollBarPolicy == ScrollBarAlwaysOn {
		return true
	}
	return s.contentHeight > s.viewportBounds().Height
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

	// Update horizontal scrollbar
	viewCellWidth := metrics.CharsForWidth(viewport.Width)
	contentCellWidth := metrics.CharsForWidth(s.contentWidth)
	s.hScrollBar.SetRange(0, contentCellWidth-viewCellWidth)
	s.hScrollBar.SetPageStep(viewCellWidth)

	// Update vertical scrollbar
	viewCellHeight := int(viewport.Height / metrics.CellHeight)
	contentCellHeight := int(s.contentHeight / metrics.CellHeight)
	s.vScrollBar.SetRange(0, contentCellHeight-viewCellHeight)
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

	// Draw background
	p.FillRect(core.UnitRect{Width: bounds.Width, Height: bounds.Height}, ' ', theme.Normal)

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

	// Draw vertical scrollbar
	if s.needsVScrollBar() {
		s.vScrollBar.SetBounds(core.UnitRect{
			X:      viewport.Width,
			Y:      0,
			Width:  metrics.CellWidth,
			Height: viewport.Height,
		})
		s.vScrollBar.Paint(p)
	}

	// Draw horizontal scrollbar
	if s.needsHScrollBar() {
		s.hScrollBar.SetBounds(core.UnitRect{
			X:      0,
			Y:      viewport.Height,
			Width:  viewport.Width,
			Height: metrics.CellHeight,
		})
		s.hScrollBar.Paint(p)
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

// HandleMousePress handles mouse clicks.
func (s *ScrollArea) HandleMousePress(event core.MousePressEvent) bool {
	viewport := s.viewportBounds()

	// Check scrollbars
	if s.needsVScrollBar() && event.X >= viewport.Width {
		return s.vScrollBar.HandleMousePress(core.MousePressEvent{
			X:      event.X - viewport.Width,
			Y:      event.Y,
			Button: event.Button,
		})
	}

	if s.needsHScrollBar() && event.Y >= viewport.Height {
		return s.hScrollBar.HandleMousePress(core.MousePressEvent{
			X:      event.X,
			Y:      event.Y - viewport.Height,
			Button: event.Button,
		})
	}

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

// HandleFocusIn is called when focus is gained.
func (s *ScrollArea) HandleFocusIn() {
	s.Update()
}

// HandleFocusOut is called when focus is lost.
func (s *ScrollArea) HandleFocusOut() {
	s.Update()
}

// AccessibleInfo returns accessibility information.
func (s *ScrollArea) AccessibleInfo() core.AccessibleInfo {
	info := s.AccessibleWidget.AccessibleInfo()
	info.Role = core.RoleGroup
	return info
}
