//go:build sdl

package sdlcompat

import (
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// Gap-filling: a few SDL3 entry points the binding registers internally
// but never exports. Rather than vendor or fork it, we bind them
// ourselves — dlopen is refcounted, so opening libSDL3 again hands back
// the handle the binding already loaded, and purego registers the
// symbol against it. Roughly the cost of a struct literal, and it keeps
// the dependency stock.

var sdlMetalGetLayer func(view uintptr) unsafe.Pointer
var gapfillOnce sync.Once

// bindGaps runs after Init has opened libSDL3.
func bindGaps() {
	// Reopen the library the binding already loaded. dlopen is
	// refcounted, so this hands back the same handle — but it has to
	// find the file by the same widened search, since Homebrew's
	// prefixes are not on dyld's default path.
	var lib uintptr
	for _, name := range libraryCandidates() {
		if h, err := purego.Dlopen(name, purego.RTLD_LAZY|purego.RTLD_GLOBAL); err == nil {
			lib = h
			break
		}
	}
	if lib == 0 {
		return // no SDL3 present; Init reports it
	}
	// Registration panics on a missing symbol, so probe first: an SDL3
	// built without the Metal backend simply has no layer to hand back.
	if sym, err := purego.Dlsym(lib, "SDL_Metal_GetLayer"); err == nil && sym != 0 {
		purego.RegisterFunc(&sdlMetalGetLayer, sym)
	}
}

// metalGetLayer returns the CAMetalLayer behind an SDL Metal view, or
// nil when this build has no Metal support.
func metalGetLayer(view uintptr) unsafe.Pointer {
	gapfillOnce.Do(bindGaps)
	if sdlMetalGetLayer == nil || view == 0 {
		return nil
	}
	return sdlMetalGetLayer(view)
}
