//go:build sdl

package trinkets

import (
	"testing"

	"github.com/phroun/kittytk/core"
	"github.com/phroun/kittytk/raster"
	"github.com/phroun/kittytk/style"
)

// recBackend records the text draws and the caret draw so a test can check
// that the caret lands exactly where the glyphs paint. It wraps a real
// raster backend, so DrawText returns genuine shaped advances.
type recBackend struct {
	*raster.Backend
	texts  []recText
	caretX core.Unit
	caret  bool
}

type recText struct {
	x, adv core.Unit
	s      string
}

func (r *recBackend) DrawText(x, y core.Unit, s string, st style.CellStyle, f *core.Font) core.Unit {
	adv := r.Backend.DrawText(x, y, s, st, f)
	if s != "" {
		r.texts = append(r.texts, recText{x: x, adv: adv, s: s})
	}
	return adv
}

func (r *recBackend) DrawCaret(x, y, h core.Unit, st style.CellStyle) {
	r.caretX = x
	r.caret = true
	r.Backend.DrawCaret(x, y, h, st)
}

// At a fractional pixels-per-unit, the caret must sit exactly at the left
// edge of the text after the cursor: the render draws pre-cursor and
// post-cursor chunks, and the caret is the accumulated width between them.
// The old whole-string render placed glyphs at shaped sub-pixel positions
// while the caret came from a separate substring measurement, so the caret
// drifted right of the glyph boundary.
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
	// Two chunks: "Hello" at 0, "World" at the caret.
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
	if pre.x != 0 {
		t.Errorf("pre-cursor chunk drawn at x=%d, want 0", pre.x)
	}
	if pre.x+pre.adv != rec.caretX {
		t.Errorf("caret at %d, but the pre-cursor text ends at %d (drift)", rec.caretX, pre.x+pre.adv)
	}
	if post.x != rec.caretX {
		t.Errorf("post-cursor text drawn at x=%d, but the caret is at %d - they must coincide",
			post.x, rec.caretX)
	}
}
