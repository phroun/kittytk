// Package widgets provides standard UI widgets for the TUI toolkit.
package widgets

import (
	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/style"
)

// ProgressBar displays progress as a horizontal bar.
type ProgressBar struct {
	core.WidgetBase
	core.AccessibleWidget

	value       int
	minimum     int
	maximum     int
	orientation core.Orientation
	textVisible bool
	format      string // e.g., "%p%" for percentage

	// Indeterminate mode (unknown progress)
	indeterminate     bool
	indeterminatePos  int
	indeterminateDir  int
}

// NewProgressBar creates a new progress bar.
func NewProgressBar() *ProgressBar {
	p := &ProgressBar{
		minimum:     0,
		maximum:     100,
		orientation: core.Horizontal,
		textVisible: true,
		format:      "%p%",
		indeterminateDir: 1,
	}
	p.WidgetBase = *core.NewWidgetBase()
	p.Init(p)
	p.SetFocusPolicy(core.NoFocus)
	p.SetAccessibleRole(core.RoleProgressBar)
	return p
}

// Value returns the current value.
func (p *ProgressBar) Value() int {
	return p.value
}

// SetValue sets the current value.
func (p *ProgressBar) SetValue(value int) {
	if value < p.minimum {
		value = p.minimum
	}
	if value > p.maximum {
		value = p.maximum
	}
	if p.value == value {
		return
	}
	p.value = value
	p.Update()
}

// Minimum returns the minimum value.
func (p *ProgressBar) Minimum() int {
	return p.minimum
}

// SetMinimum sets the minimum value.
func (p *ProgressBar) SetMinimum(min int) {
	p.minimum = min
	if p.value < min {
		p.value = min
	}
	p.Update()
}

// Maximum returns the maximum value.
func (p *ProgressBar) Maximum() int {
	return p.maximum
}

// SetMaximum sets the maximum value.
func (p *ProgressBar) SetMaximum(max int) {
	p.maximum = max
	if p.value > max {
		p.value = max
	}
	p.Update()
}

// SetRange sets both minimum and maximum.
func (p *ProgressBar) SetRange(min, max int) {
	p.minimum = min
	p.maximum = max
	if p.value < min {
		p.value = min
	}
	if p.value > max {
		p.value = max
	}
	p.Update()
}

// Orientation returns the orientation.
func (p *ProgressBar) Orientation() core.Orientation {
	return p.orientation
}

// SetOrientation sets the orientation.
func (p *ProgressBar) SetOrientation(orientation core.Orientation) {
	p.orientation = orientation
	p.Update()
}

// IsTextVisible returns whether the text is visible.
func (p *ProgressBar) IsTextVisible() bool {
	return p.textVisible
}

// SetTextVisible sets whether the text is visible.
func (p *ProgressBar) SetTextVisible(visible bool) {
	p.textVisible = visible
	p.Update()
}

// Format returns the text format.
func (p *ProgressBar) Format() string {
	return p.format
}

// SetFormat sets the text format.
// %p = percentage (0-100)
// %v = value
// %m = maximum
func (p *ProgressBar) SetFormat(format string) {
	p.format = format
	p.Update()
}

// IsIndeterminate returns whether the progress bar is in indeterminate mode.
func (p *ProgressBar) IsIndeterminate() bool {
	return p.indeterminate
}

// SetIndeterminate sets whether the progress bar is in indeterminate mode.
func (p *ProgressBar) SetIndeterminate(indeterminate bool) {
	p.indeterminate = indeterminate
	p.Update()
}

// Percentage returns the current percentage (0-100).
func (p *ProgressBar) Percentage() int {
	if p.maximum == p.minimum {
		return 0
	}
	return (p.value - p.minimum) * 100 / (p.maximum - p.minimum)
}

// Reset resets the progress bar to minimum.
func (p *ProgressBar) Reset() {
	p.value = p.minimum
	p.Update()
}

// Advance advances the value by the given amount.
func (p *ProgressBar) Advance(amount int) {
	p.SetValue(p.value + amount)
}

// SizeHint returns the preferred size.
func (p *ProgressBar) SizeHint() core.UnitSize {
	metrics := core.DefaultCellMetrics()
	if p.orientation == core.Horizontal {
		return core.UnitSize{
			Width:  metrics.TextWidth(20),
			Height: metrics.TextHeight(1),
		}
	}
	return core.UnitSize{
		Width:  metrics.TextWidth(2),
		Height: metrics.TextHeight(10),
	}
}

// IsInlineWidget returns true to indicate this is a text-style widget
// that should receive horizontal margins when in a vertical box layout.
func (p *ProgressBar) IsInlineWidget() bool {
	return true
}

// Paint renders the progress bar.
func (p *ProgressBar) Paint(painter *core.Painter) {
	bounds := p.Bounds()
	scheme := p.GetScheme()
	metrics := painter.Metrics()

	if p.orientation == core.Horizontal {
		p.paintHorizontal(painter, bounds, scheme, metrics)
	} else {
		p.paintVertical(painter, bounds, scheme, metrics)
	}
}

func (p *ProgressBar) paintHorizontal(painter *core.Painter, bounds core.UnitRect, scheme *style.Scheme, metrics core.CellMetrics) {
	// Get progress bar styles from scheme
	completedStyle := scheme.GetProgressFull()
	incompleteStyle := scheme.GetProgressEmpty()

	// Draw incomplete background first
	for i := 0; i < metrics.CharsForWidth(bounds.Width); i++ {
		x := core.Unit(i) * metrics.CellWidth
		painter.DrawCell(x, 0, '░', incompleteStyle)
	}

	totalCells := metrics.CharsForWidth(bounds.Width)

	if p.indeterminate {
		// Animate indeterminate bar - moving part uses completed style
		blockSize := 5
		if p.indeterminatePos+blockSize >= totalCells {
			p.indeterminateDir = -1
		} else if p.indeterminatePos <= 0 {
			p.indeterminateDir = 1
		}

		for i := 0; i < blockSize && p.indeterminatePos+i < totalCells; i++ {
			x := core.Unit(p.indeterminatePos+i) * metrics.CellWidth
			painter.DrawCell(x, 0, '▓', completedStyle)
		}
		p.indeterminatePos += p.indeterminateDir
	} else {
		// Calculate filled portion
		filledCells := totalCells * p.Percentage() / 100

		// Draw filled portion
		for i := 0; i < filledCells; i++ {
			x := core.Unit(i) * metrics.CellWidth
			painter.DrawCell(x, 0, '▓', completedStyle)
		}
	}

	// Draw text in center
	if p.textVisible && !p.indeterminate {
		text := p.formatText()
		textLen := len(text)
		startX := (totalCells - textLen) / 2
		if startX < 0 {
			startX = 0
		}

		// Get text styles from scheme
		activeTextStyle := scheme.GetProgressFullText()
		inactiveTextStyle := scheme.GetProgressEmptyText()

		filledCells := totalCells * p.Percentage() / 100
		for i, ch := range text {
			x := core.Unit(startX+i) * metrics.CellWidth
			// Use appropriate style based on position
			var s style.CellStyle
			if startX+i < filledCells {
				s = activeTextStyle
			} else {
				s = inactiveTextStyle
			}
			painter.DrawCell(x, 0, ch, s)
		}
	}
}

func (p *ProgressBar) paintVertical(painter *core.Painter, bounds core.UnitRect, scheme *style.Scheme, metrics core.CellMetrics) {
	// Get progress bar styles from scheme
	completedStyle := scheme.GetProgressFull()
	incompleteStyle := scheme.GetProgressEmpty()

	totalCells := int(bounds.Height / metrics.CellHeight)

	// Draw incomplete background first (entire bar)
	for i := 0; i < totalCells; i++ {
		y := core.Unit(i) * metrics.CellHeight
		painter.FillRect(core.UnitRect{
			Y:      y,
			Width:  bounds.Width,
			Height: metrics.CellHeight,
		}, '░', incompleteStyle)
	}

	// Calculate filled portion (from bottom)
	filledCells := totalCells * p.Percentage() / 100

	// Draw filled portion from bottom
	for i := 0; i < filledCells; i++ {
		y := bounds.Height - core.Unit(i+1)*metrics.CellHeight
		painter.FillRect(core.UnitRect{
			Y:      y,
			Width:  bounds.Width,
			Height: metrics.CellHeight,
		}, '▓', completedStyle)
	}
}

func (p *ProgressBar) formatText() string {
	// Format percentage properly (handles 0-100)
	pct := p.Percentage()
	if pct >= 100 {
		return "100%"
	} else if pct >= 10 {
		return string(rune('0'+pct/10)) + string(rune('0'+pct%10)) + "%"
	} else {
		return string(rune('0'+pct)) + "%"
	}
}

// AnimateIndeterminate advances the indeterminate animation.
// Call this periodically (e.g., every 100ms) when in indeterminate mode.
func (p *ProgressBar) AnimateIndeterminate() {
	if p.indeterminate {
		p.Update()
	}
}

// AccessibleInfo returns accessibility information.
func (p *ProgressBar) AccessibleInfo() core.AccessibleInfo {
	info := p.AccessibleWidget.AccessibleInfo()
	info.Role = core.RoleProgressBar
	info.Value = p.formatText()
	info.ValueMin = string(rune('0' + p.minimum))
	info.ValueMax = string(rune('0' + p.maximum))

	if p.indeterminate {
		info.State |= core.StateBusy
	}

	if !p.IsEnabled() {
		info.State |= core.StateDisabled
	}

	return info
}
