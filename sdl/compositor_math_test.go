package sdl

import (
	"image"
	"testing"

	"github.com/phroun/kittytk/core"
)

func almostEqual(a, b float32) bool {
	d := a - b
	return d < 1e-5 && d > -1e-5
}

// windowNDC places a child window's unit bounds on the surface's NDC
// quad. The full surface maps to the (-1,-1) 2x2 quad; sub-rects scale
// and translate, with the unit y axis (down) flipped to NDC (up).
func TestWindowNDC(t *testing.T) {
	full := core.UnitSize{Width: 800, Height: 600}

	x, y, w, h := windowNDC(core.UnitRect{X: 0, Y: 0, Width: 800, Height: 600}, full)
	if !almostEqual(x, -1) || !almostEqual(y, -1) || !almostEqual(w, 2) || !almostEqual(h, 2) {
		t.Errorf("fullscreen = (%v,%v,%v,%v), want (-1,-1,2,2)", x, y, w, h)
	}

	// Top-left quadrant: upper half of NDC, left half of x.
	x, y, w, h = windowNDC(core.UnitRect{X: 0, Y: 0, Width: 400, Height: 300}, full)
	if !almostEqual(x, -1) || !almostEqual(y, 0) || !almostEqual(w, 1) || !almostEqual(h, 1) {
		t.Errorf("top-left quadrant = (%v,%v,%v,%v), want (-1,0,1,1)", x, y, w, h)
	}

	// Bottom-right quadrant.
	x, y, w, h = windowNDC(core.UnitRect{X: 400, Y: 300, Width: 400, Height: 300}, full)
	if !almostEqual(x, 0) || !almostEqual(y, -1) || !almostEqual(w, 1) || !almostEqual(h, 1) {
		t.Errorf("bottom-right quadrant = (%v,%v,%v,%v), want (0,-1,1,1)", x, y, w, h)
	}

	// A degenerate surface produces a degenerate quad, not NaN/Inf.
	x, y, w, h = windowNDC(core.UnitRect{X: 10, Y: 10, Width: 100, Height: 100}, core.UnitSize{})
	if x != 0 || y != 0 || w != 0 || h != 0 {
		t.Errorf("degenerate surface = (%v,%v,%v,%v), want zeros", x, y, w, h)
	}
}

// The same window bounds against a RESIZED surface must produce different
// NDC — this is the transform the compositor now refreshes every frame.
// Regression coverage for windows keeping stale scale/position after the
// desktop window was resized.
func TestWindowNDCTracksSurfaceResize(t *testing.T) {
	bounds := core.UnitRect{X: 100, Y: 100, Width: 200, Height: 150}

	x1, y1, w1, h1 := windowNDC(bounds, core.UnitSize{Width: 800, Height: 600})
	x2, y2, w2, h2 := windowNDC(bounds, core.UnitSize{Width: 1600, Height: 1200})

	if almostEqual(x1, x2) && almostEqual(y1, y2) && almostEqual(w1, w2) && almostEqual(h1, h2) {
		t.Error("NDC must change when the surface size changes; a stale uniform means a mis-scaled window")
	}
	// Doubling the surface halves the window's NDC extent.
	if !almostEqual(w2, w1/2) || !almostEqual(h2, h1/2) {
		t.Errorf("doubled surface: extent (%v,%v), want (%v,%v)", w2, h2, w1/2, h1/2)
	}
}

func TestOutsetBounds(t *testing.T) {
	got := outsetBounds(core.UnitRect{X: 10, Y: 20, Width: 30, Height: 40}, 2)
	want := core.UnitRect{X: 8, Y: 18, Width: 34, Height: 44}
	if got != want {
		t.Errorf("outsetBounds = %+v, want %+v", got, want)
	}
}

// An overlay's texture must describe the same physical size as the
// outset quad it is drawn onto, at ANY pixel density — otherwise the
// GPU stretches it (distorted glyphs) and the painted outer stroke
// falls off the texture's right/bottom edge. Regression for the padding
// being applied in raw pixels while the quad outset was in units.
func TestOverlayTexturePxMatchesOutsetQuad(t *testing.T) {
	bounds := core.UnitRect{X: 40, Y: 30, Width: 75, Height: 42}
	const pad = core.Unit(2)

	for _, ppu := range []float64{1.0, 2.0, 1.1666666} {
		w, h, padPxW, padPxH := overlayTexturePx(bounds, ppu, ppu, pad)

		outset := outsetBounds(bounds, pad)
		wantW := int(float64(outset.Width)*ppu + 0.5)
		wantH := int(float64(outset.Height)*ppu + 0.5)

		// Within a pixel of the quad's physical size (independent
		// roundings), never the old pixels-vs-units mismatch that grew
		// with ppu.
		if diff := w - wantW; diff < -1 || diff > 1 {
			t.Errorf("ppu=%v: texture width %d vs quad width %d", ppu, w, wantW)
		}
		if diff := h - wantH; diff < -1 || diff > 1 {
			t.Errorf("ppu=%v: texture height %d vs quad height %d", ppu, h, wantH)
		}

		// The padding scales with density: at 2x, a 2-unit pad is 4px.
		wantPad := int(float64(pad)*ppu + 0.5)
		if padPxW != wantPad || padPxH != wantPad {
			t.Errorf("ppu=%v: padPx = (%d,%d), want %d", ppu, padPxW, padPxH, wantPad)
		}
	}
}

// scissorPx maps the client area (units) to a framebuffer scissor rect
// (pixels), scaled by density and clamped to the surface — the clip that
// keeps composited windows from painting over the status bar and dock.
func TestScissorPx(t *testing.T) {
	// A client area below a 20-unit menu bar and above an 80-unit
	// status/dock band, at 2x density on a 800x600-unit surface.
	area := core.UnitRect{X: 0, Y: 20, Width: 800, Height: 500}
	x, y, w, h, ok := scissorPx(area, 2, 2, 1600, 1200)
	if !ok || x != 0 || y != 40 || w != 1600 || h != 1000 {
		t.Errorf("scissor = (%d,%d %dx%d, ok=%v), want (0,40 1600x1000, true)", x, y, w, h, ok)
	}

	// Clamped to the surface even if the area overshoots.
	x, y, w, h, ok = scissorPx(core.UnitRect{X: -10, Y: -10, Width: 900, Height: 700}, 2, 2, 1600, 1200)
	if !ok || x != 0 || y != 0 || w != 1600 || h != 1200 {
		t.Errorf("overshoot clamp = (%d,%d %dx%d, ok=%v), want full surface", x, y, w, h, ok)
	}

	// A degenerate area reports not-ok rather than a zero-size scissor.
	if _, _, _, _, ok := scissorPx(core.UnitRect{X: 100, Y: 100}, 2, 2, 1600, 1200); ok {
		t.Error("empty area should not produce a scissor")
	}
}

// Shadow geometry: the quad covers the caster (plus anchor) shifted by
// the cast offset and outset by the blur; the SDF rects land inside it
// at pixel coordinates that keep the shadow visibly displaced.
func TestShadowQuadGeometry(t *testing.T) {
	caster := core.UnitRect{X: 100, Y: 100, Width: 200, Height: 150}
	anchor := core.UnitRect{X: 120, Y: 80, Width: 60, Height: 20}
	spec := shadowSpec{offsetX: 2, offsetY: 3, blur: 8, radius: 4, alpha: 0.35}

	// Union covers both rects.
	u := unionRect(caster, anchor)
	if u.X != 100 || u.Y != 80 || u.Width != 200 || u.Height != 170 {
		t.Errorf("union = %+v, want {100 80 200 170}", u)
	}
	// Empty rects are identities.
	if got := unionRect(caster, core.UnitRect{}); got != caster {
		t.Errorf("union with empty = %+v, want caster", got)
	}

	quad := shadowQuadBounds(caster, anchor, spec)
	want := core.UnitRect{X: 100 + 2 - 8, Y: 80 + 3 - 8, Width: 200 + 16, Height: 170 + 16}
	if quad != want {
		t.Errorf("shadow quad = %+v, want %+v", quad, want)
	}

	// The shifted caster maps into the quad with blur-sized margins at
	// density 2: its min corner sits blur*ppu inside on the axes the
	// union starts at.
	shifted := caster.Translated(spec.offsetX, spec.offsetY)
	minX, minY, maxX, maxY := rectPxIn(quad, shifted, 2, 2)
	if minX != 16 { // (caster.X+2 - quad.X) * 2 = blur*2
		t.Errorf("caster minX = %v, want 16", minX)
	}
	if minY != 56 { // caster is 20 units below the union top: (8+20)*2
		t.Errorf("caster minY = %v, want 56", minY)
	}
	if maxX-minX != 400 || maxY-minY != 300 {
		t.Errorf("caster extent = %vx%v px, want 400x300", maxX-minX, maxY-minY)
	}
}

// bgraPixels swaps R<->B, keeps G/A, and pads rows to the GPU's 256-byte
// upload alignment.
func TestBGRAPixels(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	// Pixel (0,0): R=1 G=2 B=3 A=4; pixel (2,1): R=9 G=8 B=7 A=255.
	copy(img.Pix[0:4], []byte{1, 2, 3, 4})
	i := img.PixOffset(2, 1)
	copy(img.Pix[i:i+4], []byte{9, 8, 7, 255})

	data, bytesPerRow := bgraPixels(img)

	if bytesPerRow != 256 {
		t.Errorf("bytesPerRow = %d, want 256 (3px rows round up to one alignment block)", bytesPerRow)
	}
	if len(data) != int(bytesPerRow)*2 {
		t.Errorf("len(data) = %d, want %d", len(data), int(bytesPerRow)*2)
	}
	if data[0] != 3 || data[1] != 2 || data[2] != 1 || data[3] != 4 {
		t.Errorf("pixel (0,0) BGRA = %v, want [3 2 1 4]", data[0:4])
	}
	j := bytesPerRow + 2*4
	if data[j] != 7 || data[j+1] != 8 || data[j+2] != 9 || data[j+3] != 255 {
		t.Errorf("pixel (2,1) BGRA = %v, want [7 8 9 255]", data[j:j+4])
	}
}

// A rounded window carries its shape in its own pixels: the corners are
// cleared (premultiplied, so every channel goes to zero — an
// alpha-only clear would leave an additive black fringe), the curve is
// antialiased, and the interior is untouched.
func TestPunchRoundedCorners(t *testing.T) {
	const w, h, r = 40, 30, 8
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i+0], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = 200, 100, 50, 255
	}

	punchRoundedCorners(img, r)

	at := func(x, y int) (uint8, uint8, uint8, uint8) {
		o := img.PixOffset(x, y)
		return img.Pix[o], img.Pix[o+1], img.Pix[o+2], img.Pix[o+3]
	}

	// Every extreme corner pixel is fully cleared, all channels.
	for _, pt := range [][2]int{{0, 0}, {w - 1, 0}, {0, h - 1}, {w - 1, h - 1}} {
		cr, cg, cb, ca := at(pt[0], pt[1])
		if cr|cg|cb|ca != 0 {
			t.Errorf("corner (%d,%d) = (%d,%d,%d,%d), want all zero", pt[0], pt[1], cr, cg, cb, ca)
		}
	}

	// The interior is untouched.
	if cr, cg, cb, ca := at(w/2, h/2); cr != 200 || cg != 100 || cb != 50 || ca != 255 {
		t.Errorf("center = (%d,%d,%d,%d), want the original color", cr, cg, cb, ca)
	}
	// Well inside the corner's arc is also untouched.
	if _, _, _, ca := at(r, r); ca != 255 {
		t.Errorf("inside the arc alpha = %d, want 255", ca)
	}

	// The curve is antialiased: at least one partially covered pixel.
	partial := false
	for j := 0; j < r; j++ {
		for i := 0; i < r; i++ {
			if _, _, _, a := at(i, j); a > 0 && a < 255 {
				partial = true
			}
		}
	}
	if !partial {
		t.Error("no partially covered pixels: the corner curve is not antialiased")
	}

	// A zero radius is a no-op.
	plain := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for i := range plain.Pix {
		plain.Pix[i] = 255
	}
	punchRoundedCorners(plain, 0)
	for i, v := range plain.Pix {
		if v != 255 {
			t.Fatalf("zero radius modified pixel %d", i)
		}
	}
}
