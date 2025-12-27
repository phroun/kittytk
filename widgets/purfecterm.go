// Package widgets provides standard UI widgets for the TUI toolkit.
package widgets

import (
	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/style"
)

// TODO: This widget requires github.com/phroun/purfecterm/cli which is not yet
// included in the published purfecterm module. Once the cli package is tagged,
// uncomment the full implementation below.

// PurfecTerm is a terminal emulator widget that embeds PurfecTerm's CLI adapter.
// It provides a fully functional terminal within the TUI application.
type PurfecTerm struct {
	core.WidgetBase
	core.AccessibleWidget

	// Cached size in cells
	cols, rows int
}

// NewPurfecTerm creates a new terminal emulator widget.
func NewPurfecTerm() *PurfecTerm {
	t := &PurfecTerm{
		cols: 80,
		rows: 24,
	}
	t.WidgetBase = *core.NewWidgetBase()
	t.Init(t)
	t.SetFocusPolicy(core.StrongFocus)
	t.SetAccessibleRole(core.RoleTerminal)
	return t
}

// Start starts the terminal with a shell.
// TODO: Implement when purfecterm/cli is available
func (t *PurfecTerm) Start() error {
	return nil
}

// StartCommand starts the terminal with a specific command.
// TODO: Implement when purfecterm/cli is available
func (t *PurfecTerm) StartCommand(name string, args ...string) error {
	return nil
}

// Close stops the terminal and cleans up resources.
func (t *PurfecTerm) Close() {
	// TODO: Implement when purfecterm/cli is available
}

// SizeHint returns the preferred size based on terminal dimensions.
func (t *PurfecTerm) SizeHint() core.UnitSize {
	metrics := core.DefaultCellMetrics()
	return core.UnitSize{
		Width:  metrics.TextWidth(t.cols),
		Height: metrics.TextHeight(t.rows),
	}
}

// Paint renders the terminal content.
func (t *PurfecTerm) Paint(p *core.Painter) {
	bounds := t.Bounds()
	theme := t.Theme()

	// Draw placeholder background
	p.FillRect(core.UnitRect{Width: bounds.Width, Height: bounds.Height}, ' ', theme.Normal)

	// Draw placeholder message
	metrics := p.Metrics()
	msg := "Terminal (awaiting purfecterm/cli)"
	x := (bounds.Width - metrics.TextWidth(len(msg))) / 2
	y := bounds.Height / 2
	msgStyle := style.DefaultStyle().WithFg(style.ColorWhite).WithBg(style.ColorBlack)
	for i, ch := range msg {
		p.DrawCell(x+metrics.CellToUnitsX(i), y, ch, msgStyle)
	}
}

// HandleKeyPress handles keyboard input.
func (t *PurfecTerm) HandleKeyPress(event core.KeyPressEvent) bool {
	// TODO: Forward to terminal when purfecterm/cli is available
	return false
}

// AccessibleInfo returns accessibility information.
func (t *PurfecTerm) AccessibleInfo() core.AccessibleInfo {
	info := t.AccessibleWidget.AccessibleInfo()
	info.Role = core.RoleTerminal
	info.Name = "Terminal"
	return info
}
