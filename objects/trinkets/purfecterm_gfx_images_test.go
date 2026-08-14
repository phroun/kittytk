package trinkets

import (
	"fmt"
	"image/color"
	"math"
	"strings"
	"testing"

	"github.com/phroun/kittytk/backend/raster"
	"github.com/phroun/kittytk/core"
	"github.com/phroun/kittytk/style"
)

// sixelSolidBlock builds a DCS Sixel sequence painting a solid w x h block in
// one color register. h must be a multiple of 6: a sixel carries six vertical
// pixels, so each band is one row of "~" (all six bits set), run-length encoded
// across the width. r, g and b are DEC percentages (0-100), as Sixel defines
// its RGB registers.
func sixelSolidBlock(w, h, r, g, b int) []byte {
	var s strings.Builder
	s.WriteString("\x1bP0;1;0q")              // P2=1: unset pixels stay transparent
	fmt.Fprintf(&s, "#0;2;%d;%d;%d", r, g, b) // define + select register 0 as RGB
	for band := 0; band < h/6; band++ {
		if band > 0 {
			s.WriteByte('-') // next band
		}
		fmt.Fprintf(&s, "!%d~", w)
	}
	s.WriteString("\x1b\\")
	return []byte(s.String())
}

// colorExtent returns the bounding box (inclusive) and count of framebuffer
// pixels exactly matching want.
func colorExtent(b *raster.Backend, want color.RGBA) (x0, y0, x1, y1, n int) {
	img := b.Image()
	x0, y0 = math.MaxInt32, math.MaxInt32
	x1, y1 = -1, -1
	for y := 0; y < img.Rect.Max.Y; y++ {
		for x := 0; x < img.Rect.Max.X; x++ {
			if img.RGBAAt(x, y) != want {
				continue
			}
			n++
			if x < x0 {
				x0 = x
			}
			if y < y0 {
				y0 = y
			}
			if x > x1 {
				x1 = x
			}
			if y > y1 {
				y1 = y
			}
		}
	}
	return
}

// A Sixel image placed by the emulator blits onto the graphical terminal at its
// anchor cell, one image pixel per device pixel. The decoder hands back a plain
// RGBA byte slice; the renderer wraps it and hands it to the painter, so any
// scaling, stride or premultiply mistake shows up as the wrong coverage or the
// wrong extent - not merely the wrong shade.
func TestGfxSixelImageBlit(t *testing.T) {
	t.Cleanup(func() { core.SetTextMeasurer(nil) })
	b, err := raster.New(640, 400)
	if err != nil {
		t.Fatal(err)
	}
	core.SetTextMeasurer(b)

	term := NewPurfecTerm()
	if term.Terminal() == nil {
		t.Skip("terminal unavailable")
	}
	term.SetBounds(core.UnitRect{Width: 640, Height: 400})
	// Settle the grid (and the cell pixel size the emulator anchors against)
	// before the image is placed.
	b.Clear(style.DefaultStyle())
	term.Paint(core.NewPainter(b))

	const imgW, imgH = 24, 24
	const anchorRow, anchorCol = 2, 3
	term.Feed([]byte(fmt.Sprintf("\x1b[%d;%dH", anchorRow+1, anchorCol+1)))
	term.Feed(sixelSolidBlock(imgW, imgH, 100, 0, 100))

	if got := len(term.Terminal().Buffer().GetImages()); got != 1 {
		t.Fatalf("emulator placed %d images, want 1", got)
	}

	b.Clear(style.DefaultStyle())
	term.Paint(core.NewPainter(b))

	magenta := color.RGBA{255, 0, 255, 255}
	x0, y0, x1, y1, n := colorExtent(b, magenta)
	if n == 0 {
		t.Fatal("sixel image did not render: no image pixels in the framebuffer")
	}
	if w, h := x1-x0+1, y1-y0+1; w != imgW || h != imgH {
		t.Errorf("image extent = %dx%d, want %dx%d (the blit is being scaled)", w, h, imgW, imgH)
	}
	if n != imgW*imgH {
		t.Errorf("image coverage = %d px, want %d (solid block, one image pixel per device pixel)", n, imgW*imgH)
	}

	// The anchor is the cell grid's, at the same scaled cell pitch the text
	// layer paints with - not the unit origin, and not the raw row/col.
	cwU, chU := term.cellDims()
	ppu := term.gfx.ppu
	wantX := int(math.Floor(float64(anchorCol*cwU) * ppu))
	wantY := int(math.Floor(float64(anchorRow*chU) * ppu))
	if x0 != wantX || y0 != wantY {
		t.Errorf("image anchored at (%d,%d) px, want (%d,%d) for cell (%d,%d) at %.3f px/unit",
			x0, y0, wantX, wantY, anchorCol, anchorRow, ppu)
	}
}

// A transparent Sixel pixel leaves the terminal background alone: the decoder
// emits alpha 0 for unset pixels and the blit must composite, not overwrite.
func TestGfxSixelImageTransparency(t *testing.T) {
	t.Cleanup(func() { core.SetTextMeasurer(nil) })
	b, err := raster.New(640, 400)
	if err != nil {
		t.Fatal(err)
	}
	core.SetTextMeasurer(b)

	term := NewPurfecTerm()
	if term.Terminal() == nil {
		t.Skip("terminal unavailable")
	}
	term.SetBounds(core.UnitRect{Width: 640, Height: 400})
	b.Clear(style.DefaultStyle())
	term.Paint(core.NewPainter(b))

	// Two bands, 12 px wide: the first painted, the second left unset.
	// "#0;2;0;100;0" defines register 0 as pure green.
	term.Feed([]byte("\x1b[1;1H"))
	term.Feed([]byte("\x1bP0;1;0q#0;2;0;100;0!12~-\x1b\\"))

	b.Clear(style.DefaultStyle())
	term.Paint(core.NewPainter(b))

	green := color.RGBA{0, 255, 0, 255}
	_, _, _, y1, n := colorExtent(b, green)
	if n != 12*6 {
		t.Errorf("painted band = %d px, want %d", n, 12*6)
	}
	if y1 != 5 {
		t.Errorf("painted band reaches y=%d, want 5: the unset band must stay transparent", y1)
	}
}
