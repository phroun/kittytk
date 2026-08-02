//go:build sdl

package sdl

import (
	"fmt"

	"github.com/phroun/kittytk/backend/raster"
)

// Renderer is the interface for different rendering backends (software, WebGPU).
// Each renderer is responsible for:
// - Creating and managing render targets for windows
// - Presenting rendered content to the screen
// - Handling window resize/reshape operations
type Renderer interface {
	// Initialize sets up the renderer (called once before any windows)
	Initialize() error

	// Shutdown cleans up renderer resources
	Shutdown()

	// CreateWindowRenderer creates rendering resources for a new window
	CreateWindowRenderer(w *nativeWin, pxW, pxH int) error

	// DestroyWindowRenderer cleans up rendering resources for a window
	DestroyWindowRenderer(w *nativeWin)

	// ResizeWindowRenderer adjusts rendering resources when window size changes
	ResizeWindowRenderer(w *nativeWin, pxW, pxH int) error

	// Present renders the backend's pixel buffer to the window and displays it
	Present(w *nativeWin, backend *raster.Backend) error

	// ApplyWindowShape applies rounded corners to a window (for torn-off windows)
	// radiusPx is in device pixels, transparent indicates per-pixel alpha support
	ApplyWindowShape(w *nativeWin, radiusPx int, transparent bool) error

	// SetRotationEnabled toggles 2D rotation effects (no-op for software renderer)
	SetRotationEnabled(enabled bool)
	
	// SetWindowTransform sets the transform for a window (for compositing)
	SetWindowTransform(windowID uint32, translateX, translateY, rotation, scaleX, scaleY, opacity float32)

	// SupportsFeature checks if a feature is supported by this renderer
	SupportsFeature(feature RendererFeature) bool
}

// RendererFeature represents optional renderer capabilities
type RendererFeature int

const (
	FeatureRotation   RendererFeature = iota // 2D rotation transforms
	FeatureScale                              // 2D scaling transforms
	Feature3DCube                             // 3D rendering (demo cube)
	FeatureCompositing                        // GPU compositing with effects
)

// NewRenderer creates a renderer based on the requested type.
// Returns an error if the renderer type is unavailable (e.g., webgpu
// requested but not compiled in with -tags webgpu).
func NewRenderer(rendererType string, vsync bool) (Renderer, error) {
	switch rendererType {
	case "software", "":
		return NewSoftwareRenderer(vsync), nil
	case "webgpu":
		return NewWebGPURenderer(vsync)
	default:
		return nil, fmt.Errorf("unknown renderer type: %s (valid: software, webgpu)", rendererType)
	}
}
