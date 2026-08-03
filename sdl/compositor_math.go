package sdl

import (
	"image"
	"math"

	"github.com/phroun/kittytk/core"
)

// This file is intentionally free of build tags: it holds the pure math
// the GPU compositor rests on, so tests exercise it on any platform
// without SDL or a GPU.

// windowNDC maps a child window's unit bounds within a surface of the
// given unit size to the normalized-device-coordinate quad the blit
// shader draws: x/y is the quad's bottom-left corner, w/h its extent.
// NDC spans [-1,1] on both axes with +y up, while unit bounds have +y
// down from the top-left, hence the flip.
func windowNDC(bounds core.UnitRect, surfaceSize core.UnitSize) (x, y, w, h float32) {
	if surfaceSize.Width <= 0 || surfaceSize.Height <= 0 {
		return 0, 0, 0, 0
	}
	x = (float32(bounds.X)/float32(surfaceSize.Width))*2.0 - 1.0
	top := 1.0 - (float32(bounds.Y)/float32(surfaceSize.Height))*2.0
	w = (float32(bounds.Width) / float32(surfaceSize.Width)) * 2.0
	h = (float32(bounds.Height) / float32(surfaceSize.Height)) * 2.0
	y = top - h
	return x, y, w, h
}

// outsetBounds grows a rect by the stroke offset on every side — the
// overlay texture is padded so outer strokes drawn just outside the
// nominal bounds still land on it.
func outsetBounds(bounds core.UnitRect, offset core.Unit) core.UnitRect {
	return core.UnitRect{
		X:      bounds.X - offset,
		Y:      bounds.Y - offset,
		Width:  bounds.Width + offset*2,
		Height: bounds.Height + offset*2,
	}
}

// overlayTexturePx sizes an overlay layer's texture: the bounds mapped
// at the surface's pixels-per-unit, plus the stroke padding converted at
// the SAME density on every side. Texture pixels and the outset
// on-screen quad must describe the same physical size — padding the
// texture by raw pixels while outsetting the quad by units stretched
// the texture (distorted glyphs) and pushed the painted stroke off the
// texture's right/bottom edge at any scale above 1.
func overlayTexturePx(bounds core.UnitRect, ppuW, ppuH float64, pad core.Unit) (w, h, padPxW, padPxH int) {
	padPxW = int(math.Round(float64(pad) * ppuW))
	padPxH = int(math.Round(float64(pad) * ppuH))
	w = int(math.Round(float64(bounds.Width)*ppuW)) + padPxW*2
	h = int(math.Round(float64(bounds.Height)*ppuH)) + padPxH*2
	return w, h, padPxW, padPxH
}

// scissorPx maps a clip region in surface units to a framebuffer scissor
// rectangle in pixels, clamped to the surface. ok is false for an empty
// region (draw nothing) — callers treat a zero input rect as "no
// clipping" BEFORE calling this.
func scissorPx(area core.UnitRect, ppuW, ppuH float64, maxW, maxH int) (x, y, w, h int, ok bool) {
	x0 := int(math.Round(float64(area.X) * ppuW))
	y0 := int(math.Round(float64(area.Y) * ppuH))
	x1 := int(math.Round(float64(area.X+area.Width) * ppuW))
	y1 := int(math.Round(float64(area.Y+area.Height) * ppuH))
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > maxW {
		x1 = maxW
	}
	if y1 > maxH {
		y1 = maxH
	}
	if x1 <= x0 || y1 <= y0 {
		return 0, 0, 0, 0, false
	}
	return x0, y0, x1 - x0, y1 - y0, true
}

// shadowSpec describes one drop-shadow style, all lengths in surface
// units (so shadows scale with density like everything else).
type shadowSpec struct {
	offsetX core.Unit // cast down-right
	offsetY core.Unit
	blur    core.Unit // falloff distance around the caster
	radius  core.Unit // caster corner rounding
	alpha   float32   // peak opacity
}

// windowShadowSpec is the soft, larger shadow under desktop windows;
// overlayShadowSpec the tighter one under menus, popups, and combo
// lists.
var (
	windowShadowSpec  = shadowSpec{offsetX: 2, offsetY: 3, blur: 8, radius: 4, alpha: 0.35}
	overlayShadowSpec = shadowSpec{offsetX: 1, offsetY: 2, blur: 4, radius: 2, alpha: 0.40}
)

// unionRect returns the smallest rect containing both. An empty rect is
// the identity.
func unionRect(a, b core.UnitRect) core.UnitRect {
	if a.IsEmpty() {
		return b
	}
	if b.IsEmpty() {
		return a
	}
	x0, y0 := a.X, a.Y
	if b.X < x0 {
		x0 = b.X
	}
	if b.Y < y0 {
		y0 = b.Y
	}
	x1, y1 := a.X+a.Width, a.Y+a.Height
	if b.X+b.Width > x1 {
		x1 = b.X + b.Width
	}
	if b.Y+b.Height > y1 {
		y1 = b.Y + b.Height
	}
	return core.UnitRect{X: x0, Y: y0, Width: x1 - x0, Height: y1 - y0}
}

// shadowQuadBounds returns the on-screen quad a shadow needs: the caster
// (unioned with its anchor when present) shifted by the spec's offset
// and outset by the blur so the falloff has room on every side.
func shadowQuadBounds(caster, anchor core.UnitRect, spec shadowSpec) core.UnitRect {
	shape := unionRect(caster, anchor)
	shape.X += spec.offsetX
	shape.Y += spec.offsetY
	return outsetBounds(shape, spec.blur)
}

// rectPxIn maps a unit rect to pixel min/max coordinates relative to a
// quad's origin — the coordinates a shadow image is rasterized in.
func rectPxIn(quad, r core.UnitRect, ppuW, ppuH float64) (minX, minY, maxX, maxY float32) {
	minX = float32(float64(r.X-quad.X) * ppuW)
	minY = float32(float64(r.Y-quad.Y) * ppuH)
	maxX = float32(float64(r.X+r.Width-quad.X) * ppuW)
	maxY = float32(float64(r.Y+r.Height-quad.Y) * ppuH)
	return minX, minY, maxX, maxY
}

// shadowImage rasterizes a drop shadow into an RGBA image sized to the
// shadow quad: the caster rect (unioned with anchor when non-empty),
// shifted by the cast offset, rounded, then blurred. The pixels are
// black with the falloff in alpha, which the compositor's standard
// SrcAlpha blend lays over the scene as a shadow. Generated on the CPU
// and uploaded like every other layer — no bespoke GPU pipeline to go
// wrong.
func shadowImage(caster, anchor core.UnitRect, spec shadowSpec, ppuW, ppuH float64) *image.RGBA {
	quad := shadowQuadBounds(caster, anchor, spec)
	w := int(math.Round(float64(quad.Width) * ppuW))
	h := int(math.Round(float64(quad.Height) * ppuH))
	if w <= 0 || h <= 0 {
		return nil
	}

	// Hard mask of the (shifted) caster and anchor, with rounded corners.
	mask := make([]float64, w*h)
	radius := float64(spec.radius) * ppuW
	addRect := func(r core.UnitRect) {
		if r.IsEmpty() {
			return
		}
		minX, minY, maxX, maxY := rectPxIn(quad, r.Translated(spec.offsetX, spec.offsetY), ppuW, ppuH)
		for y := 0; y < h; y++ {
			py := float64(y) + 0.5
			for x := 0; x < w; x++ {
				px := float64(x) + 0.5
				if px < float64(minX) || px > float64(maxX) || py < float64(minY) || py > float64(maxY) {
					continue
				}
				// Corner rounding: reject pixels outside the quarter-circle
				// at whichever corner this pixel is nearest.
				if radius > 0 {
					dx := 0.0
					if cx := float64(minX) + radius; px < cx {
						dx = cx - px
					} else if cx := float64(maxX) - radius; px > cx {
						dx = px - cx
					}
					dy := 0.0
					if cy := float64(minY) + radius; py < cy {
						dy = cy - py
					} else if cy := float64(maxY) - radius; py > cy {
						dy = py - cy
					}
					if dx > 0 && dy > 0 && dx*dx+dy*dy > radius*radius {
						continue
					}
				}
				mask[y*w+x] = 1
			}
		}
	}
	addRect(caster)
	addRect(anchor)

	// Blur happens below; the anchor punch-out happens after it, so the
	// hole has hard edges that line up exactly with the control.

	// Separable box blur, three passes — a close Gaussian approximation.
	blurPx := int(math.Round(float64(spec.blur) * ppuW / 2))
	if blurPx > 0 {
		buf := make([]float64, w*h)
		for pass := 0; pass < 3; pass++ {
			boxBlurH(mask, buf, w, h, blurPx)
			boxBlurV(buf, mask, w, h, blurPx)
		}
	}

	// Punch the anchor's OWN rect (undisplaced) out of the shadow. The
	// opening control sits on a layer below the popup and is never
	// redrawn above the shadow, so without this the shadow's anchor lobe
	// darkens the very control it belongs to. Clearing it here — rather
	// than re-presenting the control over the shadow — also keeps each
	// layer owning its own pixels: the shadow never has to carry the
	// anchor's content, so live changes under it still show through.
	// The hole's edges coincide with the control's edges, so the cut is
	// invisible.
	if !anchor.IsEmpty() {
		minX, minY, maxX, maxY := rectPxIn(quad, anchor, ppuW, ppuH)
		for y := 0; y < h; y++ {
			py := float64(y) + 0.5
			if py < float64(minY) || py > float64(maxY) {
				continue
			}
			for x := 0; x < w; x++ {
				px := float64(x) + 0.5
				if px >= float64(minX) && px <= float64(maxX) {
					mask[y*w+x] = 0
				}
			}
		}
	}

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i, v := range mask {
		a := v * float64(spec.alpha)
		if a <= 0 {
			continue
		}
		if a > 1 {
			a = 1
		}
		// Black with the falloff in alpha; RGB stay 0.
		img.Pix[i*4+3] = uint8(a*255 + 0.5)
	}
	return img
}

// boxBlurH/boxBlurV are one separable box-blur pass each, with a running
// sum over a (2*radius+1) window and edge clamping.
func boxBlurH(src, dst []float64, w, h, radius int) {
	for y := 0; y < h; y++ {
		row := y * w
		sum := 0.0
		for i := -radius; i <= radius; i++ {
			sum += src[row+clampIdx(i, w)]
		}
		inv := 1.0 / float64(2*radius+1)
		for x := 0; x < w; x++ {
			dst[row+x] = sum * inv
			sum += src[row+clampIdx(x+radius+1, w)] - src[row+clampIdx(x-radius, w)]
		}
	}
}

func boxBlurV(src, dst []float64, w, h, radius int) {
	for x := 0; x < w; x++ {
		sum := 0.0
		for i := -radius; i <= radius; i++ {
			sum += src[clampIdx(i, h)*w+x]
		}
		inv := 1.0 / float64(2*radius+1)
		for y := 0; y < h; y++ {
			dst[y*w+x] = sum * inv
			sum += src[clampIdx(y+radius+1, h)*w+x] - src[clampIdx(y-radius, h)*w+x]
		}
	}
}

func clampIdx(i, n int) int {
	if i < 0 {
		return 0
	}
	if i >= n {
		return n - 1
	}
	return i
}

// punchRoundedCorners clears the pixels outside a rounded rectangle
// covering the whole image, with antialiased coverage on the curve.
//
// This is how a rounded window actually gets its shape: the pixels we
// present carry it. CAMetalLayer ignores cornerRadius/masksToBounds for
// its drawable (the drawable goes to the window server rather than
// through the layer's mask), and SDL's shaped-window API is gone in
// SDL3 — but a transparent corner in the framebuffer works on every
// renderer and every platform.
//
// image.RGBA is alpha-premultiplied, so every channel scales by the
// coverage: a cleared pixel is (0,0,0,0), which composites as nothing
// rather than as an additive black fringe.
func punchRoundedCorners(img *image.RGBA, radiusPx int) {
	if img == nil || radiusPx <= 0 {
		return
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	r := radiusPx
	if max := w / 2; r > max {
		r = max
	}
	if max := h / 2; r > max {
		r = max
	}
	if r <= 0 {
		return
	}

	rf := float64(r)
	for j := 0; j < r; j++ {
		for i := 0; i < r; i++ {
			// Distance from the corner circle's center to this pixel's
			// center; coverage ramps across the last pixel of the edge.
			dx := rf - float64(i) - 0.5
			dy := rf - float64(j) - 0.5
			d := math.Sqrt(dx*dx + dy*dy)
			cov := rf - d + 0.5
			if cov >= 1 {
				continue // fully inside
			}
			if cov < 0 {
				cov = 0
			}
			scale := func(x, y int) {
				o := img.PixOffset(b.Min.X+x, b.Min.Y+y)
				img.Pix[o+0] = uint8(float64(img.Pix[o+0])*cov + 0.5)
				img.Pix[o+1] = uint8(float64(img.Pix[o+1])*cov + 0.5)
				img.Pix[o+2] = uint8(float64(img.Pix[o+2])*cov + 0.5)
				img.Pix[o+3] = uint8(float64(img.Pix[o+3])*cov + 0.5)
			}
			scale(i, j)
			scale(w-1-i, j)
			scale(i, h-1-j)
			scale(w-1-i, h-1-j)
		}
	}
}

// gpuRowAlignment is WebGPU's required bytes-per-row alignment for
// texture uploads.
const gpuRowAlignment = 256

// bgraPixels converts an RGBA image to BGRA with rows padded to the GPU
// upload alignment, returning the pixel data and its bytes-per-row.
func bgraPixels(img *image.RGBA) (data []byte, bytesPerRow uint32) {
	bounds := img.Bounds()
	width := uint32(bounds.Dx())
	height := uint32(bounds.Dy())

	bytesPerRow = ((width*4 + gpuRowAlignment - 1) / gpuRowAlignment) * gpuRowAlignment
	data = make([]byte, bytesPerRow*height)

	for y := uint32(0); y < height; y++ {
		srcOffset := y * uint32(img.Stride)
		dstOffset := y * bytesPerRow
		for x := uint32(0); x < width; x++ {
			srcIdx := srcOffset + x*4
			dstIdx := dstOffset + x*4
			data[dstIdx+0] = img.Pix[srcIdx+2] // B
			data[dstIdx+1] = img.Pix[srcIdx+1] // G
			data[dstIdx+2] = img.Pix[srcIdx+0] // R
			data[dstIdx+3] = img.Pix[srcIdx+3] // A
		}
	}
	return data, bytesPerRow
}
