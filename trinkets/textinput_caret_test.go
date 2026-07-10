//go:build sdl

package trinkets

import (
	"testing"

	"github.com/phroun/kittytk/core"
	"github.com/phroun/kittytk/raster"
	"github.com/phroun/kittytk/style"
)

// recBackend records the pixel-space text draws and the caret fill so a
// test can check that the caret lands exactly where the glyphs paint. It
// wraps a real raster backend, so DrawTextPx returns genuine rasterized
// pixel advances (the same widths the visible glyphs occupy).
type recBackend struct {
	*raster.Backend
	texts   []recText
	caretPx int
	caret   bool
}

type recText struct {
	xPx, advPx int
	s          string
}

func (r *recBackend) DrawTextPx(xPx, yPx int, s string, st style.CellStyle, f *core.Font) int {
	adv := r.Backend.DrawTextPx(xPx, yPx, s, st, f)
	if s != "" {
		r.texts = append(r.texts, recText{xPx: xPx, advPx: adv, s: s})
	}
	return adv
}

func (r *recBackend) FillRectPx(xPx, yPx, wPx, hPx int, st style.CellStyle) {
	// In this fixture the only device-pixel fill is the caret bar (the
	// focused speckled fill and any selection use whole-unit FillRect).
	r.caretPx = xPx
	r.caret = true
	r.Backend.FillRectPx(xPx, yPx, wPx, hPx, st)
}

// At a fractional pixels-per-unit, the caret must sit exactly at the left
// edge of the text after the cursor. The render draws the pre-cursor and
// post-cursor chunks by accumulating device-pixel advances from a single
// anchor, and the caret is that same accumulated pixel offset - so the bar,
// the end of the pre-cursor glyphs, and the start of the post-cursor glyphs
// all coincide. The old path measured the caret in units and re-snapped it
// through the (coarser) cell rate, so it drifted right of the glyphs, more
// the deeper into the text the cursor sat.
func TestTextInputCaretMatchesGlyphBoundary(t *testing.T) {
	t.Cleanup(func() { core.SetTextMeasurer(nil) })
	b, err := raster.NewScaled(600, 40, 2)
	if err != nil {
		t.Fatal(err)
	}
	b.SetFontSize(10) // fractional ppu (2*10/12)
	core.SetTextMeasurer(b)
	rec := &recBackend{Backend: b}

	ti := NewTextInput()
	ti.SetText("HelloWorld")
	ti.SetBounds(core.UnitRect{Width: 300, Height: b.LineHeight(ti.EffectiveFont())})
	ti.SetFocus()
	ti.SetCursorPosition(5) // between "Hello" and "World"

	b.Clear(style.DefaultStyle())
	ti.Paint(core.NewPainter(rec))

	if !rec.caret {
		t.Fatal("no caret was drawn")
	}
	// Two chunks: "Hello" at pixel 0, "World" at the caret pixel.
	var pre, post *recText
	for i := range rec.texts {
		switch rec.texts[i].s {
		case "Hello":
			pre = &rec.texts[i]
		case "World":
			post = &rec.texts[i]
		}
	}
	if pre == nil || post == nil {
		t.Fatalf("expected the text split into \"Hello\"+\"World\" chunks, got %+v", rec.texts)
	}
	if pre.xPx != 0 {
		t.Errorf("pre-cursor chunk drawn at xPx=%d, want 0", pre.xPx)
	}
	if pre.xPx+pre.advPx != rec.caretPx {
		t.Errorf("caret at %dpx, but the pre-cursor text ends at %dpx (drift)",
			rec.caretPx, pre.xPx+pre.advPx)
	}
	if post.xPx != rec.caretPx {
		t.Errorf("post-cursor text drawn at xPx=%d, but the caret is at %dpx - they must coincide",
			post.xPx, rec.caretPx)
	}
}
