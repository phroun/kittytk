// Package widgets provides standard UI widgets for the TUI toolkit.
package widgets

import (
	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/style"
)

// Panel is a container widget that can hold other widgets with a layout.
type Panel struct {
	core.WidgetBase
	core.AccessibleWidget

	children      []core.Widget
	layoutManager core.LayoutManager

	// Appearance
	background style.CellStyle
	border     bool
	borderStyle style.BorderStyle
}

// NewPanel creates a new panel.
func NewPanel() *Panel {
	p := &Panel{
		background: style.DefaultStyle(),
	}
	p.WidgetBase = *core.NewWidgetBase()
	p.SetFocusPolicy(core.NoFocus)
	p.SetAccessibleRole(core.RoleGroup)
	return p
}

// Children returns all child widgets.
func (p *Panel) Children() []core.Widget {
	return p.children
}

// AddChild adds a child widget.
func (p *Panel) AddChild(child core.Widget) {
	if child == nil {
		return
	}
	child.SetParent(p)
	p.children = append(p.children, child)
	if p.layoutManager != nil {
		if adder, ok := p.layoutManager.(interface{ AddWidget(core.Widget) }); ok {
			adder.AddWidget(child)
		}
	}
	p.Update()
}

// RemoveChild removes a child widget.
func (p *Panel) RemoveChild(child core.Widget) {
	for i, c := range p.children {
		if c == child {
			p.children = append(p.children[:i], p.children[i+1:]...)
			child.SetParent(nil)
			if p.layoutManager != nil {
				if remover, ok := p.layoutManager.(interface{ RemoveWidget(core.Widget) }); ok {
					remover.RemoveWidget(child)
				}
			}
			break
		}
	}
	p.Update()
}

// ChildAt returns the child at the given position.
func (p *Panel) ChildAt(pos core.UnitPoint) core.Widget {
	for _, child := range p.children {
		if !child.IsVisible() {
			continue
		}
		bounds := child.Bounds()
		if pos.X >= bounds.X && pos.X < bounds.X+bounds.Width &&
			pos.Y >= bounds.Y && pos.Y < bounds.Y+bounds.Height {
			return child
		}
	}
	return nil
}

// Layout arranges children within this container.
func (p *Panel) Layout() {
	if p.layoutManager != nil {
		bounds := p.Bounds()
		contentBounds := bounds
		if p.border {
			metrics := core.DefaultCellMetrics()
			contentBounds = core.UnitRect{
				X:      metrics.CellWidth,
				Y:      metrics.CellHeight,
				Width:  bounds.Width - 2*metrics.CellWidth,
				Height: bounds.Height - 2*metrics.CellHeight,
			}
		}
		p.layoutManager.Layout(p, contentBounds)
	}
}

// LayoutManager returns the layout manager.
func (p *Panel) LayoutManager() core.LayoutManager {
	return p.layoutManager
}

// SetLayoutManager sets the layout manager.
func (p *Panel) SetLayoutManager(layout core.LayoutManager) {
	p.layoutManager = layout
	// Add existing children to the new layout
	if adder, ok := layout.(interface{ AddWidget(core.Widget) }); ok {
		for _, child := range p.children {
			adder.AddWidget(child)
		}
	}
	p.Layout()
	p.Update()
}

// SetBorder enables or disables the border.
func (p *Panel) SetBorder(enabled bool) {
	p.border = enabled
	p.Update()
}

// SetBorderStyle sets the border style.
func (p *Panel) SetBorderStyle(s style.BorderStyle) {
	p.borderStyle = s
	p.Update()
}

// SetBackground sets the background style.
func (p *Panel) SetBackground(s style.CellStyle) {
	p.background = s
	p.Update()
}

// SizeHint returns the preferred size.
func (p *Panel) SizeHint() core.UnitSize {
	if p.layoutManager != nil {
		return p.layoutManager.SizeHint(p)
	}
	metrics := core.DefaultCellMetrics()
	return core.UnitSize{
		Width:  metrics.TextWidth(20),
		Height: metrics.TextHeight(10),
	}
}

// MinimumSize returns the minimum size.
func (p *Panel) MinimumSize() core.UnitSize {
	if p.layoutManager != nil {
		return p.layoutManager.MinimumSize(p)
	}
	return core.UnitSize{Width: 16, Height: 16}
}

// Paint renders the panel.
func (p *Panel) Paint(painter *core.Painter) {
	bounds := p.Bounds()

	// Draw background
	painter.FillRect(core.UnitRect{Width: bounds.Width, Height: bounds.Height}, ' ', p.background)

	// Draw border if enabled
	if p.border {
		painter.DrawRect(core.UnitRect{Width: bounds.Width, Height: bounds.Height}, p.borderStyle, p.background)
	}

	// Paint children
	for _, child := range p.children {
		if child.IsVisible() {
			childBounds := child.Bounds()
			childPainter := painter.WithOffset(childBounds.X, childBounds.Y)
			child.Paint(childPainter)
		}
	}
}

// HandleKeyPress handles keyboard input.
func (p *Panel) HandleKeyPress(event core.KeyPressEvent) bool {
	// Panels don't handle keys directly
	return false
}

// HandleMousePress handles mouse clicks.
func (p *Panel) HandleMousePress(event core.MousePressEvent) bool {
	// Find child under mouse and forward event
	child := p.ChildAt(core.UnitPoint{X: event.X, Y: event.Y})
	if child != nil {
		if handler, ok := child.(interface{ HandleMousePress(core.MousePressEvent) bool }); ok {
			childBounds := child.Bounds()
			childEvent := event
			childEvent.X -= childBounds.X
			childEvent.Y -= childBounds.Y
			return handler.HandleMousePress(childEvent)
		}
	}
	return false
}

// HandleMouseMove handles mouse movement.
func (p *Panel) HandleMouseMove(event core.MouseMoveEvent) bool {
	// Forward to all children (needed for drag operations)
	for _, child := range p.children {
		if handler, ok := child.(interface {
			HandleMouseMove(core.MouseMoveEvent) bool
		}); ok {
			childBounds := child.Bounds()
			childEvent := event
			childEvent.X -= childBounds.X
			childEvent.Y -= childBounds.Y
			if handler.HandleMouseMove(childEvent) {
				return true
			}
		}
	}
	return false
}

// HandleMouseRelease handles mouse button release.
func (p *Panel) HandleMouseRelease(event core.MouseReleaseEvent) bool {
	// Forward to all children (needed for drag operations)
	for _, child := range p.children {
		if handler, ok := child.(interface {
			HandleMouseRelease(core.MouseReleaseEvent) bool
		}); ok {
			childBounds := child.Bounds()
			childEvent := event
			childEvent.X -= childBounds.X
			childEvent.Y -= childBounds.Y
			if handler.HandleMouseRelease(childEvent) {
				return true
			}
		}
	}
	return false
}

// SetBounds sets the panel bounds and triggers layout.
func (p *Panel) SetBounds(bounds core.UnitRect) {
	oldSize := p.Bounds().Size()
	p.WidgetBase.SetBounds(bounds)
	newSize := bounds.Size()
	if oldSize != newSize {
		p.HandleResize(oldSize, newSize)
	}
}

// HandleResize is called when the panel is resized.
func (p *Panel) HandleResize(oldSize, newSize core.UnitSize) {
	p.Layout()
}

// AccessibleInfo returns accessibility information.
func (p *Panel) AccessibleInfo() core.AccessibleInfo {
	info := p.AccessibleWidget.AccessibleInfo()
	info.Role = core.RoleGroup
	return info
}
