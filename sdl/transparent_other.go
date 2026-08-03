//go:build sdl && !darwin

package sdl

import (
	sdl2 "github.com/phroun/kittytk/sdl/sdlcompat"
)

// platformPerPixelAlpha: no per-pixel window alpha off macOS; rounded
// borderless surfaces fall back to SDL shaped windows (X11/Windows).
const platformPerPixelAlpha = false

// makeWindowTransparent is the non-macOS stub.
func makeWindowTransparent(*sdl2.Window) bool { return false }

// makeWindowMiniaturizable is the non-macOS stub: SDL's plain
// Minimize is the best available.
func makeWindowMiniaturizable(*sdl2.Window) {}
