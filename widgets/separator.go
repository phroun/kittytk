// Package widgets provides standard UI widgets for the TUI toolkit.
package widgets

import (
	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/style"
)

// LineSeparator is a visual divider widget that draws a horizontal or vertical line.
// For horizontal separators, it draws: ────·· Title ··────
// For vertical separators, it draws a vertical line with optional title.
type LineSeparator struct {
	core.WidgetBase
	core.AccessibleWidget

	title       string
	orientation core.Orientation
}

// NewLineSeparator creates a new horizontal line separator.
func NewLineSeparator() *LineSeparator {
	s := &LineSeparator{
		orientation: core.Horizontal,
	}
	s.WidgetBase = *core.NewWidgetBase()
	s.Init(s)
	s.SetFocusPolicy(core.NoFocus)
	s.SetAccessibleRole(core.RoleSeparator)
	return s
}

// NewHSeparator creates a new horizontal separator with optional title.
func NewHSeparator(title string) *LineSeparator {
	s := NewLineSeparator()
	s.title = title
	s.orientation = core.Horizontal
	// Horizontal separator expands horizontally, fixed height
	s.SetSizePolicy(core.NewSizePolicy(core.SizeExpanding, core.SizeFixed))
	return s
}

// NewVSeparator creates a new vertical separator with optional title.
func NewVSeparator(title string) *LineSeparator {
	s := NewLineSeparator()
	s.title = title
	s.orientation = core.Vertical
	// Vertical separator expands vertically, fixed width
	s.SetSizePolicy(core.NewSizePolicy(core.SizeFixed, core.SizeExpanding))
	return s
}

// Title returns the separator title.
func (s *LineSeparator) Title() string {
	return s.title
}

// SetTitle sets the separator title.
func (s *LineSeparator) SetTitle(title string) {
	s.title = title
	s.Update()
}

// Orientation returns the separator orientation.
func (s *LineSeparator) Orientation() core.Orientation {
	return s.orientation
}

// SetOrientation sets the separator orientation.
func (s *LineSeparator) SetOrientation(o core.Orientation) {
	s.orientation = o
	s.Update()
}

// SizeHint returns the preferred size.
func (s *LineSeparator) SizeHint() core.UnitSize {
	metrics := s.EffectiveCellMetrics()
	font := s.EffectiveFont()
	if s.orientation == core.Horizontal {
		// Horizontal separator: 1 cell tall, width depends on title
		titleWidth := font.MeasureText(s.title)
		decorWidth := font.MeasureText("····    ") // Decoration around title
		return core.UnitSize{
			Width:  titleWidth + decorWidth,
			Height: metrics.CellHeight,
		}
	}
	// Vertical separator: 1 cell wide, height depends on title
	return core.UnitSize{
		Width:  metrics.CellWidth,
		Height: metrics.TextHeight(5),
	}
}

// Paint renders the separator.
func (s *LineSeparator) Paint(p *core.Painter) {
	bounds := s.Bounds()
	theme := s.Theme()
	metrics := s.EffectiveCellMetrics()

	// Use a subtle style for the separator
	lineStyle := theme.Normal

	// Use custom style if set
	if customStyle := s.Style(); customStyle != nil {
		lineStyle = *customStyle
	}

	if s.orientation == core.Horizontal {
		s.paintHorizontal(p, bounds, lineStyle, metrics)
	} else {
		s.paintVertical(p, bounds, lineStyle, metrics)
	}
}

// paintHorizontal draws: ────·· Title ··────
func (s *LineSeparator) paintHorizontal(p *core.Painter, bounds core.UnitRect, lineStyle style.CellStyle, metrics core.CellMetrics) {
	width := int(bounds.Width / metrics.CellWidth)
	if width <= 0 {
		return
	}

	y := core.Unit(0)
	titleRunes := []rune(s.title)
	titleLen := len(titleRunes)

	if titleLen == 0 {
		// No title: draw line with 4 middots centered
		// ────────··  ··────────
		center := width / 2
		for x := 0; x < width; x++ {
			ch := '─'
			// Draw ·· at center-1, center, center+1, center+2 (4 dots total)
			if x == center-1 || x == center || x == center+1 || x == center+2 {
				ch = '·'
			}
			p.DrawCell(metrics.CellToUnitsX(x), y, ch, lineStyle)
		}
	} else {
		// With title: ────·· Title ··────
		// Format: dots + space + title + space + dots
		middleContent := "·· " + s.title + " ··"
		middleLen := len([]rune(middleContent))
		startMiddle := (width - middleLen) / 2

		for x := 0; x < width; x++ {
			var ch rune
			if x < startMiddle {
				ch = '─'
			} else if x < startMiddle+middleLen {
				ch = []rune(middleContent)[x-startMiddle]
			} else {
				ch = '─'
			}
			p.DrawCell(metrics.CellToUnitsX(x), y, ch, lineStyle)
		}
	}
}

// paintVertical draws a vertical line with optional centered title.
func (s *LineSeparator) paintVertical(p *core.Painter, bounds core.UnitRect, lineStyle style.CellStyle, metrics core.CellMetrics) {
	height := int(bounds.Height / metrics.CellHeight)
	if height <= 0 {
		return
	}

	x := core.Unit(0)
	titleRunes := []rune(s.title)
	titleLen := len(titleRunes)

	if titleLen == 0 {
		// No title: draw line with 4 middots centered
		center := height / 2
		for y := 0; y < height; y++ {
			ch := '│'
			// Draw · at center-1, center, center+1, center+2 (4 dots total, stacked)
			if y == center-1 || y == center || y == center+1 || y == center+2 {
				ch = '·'
			}
			p.DrawCell(x, metrics.CellToUnitsY(y), ch, lineStyle)
		}
	} else {
		// With title: each character of title on its own row, centered
		// ·
		// ·
		// T
		// i
		// t
		// l
		// e
		// ·
		// ·
		middleLen := titleLen + 4 // 2 dots above, title, 2 dots below
		startMiddle := (height - middleLen) / 2
		if startMiddle < 0 {
			startMiddle = 0
		}

		for y := 0; y < height; y++ {
			var ch rune
			relY := y - startMiddle
			if y < startMiddle || relY >= middleLen {
				ch = '│'
			} else if relY == 0 || relY == 1 {
				ch = '·'
			} else if relY >= 2 && relY < 2+titleLen {
				ch = titleRunes[relY-2]
			} else if relY == 2+titleLen || relY == 3+titleLen {
				ch = '·'
			} else {
				ch = '│'
			}
			p.DrawCell(x, metrics.CellToUnitsY(y), ch, lineStyle)
		}
	}
}

// AccessibleInfo returns accessibility information.
func (s *LineSeparator) AccessibleInfo() core.AccessibleInfo {
	info := s.AccessibleWidget.AccessibleInfo()
	info.Role = core.RoleSeparator
	if s.title != "" {
		info.Name = s.title
	}
	return info
}
