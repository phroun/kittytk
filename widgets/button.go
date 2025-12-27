// Package widgets provides standard UI widgets for the TUI toolkit.
package widgets

import (
	"time"

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
	pressed        bool
	hovered        bool // Mouse is over button while pressed
	spacePressed   bool // Space key is being held down
	animatingPress bool // Showing press animation (250ms visual feedback)
	flat           bool // No border when not focused/hovered
	isDefault      bool // Default button (shown bold when not focused)
	isCancel       bool // Cancel button (activated by Escape)

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
	b.Init(b) // Enable polymorphic focus handling
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

// IsDefault returns whether this is the default button.
func (b *Button) IsDefault() bool {
	return b.isDefault
}

// SetDefault makes this the default button (shown bold when not focused).
func (b *Button) SetDefault(isDefault bool) {
	b.isDefault = isDefault
	b.Update()
}

// IsCancel returns whether this is the cancel button.
func (b *Button) IsCancel() bool {
	return b.isCancel
}

// SetCancel makes this the cancel button (activated by Escape key).
func (b *Button) SetCancel(isCancel bool) {
	b.isCancel = isCancel
}

// AnimatePress shows the pressed state briefly (250ms) then triggers click.
// This provides visual feedback for keyboard-triggered button presses.
func (b *Button) AnimatePress() {
	if !b.IsEnabled() || b.animatingPress {
		return
	}

	// If already showing pressed state (e.g., space held), just click
	if b.spacePressed || (b.pressed && b.hovered) {
		b.Click()
		return
	}

	// Show pressed state
	b.animatingPress = true
	b.Update()

	// After 250ms, clear animation and trigger click
	go func() {
		time.Sleep(250 * time.Millisecond)
		b.animatingPress = false
		b.Update()
		b.Click()
	}()
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

	// Add brackets/spaces: "<text>" or " text "
	// Plus 1 for drop shadow on the right
	totalChars := textLen + iconWidth + 2 + 1 // bracket + text + bracket + shadow

	return core.UnitSize{
		Width:  metrics.TextWidth(totalChars),
		Height: metrics.TextHeight(2), // 2 rows: button + shadow row
	}
}

// Paint renders the button.
// TUI button rendering with drop shadow:
//   - Normal:  " OK ▄" on top row, " ▀▀▀" on bottom row (shifted right)
//   - Pressed: "  OK " shifted right by 1, no shadow
//   - Focused: "<OK>" with angle brackets
func (b *Button) Paint(p *core.Painter) {
	bounds := b.Bounds()
	theme := b.Theme()
	focused := b.HasFocus()
	metrics := p.Metrics()

	// Determine if showing pressed visual (pressed and hovering, space held, animating, or checked)
	showPressed := (b.pressed && b.hovered) || b.spacePressed || b.animatingPress || b.checked

	// Determine style
	var s style.CellStyle
	if !b.IsEnabled() {
		s = theme.Disabled
	} else if showPressed {
		s = theme.ButtonPressed
	} else if focused {
		s = theme.ButtonFocused
	} else if b.isDefault {
		// Default button gets bold text when not focused
		s = theme.Button.WithAttrs(style.StyleBold)
	} else {
		s = theme.Button
	}

	// Use custom style if set
	if customStyle := b.Style(); customStyle != nil {
		s = *customStyle
	}

	// Shadow style (black foreground on default/inherited background)
	shadowStyle := style.DefaultStyle().WithFg(style.ColorBlack)

	// Calculate button content width (excluding shadow)
	textLen := len([]rune(b.text))
	buttonWidth := metrics.TextWidth(textLen + 2) // brackets + text

	// Icon handling
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
			buttonWidth += iconWidth
		}
	}

	// X offset: pressed state shifts right by 1 cell
	xOffset := core.Unit(0)
	if showPressed {
		xOffset = metrics.CellWidth
	}

	// Clear the entire button area first (to handle pressed state transition)
	p.FillRect(core.UnitRect{Width: bounds.Width, Height: bounds.Height}, ' ', theme.Normal)

	// Draw button background
	if !b.flat || focused || showPressed {
		p.FillRect(core.UnitRect{
			X:      xOffset,
			Y:      0,
			Width:  buttonWidth,
			Height: metrics.CellHeight,
		}, ' ', s)
	}

	// Draw drop shadow (only when not pressed)
	if !showPressed && b.IsEnabled() {
		// Bottom half block on right edge of button (top row)
		shadowX := xOffset + buttonWidth
		p.DrawCell(shadowX, 0, '▄', shadowStyle)

		// Top half blocks on second row (shifted right by 1)
		shadowY := metrics.CellHeight
		for i := 0; i < int(buttonWidth/metrics.CellWidth); i++ {
			p.DrawCell(metrics.CellWidth+metrics.CellToUnitsX(i), shadowY, '▀', shadowStyle)
		}
	}

	// Draw icon if present
	if b.icon != nil && iconWidth > 0 {
		var textIcon style.TextIcon
		if b.iconSize == style.IconSmall && b.icon.HasText(style.IconSmall) {
			textIcon = b.icon.TextSmall
		} else if b.icon.HasText(style.IconLarge) {
			textIcon = b.icon.TextLarge
		}

		if textIcon.Width > 0 {
			x := xOffset + metrics.CellWidth
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

	// Draw button with TUI-style brackets
	// Focused: <text>  Normal: " text "
	leftBracket := ' '
	rightBracket := ' '
	if focused {
		leftBracket = '<'
		rightBracket = '>'
	}

	// Draw left bracket/space
	p.DrawCell(xOffset+iconWidth, 0, leftBracket, s)

	// Draw text
	if b.text != "" {
		textX := xOffset + iconWidth + metrics.CellWidth
		for i, ch := range b.text {
			p.DrawCell(textX+metrics.CellToUnitsX(i), 0, ch, s)
		}
	}

	// Draw right bracket/space
	rightX := xOffset + buttonWidth - metrics.CellWidth
	p.DrawCell(rightX, 0, rightBracket, s)
}

// HandleKeyPress handles keyboard input.
func (b *Button) HandleKeyPress(event core.KeyPressEvent) bool {
	switch event.Key {
	case "Enter":
		// Enter triggers with animation for visual feedback
		b.AnimatePress()
		return true
	case " ", "Space":
		// Space shows pressed state, waits for release to trigger
		if !b.spacePressed {
			b.spacePressed = true
			b.Update()
		}
		return true
	case "Escape":
		// Escape cancels space press first
		if b.spacePressed {
			b.spacePressed = false
			b.Update()
			return true
		}
		// If this is a cancel button, activate it
		if b.isCancel {
			b.AnimatePress()
			return true
		}
	}
	return false
}

// HandleKeyRelease handles key release.
func (b *Button) HandleKeyRelease(event core.KeyReleaseEvent) bool {
	switch event.Key {
	case " ", "Space":
		if b.spacePressed {
			b.spacePressed = false
			b.Update()
			b.Click()
			return true
		}
	}
	return false
}

// HandleMousePress handles mouse clicks.
func (b *Button) HandleMousePress(event core.MousePressEvent) bool {
	if event.Button == core.LeftButton {
		b.SetFocus() // Focus on mouse down
		b.pressed = true
		b.hovered = true
		b.Update()
		return true
	}
	return false
}

// HandleMouseMove handles mouse movement during press.
func (b *Button) HandleMouseMove(event core.MouseMoveEvent) bool {
	if !b.pressed {
		return false
	}

	// Check if mouse is still inside button area
	bounds := b.Bounds()
	metrics := core.DefaultCellMetrics()

	// Simple bounds check for first row (button area, not shadow)
	newHovered := event.X >= 0 && event.X < bounds.Width &&
		event.Y >= 0 && event.Y < metrics.CellHeight

	if newHovered != b.hovered {
		b.hovered = newHovered
		b.Update()
	}

	return true // Capture mouse while pressed
}

// HandleMouseRelease handles mouse release.
func (b *Button) HandleMouseRelease(event core.MouseReleaseEvent) bool {
	if b.pressed {
		wasHovered := b.hovered
		b.pressed = false
		b.hovered = false
		b.Update()

		// Only trigger click if mouse was still on the button
		if wasHovered {
			b.Click()
		}
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
	b.hovered = false
	b.spacePressed = false
	b.animatingPress = false
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
