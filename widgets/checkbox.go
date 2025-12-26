// Package widgets provides standard UI widgets for the TUI toolkit.
package widgets

import (
	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/style"
)

// Checkbox is a widget with a checkable state.
type Checkbox struct {
	core.WidgetBase
	core.AccessibleWidget

	text    string
	checked bool
	triState bool // If true, supports indeterminate state
	checkState CheckState

	// Callbacks
	onStateChanged func(state CheckState)
	onToggled      func(checked bool)
}

// CheckState represents the state of a checkbox.
type CheckState int

const (
	Unchecked CheckState = iota
	PartiallyChecked
	Checked
)

// NewCheckbox creates a new checkbox with the given text.
func NewCheckbox(text string) *Checkbox {
	c := &Checkbox{
		text:       text,
		checkState: Unchecked,
	}
	c.WidgetBase = *core.NewWidgetBase()
	c.SetFocusPolicy(core.StrongFocus)
	c.SetAccessibleRole(core.RoleCheckbox)
	c.SetAccessibleName(text)
	return c
}

// Text returns the checkbox text.
func (c *Checkbox) Text() string {
	return c.text
}

// SetText sets the checkbox text.
func (c *Checkbox) SetText(text string) {
	c.text = text
	c.SetAccessibleName(text)
	c.Update()
}

// IsChecked returns whether the checkbox is checked.
func (c *Checkbox) IsChecked() bool {
	return c.checkState == Checked
}

// SetChecked sets the checked state.
func (c *Checkbox) SetChecked(checked bool) {
	if checked {
		c.SetCheckState(Checked)
	} else {
		c.SetCheckState(Unchecked)
	}
}

// CheckState returns the current check state.
func (c *Checkbox) CheckState() CheckState {
	return c.checkState
}

// SetCheckState sets the check state.
func (c *Checkbox) SetCheckState(state CheckState) {
	if c.checkState == state {
		return
	}

	c.checkState = state
	c.checked = state == Checked
	c.Update()

	if c.onStateChanged != nil {
		c.onStateChanged(state)
	}

	if c.onToggled != nil {
		c.onToggled(state == Checked)
	}
}

// IsTriState returns whether the checkbox supports tri-state.
func (c *Checkbox) IsTriState() bool {
	return c.triState
}

// SetTriState sets whether the checkbox supports tri-state.
func (c *Checkbox) SetTriState(triState bool) {
	c.triState = triState
	if !triState && c.checkState == PartiallyChecked {
		c.SetCheckState(Unchecked)
	}
}

// Toggle toggles the checkbox state.
func (c *Checkbox) Toggle() {
	switch c.checkState {
	case Unchecked:
		c.SetCheckState(Checked)
	case Checked:
		if c.triState {
			c.SetCheckState(PartiallyChecked)
		} else {
			c.SetCheckState(Unchecked)
		}
	case PartiallyChecked:
		c.SetCheckState(Unchecked)
	}
}

// SetOnStateChanged sets the state changed callback.
func (c *Checkbox) SetOnStateChanged(handler func(state CheckState)) {
	c.onStateChanged = handler
}

// SetOnToggled sets the toggled callback.
func (c *Checkbox) SetOnToggled(handler func(checked bool)) {
	c.onToggled = handler
}

// SizeHint returns the preferred size.
func (c *Checkbox) SizeHint() core.UnitSize {
	metrics := core.DefaultCellMetrics()
	// [X] Text
	width := metrics.TextWidth(len(c.text) + 4) // "[ ] " prefix
	return core.UnitSize{
		Width:  width,
		Height: metrics.TextHeight(1),
	}
}

// Paint renders the checkbox.
func (c *Checkbox) Paint(p *core.Painter) {
	bounds := c.Bounds()
	theme := c.Theme()
	focused := c.HasFocus()
	metrics := p.Metrics()

	// Determine style
	var s style.CellStyle
	if !c.IsEnabled() {
		s = theme.Disabled
	} else if focused {
		s = theme.Focused
	} else {
		s = theme.Normal
	}

	// Draw checkbox indicator
	var indicator string
	switch c.checkState {
	case Unchecked:
		indicator = "[ ]"
	case Checked:
		indicator = "[x]"
	case PartiallyChecked:
		indicator = "[-]"
	}

	x := core.Unit(0)
	for _, ch := range indicator {
		p.DrawCell(x, 0, ch, s)
		x += metrics.CellWidth
	}

	// Draw space
	p.DrawCell(x, 0, ' ', s)
	x += metrics.CellWidth

	// Draw text
	textStyle := s
	if !c.IsEnabled() {
		textStyle = theme.Disabled
	}
	for _, ch := range c.text {
		if x >= bounds.Width {
			break
		}
		p.DrawCell(x, 0, ch, textStyle)
		x += metrics.CellWidth
	}
}

// HandleKeyPress handles keyboard input.
func (c *Checkbox) HandleKeyPress(event core.KeyPressEvent) bool {
	switch event.Key {
	case " ", "Space", "Enter":
		c.Toggle()
		return true
	}
	return false
}

// HandleMousePress handles mouse clicks.
func (c *Checkbox) HandleMousePress(event core.MousePressEvent) bool {
	if event.Button == core.LeftButton {
		c.SetFocus()
		c.Toggle()
		return true
	}
	return false
}

// HandleFocusIn is called when focus is gained.
func (c *Checkbox) HandleFocusIn() {
	c.Update()
}

// HandleFocusOut is called when focus is lost.
func (c *Checkbox) HandleFocusOut() {
	c.Update()
}

// AccessibleInfo returns accessibility information.
func (c *Checkbox) AccessibleInfo() core.AccessibleInfo {
	info := c.AccessibleWidget.AccessibleInfo()
	info.Role = core.RoleCheckbox
	info.Name = c.text

	switch c.checkState {
	case Checked:
		info.State |= core.StateChecked
	case PartiallyChecked:
		info.State |= core.StateChecked
		info.Value = "indeterminate"
	}

	if !c.IsEnabled() {
		info.State |= core.StateDisabled
	}

	return info
}
