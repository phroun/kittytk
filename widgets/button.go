// Package widgets provides standard UI widgets for the TUI toolkit.
package widgets

import (
	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/style"
)

// Button is a clickable button widget.
type Button struct {
	core.WidgetBase
	core.AccessibleWidget

	text      string
	icon      *style.Icon
	iconSize  style.IconSize
	checkable bool
	checked   bool
	pressed   bool
	flat      bool // No border when not focused/hovered

	onClick  func()
	onToggle func(checked bool)
}

// NewButton creates a new button with the given text.
func NewButton(text string) *Button {
	b := &Button{
		text:     text,
		iconSize: style.IconSmall,
	}
	b.WidgetBase = *core.NewWidgetBase()
	b.SetFocusPolicy(core.StrongFocus)
	b.SetAccessibleRole(core.RoleButton)
	b.SetAccessibleName(text)
	return b
}

// NewIconButton creates a button with an icon.
func NewIconButton(icon *style.Icon) *Button {
	b := NewButton("")
	b.icon = icon
	if icon != nil {
		b.SetAccessibleName(icon.ID)
	}
	return b
}

// Text returns the button text.
func (b *Button) Text() string {
	return b.text
}

// SetText sets the button text.
func (b *Button) SetText(text string) {
	b.text = text
	b.SetAccessibleName(text)
	b.Update()
}

// Icon returns the button icon.
func (b *Button) Icon() *style.Icon {
	return b.icon
}

// SetIcon sets the button icon.
func (b *Button) SetIcon(icon *style.Icon) {
	b.icon = icon
	b.Update()
}

// SetIconSize sets the icon size.
func (b *Button) SetIconSize(size style.IconSize) {
	b.iconSize = size
	b.Update()
}

// IsCheckable returns whether the button is checkable.
func (b *Button) IsCheckable() bool {
	return b.checkable
}

// SetCheckable makes the button checkable (toggle button).
func (b *Button) SetCheckable(checkable bool) {
	b.checkable = checkable
	b.Update()
}

// IsChecked returns whether the button is checked.
func (b *Button) IsChecked() bool {
	return b.checked
}

// SetChecked sets the checked state.
func (b *Button) SetChecked(checked bool) {
	if b.checked == checked {
		return
	}
	b.checked = checked
	b.Update()
	if b.onToggle != nil {
		b.onToggle(checked)
	}
}

// IsFlat returns whether the button is flat (borderless).
func (b *Button) IsFlat() bool {
	return b.flat
}

// SetFlat makes the button flat.
func (b *Button) SetFlat(flat bool) {
	b.flat = flat
	b.Update()
}

// SetOnClick sets the click handler.
func (b *Button) SetOnClick(handler func()) {
	b.onClick = handler
}

// SetOnToggle sets the toggle handler.
func (b *Button) SetOnToggle(handler func(checked bool)) {
	b.onToggle = handler
}

// Click simulates a button click.
func (b *Button) Click() {
	if !b.IsEnabled() {
		return
	}

	if b.checkable {
		b.SetChecked(!b.checked)
	}

	if b.onClick != nil {
		b.onClick()
	}
}

// SizeHint returns the preferred size.
func (b *Button) SizeHint() core.UnitSize {
	metrics := core.DefaultCellMetrics()

	// Calculate text width
	textLen := len([]rune(b.text))

	// Add icon width if present
	iconWidth := 0
	if b.icon != nil {
		if b.iconSize == style.IconSmall {
			iconWidth = 3
		} else {
			iconWidth = 5
		}
		if textLen > 0 {
			iconWidth++ // Space between icon and text
		}
	}

	// Add padding and borders
	totalChars := textLen + iconWidth + 4 // [ text ]

	return core.UnitSize{
		Width:  metrics.TextWidth(totalChars),
		Height: metrics.TextHeight(1),
	}
}

// Paint renders the button.
func (b *Button) Paint(p *core.Painter) {
	bounds := b.Bounds()
	theme := b.Theme()
	focused := b.HasFocus()

	// Determine style
	var s style.CellStyle
	if !b.IsEnabled() {
		s = theme.Disabled
	} else if b.pressed || b.checked {
		s = theme.ButtonPressed
	} else if focused {
		s = theme.ButtonFocused
	} else {
		s = theme.Button
	}

	// Use custom style if set
	if customStyle := b.Style(); customStyle != nil {
		s = *customStyle
	}

	// Draw background
	if !b.flat || focused || b.pressed || b.checked {
		p.FillRect(core.UnitRect{Width: bounds.Width, Height: bounds.Height}, ' ', s)
	}

	// Calculate content
	metrics := p.Metrics()
	content := b.text

	// Draw icon if present
	iconWidth := core.Unit(0)
	if b.icon != nil {
		var textIcon style.TextIcon
		if b.iconSize == style.IconSmall && b.icon.HasText(style.IconSmall) {
			textIcon = b.icon.TextSmall
		} else if b.icon.HasText(style.IconLarge) {
			textIcon = b.icon.TextLarge
		}

		if textIcon.Width > 0 {
			iconWidth = metrics.TextWidth(textIcon.Width + 1)
			// Draw icon at left side
			x := metrics.CellWidth
			y := core.Unit(0)
			for row := 0; row < textIcon.Height; row++ {
				for col := 0; col < textIcon.Width; col++ {
					cell := textIcon.CellAt(col, row)
					p.DrawCell(x+metrics.CellToUnitsX(col), y+metrics.CellToUnitsY(row),
						cell.Char, cell.Style)
				}
			}
		}
	}

	// Draw text centered
	if content != "" {
		textRect := core.UnitRect{
			X:      iconWidth,
			Y:      0,
			Width:  bounds.Width - iconWidth,
			Height: bounds.Height,
		}
		p.DrawTextAligned(textRect, content, core.AlignCenter, core.AlignMiddle, s)
	}

	// Draw focus indicator
	if focused && !b.flat {
		// Draw brackets around content
		p.DrawCell(0, 0, '[', s)
		p.DrawCell(bounds.Width-metrics.CellWidth, 0, ']', s)
	}
}

// HandleKeyPress handles keyboard input.
func (b *Button) HandleKeyPress(event core.KeyPressEvent) bool {
	switch event.Key {
	case "Enter", " ":
		b.Click()
		return true
	}
	return false
}

// HandleMousePress handles mouse clicks.
func (b *Button) HandleMousePress(event core.MousePressEvent) bool {
	if event.Button == core.LeftButton {
		b.pressed = true
		b.Update()
		return true
	}
	return false
}

// HandleMouseRelease handles mouse release.
func (b *Button) HandleMouseRelease(event core.MouseReleaseEvent) bool {
	if b.pressed {
		b.pressed = false
		b.Update()
		b.Click()
		return true
	}
	return false
}

// HandleFocusIn is called when focus is gained.
func (b *Button) HandleFocusIn() {
	b.Update()
}

// HandleFocusOut is called when focus is lost.
func (b *Button) HandleFocusOut() {
	b.pressed = false
	b.Update()
}

// AccessibleInfo returns accessibility information.
func (b *Button) AccessibleInfo() core.AccessibleInfo {
	info := b.AccessibleWidget.AccessibleInfo()
	info.Role = core.RoleButton
	info.Name = b.text
	if b.checkable {
		if b.checked {
			info.State |= core.StateChecked
		}
	}
	if b.pressed {
		info.State |= core.StatePressed
	}
	if !b.IsEnabled() {
		info.State |= core.StateDisabled
	}
	return info
}
