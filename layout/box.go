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

// isInlineWidget returns true if the widget is an inline (non-container) widget.
func isInlineWidget(w core.Widget) bool {
	// If it implements InlineWidget interface and returns true, it's inline
	if inline, ok := w.(core.InlineWidget); ok && inline.IsInlineWidget() {
		return true
	}
	// If it's a Container, it's not inline
	if _, ok := w.(core.Container); ok {
		return false
	}
	// Default: treat as inline if not a container
	return true
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

	// For horizontal layout, calculate additional spacing for inline widgets
	var inlineSpacingTotal core.Unit
	if l.orientation == core.Horizontal && len(l.items) > 0 {
		// Space before first inline widget
		if isInlineWidget(l.items[0].Widget) {
			inlineSpacingTotal += metrics.CellWidth
		}
		// Space between items where at least one is inline
		for i := 0; i < len(l.items)-1; i++ {
			if isInlineWidget(l.items[i].Widget) || isInlineWidget(l.items[i+1].Widget) {
				inlineSpacingTotal += metrics.CellWidth
			}
		}
		// Space after last inline widget
		if isInlineWidget(l.items[len(l.items)-1].Widget) {
			inlineSpacingTotal += metrics.CellWidth
		}
	}

	// Calculate sizes along the primary axis
	var sizes []core.Unit
	if l.orientation == core.Horizontal {
		sizes = l.horizontalItemWidths(rect.Width, metrics, spacing, inlineSpacingTotal)
	} else {
		totalSpacing := spacing * core.Unit(len(l.items)-1)
		stretchItems := make([]stretchItem, len(l.items))

		for i, item := range l.items {
			hint := item.Widget.SizeHint()
			policy := item.Widget.SizePolicy()

			minSize := hint.Height
			// Height-for-width widgets (e.g. wrapped text) report their
			// real height at the width they will actually receive.
			if h := itemHeightForWidth(item.Widget, l.verticalItemWidth(rect.Width, item, metrics)); h > 0 {
				minSize = h
			}

			stretch := 0
			if policy.Vertical == core.SizeExpanding || item.Stretch > 0 {
				stretch = item.Stretch
				if stretch == 0 {
					stretch = 1
				}
			}

			stretchItems[i] = stretchItem{
				minimum: minSize,
				stretch: stretch,
			}
		}

		sizes = calculateStretch(rect.Height-totalSpacing, stretchItems)
	}

	// Position widgets
	var pos core.Unit
	if l.orientation == core.Horizontal {
		pos = rect.X
		// Add margin before first inline widget
		if len(l.items) > 0 && isInlineWidget(l.items[0].Widget) {
			pos += metrics.CellWidth
		}
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
			pos += sizes[i]

			// Add spacing after this item (before the next one)
			// For inline widgets, use inline spacing; for containers, use base spacing
			if i < len(l.items)-1 {
				if isInlineWidget(item.Widget) || isInlineWidget(l.items[i+1].Widget) {
					pos += metrics.CellWidth // Inline spacing
				} else {
					pos += spacing // Container-to-container spacing
				}
			}
		} else {
			// In vertical layout, apply horizontal margin to inline widgets
			itemX := rect.X
			itemWidth := rect.Width

			if inlineWidget, ok := item.Widget.(core.InlineWidget); ok && inlineWidget.IsInlineWidget() {
				// Add 1-cell horizontal margin on each side
				itemX += metrics.CellWidth
				itemWidth -= metrics.CellWidth * 2
				if itemWidth < 0 {
					itemWidth = 0
				}
			}

			itemBounds = core.UnitRect{
				X:      itemX,
				Y:      pos,
				Width:  itemWidth,
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
		case core.AlignFill:
			// Fill available space - no adjustment needed
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
		case core.AlignFill:
			// Fill available space - no adjustment needed
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

// horizontalItemWidths computes item widths for the horizontal
// orientation given the content width (margins already removed),
// mirroring Layout's spacing rules.
func (l *BoxLayout) horizontalItemWidths(contentWidth core.Unit, metrics core.CellMetrics, baseSpacing, inlineSpacingTotal core.Unit) []core.Unit {
	// For inline gaps, use inline spacing; for container gaps, use base spacing
	totalSpacing := inlineSpacingTotal
	for i := 0; i < len(l.items)-1; i++ {
		if !isInlineWidget(l.items[i].Widget) && !isInlineWidget(l.items[i+1].Widget) {
			totalSpacing += baseSpacing
		}
	}

	stretchItems := make([]stretchItem, len(l.items))
	for i, item := range l.items {
		hint := item.Widget.SizeHint()
		policy := item.Widget.SizePolicy()

		stretch := 0
		if policy.Horizontal == core.SizeExpanding || item.Stretch > 0 {
			stretch = item.Stretch
			if stretch == 0 {
				stretch = 1
			}
		}

		stretchItems[i] = stretchItem{
			minimum: hint.Width,
			stretch: stretch,
		}
	}

	return calculateStretch(contentWidth-totalSpacing, stretchItems)
}

// verticalItemWidth returns the width an item will receive in a
// vertical layout (inline widgets are inset one cell per side).
func (l *BoxLayout) verticalItemWidth(contentWidth core.Unit, item *LayoutItem, metrics core.CellMetrics) core.Unit {
	if isInlineWidget(item.Widget) {
		contentWidth -= metrics.CellWidth * 2
	}
	if contentWidth < 0 {
		contentWidth = 0
	}
	return contentWidth
}

// itemHeightForWidth returns a widget's height at the given width,
// consulting core.HeightForWidther when implemented and falling back
// to the size hint.
func itemHeightForWidth(w core.Widget, width core.Unit) core.Unit {
	if hfw, ok := w.(core.HeightForWidther); ok && hfw.HasHeightForWidth() {
		if h := hfw.HeightForWidth(width); h > 0 {
			return h
		}
	}
	return w.SizeHint().Height
}

// inlineSpacingForItems computes the extra horizontal spacing inline
// widgets receive in a horizontal layout (mirrors Layout).
func (l *BoxLayout) inlineSpacingForItems(metrics core.CellMetrics) core.Unit {
	var total core.Unit
	if len(l.items) == 0 {
		return 0
	}
	if isInlineWidget(l.items[0].Widget) {
		total += metrics.CellWidth
	}
	for i := 0; i < len(l.items)-1; i++ {
		if isInlineWidget(l.items[i].Widget) || isInlineWidget(l.items[i+1].Widget) {
			total += metrics.CellWidth
		}
	}
	if isInlineWidget(l.items[len(l.items)-1].Widget) {
		total += metrics.CellWidth
	}
	return total
}

// HasHeightForWidth reports whether any item in this layout has
// width-dependent height. Together with HeightForWidth this lets
// containers (Panel) propagate core.HeightForWidther upward.
func (l *BoxLayout) HasHeightForWidth() bool {
	for _, item := range l.items {
		if hfw, ok := item.Widget.(core.HeightForWidther); ok && hfw.HasHeightForWidth() {
			return true
		}
	}
	return false
}

// HeightForWidth returns the height this layout requires at the given
// container width.
func (l *BoxLayout) HeightForWidth(width core.Unit) core.Unit {
	if len(l.items) == 0 {
		return 0
	}

	metrics := core.DefaultCellMetrics()
	contentWidth := width - l.margins.Horizontal()
	if contentWidth < 0 {
		contentWidth = 0
	}

	var height core.Unit
	if l.orientation == core.Vertical {
		spacing := core.Unit(metrics.UnitsToCellY(l.spacing)) * metrics.CellHeight
		for i, item := range l.items {
			height += itemHeightForWidth(item.Widget, l.verticalItemWidth(contentWidth, item, metrics))
			if i < len(l.items)-1 {
				height += spacing
			}
		}
	} else {
		spacing := core.Unit(metrics.UnitsToCellX(l.spacing)) * metrics.CellWidth
		widths := l.horizontalItemWidths(contentWidth, metrics, spacing, l.inlineSpacingForItems(metrics))
		for i, item := range l.items {
			if h := itemHeightForWidth(item.Widget, widths[i]); h > height {
				height = h
			}
		}
	}

	return height + l.margins.Vertical()
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
