// Package widgets provides standard UI widgets for the TUI toolkit.
package widgets

import (
	"github.com/phroun/tuitk/core"
)

// Spacer is a simple widget that takes up space in a layout.
// It can be used to add margins or gaps between other widgets.
type Spacer struct {
	core.WidgetBase

	// Size in cells (default 1x1)
	widthCells  int
	heightCells int
}

// NewSpacer creates a new spacer with default size of 1x1 cell.
func NewSpacer() *Spacer {
	s := &Spacer{
		widthCells:  1,
		heightCells: 1,
	}
	s.WidgetBase = *core.NewWidgetBase()
	s.Init(s)
	s.SetFocusPolicy(core.NoFocus)
	return s
}

// NewSpacerWithSize creates a new spacer with the specified size in cells.
func NewSpacerWithSize(widthCells, heightCells int) *Spacer {
	s := NewSpacer()
	s.widthCells = widthCells
	s.heightCells = heightCells
	return s
}

// SetSizeCells sets the spacer size in cells.
func (s *Spacer) SetSizeCells(widthCells, heightCells int) {
	s.widthCells = widthCells
	s.heightCells = heightCells
	s.Update()
}

// SizeHint returns the preferred size based on cell dimensions.
func (s *Spacer) SizeHint() core.UnitSize {
	metrics := core.DefaultCellMetrics()
	return core.UnitSize{
		Width:  core.Unit(s.widthCells) * metrics.CellWidth,
		Height: core.Unit(s.heightCells) * metrics.CellHeight,
	}
}

// Paint renders the spacer (which is invisible - just takes up space).
func (s *Spacer) Paint(p *core.Painter) {
	// Spacer is invisible - nothing to paint
}
