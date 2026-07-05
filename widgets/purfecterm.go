// Package widgets provides standard UI widgets for the TUI toolkit.
package widgets

import (
	"fmt"

	"github.com/phroun/purfecterm"
	"github.com/phroun/purfecterm/cli"
	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/style"
)

// PurfecTerm is a terminal emulator widget that embeds PurfecTerm's CLI adapter.
// It provides a fully functional terminal within the TUI application.
type PurfecTerm struct {
	core.WidgetBase
	core.AccessibleWidget

	// The underlying terminal emulator
	terminal *cli.Terminal

	// Cached size in cells
	cols, rows int

	// Track which mouse button is currently held for drag events
	heldButton core.MouseButton

	// Debug callback for cell inspection
	onCellClicked func(info CellDebugInfo)
}

// CellDebugInfo contains debug information about a clicked cell.
type CellDebugInfo struct {
	Col, Row  int
	Char      rune
	FgType    string
	FgR, FgG, FgB uint8
	FgIndex   uint8
	BgType    string
	BgR, BgG, BgB uint8
	BgIndex   uint8
	Bold      bool
	Underline bool
	Reverse   bool
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

	// Create terminal in embedded mode
	term, err := cli.New(cli.Options{
		Cols:           t.cols,
		Rows:           t.rows,
		ScrollbackSize: 1000,
		Embedded:       true,
	})
	if err != nil {
		// Terminal creation failed - widget will show error state
		return t
	}
	t.terminal = term

	// Set up callbacks
	t.terminal.SetOnBell(func() {
		// Could trigger a visual bell or notification
	})

	return t
}

// Start starts the terminal with a shell.
func (t *PurfecTerm) Start() error {
	if t.terminal == nil {
		return nil
	}
	return t.terminal.RunShell()
}

// SetOnCellClicked sets a callback for cell debug inspection.
// The callback receives detailed info about the clicked cell.
func (t *PurfecTerm) SetOnCellClicked(callback func(info CellDebugInfo)) {
	t.onCellClicked = callback
}

// StartCommand starts the terminal with a specific command.
func (t *PurfecTerm) StartCommand(name string, args ...string) error {
	if t.terminal == nil {
		return nil
	}
	return t.terminal.RunCommand(name, args...)
}

// Terminal returns the underlying cli.Terminal for advanced usage.
func (t *PurfecTerm) Terminal() *cli.Terminal {
	return t.terminal
}

// Close stops the terminal and cleans up resources.
func (t *PurfecTerm) Close() {
	if t.terminal != nil {
		t.terminal.Close()
	}
}

// SizeHint returns the preferred size based on terminal dimensions.
func (t *PurfecTerm) SizeHint() core.UnitSize {
	metrics := t.EffectiveCellMetrics()
	return core.UnitSize{
		Width:  metrics.TextWidth(t.cols),
		Height: metrics.TextHeight(t.rows),
	}
}

// SetBounds updates the widget bounds and resizes the terminal.
func (t *PurfecTerm) SetBounds(bounds core.UnitRect) {
	t.WidgetBase.SetBounds(bounds)
	t.updateTerminalSize()
}

// updateTerminalSize recalculates and applies the terminal size.
func (t *PurfecTerm) updateTerminalSize() {
	if t.terminal == nil {
		return
	}
	bounds := t.Bounds()
	metrics := t.EffectiveCellMetrics()

	newCols := int(bounds.Width / metrics.CellWidth)
	newRows := int(bounds.Height / metrics.CellHeight)

	if newCols > 0 && newRows > 0 && (newCols != t.cols || newRows != t.rows) {
		t.cols = newCols
		t.rows = newRows
		t.terminal.Resize(t.cols, t.rows)
	}
}

// Paint renders the terminal content.
func (t *PurfecTerm) Paint(p *core.Painter) {
	bounds := t.Bounds()
	metrics := t.EffectiveCellMetrics()
	theme := t.Theme()

	if t.terminal == nil {
		// Draw error state
		p.FillRect(core.UnitRect{Width: bounds.Width, Height: bounds.Height}, ' ', theme.Normal)
		return
	}

	// Get terminal cells
	cells := t.terminal.GetCells()

	// Render each cell
	for row, rowCells := range cells {
		y := metrics.CellToUnitsY(row)
		if y >= bounds.Height {
			break
		}

		for col, cell := range rowCells {
			x := metrics.CellToUnitsX(col)
			if x >= bounds.Width {
				break
			}

			// Convert purfecterm cell to tuitk style
			cellStyle := t.cellToStyle(cell)

			// Get the character (use space if empty)
			ch := cell.Char
			if ch == 0 {
				ch = ' '
			}

			p.DrawCell(x, y, ch, cellStyle)
		}
	}

	// Draw cursor if focused AND the terminal hasn't hidden its cursor
	// (some apps like vim/emacs manage their own cursor display)
	if t.HasFocus() && t.terminal.Buffer().IsCursorVisible() {
		cursorCol, cursorRow := t.terminal.Buffer().GetCursor()
		if cursorRow < len(cells) && cursorCol < t.cols {
			cursorX := metrics.CellToUnitsX(cursorCol)
			cursorY := metrics.CellToUnitsY(cursorRow)
			if cursorX < bounds.Width && cursorY < bounds.Height {
				// Draw cursor as reverse video
				var ch rune = ' '
				if cursorRow < len(cells) && cursorCol < len(cells[cursorRow]) {
					ch = cells[cursorRow][cursorCol].Char
					if ch == 0 {
						ch = ' '
					}
				}
				cursorStyle := style.DefaultStyle().
					WithFg(style.ColorBlack).
					WithBg(style.ColorWhite)
				p.DrawCell(cursorX, cursorY, ch, cursorStyle)
			}
		}
	}
}

// cellToStyle converts a purfecterm RenderedCell to a tuitk CellStyle.
func (t *PurfecTerm) cellToStyle(cell cli.RenderedCell) style.CellStyle {
	s := style.DefaultStyle()

	// Convert colors
	s = s.WithFg(t.convertColor(cell.Fg))
	s = s.WithBg(t.convertColor(cell.Bg))

	// Apply attributes
	if cell.Bold {
		s = s.Bold()
	}
	if cell.Underline {
		s = s.Underline()
	}
	if cell.Reverse {
		s = s.Reverse()
	}

	return s
}

// convertColor converts a purfecterm color to a tuitk color.
func (t *PurfecTerm) convertColor(c purfecterm.Color) style.Color {
	// Check color type
	switch c.Type {
	case purfecterm.ColorTypeTrueColor:
		return style.RGB(int(c.R), int(c.G), int(c.B))
	case purfecterm.ColorTypePalette:
		return style.Color256(int(c.Index))
	case purfecterm.ColorTypeStandard:
		// Basic 16 colors
		switch c.Index {
		case 0:
			return style.ColorBlack
		case 1:
			return style.ColorRed
		case 2:
			return style.ColorGreen
		case 3:
			return style.ColorYellow
		case 4:
			return style.ColorBlue
		case 5:
			return style.ColorMagenta
		case 6:
			return style.ColorCyan
		case 7:
			return style.ColorWhite
		case 8:
			return style.ColorBrightBlack
		case 9:
			return style.ColorBrightRed
		case 10:
			return style.ColorBrightGreen
		case 11:
			return style.ColorBrightYellow
		case 12:
			return style.ColorBrightBlue
		case 13:
			return style.ColorBrightMagenta
		case 14:
			return style.ColorBrightCyan
		case 15:
			return style.ColorBrightWhite
		}
	}
	return style.ColorDefault
}

// HandleKeyPress handles keyboard input and forwards to the terminal.
func (t *PurfecTerm) HandleKeyPress(event core.KeyPressEvent) bool {
	if t.terminal == nil {
		return false
	}

	// Ensure terminal knows it's focused before handling input
	t.terminal.SetFocused(true)

	// Forward the key to the terminal
	t.terminal.HandleKeyString(event.Key)
	t.Update()
	return true
}

// HandleMousePress handles mouse clicks to focus the terminal and forward to CLI.
func (t *PurfecTerm) HandleMousePress(event core.MousePressEvent) bool {
	t.SetFocus()
	if t.terminal == nil {
		return true
	}

	// Track held button for drag events
	t.heldButton = event.Button

	// Convert unit coordinates to cell coordinates
	metrics := t.EffectiveCellMetrics()
	cellCol := int(event.X / metrics.CellWidth)  // 0-based for internal use
	cellRow := int(event.Y / metrics.CellHeight) // 0-based for internal use

	// Debug callback - extract cell info
	if t.onCellClicked != nil {
		cells := t.terminal.GetCells()
		if cellRow < len(cells) && cellCol < len(cells[cellRow]) {
			cell := cells[cellRow][cellCol]
			info := CellDebugInfo{
				Col:       cellCol,
				Row:       cellRow,
				Char:      cell.Char,
				Bold:      cell.Bold,
				Underline: cell.Underline,
				Reverse:   cell.Reverse,
			}
			// Extract foreground color info
			switch cell.Fg.Type {
			case purfecterm.ColorTypeTrueColor:
				info.FgType = "RGB"
				info.FgR, info.FgG, info.FgB = cell.Fg.R, cell.Fg.G, cell.Fg.B
			case purfecterm.ColorTypePalette:
				info.FgType = "256"
				info.FgIndex = cell.Fg.Index
			case purfecterm.ColorTypeStandard:
				info.FgType = "Std"
				info.FgIndex = cell.Fg.Index
			default:
				info.FgType = "Def"
			}
			// Extract background color info
			switch cell.Bg.Type {
			case purfecterm.ColorTypeTrueColor:
				info.BgType = "RGB"
				info.BgR, info.BgG, info.BgB = cell.Bg.R, cell.Bg.G, cell.Bg.B
			case purfecterm.ColorTypePalette:
				info.BgType = "256"
				info.BgIndex = cell.Bg.Index
			case purfecterm.ColorTypeStandard:
				info.BgType = "Std"
				info.BgIndex = cell.Bg.Index
			default:
				info.BgType = "Def"
			}
			t.onCellClicked(info)
		}
	}

	// Convert to 1-based coordinates for CLI adapter
	cellX := cellCol + 1
	cellY := cellRow + 1

	// Send position update first
	t.terminal.HandleKeyString(fmt.Sprintf("Mouse@%d,%d", cellX, cellY))

	// Send button press
	var buttonStr string
	switch event.Button {
	case core.LeftButton:
		buttonStr = "MouseLeftPress"
	case core.MiddleButton:
		buttonStr = "MouseMiddlePress"
	case core.RightButton:
		buttonStr = "MouseRightPress"
	default:
		return true
	}
	t.terminal.HandleKeyString(buttonStr)
	t.Update()
	return true
}

// HandleMouseRelease handles mouse button releases.
func (t *PurfecTerm) HandleMouseRelease(event core.MouseReleaseEvent) bool {
	if t.terminal == nil {
		return false
	}

	// Clear held button
	if t.heldButton == event.Button {
		t.heldButton = core.NoButton
	}

	// Convert unit coordinates to 1-based cell coordinates
	metrics := t.EffectiveCellMetrics()
	cellX := int(event.X/metrics.CellWidth) + 1
	cellY := int(event.Y/metrics.CellHeight) + 1

	// Send position update first
	t.terminal.HandleKeyString(fmt.Sprintf("Mouse@%d,%d", cellX, cellY))

	// Send button release
	var buttonStr string
	switch event.Button {
	case core.LeftButton:
		buttonStr = "MouseLeftRelease"
	case core.MiddleButton:
		buttonStr = "MouseMiddleRelease"
	case core.RightButton:
		buttonStr = "MouseRightRelease"
	default:
		return false
	}
	t.terminal.HandleKeyString(buttonStr)
	t.Update()
	return true
}

// HandleMouseMove handles mouse movement/drag events.
func (t *PurfecTerm) HandleMouseMove(event core.MouseMoveEvent) bool {
	if t.terminal == nil {
		return false
	}

	// Convert unit coordinates to 1-based cell coordinates
	metrics := t.EffectiveCellMetrics()
	cellX := int(event.X/metrics.CellWidth) + 1
	cellY := int(event.Y/metrics.CellHeight) + 1

	// Use tracked button state for drag events (since event.Buttons may not be set)
	switch t.heldButton {
	case core.LeftButton:
		t.terminal.HandleKeyString(fmt.Sprintf("MouseLeftDrag@%d,%d", cellX, cellY))
	case core.MiddleButton:
		t.terminal.HandleKeyString(fmt.Sprintf("MouseMiddleDrag@%d,%d", cellX, cellY))
	case core.RightButton:
		t.terminal.HandleKeyString(fmt.Sprintf("MouseRightDrag@%d,%d", cellX, cellY))
	default:
		// Plain movement (for mouse tracking modes)
		t.terminal.HandleKeyString(fmt.Sprintf("Mouse@%d,%d", cellX, cellY))
	}
	t.Update()
	return true
}

// HandleMouseWheel handles scroll wheel events.
func (t *PurfecTerm) HandleMouseWheel(event core.MouseWheelEvent) bool {
	if t.terminal == nil {
		return false
	}

	// Convert unit coordinates to 1-based cell coordinates
	metrics := t.EffectiveCellMetrics()
	cellX := int(event.X/metrics.CellWidth) + 1
	cellY := int(event.Y/metrics.CellHeight) + 1

	// Send position update first
	t.terminal.HandleKeyString(fmt.Sprintf("Mouse@%d,%d", cellX, cellY))

	// Send scroll event based on direction
	if event.DeltaY < 0 {
		t.terminal.HandleKeyString("MouseScrollUp")
	} else if event.DeltaY > 0 {
		t.terminal.HandleKeyString("MouseScrollDown")
	}
	t.Update()
	return true
}

// HandleFocusIn is called when the widget gains focus.
func (t *PurfecTerm) HandleFocusIn() {
	t.WidgetBase.HandleFocusIn()
	if t.terminal != nil {
		t.terminal.SetFocused(true)
	}
	t.Update()
}

// HandleFocusOut is called when the widget loses focus.
func (t *PurfecTerm) HandleFocusOut() {
	t.WidgetBase.HandleFocusOut()
	if t.terminal != nil {
		t.terminal.SetFocused(false)
	}
	t.Update()
}

// Write sends data directly to the terminal.
func (t *PurfecTerm) Write(data []byte) (int, error) {
	if t.terminal == nil {
		return 0, nil
	}
	return t.terminal.Write(data)
}

// ScrollUp scrolls the terminal view up by n lines.
func (t *PurfecTerm) ScrollUp(n int) {
	if t.terminal != nil {
		t.terminal.ScrollUp(n)
		t.Update()
	}
}

// ScrollDown scrolls the terminal view down by n lines.
func (t *PurfecTerm) ScrollDown(n int) {
	if t.terminal != nil {
		t.terminal.ScrollDown(n)
		t.Update()
	}
}

// ScrollToTop scrolls to the top of the scrollback buffer.
func (t *PurfecTerm) ScrollToTop() {
	if t.terminal != nil {
		t.terminal.ScrollToTop()
		t.Update()
	}
}

// ScrollToBottom scrolls to the bottom (current output).
func (t *PurfecTerm) ScrollToBottom() {
	if t.terminal != nil {
		t.terminal.ScrollToBottom()
		t.Update()
	}
}

// AccessibleInfo returns accessibility information.
func (t *PurfecTerm) AccessibleInfo() core.AccessibleInfo {
	info := t.AccessibleWidget.AccessibleInfo()
	info.Role = core.RoleTerminal
	info.Name = "Terminal"
	return info
}
