// Package widgets provides standard UI widgets for the TUI toolkit.
package widgets

import (
	"github.com/phroun/tuitk/core"
)

// Splitter is a container widget that divides space between two children
// with a draggable divider.
// For vertical splitters (horizontal divider): ────·· Title ··────
// For horizontal splitters (vertical divider): │ with : handle
type Splitter struct {
	core.WidgetBase
	core.AccessibleWidget

	// Child widgets
	first  core.Widget
	second core.Widget

	// Orientation (Horizontal or Vertical)
	orientation core.Orientation

	// Split position (0.0-1.0 ratio, or absolute if > 1)
	position float64

	// Divider dragging state
	dragging bool

	// Optional title displayed in the divider
	title string

	// Background - only fills if explicitly set
	backgroundSet bool
}

// NewSplitter creates a new splitter with the given orientation.
func NewSplitter(orientation core.Orientation) *Splitter {
	s := &Splitter{
		orientation: orientation,
		position:    0.5, // Default to 50/50 split
	}
	s.WidgetBase = *core.NewWidgetBase()
	s.Init(s)
	s.SetFocusPolicy(core.StrongFocus) // Focusable for keyboard navigation
	s.SetFurtive(true)                 // Furtive: no focus on click, skip for initial focus
	s.SetAccessibleRole(core.RoleSplitter)
	return s
}

// NewHSplitter creates a horizontal splitter (children side by side).
func NewHSplitter() *Splitter {
	return NewSplitter(core.Horizontal)
}

// NewVSplitter creates a vertical splitter (children stacked).
func NewVSplitter() *Splitter {
	return NewSplitter(core.Vertical)
}

// SetFirst sets the first child widget (left/top).
func (s *Splitter) SetFirst(w core.Widget) {
	if s.first != nil {
		s.first.SetParent(nil)
	}
	s.first = w
	if w != nil {
		w.SetParent(s)
	}
	s.Update()
}

// First returns the first child widget.
func (s *Splitter) First() core.Widget {
	return s.first
}

// SetSecond sets the second child widget (right/bottom).
func (s *Splitter) SetSecond(w core.Widget) {
	if s.second != nil {
		s.second.SetParent(nil)
	}
	s.second = w
	if w != nil {
		w.SetParent(s)
	}
	s.Update()
}

// Second returns the second child widget.
func (s *Splitter) Second() core.Widget {
	return s.second
}

// SetPosition sets the split position (0.0-1.0 as ratio).
func (s *Splitter) SetPosition(pos float64) {
	if pos < 0 {
		pos = 0
	} else if pos > 1 {
		pos = 1
	}
	s.position = pos
	s.Update()
}

// Position returns the split position.
func (s *Splitter) Position() float64 {
	return s.position
}

// Orientation returns the splitter orientation.
func (s *Splitter) Orientation() core.Orientation {
	return s.orientation
}

// SetOrientation sets the splitter orientation.
func (s *Splitter) SetOrientation(o core.Orientation) {
	s.orientation = o
	s.Update()
}

// Title returns the splitter divider title.
func (s *Splitter) Title() string {
	return s.title
}

// SetTitle sets the splitter divider title.
func (s *Splitter) SetTitle(title string) {
	s.title = title
	s.Update()
}

// Children returns all child widgets.
func (s *Splitter) Children() []core.Widget {
	var children []core.Widget
	if s.first != nil {
		children = append(children, s.first)
	}
	if s.second != nil {
		children = append(children, s.second)
	}
	return children
}

// AddChild adds a child widget.
func (s *Splitter) AddChild(child core.Widget) {
	if s.first == nil {
		s.SetFirst(child)
	} else if s.second == nil {
		s.SetSecond(child)
	}
}

// RemoveChild removes a child widget.
func (s *Splitter) RemoveChild(child core.Widget) {
	if s.first == child {
		s.first = nil
	} else if s.second == child {
		s.second = nil
	}
}

// ChildAt returns the child at the given position.
func (s *Splitter) ChildAt(pos core.UnitPoint) core.Widget {
	dividerRect := s.dividerBounds()
	if dividerRect.Contains(pos) {
		return nil // On divider
	}

	firstBounds, secondBounds := s.childBounds()
	if firstBounds.Contains(pos) && s.first != nil {
		return s.first
	}
	if secondBounds.Contains(pos) && s.second != nil {
		return s.second
	}
	return nil
}

// Layout arranges children.
func (s *Splitter) Layout() {
	firstBounds, secondBounds := s.childBounds()
	if s.first != nil {
		s.first.SetBounds(firstBounds)
		// Force content to re-layout with fresh SizeHints
		if container, ok := s.first.(core.Container); ok {
			container.Layout()
		}
	}
	if s.second != nil {
		s.second.SetBounds(secondBounds)
		// Force content to re-layout with fresh SizeHints
		if container, ok := s.second.(core.Container); ok {
			container.Layout()
		}
	}
}

// LayoutManager returns nil (Splitter manages its own layout).
func (s *Splitter) LayoutManager() core.LayoutManager {
	return nil
}

// SetLayoutManager is a no-op (Splitter manages its own layout).
func (s *Splitter) SetLayoutManager(layout core.LayoutManager) {
	// Splitter manages its own layout
}

// dividerBounds returns the bounds of the divider bar.
func (s *Splitter) dividerBounds() core.UnitRect {
	bounds := s.Bounds()
	metrics := s.EffectiveCellMetrics()

	// Cell surfaces snap the divider to whole rows/columns; smooth
	// (pixel) surfaces track the split ratio at unit granularity -
	// the same adjustment window drag/resize received.
	smooth := core.FindSmoothPositioning(s.Self())

	if s.orientation == core.Horizontal {
		// Horizontal splitter has a vertical divider bar (use cell width)
		dividerSize := metrics.CellWidth
		totalWidth := bounds.Width - dividerSize
		firstWidth := core.Unit(float64(totalWidth) * s.position)
		if !smooth {
			// Round to cell boundary
			firstWidth = core.Unit(metrics.UnitsToCellX(firstWidth)) * metrics.CellWidth
		}

		return core.UnitRect{
			X:      firstWidth,
			Y:      0,
			Width:  dividerSize,
			Height: bounds.Height,
		}
	}

	// Vertical splitter has a horizontal divider bar (use cell height)
	dividerSize := metrics.CellHeight
	totalHeight := bounds.Height - dividerSize
	firstHeight := core.Unit(float64(totalHeight) * s.position)
	if !smooth {
		// Round to cell boundary
		firstHeight = core.Unit(metrics.UnitsToCellY(firstHeight)) * metrics.CellHeight
	}

	return core.UnitRect{
		X:      0,
		Y:      firstHeight,
		Width:  bounds.Width,
		Height: dividerSize,
	}
}

// childBounds returns the bounds for both children.
func (s *Splitter) childBounds() (core.UnitRect, core.UnitRect) {
	bounds := s.Bounds()
	divider := s.dividerBounds()

	if s.orientation == core.Horizontal {
		return core.UnitRect{
				X:      0,
				Y:      0,
				Width:  divider.X,
				Height: bounds.Height,
			}, core.UnitRect{
				X:      divider.X + divider.Width,
				Y:      0,
				Width:  bounds.Width - divider.X - divider.Width,
				Height: bounds.Height,
			}
	}

	// Vertical
	return core.UnitRect{
			X:      0,
			Y:      0,
			Width:  bounds.Width,
			Height: divider.Y,
		}, core.UnitRect{
			X:      0,
			Y:      divider.Y + divider.Height,
			Width:  bounds.Width,
			Height: bounds.Height - divider.Y - divider.Height,
		}
}

// SizeHint returns a modest fixed preference; splitters are meant to
// be stretched by their layout. (Returning the current bounds made the
// hint a ratchet: layouts could grow the splitter but never shrink it
// back, since stretch distribution treats the hint as a floor.)
func (s *Splitter) SizeHint() core.UnitSize {
	metrics := s.EffectiveCellMetrics()
	return core.UnitSize{
		Width:  metrics.CellWidth * 20,
		Height: metrics.CellHeight * 5,
	}
}

// Paint renders the splitter.
func (sp *Splitter) Paint(p *core.Painter) {
	bounds := sp.Bounds()
	scheme := sp.GetScheme()
	metrics := sp.EffectiveCellMetrics()

	// Only draw background if explicitly set (allows parent backgrounds to show through)
	if sp.backgroundSet {
		p.FillRect(core.UnitRect{Width: bounds.Width, Height: bounds.Height}, ' ', scheme.GetNormal(true))
	}

	// Update child bounds
	sp.Layout()

	// Draw first child
	if sp.first != nil {
		firstBounds, _ := sp.childBounds()
		firstPainter := p.WithOffset(firstBounds.X, firstBounds.Y).
			WithClip(core.UnitRect{Width: firstBounds.Width, Height: firstBounds.Height})
		sp.first.Paint(firstPainter)
	}

	// Draw divider with middot drag handle styling
	divider := sp.dividerBounds()
	focused := sp.HasFocus()
	dividerStyle := scheme.GetSplitter()
	if sp.dragging {
		dividerStyle = scheme.GetPressedSplitter()
	} else if focused {
		dividerStyle = scheme.GetFocusedSplitter()
	}

	if sp.orientation == core.Horizontal {
		// Vertical divider bar with ':' handle
		midY := bounds.Height / 2
		// Round to cell boundary
		midY = (midY / metrics.CellHeight) * metrics.CellHeight
		for y := core.Unit(0); y < bounds.Height; y += metrics.CellHeight {
			ch := '│'
			// Draw drag handle indicator in the middle
			if y == midY {
				ch = ':'
			}
			p.DrawCell(divider.X, y, ch, dividerStyle)
		}
	} else {
		// Horizontal divider bar: ────·· Title ··────
		width := int(bounds.Width / metrics.CellWidth)
		titleRunes := []rune(sp.title)
		titleLen := len(titleRunes)

		if titleLen == 0 {
			// No title: draw line with 4 middots centered
			center := width / 2
			for xi := 0; xi < width; xi++ {
				x := metrics.CellToUnitsX(xi)
				ch := '─'
				// Draw ·· ·· (4 dots) at center
				if xi == center-1 || xi == center || xi == center+1 || xi == center+2 {
					ch = '·'
				}
				p.DrawCell(x, divider.Y, ch, dividerStyle)
			}
		} else {
			// With title: ────·· Title ··────
			middleContent := "·· " + sp.title + " ··"
			middleRunes := []rune(middleContent)
			middleLen := len(middleRunes)
			startMiddle := (width - middleLen) / 2

			for xi := 0; xi < width; xi++ {
				x := metrics.CellToUnitsX(xi)
				var ch rune
				if xi < startMiddle {
					ch = '─'
				} else if xi < startMiddle+middleLen {
					ch = middleRunes[xi-startMiddle]
				} else {
					ch = '─'
				}
				p.DrawCell(x, divider.Y, ch, dividerStyle)
			}
		}
	}

	// Draw second child
	if sp.second != nil {
		_, secondBounds := sp.childBounds()
		secondPainter := p.WithOffset(secondBounds.X, secondBounds.Y).
			WithClip(core.UnitRect{Width: secondBounds.Width, Height: secondBounds.Height})
		sp.second.Paint(secondPainter)
	}
}

// HandleMousePress handles mouse button presses.
func (s *Splitter) HandleMousePress(event core.MousePressEvent) bool {
	if event.Button != core.LeftButton {
		return false
	}

	// Check if click is on divider
	divider := s.dividerBounds()
	if s.orientation == core.Horizontal {
		// Hit area is just the divider itself (no extension into child areas)
		if divider.Contains(core.UnitPoint{X: event.X, Y: event.Y}) {
			s.dragging = true
			s.Update()
			return true
		}
	} else {
		// Hit area is just the divider itself
		if divider.Contains(core.UnitPoint{X: event.X, Y: event.Y}) {
			s.dragging = true
			s.Update()
			return true
		}
	}

	// Forward to children
	firstBounds, secondBounds := s.childBounds()
	pos := core.UnitPoint{X: event.X, Y: event.Y}

	if firstBounds.Contains(pos) && s.first != nil {
		// Cancel any drag on the other child since a new press is happening elsewhere
		if s.second != nil {
			if handler, ok := s.second.(interface {
				HandleMouseRelease(core.MouseReleaseEvent) bool
			}); ok {
				handler.HandleMouseRelease(core.MouseReleaseEvent{Button: event.Button})
			}
		}
		localEvent := event
		localEvent.X -= firstBounds.X
		localEvent.Y -= firstBounds.Y
		return s.first.HandleMousePress(localEvent)
	}

	if secondBounds.Contains(pos) && s.second != nil {
		// Cancel any drag on the other child since a new press is happening elsewhere
		if s.first != nil {
			if handler, ok := s.first.(interface {
				HandleMouseRelease(core.MouseReleaseEvent) bool
			}); ok {
				handler.HandleMouseRelease(core.MouseReleaseEvent{Button: event.Button})
			}
		}
		localEvent := event
		localEvent.X -= secondBounds.X
		localEvent.Y -= secondBounds.Y
		return s.second.HandleMousePress(localEvent)
	}

	// Click is not on either child (maybe on divider that wasn't handled, or outside)
	// Cancel drags on both children
	if s.first != nil {
		if handler, ok := s.first.(interface {
			HandleMouseRelease(core.MouseReleaseEvent) bool
		}); ok {
			handler.HandleMouseRelease(core.MouseReleaseEvent{Button: event.Button})
		}
	}
	if s.second != nil {
		if handler, ok := s.second.(interface {
			HandleMouseRelease(core.MouseReleaseEvent) bool
		}); ok {
			handler.HandleMouseRelease(core.MouseReleaseEvent{Button: event.Button})
		}
	}

	return false
}

// HandleMouseMove handles mouse movement for dragging.
func (s *Splitter) HandleMouseMove(event core.MouseMoveEvent) bool {
	if s.dragging {
		bounds := s.Bounds()
		metrics := s.EffectiveCellMetrics()

		if s.orientation == core.Horizontal {
			dividerSize := metrics.CellWidth
			totalWidth := bounds.Width - dividerSize
			if totalWidth > 0 {
				newPos := float64(event.X) / float64(totalWidth)
				if newPos < 0.1 {
					newPos = 0.1
				} else if newPos > 0.9 {
					newPos = 0.9
				}
				s.position = newPos
				s.Update()
			}
		} else {
			dividerSize := metrics.CellHeight
			totalHeight := bounds.Height - dividerSize
			if totalHeight > 0 {
				newPos := float64(event.Y) / float64(totalHeight)
				if newPos < 0.1 {
					newPos = 0.1
				} else if newPos > 0.9 {
					newPos = 0.9
				}
				s.position = newPos
				s.Update()
			}
		}

		return true
	}

	// Forward to children (needed for drag operations within children)
	firstBounds, secondBounds := s.childBounds()

	if s.first != nil {
		if handler, ok := s.first.(interface {
			HandleMouseMove(core.MouseMoveEvent) bool
		}); ok {
			localEvent := event
			localEvent.X -= firstBounds.X
			localEvent.Y -= firstBounds.Y
			if handler.HandleMouseMove(localEvent) {
				return true
			}
		}
	}

	if s.second != nil {
		if handler, ok := s.second.(interface {
			HandleMouseMove(core.MouseMoveEvent) bool
		}); ok {
			localEvent := event
			localEvent.X -= secondBounds.X
			localEvent.Y -= secondBounds.Y
			if handler.HandleMouseMove(localEvent) {
				return true
			}
		}
	}

	return false
}

// HandleMouseRelease handles mouse button releases.
func (s *Splitter) HandleMouseRelease(event core.MouseReleaseEvent) bool {
	if s.dragging {
		s.dragging = false
		s.Update()
		return true
	}

	// Forward to children (needed for drag operations within children)
	firstBounds, secondBounds := s.childBounds()

	if s.first != nil {
		if handler, ok := s.first.(interface {
			HandleMouseRelease(core.MouseReleaseEvent) bool
		}); ok {
			localEvent := event
			localEvent.X -= firstBounds.X
			localEvent.Y -= firstBounds.Y
			if handler.HandleMouseRelease(localEvent) {
				return true
			}
		}
	}

	if s.second != nil {
		if handler, ok := s.second.(interface {
			HandleMouseRelease(core.MouseReleaseEvent) bool
		}); ok {
			localEvent := event
			localEvent.X -= secondBounds.X
			localEvent.Y -= secondBounds.Y
			if handler.HandleMouseRelease(localEvent) {
				return true
			}
		}
	}

	return false
}

// IsDragging returns whether the divider is being dragged.
func (s *Splitter) IsDragging() bool {
	return s.dragging
}

// HandleKeyPress handles keyboard input.
func (s *Splitter) HandleKeyPress(event core.KeyPressEvent) bool {
	// If the splitter itself has focus, handle arrow keys for divider adjustment
	if s.HasFocus() {
		bounds := s.Bounds()
		metrics := s.EffectiveCellMetrics()

		// Calculate step sizes
		// Normal: small step (1 cell equivalent in position terms)
		// Large step (10 cells horizontal, 4 cells vertical) for modified keys
		var smallStep, largeStep float64
		if s.orientation == core.Horizontal {
			totalWidth := float64(bounds.Width - metrics.CellWidth) // Subtract divider
			if totalWidth > 0 {
				smallStep = float64(metrics.CellWidth) / totalWidth
				largeStep = float64(metrics.CellWidth*10) / totalWidth
			} else {
				smallStep = 0.02
				largeStep = 0.1
			}
		} else {
			totalHeight := float64(bounds.Height - metrics.CellHeight)
			if totalHeight > 0 {
				smallStep = float64(metrics.CellHeight) / totalHeight
				largeStep = float64(metrics.CellHeight*4) / totalHeight
			} else {
				smallStep = 0.02
				largeStep = 0.1
			}
		}

		// Handle arrow keys - plain keys use small step, prefixed keys use large step
		switch event.Key {
		case "Left":
			if s.orientation == core.Horizontal {
				s.adjustPosition(-smallStep)
				return true
			}
		case "M-Left", "C-Left", "A-Left":
			if s.orientation == core.Horizontal {
				s.adjustPosition(-largeStep)
				return true
			}
		case "Right":
			if s.orientation == core.Horizontal {
				s.adjustPosition(smallStep)
				return true
			}
		case "M-Right", "C-Right", "A-Right":
			if s.orientation == core.Horizontal {
				s.adjustPosition(largeStep)
				return true
			}
		case "Up":
			if s.orientation == core.Vertical {
				s.adjustPosition(-smallStep)
				return true
			}
		case "M-Up", "C-Up", "A-Up":
			if s.orientation == core.Vertical {
				s.adjustPosition(-largeStep)
				return true
			}
		case "Down":
			if s.orientation == core.Vertical {
				s.adjustPosition(smallStep)
				return true
			}
		case "M-Down", "C-Down", "A-Down":
			if s.orientation == core.Vertical {
				s.adjustPosition(largeStep)
				return true
			}
		}
	}

	// Forward to focused child
	if s.first != nil && s.first.HasFocus() {
		return s.first.HandleKeyPress(event)
	}
	if s.second != nil && s.second.HasFocus() {
		return s.second.HandleKeyPress(event)
	}
	return false
}

// adjustPosition adjusts the split position by the given delta.
func (s *Splitter) adjustPosition(delta float64) {
	newPos := s.position + delta
	if newPos < 0.1 {
		newPos = 0.1
	} else if newPos > 0.9 {
		newPos = 0.9
	}
	s.position = newPos
	s.Update()
}

// HandleFocusIn is called when focus is gained.
func (s *Splitter) HandleFocusIn() {
	s.Update()
}

// HandleFocusOut is called when focus is lost.
func (s *Splitter) HandleFocusOut() {
	s.Update()
}

// CollectFocusChain implements core.FocusChainProvider to ensure the splitter
// appears between its first and second children in the focus order.
func (s *Splitter) CollectFocusChain(collector func(core.Widget)) {
	// First child and its descendants
	if s.first != nil {
		collector(s.first)
	}

	// Splitter itself (between children)
	collector(s)

	// Second child and its descendants
	if s.second != nil {
		collector(s.second)
	}
}

// AccessibleInfo returns accessibility information.
func (s *Splitter) AccessibleInfo() core.AccessibleInfo {
	info := s.AccessibleWidget.AccessibleInfo()
	info.Role = core.RoleSplitter
	info.Value = ""
	return info
}
