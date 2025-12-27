// Package widgets provides standard UI widgets for the TUI toolkit.
package widgets

import (
	"sync"

	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/style"
)

// RadioButton is a mutually exclusive option button.
type RadioButton struct {
	core.WidgetBase
	core.AccessibleWidget

	text     string
	checked  bool
	group    *RadioGroup

	// Callbacks
	onToggled func(checked bool)
}

// NewRadioButton creates a new radio button with the given text.
func NewRadioButton(text string) *RadioButton {
	r := &RadioButton{
		text: text,
	}
	r.WidgetBase = *core.NewWidgetBase()
	r.Init(r) // Enable polymorphic focus handling
	r.SetFocusPolicy(core.StrongFocus)
	r.SetAccessibleRole(core.RoleRadioButton)
	r.SetAccessibleName(text)
	return r
}

// Text returns the radio button text.
func (r *RadioButton) Text() string {
	return r.text
}

// SetText sets the radio button text.
func (r *RadioButton) SetText(text string) {
	r.text = text
	r.SetAccessibleName(text)
	r.Update()
}

// IsChecked returns whether the radio button is checked.
func (r *RadioButton) IsChecked() bool {
	return r.checked
}

// SetChecked sets the checked state.
// This will uncheck other buttons in the same group.
func (r *RadioButton) SetChecked(checked bool) {
	if r.checked == checked {
		return
	}

	if checked && r.group != nil {
		r.group.selectButton(r)
	} else {
		r.checked = checked
		r.Update()
		if r.onToggled != nil {
			r.onToggled(checked)
		}
	}
}

// Group returns the radio group this button belongs to.
func (r *RadioButton) Group() *RadioGroup {
	return r.group
}

// SetOnToggled sets the toggled callback.
func (r *RadioButton) SetOnToggled(handler func(checked bool)) {
	r.onToggled = handler
}

// SizeHint returns the preferred size.
func (r *RadioButton) SizeHint() core.UnitSize {
	metrics := core.DefaultCellMetrics()
	// (o) Text
	width := metrics.TextWidth(len(r.text) + 4) // "( ) " prefix
	return core.UnitSize{
		Width:  width,
		Height: metrics.TextHeight(1),
	}
}

// IsInlineWidget returns true to indicate this is a text-style widget
// that should receive horizontal margins when in a vertical box layout.
func (r *RadioButton) IsInlineWidget() bool {
	return true
}

// Paint renders the radio button.
func (r *RadioButton) Paint(p *core.Painter) {
	bounds := r.Bounds()
	theme := r.Theme()
	focused := r.HasFocus()
	metrics := p.Metrics()

	// Determine style
	var s style.CellStyle
	if !r.IsEnabled() {
		s = theme.Disabled
	} else if focused {
		s = theme.Focused
	} else {
		s = theme.Normal
	}

	// Draw radio indicator
	var indicator string
	if r.checked {
		indicator = "(*)"
	} else {
		indicator = "( )"
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
	if !r.IsEnabled() {
		textStyle = theme.Disabled
	}
	for _, ch := range r.text {
		if x >= bounds.Width {
			break
		}
		p.DrawCell(x, 0, ch, textStyle)
		x += metrics.CellWidth
	}
}

// HandleKeyPress handles keyboard input.
func (r *RadioButton) HandleKeyPress(event core.KeyPressEvent) bool {
	switch event.Key {
	case " ", "Space", "Enter":
		r.SetChecked(true)
		return true
	case "Up", "Left":
		if r.group != nil {
			r.group.SelectPrevious()
			return true
		}
	case "Down", "Right":
		if r.group != nil {
			r.group.SelectNext()
			return true
		}
	}
	return false
}

// HandleMousePress handles mouse clicks.
func (r *RadioButton) HandleMousePress(event core.MousePressEvent) bool {
	if event.Button == core.LeftButton {
		r.SetFocus()
		r.SetChecked(true)
		return true
	}
	return false
}

// HandleFocusIn is called when focus is gained.
func (r *RadioButton) HandleFocusIn() {
	r.Update()
}

// HandleFocusOut is called when focus is lost.
func (r *RadioButton) HandleFocusOut() {
	r.Update()
}

// AccessibleInfo returns accessibility information.
func (r *RadioButton) AccessibleInfo() core.AccessibleInfo {
	info := r.AccessibleWidget.AccessibleInfo()
	info.Role = core.RoleRadioButton
	info.Name = r.text

	if r.checked {
		info.State |= core.StateChecked
	}

	if !r.IsEnabled() {
		info.State |= core.StateDisabled
	}

	if r.group != nil {
		info.SetSize = len(r.group.buttons)
		for i, btn := range r.group.buttons {
			if btn == r {
				info.PositionInSet = i + 1
				break
			}
		}
	}

	return info
}

// RadioGroup manages a group of mutually exclusive radio buttons.
type RadioGroup struct {
	mu      sync.RWMutex
	buttons []*RadioButton
	selected *RadioButton

	// Callbacks
	onSelectionChanged func(*RadioButton)
}

// NewRadioGroup creates a new radio group.
func NewRadioGroup() *RadioGroup {
	return &RadioGroup{}
}

// AddButton adds a radio button to the group.
func (g *RadioGroup) AddButton(button *RadioButton) {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Remove from old group if any
	if button.group != nil && button.group != g {
		button.group.RemoveButton(button)
	}

	button.group = g
	g.buttons = append(g.buttons, button)

	// If this is the first button and it's checked, or no button is selected,
	// consider auto-selection
	if button.checked {
		g.selected = button
	}
}

// RemoveButton removes a radio button from the group.
func (g *RadioGroup) RemoveButton(button *RadioButton) {
	g.mu.Lock()
	defer g.mu.Unlock()

	for i, b := range g.buttons {
		if b == button {
			g.buttons = append(g.buttons[:i], g.buttons[i+1:]...)
			button.group = nil
			if g.selected == button {
				g.selected = nil
			}
			break
		}
	}
}

// Buttons returns all buttons in the group.
func (g *RadioGroup) Buttons() []*RadioButton {
	g.mu.RLock()
	defer g.mu.RUnlock()
	result := make([]*RadioButton, len(g.buttons))
	copy(result, g.buttons)
	return result
}

// Selected returns the currently selected button.
func (g *RadioGroup) Selected() *RadioButton {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.selected
}

// selectButton selects a button and unselects others.
func (g *RadioGroup) selectButton(button *RadioButton) {
	g.mu.Lock()
	if g.selected == button {
		g.mu.Unlock()
		return
	}

	oldSelected := g.selected
	g.selected = button
	handler := g.onSelectionChanged
	g.mu.Unlock()

	// Update old button
	if oldSelected != nil {
		oldSelected.checked = false
		oldSelected.Update()
		if oldSelected.onToggled != nil {
			oldSelected.onToggled(false)
		}
	}

	// Update new button
	button.checked = true
	button.Update()
	if button.onToggled != nil {
		button.onToggled(true)
	}

	if handler != nil {
		handler(button)
	}
}

// SelectNext selects the next button in the group.
func (g *RadioGroup) SelectNext() {
	g.mu.RLock()
	buttons := g.buttons
	selected := g.selected
	g.mu.RUnlock()

	if len(buttons) == 0 {
		return
	}

	currentIdx := -1
	for i, b := range buttons {
		if b == selected {
			currentIdx = i
			break
		}
	}

	// Find next enabled button
	for i := 1; i <= len(buttons); i++ {
		nextIdx := (currentIdx + i) % len(buttons)
		if buttons[nextIdx].IsEnabled() {
			buttons[nextIdx].SetFocus()
			g.selectButton(buttons[nextIdx])
			return
		}
	}
}

// SelectPrevious selects the previous button in the group.
func (g *RadioGroup) SelectPrevious() {
	g.mu.RLock()
	buttons := g.buttons
	selected := g.selected
	g.mu.RUnlock()

	if len(buttons) == 0 {
		return
	}

	currentIdx := len(buttons)
	for i, b := range buttons {
		if b == selected {
			currentIdx = i
			break
		}
	}

	// Find previous enabled button
	for i := 1; i <= len(buttons); i++ {
		prevIdx := (currentIdx - i + len(buttons)) % len(buttons)
		if buttons[prevIdx].IsEnabled() {
			buttons[prevIdx].SetFocus()
			g.selectButton(buttons[prevIdx])
			return
		}
	}
}

// SetOnSelectionChanged sets the selection changed callback.
func (g *RadioGroup) SetOnSelectionChanged(handler func(*RadioButton)) {
	g.mu.Lock()
	g.onSelectionChanged = handler
	g.mu.Unlock()
}
