//go:build sdl && !darwin

package sdl

import (
	sdl2 "github.com/veandco/go-sdl2/sdl"
)

// makeWindowTransparent is the non-macOS stub: no per-pixel window
// alpha; the caller falls back to shaped windows (X11/Windows).
func makeWindowTransparent(*sdl2.Window) bool { return false }
