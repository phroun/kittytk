//go:build sdl

package sdl

import (
	"fmt"
	"unsafe"
	
	sdl2 "github.com/veandco/go-sdl2/sdl"

	"github.com/phroun/kittytk/backend/raster"
	"github.com/phroun/kittytk/platform"
)

// SoftwareRenderer implements CPU-based rasterization with SDL2's
// renderer and texture presentation. This is the traditional path:
// trinkets paint to a shared raster.Backend, then SDL blits it to screen.
type SoftwareRenderer struct {
	vsync bool
}

// NewSoftwareRenderer creates a CPU-based renderer
func NewSoftwareRenderer(vsync bool) *SoftwareRenderer {
	return &SoftwareRenderer{vsync: vsync}
}

// Initialize sets up the software renderer (no-op, SDL handles initialization)
func (r *SoftwareRenderer) Initialize() error {
	return nil
}

// Shutdown cleans up software renderer resources (no-op)
func (r *SoftwareRenderer) Shutdown() {
}

// CreateWindowRenderer creates SDL renderer and texture for a window
func (r *SoftwareRenderer) CreateWindowRenderer(w *nativeWin, pxW, pxH int) error {
	var flags uint32
	if r.vsync {
		flags = sdl2.RENDERER_ACCELERATED | sdl2.RENDERER_PRESENTVSYNC
	} else {
		flags = sdl2.RENDERER_ACCELERATED
	}

	renderer, err := sdl2.CreateRenderer(w.window, -1, flags)
	if err != nil {
		return err
	}
	w.renderer = renderer

	// Create streaming texture matching backend pixel format
	texture, err := renderer.CreateTexture(
		sdl2.PIXELFORMAT_ARGB8888,
		sdl2.TEXTUREACCESS_STREAMING,
		int32(pxW),
		int32(pxH),
	)
	if err != nil {
		renderer.Destroy()
		w.renderer = nil
		return err
	}
	w.texture = texture

	return nil
}

// DestroyWindowRenderer cleans up SDL renderer and texture
func (r *SoftwareRenderer) DestroyWindowRenderer(w *nativeWin) {
	if w.texture != nil {
		w.texture.Destroy()
		w.texture = nil
	}
	if w.renderer != nil {
		w.renderer.Destroy()
		w.renderer = nil
	}
}

// ResizeWindowRenderer recreates texture at new size
func (r *SoftwareRenderer) ResizeWindowRenderer(w *nativeWin, pxW, pxH int) error {
	if w.texture != nil {
		w.texture.Destroy()
	}

	texture, err := w.renderer.CreateTexture(
		sdl2.PIXELFORMAT_ARGB8888,
		sdl2.TEXTUREACCESS_STREAMING,
		int32(pxW),
		int32(pxH),
	)
	if err != nil {
		w.texture = nil
		return err
	}
	w.texture = texture
	return nil
}

// Present copies backend pixels to SDL texture and displays it
func (r *SoftwareRenderer) Present(w *nativeWin, backend *raster.Backend) error {
	if w.texture == nil || w.renderer == nil {
		return nil // Window not ready
	}

	// Get backend image
	img := backend.Image()

	// Update texture with backend pixels
	_ = w.texture.Update(nil, unsafe.Pointer(&img.Pix[0]), img.Stride)

	// Render to screen
	if w.transparent {
		// Alpha-0 clear for transparency (macOS per-pixel alpha)
		_ = w.renderer.SetDrawColor(0, 0, 0, 0)
	}
	w.renderer.Clear()
	w.renderer.Copy(w.texture, nil, nil)
	w.renderer.Present()

	return nil
}

// RenderFrame is not used by software renderer (uses Present directly)
func (r *SoftwareRenderer) RenderFrame(w *nativeWin, windows []*nativeWin, renderWindow func(*nativeWin)) error {
	// Software renderer doesn't do compositing - just calls Present per window
	return nil
}

// ApplyWindowShape applies rounded corners using SDL shape
func (r *SoftwareRenderer) ApplyWindowShape(w *nativeWin, radiusPx int, transparent bool) error {
	// Transparent windows don't need shape masks (macOS per-pixel alpha)
	if transparent {
		return nil

	}

	if radiusPx <= 0 {
		return nil
	}

	// TODO: Implement window shaping with rounded corners
	// This will be needed for torn-off windows with graphical frames
	// For now, just return success (no shaping applied)

	return nil
}

// SetRotationEnabled is a no-op for software renderer
func (r *SoftwareRenderer) SetRotationEnabled(enabled bool) {
	// Software renderer doesn't support rotation effects
}

// SetWindowTransform is a no-op for software renderer
func (r *SoftwareRenderer) SetWindowTransform(windowID uint32, translateX, translateY, rotation, scaleX, scaleY, opacity float32) {
	// Software renderer doesn't support per-window transforms
}

// SupportsFeature checks software renderer capabilities
func (r *SoftwareRenderer) SupportsFeature(feature RendererFeature) bool {
	// Software renderer doesn't support GPU features
	return false
}

// RenderFrameWithChildWindows is not implemented for software renderer.
// Software renderer uses the old direct rendering path.
func (r *SoftwareRenderer) RenderFrameWithChildWindows(w *nativeWin, childWindows *platform.ChildWindowList, scale int, renderWindow func(*nativeWin)) error {
	return fmt.Errorf("child window compositing not supported by software renderer")
}
