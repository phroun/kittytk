//go:build sdl && !darwin

package sdl

import (
	"fmt"

	sdl2 "github.com/veandco/go-sdl2/sdl"
)

// reassertWindowAlpha is the non-macOS stub: no per-pixel window alpha
// arrangement to maintain (see the darwin build for what this does).
func reassertWindowAlpha(*sdl2.Window) {}

// roundWindowLayer is the non-macOS stub: no Core Animation layer to
// round, so callers fall back to SDL's shaped-window mask.
func roundWindowLayer(*sdl2.Window, int) bool { return false }

// nativeSurfaceHandles resolves the platform handles WebGPU surface
// creation needs for one SDL window. Off macOS the handles come from
// SDL's window-manager info: X11 wants (Display*, Window) and Windows
// wants (0, HWND). Wayland sessions reach here through XWayland; a
// pure-Wayland SDL window is reported unsupported rather than guessed at.
func nativeSurfaceHandles(win *sdl2.Window) (display, window uintptr, err error) {
	info, err := win.GetWMInfo()
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get window manager info: %w", err)
	}
	switch info.Subsystem {
	case sdl2.SYSWM_X11:
		x11 := info.GetX11Info()
		return uintptr(x11.Display), uintptr(x11.Window), nil
	case sdl2.SYSWM_WINDOWS:
		w := info.GetWindowsInfo()
		return 0, uintptr(w.Window), nil
	default:
		return 0, 0, fmt.Errorf("unsupported windowing subsystem %d for WebGPU surface", info.Subsystem)
	}
}
