// Package widgets provides standard UI widgets for the TUI toolkit.
package widgets

import (
	"github.com/phroun/tuitk/core"
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

	// Keyboard navigation
	selectedIndex int // Currently selected entry when focused (-1 = none)

	// Focus transfer callback (called when Tab falls off either end)
	onFocusMenuBar func()
}

// NewDockRow creates a new dock row.
func NewDockRow() *DockRow {
	d := &DockRow{
		entryWidth:    16, // Default 16 chars per entry
		selectedIndex: -1,
	}
	d.WidgetBase = *core.NewWidgetBase()
	d.Init(d)
	d.SetFocusPolicy(core.StrongFocus)
	return d
}

// AddEntry adds a minimized window entry to the dock.
// The newly added entry becomes the selected item.
func (d *DockRow) AddEntry(entry *DockEntry) {
	d.entries = append(d.entries, entry)
	d.selectedIndex = len(d.entries) - 1
	d.Update()
}

// RemoveEntry removes an entry from the dock.
// After removal, the selection moves to the most recently added entry (last in list).
func (d *DockRow) RemoveEntry(entry *DockEntry) {
	for i, e := range d.entries {
		if e == entry {
			d.entries = append(d.entries[:i], d.entries[i+1:]...)
			// Select the last entry (most recently added)
			if len(d.entries) > 0 {
				d.selectedIndex = len(d.entries) - 1
			} else {
				d.selectedIndex = -1
			}
			d.Update()
			return
		}
	}
}

// RemoveEntryByTitle removes an entry by its title.
// After removal, the selection moves to the most recently added entry (last in list).
func (d *DockRow) RemoveEntryByTitle(title string) {
	for i, e := range d.entries {
		if e.Title == title {
			d.entries = append(d.entries[:i], d.entries[i+1:]...)
			// Select the last entry (most recently added)
			if len(d.entries) > 0 {
				d.selectedIndex = len(d.entries) - 1
			} else {
				d.selectedIndex = -1
			}
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

// SetOnFocusMenuBar sets the callback for when Tab navigation should transfer to the menu bar.
func (d *DockRow) SetOnFocusMenuBar(callback func()) {
	d.onFocusMenuBar = callback
}

// entriesPerRow returns how many entries fit per row based on current bounds.
func (d *DockRow) entriesPerRow() int {
	bounds := d.Bounds()
	metrics := d.EffectiveCellMetrics()
	entriesPerRow := int(bounds.Width / (core.Unit(d.entryWidth) * metrics.CellWidth))
	if entriesPerRow < 1 {
		entriesPerRow = 1
	}
	return entriesPerRow
}

// RowCount returns the number of rows needed to display all entries.
func (d *DockRow) RowCount() int {
	if len(d.entries) == 0 {
		return 0
	}

	bounds := d.Bounds()
	metrics := d.EffectiveCellMetrics()

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
	metrics := d.EffectiveCellMetrics()
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
	metrics := d.EffectiveCellMetrics()
	focused := d.HasFocus()
	scheme := d.GetScheme()

	// Dock styles
	dockStyle := scheme.GetDockItem()
	selectedStyle := scheme.GetFocusedDockItem()

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

		// Choose style based on selection
		entryStyle := dockStyle
		if focused && i == d.selectedIndex {
			entryStyle = selectedStyle
		}

		// Draw entry background (button-like)
		entryRect := core.UnitRect{
			X:      x,
			Y:      y,
			Width:  entryWidthUnits,
			Height: metrics.CellHeight,
		}
		p.FillRect(entryRect, ' ', entryStyle)

		// Draw border characters
		p.DrawCell(x, y, '[', entryStyle)
		p.DrawCell(x+entryWidthUnits-metrics.CellWidth, y, ']', entryStyle)

		// Draw title (truncated if needed)
		title := entry.Title
		maxWidth := d.entryWidth - 2 // Account for brackets
		if len(title) > maxWidth {
			title = title[:maxWidth-1] + "…"
		}

		textX := x + metrics.CellWidth
		for _, ch := range title {
			p.DrawCell(textX, y, ch, entryStyle)
			textX += metrics.CellWidth
		}
	}
}

// SelectedIndex returns the currently selected entry index (-1 if none).
func (d *DockRow) SelectedIndex() int {
	return d.selectedIndex
}

// SetSelectedIndex sets the selected entry index.
func (d *DockRow) SetSelectedIndex(index int) {
	if index < -1 {
		index = -1
	}
	if index >= len(d.entries) {
		index = len(d.entries) - 1
	}
	d.selectedIndex = index
	d.Update()
}

// HandleFocusIn is called when focus is gained.
func (d *DockRow) HandleFocusIn() {
	// Select first entry when gaining focus
	if len(d.entries) > 0 && d.selectedIndex < 0 {
		d.selectedIndex = 0
	}
	d.Update()
}

// HandleFocusOut is called when focus is lost.
func (d *DockRow) HandleFocusOut() {
	d.selectedIndex = -1
	d.Update()
}

// HandleKeyPress handles keyboard input.
func (d *DockRow) HandleKeyPress(event core.KeyPressEvent) bool {
	if len(d.entries) == 0 {
		return false
	}

	entriesPerRow := d.entriesPerRow()

	switch event.Key {
	case "Left":
		if d.selectedIndex > 0 {
			d.selectedIndex--
			d.Update()
		}
		return true

	case "Right":
		if d.selectedIndex < len(d.entries)-1 {
			d.selectedIndex++
			d.Update()
		}
		return true

	case "Up":
		// Move to same column in previous row
		if d.selectedIndex >= entriesPerRow {
			d.selectedIndex -= entriesPerRow
			d.Update()
		}
		return true

	case "Down":
		// Move to same column in next row
		newIndex := d.selectedIndex + entriesPerRow
		if newIndex < len(d.entries) {
			d.selectedIndex = newIndex
			d.Update()
		}
		return true

	case "Tab":
		if event.Modifiers&core.ShiftModifier != 0 {
			// Shift+Tab: move to previous item, or to menu bar if at start
			if d.selectedIndex > 0 {
				d.selectedIndex--
				d.Update()
			} else if d.onFocusMenuBar != nil {
				d.onFocusMenuBar()
			}
		} else {
			// Tab: move to next item, or to menu bar if at end
			if d.selectedIndex < len(d.entries)-1 {
				d.selectedIndex++
				d.Update()
			} else if d.onFocusMenuBar != nil {
				d.onFocusMenuBar()
			}
		}
		return true

	case "Home":
		d.selectedIndex = 0
		d.Update()
		return true

	case "End":
		d.selectedIndex = len(d.entries) - 1
		d.Update()
		return true

	case "Enter", " ", "Space":
		// Activate selected entry
		if d.selectedIndex >= 0 && d.selectedIndex < len(d.entries) {
			entry := d.entries[d.selectedIndex]
			if entry.OnClick != nil {
				entry.OnClick()
			}
		}
		return true
	}

	return false
}

// HandleMousePress handles mouse clicks.
func (d *DockRow) HandleMousePress(event core.MousePressEvent) bool {
	if len(d.entries) == 0 {
		return false
	}

	metrics := d.EffectiveCellMetrics()
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
