package sdl

import (
	"image"

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
