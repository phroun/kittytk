package main

import (
	"regexp"
	"strconv"

	"github.com/phroun/kittytk/core"
)

// dimAttrRE matches the demo scripts' absolute-unit dimension attributes
// (NOT cell-count ones like fixedbox cols=, nor ratios like position=,
// nor data like value=/selected=). Kept to a whitelist so only real
// lengths are rescaled.
// Note: entry_width (dockrow) and cols (fixedbox) are CELL counts, not
// units, so they are deliberately absent - they scale themselves.
var dimAttrRE = regexp.MustCompile(`\b(?:width|height|min_width|min_height|max_width|max_height|spacing)=\d+`)

// denominate rescales a script's absolute-unit dimensions to the given
// denomination. The demo scripts are authored for the historical 8x16
// cell (16 units per row), so a literal "spacing=16" or "width=480"
// means a fixed pixel size, not a fixed number of rows/columns - at a
// different font_size it would occupy the wrong number of cells. Scaling
// every unit-valued dimension by CellHeight/16 (which equals CellWidth/8,
// the font ratio) keeps each value the same row/column count at any
// font_size. Cell-count attributes (fixedbox cols=) are left untouched.
//
// It is a no-op at the default 8x16, so scripts render byte-identically
// unless a non-default font_size is in play.
func denominate(script string, m core.CellMetrics) string {
	if m.CellHeight <= 0 || m.CellHeight == 16 {
		return script
	}
	scale := func(v int) int {
		s := (v*int(m.CellHeight) + 8) / 16 // round(v * CellHeight/16)
		if v > 0 && s < 1 {
			s = 1
		}
		return s
	}
	return dimAttrRE.ReplaceAllStringFunc(script, func(match string) string {
		for i := 0; i < len(match); i++ {
			if match[i] == '=' {
				if n, err := strconv.Atoi(match[i+1:]); err == nil {
					return match[:i+1] + strconv.Itoa(scale(n))
				}
				break
			}
		}
		return match
	})
}
