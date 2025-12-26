// Package widgets provides standard UI widgets for the TUI toolkit.
package widgets

import (
	"github.com/phroun/tuitk/core"
)

// Desktop represents the application desktop (background behind windows).
// It can optionally display a menu bar at the top (Mac-style) and a
// status bar at the bottom.
type Desktop struct {
	core.WidgetBase

	// Menu bar at the top (Mac-style)
	menuBar *MenuBar

	// Status bar at the bottom
	statusBar *StatusBar

	// Background pattern
	bgChar rune

	// Content area (shown behind windows but below menu/status)
	content core.Widget
}

// NewDesktop creates a new desktop widget.
func NewDesktop() *Desktop {
	d := &Desktop{
		bgChar: '░', // Default pattern
	}
	d.WidgetBase = *core.NewWidgetBase()
	d.SetFocusPolicy(core.NoFocus)
	return d
}

// Children returns all child widgets.
func (d *Desktop) Children() []core.Widget {
	var children []core.Widget
	if d.menuBar != nil {
		children = append(children, d.menuBar)
	}
	if d.content != nil {
		children = append(children, d.content)
	}
	if d.statusBar != nil {
		children = append(children, d.statusBar)
	}
	return children
}

// AddChild adds a child widget (sets it as content).
func (d *Desktop) AddChild(child core.Widget) {
	d.SetContent(child)
}

// RemoveChild removes a child widget.
func (d *Desktop) RemoveChild(child core.Widget) {
	if d.content == child {
		d.content = nil
	}
}

// ChildAt returns the child at the given position.
func (d *Desktop) ChildAt(pos core.UnitPoint) core.Widget {
	metrics := core.DefaultCellMetrics()
	bounds := d.Bounds()

	// Check menu bar
	if d.menuBar != nil && pos.Y < metrics.CellHeight {
		return d.menuBar
	}

	// Check status bar
	if d.statusBar != nil && pos.Y >= bounds.Height-metrics.CellHeight {
		return d.statusBar
	}

	// Check content
	if d.content != nil {
		clientArea := d.ClientArea()
		if pos.Y >= clientArea.Y && pos.Y < clientArea.Y+clientArea.Height {
			return d.content
		}
	}

	return nil
}

// Layout arranges children within the desktop.
func (d *Desktop) Layout() {
	d.layoutChildren()
}

// LayoutManager returns nil (desktop uses custom layout).
func (d *Desktop) LayoutManager() core.LayoutManager {
	return nil
}

// SetLayoutManager does nothing (desktop uses custom layout).
func (d *Desktop) SetLayoutManager(lm core.LayoutManager) {
	// Desktop uses custom layout, ignores layout manager
}

// SetMenuBar sets the menu bar (displayed at the top of the screen).
func (d *Desktop) SetMenuBar(menuBar *MenuBar) {
	d.menuBar = menuBar
	if menuBar != nil {
		menuBar.SetParent(d)
	}
	d.Update()
}

// MenuBar returns the menu bar.
func (d *Desktop) MenuBar() *MenuBar {
	return d.menuBar
}

// SetStatusBar sets the status bar (displayed at the bottom).
func (d *Desktop) SetStatusBar(statusBar *StatusBar) {
	d.statusBar = statusBar
	if statusBar != nil {
		statusBar.SetParent(d)
	}
	d.Update()
}

// StatusBar returns the status bar.
func (d *Desktop) StatusBar() *StatusBar {
	return d.statusBar
}

// SetBackgroundChar sets the background pattern character.
func (d *Desktop) SetBackgroundChar(ch rune) {
	d.bgChar = ch
	d.Update()
}

// BackgroundChar returns the background pattern character.
func (d *Desktop) BackgroundChar() rune {
	return d.bgChar
}

// SetContent sets the content widget (shown behind windows).
func (d *Desktop) SetContent(content core.Widget) {
	d.content = content
	if content != nil {
		content.SetParent(d)
	}
	d.Update()
}

// Content returns the content widget.
func (d *Desktop) Content() core.Widget {
	return d.content
}

// SetBounds sets the desktop bounds (typically the full screen).
func (d *Desktop) SetBounds(bounds core.UnitRect) {
	d.WidgetBase.SetBounds(bounds)
	d.layoutChildren()
}

// layoutChildren updates the bounds of menu bar, status bar, and content.
func (d *Desktop) layoutChildren() {
	bounds := d.Bounds()
	metrics := core.DefaultCellMetrics()

	// Menu bar at top
	if d.menuBar != nil {
		d.menuBar.SetBounds(core.UnitRect{
			X:      0,
			Y:      0,
			Width:  bounds.Width,
			Height: metrics.CellHeight,
		})
	}

	// Status bar at bottom
	if d.statusBar != nil {
		d.statusBar.SetBounds(core.UnitRect{
			X:      0,
			Y:      bounds.Height - metrics.CellHeight,
			Width:  bounds.Width,
			Height: metrics.CellHeight,
		})
	}

	// Content in the middle
	if d.content != nil {
		clientArea := d.ClientArea()
		d.content.SetBounds(core.UnitRect{
			X:      0,
			Y:      0,
			Width:  clientArea.Width,
			Height: clientArea.Height,
		})
	}
}

// ClientArea returns the area available for windows (excluding menu/status bars).
func (d *Desktop) ClientArea() core.UnitRect {
	bounds := d.Bounds()
	metrics := core.DefaultCellMetrics()

	top := core.Unit(0)
	bottom := bounds.Height

	if d.menuBar != nil {
		top = metrics.CellHeight
	}
	if d.statusBar != nil {
		bottom -= metrics.CellHeight
	}

	return core.UnitRect{
		X:      0,
		Y:      top,
		Width:  bounds.Width,
		Height: bottom - top,
	}
}

// MenuBarHeight returns the height of the menu bar area (0 if no menu bar).
func (d *Desktop) MenuBarHeight() core.Unit {
	if d.menuBar == nil {
		return 0
	}
	return core.DefaultCellMetrics().CellHeight
}

// StatusBarHeight returns the height of the status bar area (0 if no status bar).
func (d *Desktop) StatusBarHeight() core.Unit {
	if d.statusBar == nil {
		return 0
	}
	return core.DefaultCellMetrics().CellHeight
}

// SizeHint returns the preferred size.
func (d *Desktop) SizeHint() core.UnitSize {
	// Desktop fills available space
	return d.Bounds().Size()
}

// Paint renders the desktop.
func (d *Desktop) Paint(p *core.Painter) {
	bounds := d.Bounds()
	theme := d.Theme()
	metrics := p.Metrics()

	// Draw background pattern
	bgStyle := theme.Desktop
	for y := core.Unit(0); y < bounds.Height; y += metrics.CellHeight {
		for x := core.Unit(0); x < bounds.Width; x += metrics.CellWidth {
			p.DrawCell(x, y, d.bgChar, bgStyle)
		}
	}

	// Draw content if any
	if d.content != nil {
		clientArea := d.ClientArea()
		contentPainter := p.WithOffset(clientArea.X, clientArea.Y).
			WithClip(core.UnitRect{Width: clientArea.Width, Height: clientArea.Height})
		d.content.Paint(contentPainter)
	}

	// Draw menu bar at top
	if d.menuBar != nil {
		// Set menu bar bounds
		d.menuBar.SetBounds(core.UnitRect{
			X:      0,
			Y:      0,
			Width:  bounds.Width,
			Height: metrics.CellHeight,
		})
		d.menuBar.Paint(p)
	}

	// Draw status bar at bottom
	if d.statusBar != nil {
		y := bounds.Height - metrics.CellHeight
		d.statusBar.SetBounds(core.UnitRect{
			X:      0,
			Y:      y,
			Width:  bounds.Width,
			Height: metrics.CellHeight,
		})
		statusPainter := p.WithOffset(0, y)
		d.statusBar.Paint(statusPainter)
	}
}

// HandleKeyPress handles keyboard input.
func (d *Desktop) HandleKeyPress(event core.KeyPressEvent) bool {
	// Check if menu bar wants to handle Alt+key
	if d.menuBar != nil {
		// F10 or Alt key activates menu bar
		if event.Key == "F10" {
			d.menuBar.HandleKeyPress(event)
			return true
		}
		// Alt+letter for menu shortcuts
		if event.Modifiers&core.AltModifier != 0 && len(event.Key) == 1 {
			if d.menuBar.HandleKeyPress(event) {
				return true
			}
		}
		// If menu bar is active, forward all keys to it
		if d.menuBar.ActiveMenu() != nil || d.menuBar.HasFocus() {
			return d.menuBar.HandleKeyPress(event)
		}
	}

	// Forward to content
	if d.content != nil {
		return d.content.HandleKeyPress(event)
	}

	return false
}

// HandleMousePress handles mouse clicks.
func (d *Desktop) HandleMousePress(event core.MousePressEvent) bool {
	metrics := core.DefaultCellMetrics()

	// Check menu bar first
	if d.menuBar != nil && event.Y < metrics.CellHeight {
		return d.menuBar.HandleMousePress(event)
	}

	// Check status bar
	if d.statusBar != nil {
		bounds := d.Bounds()
		statusY := bounds.Height - metrics.CellHeight
		if event.Y >= statusY {
			localEvent := event
			localEvent.Y -= statusY
			return d.statusBar.HandleMousePress(localEvent)
		}
	}

	// Check content
	if d.content != nil {
		clientArea := d.ClientArea()
		if event.Y >= clientArea.Y && event.Y < clientArea.Y+clientArea.Height {
			localEvent := event
			localEvent.X -= clientArea.X
			localEvent.Y -= clientArea.Y
			return d.content.HandleMousePress(localEvent)
		}
	}

	return false
}

// StatusBar is a simple status bar widget.
type StatusBar struct {
	core.WidgetBase

	// Status sections
	sections []StatusSection
}

// StatusSection represents a section of the status bar.
type StatusSection struct {
	Text      string
	Width     int  // 0 = auto, -1 = stretch
	Alignment int  // 0 = left, 1 = center, 2 = right
}

// NewStatusBar creates a new status bar.
func NewStatusBar() *StatusBar {
	s := &StatusBar{}
	s.WidgetBase = *core.NewWidgetBase()
	s.SetFocusPolicy(core.NoFocus)
	return s
}

// SetText sets the main status text.
func (s *StatusBar) SetText(text string) {
	if len(s.sections) == 0 {
		s.sections = []StatusSection{{Text: text, Width: -1}}
	} else {
		s.sections[0].Text = text
	}
	s.Update()
}

// Text returns the main status text.
func (s *StatusBar) Text() string {
	if len(s.sections) == 0 {
		return ""
	}
	return s.sections[0].Text
}

// AddSection adds a section to the status bar.
func (s *StatusBar) AddSection(section StatusSection) {
	s.sections = append(s.sections, section)
	s.Update()
}

// SetSections sets all sections.
func (s *StatusBar) SetSections(sections []StatusSection) {
	s.sections = sections
	s.Update()
}

// Sections returns all sections.
func (s *StatusBar) Sections() []StatusSection {
	return s.sections
}

// SizeHint returns the preferred size.
func (s *StatusBar) SizeHint() core.UnitSize {
	metrics := core.DefaultCellMetrics()
	return core.UnitSize{
		Width:  0, // Will stretch to fill
		Height: metrics.CellHeight,
	}
}

// Paint renders the status bar.
func (s *StatusBar) Paint(p *core.Painter) {
	bounds := s.Bounds()
	theme := s.Theme()
	metrics := p.Metrics()

	// Draw background
	p.FillRect(core.UnitRect{Width: bounds.Width, Height: bounds.Height}, ' ', theme.StatusBar)

	// Draw sections
	x := core.Unit(0)
	for _, section := range s.sections {
		text := section.Text
		width := core.Unit(section.Width) * metrics.CellWidth
		if section.Width == -1 {
			// Stretch to remaining space
			width = bounds.Width - x
		} else if section.Width == 0 {
			// Auto width
			width = core.Unit(len(text)+2) * metrics.CellWidth
		}

		// Draw text
		textX := x + metrics.CellWidth
		for _, ch := range text {
			if textX >= x+width {
				break
			}
			p.DrawCell(textX, 0, ch, theme.StatusBar)
			textX += metrics.CellWidth
		}

		x += width
	}
}

// HandleMousePress handles mouse clicks.
func (s *StatusBar) HandleMousePress(event core.MousePressEvent) bool {
	// Status bar clicks could be used for section-specific actions
	return true
}
