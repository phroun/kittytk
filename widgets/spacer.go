// Package widgets provides standard UI widgets for the TUI toolkit.
package widgets

import (
	"github.com/phroun/tuitk/core"
)

// Spacer is a simple widget that takes up space in a layout.
// It can be used to add margins or gaps between other widgets.
type Spacer struct {
	core.WidgetBase

	// Size in units
	size core.UnitSize
}

// NewSpacer creates a new spacer with a default size of 1x1 cell,
// resolved lazily against the effective grid metrics (a constructor
// has no parent chain to ask yet).
func NewSpacer() *Spacer {
	s := &Spacer{}
	s.WidgetBase = *core.NewWidgetBase()
	s.Init(s)
	s.SetFocusPolicy(core.NoFocus)
	return s
}

// NewSpacerWithSize creates a new spacer with the specified size in units.
func NewSpacerWithSize(size core.UnitSize) *Spacer {
	s := &Spacer{
		size: size,
	}
	s.WidgetBase = *core.NewWidgetBase()
	s.Init(s)
	s.SetFocusPolicy(core.NoFocus)
	return s
}

// SetSize sets the spacer size in units.
func (s *Spacer) SetSize(size core.UnitSize) {
	s.size = size
	s.Update()
}

// Size returns the spacer size in units.
func (s *Spacer) Size() core.UnitSize {
	return s.size
}

// SizeHint returns the preferred size. When no explicit size is set,
// it is one cell of the effective grid metrics, resolved at layout
// time when the parent chain exists.
func (s *Spacer) SizeHint() core.UnitSize {
	if s.size.Width > 0 || s.size.Height > 0 {
		return s.size
	}
	metrics := s.EffectiveCellMetrics()
	return core.UnitSize{Width: metrics.CellWidth, Height: metrics.CellHeight}
}

// Paint renders the spacer (which is invisible - just takes up space).
func (s *Spacer) Paint(p *core.Painter) {
	// Spacer is invisible - nothing to paint
}
