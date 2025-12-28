// Package widgets provides standard UI widgets for the TUI toolkit.
package widgets

import (
	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/style"
)

// Label displays static text. It cannot receive focus.
type Label struct {
	core.WidgetBase
	core.AccessibleWidget

	text      string
	alignment core.Alignment
	wordWrap  bool
}

// NewLabel creates a new label with the given text.
func NewLabel(text string) *Label {
	l := &Label{
		text:      text,
		alignment: core.AlignLeft,
	}
	l.WidgetBase = *core.NewWidgetBase()
	l.Init(l)
	l.SetFocusPolicy(core.NoFocus)
	l.SetAccessibleRole(core.RoleLabel)
	l.SetAccessibleName(text)
	return l
}

// Text returns the label text.
func (l *Label) Text() string {
	return l.text
}

// SetText sets the label text.
func (l *Label) SetText(text string) {
	l.text = text
	l.SetAccessibleName(text)
	l.Update()
}

// Alignment returns the text alignment.
func (l *Label) Alignment() core.Alignment {
	return l.alignment
}

// SetAlignment sets the text alignment.
func (l *Label) SetAlignment(align core.Alignment) {
	l.alignment = align
	l.Update()
}

// WordWrap returns whether word wrapping is enabled.
func (l *Label) WordWrap() bool {
	return l.wordWrap
}

// SetWordWrap enables or disables word wrapping.
func (l *Label) SetWordWrap(wrap bool) {
	l.wordWrap = wrap
	l.Update()
}

// SizeHint returns the preferred size.
func (l *Label) SizeHint() core.UnitSize {
	metrics := core.DefaultCellMetrics()
	textLen := len([]rune(l.text))
	return core.UnitSize{
		Width:  metrics.TextWidth(textLen),
		Height: metrics.TextHeight(1),
	}
}

// IsInlineWidget returns true to indicate this is a text-style widget
// that should receive horizontal margins when in a vertical box layout.
func (l *Label) IsInlineWidget() bool {
	return true
}

// Paint renders the label.
func (l *Label) Paint(p *core.Painter) {
	bounds := l.Bounds()
	scheme := l.GetScheme()
	inheritedBG := l.EffectiveBackgroundColor()

	// Build style from scheme colors
	var s style.CellStyle
	if !l.IsEnabled() {
		s = style.DefaultStyle().WithFg(scheme.GetDisabledLabelFG()).WithBg(inheritedBG)
	} else {
		// Note: Using true for active - TODO: query actual window active state
		s = style.DefaultStyle().WithFg(scheme.GetLabelFG(true)).WithBg(inheritedBG)
	}

	// Use custom style if set (overrides scheme)
	if customStyle := l.Style(); customStyle != nil {
		s = *customStyle
		s = s.WithBg(inheritedBG) // Still inherit background
	}

	// Clear background
	p.Clear(core.UnitRect{Width: bounds.Width, Height: bounds.Height}, s)

	// Draw text
	if l.wordWrap {
		l.paintWrapped(p, bounds, s)
	} else {
		p.DrawTextAligned(
			core.UnitRect{Width: bounds.Width, Height: bounds.Height},
			l.text,
			l.alignment,
			core.AlignMiddle,
			s,
		)
	}
}

// paintWrapped renders word-wrapped text.
func (l *Label) paintWrapped(p *core.Painter, bounds core.UnitRect, s style.CellStyle) {
	metrics := p.Metrics()
	maxChars := metrics.CharsForWidth(bounds.Width)
	maxLines := metrics.LinesForHeight(bounds.Height)

	if maxChars <= 0 || maxLines <= 0 {
		return
	}

	lines := wrapText(l.text, maxChars)
	y := core.Unit(0)

	for i, line := range lines {
		if i >= maxLines {
			break
		}

		p.DrawTextAligned(
			core.UnitRect{X: 0, Y: y, Width: bounds.Width, Height: metrics.CellHeight},
			line,
			l.alignment,
			core.AlignTop,
			s,
		)
		y += metrics.CellHeight
	}
}

// AccessibleInfo returns accessibility information.
func (l *Label) AccessibleInfo() core.AccessibleInfo {
	info := l.AccessibleWidget.AccessibleInfo()
	info.Role = core.RoleLabel
	info.Name = l.text
	return info
}

// wrapText wraps text to the given width.
func wrapText(text string, maxWidth int) []string {
	if maxWidth <= 0 {
		return nil
	}

	var lines []string
	var currentLine string
	currentWidth := 0

	for _, r := range text {
		if r == '\n' {
			lines = append(lines, currentLine)
			currentLine = ""
			currentWidth = 0
			continue
		}

		charWidth := 1 // Simplified; would need proper width calculation for CJK

		if currentWidth+charWidth > maxWidth {
			lines = append(lines, currentLine)
			currentLine = string(r)
			currentWidth = charWidth
		} else {
			currentLine += string(r)
			currentWidth += charWidth
		}
	}

	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return lines
}
