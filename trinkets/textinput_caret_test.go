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

// With no selection the whole text draws as ONE stable run (not split at
// the caret, which would re-shape the trailing text as the caret moves and
// make it jitter). The caret is measured from the prefix up to the cursor
// and overlaid, landing exactly on the glyph boundary at a fractional
// pixels-per-unit.
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
	font := ti.EffectiveFont()
	ti.SetBounds(core.UnitRect{Width: 300, Height: b.LineHeight(font)})
	ti.SetFocus()
	ti.SetCursorPosition(5) // between "Hello" and "World"

	b.Clear(style.DefaultStyle())
	p := core.NewPainter(rec)
	ti.Paint(p)

	if !rec.caret {
		t.Fatal("no caret was drawn")
	}
	// The text is drawn as a single un-split run, anchored at pixel 0.
	if len(rec.texts) != 1 || rec.texts[0].s != "HelloWorld" || rec.texts[0].xPx != 0 {
		t.Fatalf("expected one \"HelloWorld\" run at xPx=0, got %+v", rec.texts)
	}
	// The caret sits at the rasterized width of the prefix before the cursor.
	wantPx := p.UnitsToPx(font.MeasureText("Hello"))
	if rec.caretPx != wantPx {
		t.Errorf("caret at %dpx, want the \"Hello\" prefix width %dpx", rec.caretPx, wantPx)
	}
}

// Moving the caret must NOT change the drawn text (no selection): the run
// is not cut at the caret, so the material after it renders identically
// wherever the caret sits - it does not jitter as the caret sweeps through.
func TestTextInputTextStableAsCaretMoves(t *testing.T) {
	t.Cleanup(func() { core.SetTextMeasurer(nil) })
	b, err := raster.NewScaled(600, 40, 2)
	if err != nil {
		t.Fatal(err)
	}
	b.SetFontSize(10)
	core.SetTextMeasurer(b)

	ti := NewTextInput()
	ti.SetText("Hello World. Hello World.")
	ti.SetBounds(core.UnitRect{Width: 400, Height: b.LineHeight(ti.EffectiveFont())})
	ti.SetFocus()

	draw := func(cursor int) []recText {
		rec := &recBackend{Backend: b}
		ti.SetCursorPosition(cursor)
		b.Clear(style.DefaultStyle())
		ti.Paint(core.NewPainter(rec))
		return rec.texts
	}

	base := draw(3)
	for _, cursor := range []int{7, 12, 20, 25} {
		got := draw(cursor)
		if len(got) != len(base) {
			t.Fatalf("cursor %d: %d text runs, want %d (text was re-split at the caret)",
				cursor, len(got), len(base))
		}
		for i := range got {
			if got[i] != base[i] {
				t.Errorf("cursor %d: run %d = %+v, want %+v (text shifted with the caret)",
					cursor, i, got[i], base[i])
			}
		}
	}
}
