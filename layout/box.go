// Package layout provides layout managers for arranging widgets.
package layout

import (
	"github.com/phroun/tuitk/core"
)

// BoxLayout arranges widgets in a single row or column.
// This is similar to Qt's QBoxLayout, QHBoxLayout, and QVBoxLayout.
type BoxLayout struct {
	BaseLayout
	orientation core.Orientation
	items       []*LayoutItem
}

// NewBoxLayout creates a new box layout with the given orientation.
func NewBoxLayout(orientation core.Orientation) *BoxLayout {
	return &BoxLayout{
		orientation: orientation,
	}
}

// NewHBoxLayout creates a horizontal box layout.
func NewHBoxLayout() *BoxLayout {
	return NewBoxLayout(core.Horizontal)
}

// NewVBoxLayout creates a vertical box layout.
func NewVBoxLayout() *BoxLayout {
	return NewBoxLayout(core.Vertical)
}

// Orientation returns the layout orientation.
func (l *BoxLayout) Orientation() core.Orientation {
	return l.orientation
}

// SetOrientation sets the layout orientation.
func (l *BoxLayout) SetOrientation(o core.Orientation) {
	l.orientation = o
}

// AddWidget adds a widget to the layout.
func (l *BoxLayout) AddWidget(widget core.Widget) {
	l.items = append(l.items, NewLayoutItem(widget))
}

// AddWidgetWithStretch adds a widget with a stretch factor.
func (l *BoxLayout) AddWidgetWithStretch(widget core.Widget, stretch int) {
	item := NewLayoutItem(widget).WithStretch(stretch)
	l.items = append(l.items, item)
}

// AddStretch adds a stretching spacer.
func (l *BoxLayout) AddStretch(stretch int) {
	spacer := NewStretchSpacer()
	item := NewLayoutItem(spacer).WithStretch(stretch)
	l.items = append(l.items, item)
}

// AddSpacing adds fixed spacing.
func (l *BoxLayout) AddSpacing(size core.Unit) {
	var spacer *Spacer
	if l.orientation == core.Horizontal {
		spacer = NewSpacer(size, 0)
	} else {
		spacer = NewSpacer(0, size)
	}
	l.items = append(l.items, NewLayoutItem(spacer))
}

// InsertWidget inserts a widget at the given index.
func (l *BoxLayout) InsertWidget(index int, widget core.Widget) {
	item := NewLayoutItem(widget)
	if index < 0 {
		index = 0
	}
	if index >= len(l.items) {
		l.items = append(l.items, item)
		return
	}
	l.items = append(l.items[:index], append([]*LayoutItem{item}, l.items[index:]...)...)
}

// RemoveWidget removes a widget from the layout.
func (l *BoxLayout) RemoveWidget(widget core.Widget) {
	for i, item := range l.items {
		if item.Widget == widget {
			l.items = append(l.items[:i], l.items[i+1:]...)
			return
		}
	}
}

// Count returns the number of items.
func (l *BoxLayout) Count() int {
	return len(l.items)
}

// ItemAt returns the item at the given index.
func (l *BoxLayout) ItemAt(index int) *LayoutItem {
	if index < 0 || index >= len(l.items) {
		return nil
	}
	return l.items[index]
}

// Layout arranges children within the given bounds.
func (l *BoxLayout) Layout(container core.Container, bounds core.UnitRect) {
	if len(l.items) == 0 {
		return
	}

	// Apply margins
	rect := l.effectiveBounds(bounds)

	// Round spacing to whole cell size based on orientation
	metrics := core.DefaultCellMetrics()
	var spacing core.Unit
	if l.orientation == core.Horizontal {
		// Round to CellWidth
		spacing = core.Unit(metrics.UnitsToCellX(l.spacing)) * metrics.CellWidth
	} else {
		// Round to CellHeight
		spacing = core.Unit(metrics.UnitsToCellY(l.spacing)) * metrics.CellHeight
	}

	// Collect size hints and stretch factors
	stretchItems := make([]stretchItem, len(l.items))
	totalSpacing := spacing * core.Unit(len(l.items)-1)

	var availablePrimary core.Unit
	if l.orientation == core.Horizontal {
		availablePrimary = rect.Width - totalSpacing
	} else {
		availablePrimary = rect.Height - totalSpacing
	}

	for i, item := range l.items {
		hint := item.Widget.SizeHint()
		policy := item.Widget.SizePolicy()

		var minSize, stretch int

		if l.orientation == core.Horizontal {
			minSize = int(hint.Width)
			if policy.Horizontal == core.SizeExpanding || item.Stretch > 0 {
				stretch = item.Stretch
				if stretch == 0 {
					stretch = 1
				}
			}
		} else {
			minSize = int(hint.Height)
			if policy.Vertical == core.SizeExpanding || item.Stretch > 0 {
				stretch = item.Stretch
				if stretch == 0 {
					stretch = 1
				}
			}
		}

		stretchItems[i] = stretchItem{
			minimum: core.Unit(minSize),
			stretch: stretch,
		}
	}

	// Calculate sizes
	sizes := calculateStretch(availablePrimary, stretchItems)

	// Position widgets
	var pos core.Unit
	if l.orientation == core.Horizontal {
		pos = rect.X
	} else {
		pos = rect.Y
	}

	for i, item := range l.items {
		var itemBounds core.UnitRect

		if l.orientation == core.Horizontal {
			itemBounds = core.UnitRect{
				X:      pos,
				Y:      rect.Y,
				Width:  sizes[i],
				Height: rect.Height,
			}
			pos += sizes[i] + spacing
		} else {
			itemBounds = core.UnitRect{
				X:      rect.X,
				Y:      pos,
				Width:  rect.Width,
				Height: sizes[i],
			}
			pos += sizes[i] + spacing
		}

		// Apply alignment within the item bounds
		itemBounds = l.alignItem(item, itemBounds)
		item.Widget.SetBounds(itemBounds)
	}
}

// alignItem adjusts item bounds based on alignment.
func (l *BoxLayout) alignItem(item *LayoutItem, bounds core.UnitRect) core.UnitRect {
	hint := item.Widget.SizeHint()

	if l.orientation == core.Horizontal {
		// Vertical alignment in horizontal layout
		switch item.Align {
		case core.AlignTop:
			bounds.Height = hint.Height
		case core.AlignMiddle:
			if hint.Height < bounds.Height {
				bounds.Y += (bounds.Height - hint.Height) / 2
				bounds.Height = hint.Height
			}
		case core.AlignBottom:
			if hint.Height < bounds.Height {
				bounds.Y += bounds.Height - hint.Height
				bounds.Height = hint.Height
			}
		}
	} else {
		// Horizontal alignment in vertical layout
		switch item.Align {
		case core.AlignLeft:
			bounds.Width = hint.Width
		case core.AlignCenter:
			if hint.Width < bounds.Width {
				bounds.X += (bounds.Width - hint.Width) / 2
				bounds.Width = hint.Width
			}
		case core.AlignRight:
			if hint.Width < bounds.Width {
				bounds.X += bounds.Width - hint.Width
				bounds.Width = hint.Width
			}
		}
	}

	return bounds
}

// SizeHint returns the preferred size for the container.
func (l *BoxLayout) SizeHint(container core.Container) core.UnitSize {
	var width, height core.Unit

	for _, item := range l.items {
		hint := item.Widget.SizeHint()

		if l.orientation == core.Horizontal {
			width += hint.Width
			if hint.Height > height {
				height = hint.Height
			}
		} else {
			height += hint.Height
			if hint.Width > width {
				width = hint.Width
			}
		}
	}

	// Add spacing
	if len(l.items) > 1 {
		spacing := l.spacing * core.Unit(len(l.items)-1)
		if l.orientation == core.Horizontal {
			width += spacing
		} else {
			height += spacing
		}
	}

	// Add margins
	width += l.margins.Horizontal()
	height += l.margins.Vertical()

	return core.UnitSize{Width: width, Height: height}
}

// MinimumSize returns the minimum size for the container.
func (l *BoxLayout) MinimumSize(container core.Container) core.UnitSize {
	var width, height core.Unit

	for _, item := range l.items {
		minSize := item.Widget.MinimumSize()

		if l.orientation == core.Horizontal {
			width += minSize.Width
			if minSize.Height > height {
				height = minSize.Height
			}
		} else {
			height += minSize.Height
			if minSize.Width > width {
				width = minSize.Width
			}
		}
	}

	// Add spacing
	if len(l.items) > 1 {
		spacing := l.spacing * core.Unit(len(l.items)-1)
		if l.orientation == core.Horizontal {
			width += spacing
		} else {
			height += spacing
		}
	}

	// Add margins
	width += l.margins.Horizontal()
	height += l.margins.Vertical()

	return core.UnitSize{Width: width, Height: height}
}
