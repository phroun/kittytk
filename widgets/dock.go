// Package widgets provides standard UI widgets for the TUI toolkit.
package widgets

import (
	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/style"
)

// DockEntry represents a minimized window in the dock.
type DockEntry struct {
	Title   string
	OnClick func()
}

// DockRow displays minimized windows as clickable buttons.
// It expands to multiple rows if needed and hides when empty.
type DockRow struct {
	core.WidgetBase

	// Minimized window entries
	entries []*DockEntry

	// Layout configuration
	entryWidth int // Width in characters per entry
}

// NewDockRow creates a new dock row.
func NewDockRow() *DockRow {
	d := &DockRow{
		entryWidth: 16, // Default 16 chars per entry
	}
	d.WidgetBase = *core.NewWidgetBase()
	d.Init(d)
	d.SetFocusPolicy(core.NoFocus)
	return d
}

// AddEntry adds a minimized window entry to the dock.
func (d *DockRow) AddEntry(entry *DockEntry) {
	d.entries = append(d.entries, entry)
	d.Update()
}

// RemoveEntry removes an entry from the dock.
func (d *DockRow) RemoveEntry(entry *DockEntry) {
	for i, e := range d.entries {
		if e == entry {
			d.entries = append(d.entries[:i], d.entries[i+1:]...)
			d.Update()
			return
		}
	}
}

// RemoveEntryByTitle removes an entry by its title.
func (d *DockRow) RemoveEntryByTitle(title string) {
	for i, e := range d.entries {
		if e.Title == title {
			d.entries = append(d.entries[:i], d.entries[i+1:]...)
			d.Update()
			return
		}
	}
}

// Clear removes all entries.
func (d *DockRow) Clear() {
	d.entries = nil
	d.Update()
}

// EntryCount returns the number of entries.
func (d *DockRow) EntryCount() int {
	return len(d.entries)
}

// IsEmpty returns true if the dock has no entries.
func (d *DockRow) IsEmpty() bool {
	return len(d.entries) == 0
}

// SetEntryWidth sets the width per entry in characters.
func (d *DockRow) SetEntryWidth(width int) {
	d.entryWidth = width
	d.Update()
}

// RowCount returns the number of rows needed to display all entries.
func (d *DockRow) RowCount() int {
	if len(d.entries) == 0 {
		return 0
	}

	bounds := d.Bounds()
	metrics := core.DefaultCellMetrics()

	// How many entries fit per row?
	entriesPerRow := int(bounds.Width / (core.Unit(d.entryWidth) * metrics.CellWidth))
	if entriesPerRow < 1 {
		entriesPerRow = 1
	}

	// Calculate rows needed
	rows := (len(d.entries) + entriesPerRow - 1) / entriesPerRow
	return rows
}

// RequiredHeight returns the height needed to display all entries.
func (d *DockRow) RequiredHeight() core.Unit {
	rows := d.RowCount()
	if rows == 0 {
		return 0
	}
	metrics := core.DefaultCellMetrics()
	return core.Unit(rows) * metrics.CellHeight
}

// SizeHint returns the preferred size.
func (d *DockRow) SizeHint() core.UnitSize {
	return core.UnitSize{
		Width:  0, // Will stretch to fill
		Height: d.RequiredHeight(),
	}
}

// Paint renders the dock row.
func (d *DockRow) Paint(p *core.Painter) {
	if len(d.entries) == 0 {
		return
	}

	bounds := d.Bounds()
	metrics := p.Metrics()

	// Dock style: cyan on blue
	dockStyle := style.DefaultStyle().WithFg(style.ColorBrightCyan).WithBg(style.ColorBlue)

	// Draw background
	p.FillRect(core.UnitRect{Width: bounds.Width, Height: bounds.Height}, ' ', dockStyle)

	// Calculate layout
	entryWidthUnits := core.Unit(d.entryWidth) * metrics.CellWidth
	entriesPerRow := int(bounds.Width / entryWidthUnits)
	if entriesPerRow < 1 {
		entriesPerRow = 1
	}

	// Draw entries
	for i, entry := range d.entries {
		row := i / entriesPerRow
		col := i % entriesPerRow

		x := core.Unit(col) * entryWidthUnits
		y := core.Unit(row) * metrics.CellHeight

		// Draw entry background (button-like)
		entryRect := core.UnitRect{
			X:      x,
			Y:      y,
			Width:  entryWidthUnits,
			Height: metrics.CellHeight,
		}
		p.FillRect(entryRect, ' ', dockStyle)

		// Draw border characters
		p.DrawCell(x, y, '[', dockStyle)
		p.DrawCell(x+entryWidthUnits-metrics.CellWidth, y, ']', dockStyle)

		// Draw title (truncated if needed)
		title := entry.Title
		maxWidth := d.entryWidth - 2 // Account for brackets
		if len(title) > maxWidth {
			title = title[:maxWidth-1] + "…"
		}

		textX := x + metrics.CellWidth
		for _, ch := range title {
			p.DrawCell(textX, y, ch, dockStyle)
			textX += metrics.CellWidth
		}
	}
}

// HandleMousePress handles mouse clicks.
func (d *DockRow) HandleMousePress(event core.MousePressEvent) bool {
	if len(d.entries) == 0 {
		return false
	}

	metrics := core.DefaultCellMetrics()
	bounds := d.Bounds()

	// Calculate which entry was clicked
	entryWidthUnits := core.Unit(d.entryWidth) * metrics.CellWidth
	entriesPerRow := int(bounds.Width / entryWidthUnits)
	if entriesPerRow < 1 {
		entriesPerRow = 1
	}

	row := int(event.Y / metrics.CellHeight)
	col := int(event.X / entryWidthUnits)

	index := row*entriesPerRow + col
	if index >= 0 && index < len(d.entries) {
		entry := d.entries[index]
		if entry.OnClick != nil {
			entry.OnClick()
		}
		return true
	}

	return false
}
