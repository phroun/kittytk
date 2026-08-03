//go:build sdl

package sdlcompat

import (
	"runtime"
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

func init() {
	name := "libSDL3.so.0"
	switch runtime.GOOS {
	case "darwin":
		name = "libSDL3.dylib"
	case "windows":
		name = "SDL3.dll"
	}
	lib, err := purego.Dlopen(name, purego.RTLD_LAZY|purego.RTLD_GLOBAL)
	if err != nil {
		return // no SDL3 present; the binding's own load will report it
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
	if sdlMetalGetLayer == nil || view == 0 {
		return nil
	}
	return sdlMetalGetLayer(view)
}
