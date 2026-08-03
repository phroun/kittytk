package raster

import (
	"image"
	"testing"

	"github.com/phroun/kittytk/core"
	"github.com/phroun/kittytk/style"
)

// checkerTile builds a w x h tile whose pixel (x,y) has red = x + y*w,
// so a test can tell exactly which texel landed where.
func checkerTile(w, h int) *image.RGBA {
	t := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			o := t.PixOffset(x, y)
			t.Pix[o+0] = uint8(x + y*w)
			t.Pix[o+3] = 255
		}
	}
	return t
}

// The tile repeats across the rect, anchored at the surface origin —
// the same anchoring the compositor's repeat sampler produces, so the
// software and GPU paths show the wallpaper in the same place.
func TestTileImagePxRepeatsFromOrigin(t *testing.T) {
	b, err := New(40, 24)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	tile := checkerTile(4, 4)
	b.TileImagePx(core.UnitRect{Width: 40, Height: 24}, tile)

	for _, pt := range [][2]int{{0, 0}, {3, 2}, {4, 0}, {7, 3}, {39, 23}, {17, 9}} {
		x, y := pt[0], pt[1]
		want := uint8((x % 4) + (y%4)*4)
		if got := lum(b, x, y); got != int(want) {
			t.Errorf("pixel (%d,%d) = %d, want %d (tile texel %d,%d)",
				x, y, got, want, x%4, y%4)
		}
	}
}

// A rect that is not a whole number of tiles across simply stops
// mid-tile — the last row and column are partial, exactly as a repeat
// sampler leaves them.
func TestTileImagePxHandlesPartialTiles(t *testing.T) {
	b, err := New(10, 6)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	b.Clear(style.CellStyle{Bg: style.RGB(255, 255, 255)})
	b.TileImagePx(core.UnitRect{Width: 10, Height: 6}, checkerTile(4, 4))

	// 10 is 2.5 tiles: the last column is texel column 1 of a third tile.
	if got, want := lum(b, 9, 0), 1; got != want {
		t.Errorf("pixel (9,0) = %d, want %d", got, want)
	}
	if got, want := lum(b, 5, 5), (1 + 1*4); got != want {
		t.Errorf("pixel (5,5) = %d, want %d", got, want)
	}
}

// Tiling respects the clip, so a wallpaper drawn into a clipped painter
// cannot spill past it.
func TestTileImagePxRespectsClip(t *testing.T) {
	b, err := New(40, 24)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	b.Clear(style.CellStyle{Bg: style.RGB(200, 200, 200)})
	b.SetClip(core.UnitRect{Width: 16, Height: 24})
	b.TileImagePx(core.UnitRect{Width: 40, Height: 24}, checkerTile(4, 4))

	if got := lum(b, 20, 5); got != 200 {
		t.Errorf("pixel outside the clip = %d, want the untouched 200", got)
	}
	if got := lum(b, 5, 5); got == 200 {
		t.Error("pixel inside the clip was not tiled")
	}
}

// A nil or empty tile draws nothing rather than panicking.
func TestTileImagePxDegenerate(t *testing.T) {
	b, err := New(16, 16)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	b.Clear(style.CellStyle{Bg: style.RGB(255, 255, 255)})
	b.TileImagePx(core.UnitRect{Width: 16, Height: 16}, image.NewRGBA(image.Rect(0, 0, 0, 0)))
	if got := lum(b, 8, 8); got != 255 {
		t.Errorf("pixel = %d, want 255 (nothing drawn for an empty tile)", got)
	}
}

// ClearTransparent resets the whole surface, ignoring the clip: the
// compositor's base layer starts this way so the wallpaper quad under it
// shows through everywhere the chrome does not paint. A clip-respecting
// clear would leave last frame's pixels outside it.
func TestClearTransparentIgnoresClip(t *testing.T) {
	b, err := New(32, 32)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	b.Clear(style.CellStyle{Bg: style.RGB(255, 255, 255)})
	b.SetClip(core.UnitRect{Width: 8, Height: 8})
	b.ClearTransparent()

	for _, pt := range [][2]int{{0, 0}, {4, 4}, {20, 20}, {31, 31}} {
		o := b.img.PixOffset(pt[0], pt[1])
		if a := b.img.Pix[o+3]; a != 0 {
			t.Errorf("pixel (%d,%d) alpha = %d, want 0", pt[0], pt[1], a)
		}
	}
}

// The backend advertises both capabilities the painter looks for.
func TestBackendImplementsWallpaperCapabilities(t *testing.T) {
	b, err := New(16, 16)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	var _ core.ImageTiler = b
	var _ core.SurfaceClearer = b
}
