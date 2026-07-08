package text

import (
	"image/color"
	"image/draw"

	gtrender "github.com/go-text/render"

	"github.com/phroun/tuitk/core"
)

// Render rasterizes a shaped paragraph onto dst with the paragraph's
// top-left corner at (x, y) in units, where one unit covers
// pxPerUnit x pxPerUnit device pixels (the raster backend's scale).
// Glyphs are rasterized from their outlines at the effective pixel
// size - scaling sharpens, it never upsamples.
//
// This is the only path from shaped output to pixels: backends and
// trinkets never touch the shaping library's types, so the
// implementation stays swappable (D6).
func Render(dst draw.Image, sp *ShapedParagraph, x, y core.Unit, pxPerUnit int, col color.Color) {
	if pxPerUnit < 1 {
		pxPerUnit = 1
	}
	r := gtrender.Renderer{
		PixScale: float32(pxPerUnit),
		Color:    col,
	}
	for i := range sp.Lines {
		line := &sp.Lines[i]
		baseY := (int(y) + int(line.Baseline)) * pxPerUnit
		for j := range line.Runs {
			run := &line.Runs[j]
			// The run's shaped Size is its em size in units; the
			// renderer wants it as FontSize with PixScale applied on
			// top, yielding crisp glyphs at the device resolution.
			r.FontSize = float32(run.raw.Size) / 64
			startX := (int(x) + int(run.X)) * pxPerUnit
			r.DrawShapedRunAt(run.raw, dst, startX, baseY)
		}
	}
}
