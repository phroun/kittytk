//go:build sdl && darwin

package sdl

/*
#cgo LDFLAGS: -framework Cocoa -framework QuartzCore -framework Metal
#cgo darwin CFLAGS: -I/opt/homebrew/include -I/usr/local/include
#cgo darwin LDFLAGS: -L/opt/homebrew/lib -L/usr/local/lib -lSDL2
#include <objc/runtime.h>
#include <objc/message.h>
#include <SDL2/SDL.h>
#include <SDL2/SDL_syswm.h>

// Forward declare SDL_Metal functions
typedef void* SDL_MetalView;
extern SDL_MetalView SDL_Metal_CreateView(SDL_Window* window);
extern void SDL_Metal_DestroyView(SDL_MetalView view);
extern void* SDL_Metal_GetLayer(SDL_MetalView view);

// kittytk_create_metal_view creates an SDL Metal view and returns its layer
static void* kittytk_create_metal_view(SDL_Window* window) {
	if (!window) {
		return NULL;
	}
	
	SDL_MetalView view = SDL_Metal_CreateView(window);
	if (!view) {
		return NULL;
	}
	
	void* layer = SDL_Metal_GetLayer(view);
	// Note: We don't destroy the view here because it needs to stay alive
	// The view will be destroyed when the window is destroyed
	return layer;
}

// kittytk_get_metal_layer retrieves the CAMetalLayer from an SDL Metal window.
// SDL creates a Metal view with a Metal layer when SDL_WINDOW_METAL flag is used.
static void* kittytk_get_metal_layer(void *nswindow) {
	if (!nswindow) {
		return NULL;
	}
	
	id win = (id)nswindow;
	id contentView = ((id (*)(id, SEL))objc_msgSend)(win, sel_registerName("contentView"));
	if (!contentView) {
		return NULL;
	}
	
	// SDL creates subviews for Metal rendering - find the SDL_metalview
	id subviews = ((id (*)(id, SEL))objc_msgSend)(contentView, sel_registerName("subviews"));
	if (!subviews) {
		return NULL;
	}
	
	unsigned long count = ((unsigned long (*)(id, SEL))objc_msgSend)(subviews, sel_registerName("count"));
	for (unsigned long i = 0; i < count; i++) {
		id subview = ((id (*)(id, SEL, unsigned long))objc_msgSend)(
			subviews, sel_registerName("objectAtIndex:"), i);
		if (!subview) continue;
		
		// Get the layer from this subview
		id layer = ((id (*)(id, SEL))objc_msgSend)(subview, sel_registerName("layer"));
		if (!layer) continue;
		
		// Check if it's a CAMetalLayer
		Class metalLayerClass = objc_getClass("CAMetalLayer");
		if (metalLayerClass && ((BOOL (*)(id, SEL, Class))objc_msgSend)(
			layer, sel_registerName("isKindOfClass:"), metalLayerClass)) {
			return (void*)layer;
		}
	}
	
	return NULL;
}

static void kittytk_layer_nonopaque(id layer) {
	if (!layer) {
		return;
	}
	((void (*)(id, SEL, signed char))objc_msgSend)(
		layer, sel_registerName("setOpaque:"), 0);
	id subs = ((id (*)(id, SEL))objc_msgSend)(layer, sel_registerName("sublayers"));
	if (subs) {
		unsigned long n = ((unsigned long (*)(id, SEL))objc_msgSend)(
			subs, sel_registerName("count"));
		unsigned long i;
		for (i = 0; i < n; i++) {
			id sub = ((id (*)(id, SEL, unsigned long))objc_msgSend)(
				subs, sel_registerName("objectAtIndex:"), i);
			((void (*)(id, SEL, signed char))objc_msgSend)(
				sub, sel_registerName("setOpaque:"), 0);
		}
	}
}

// kittytk_make_window_transparent flips an NSWindow (and every backing
// layer under its content view - SDL's Metal/GL surface lives on a
// SUBVIEW, not a sublayer) to non-opaque with a clear background so
// the framebuffer's alpha channel composites against whatever is
// behind the window. SDL2 has no portable per-pixel window alpha;
// this is the standard Cocoa-side arrangement for it.
static void kittytk_make_window_transparent(void *nswindow) {
	id win = (id)nswindow;
	if (!win) {
		return;
	}
	((void (*)(id, SEL, signed char))objc_msgSend)(
		win, sel_registerName("setOpaque:"), 0);
	id clear = ((id (*)(Class, SEL))objc_msgSend)(
		objc_getClass("NSColor"), sel_registerName("clearColor"));
	((void (*)(id, SEL, id))objc_msgSend)(
		win, sel_registerName("setBackgroundColor:"), clear);

	id view = ((id (*)(id, SEL))objc_msgSend)(win, sel_registerName("contentView"));
	if (!view) {
		return;
	}
	kittytk_layer_nonopaque(((id (*)(id, SEL))objc_msgSend)(view, sel_registerName("layer")));
	id subviews = ((id (*)(id, SEL))objc_msgSend)(view, sel_registerName("subviews"));
	if (subviews) {
		unsigned long n = ((unsigned long (*)(id, SEL))objc_msgSend)(
			subviews, sel_registerName("count"));
		unsigned long i;
		for (i = 0; i < n; i++) {
			id sv = ((id (*)(id, SEL, unsigned long))objc_msgSend)(
				subviews, sel_registerName("objectAtIndex:"), i);
			kittytk_layer_nonopaque(((id (*)(id, SEL))objc_msgSend)(sv, sel_registerName("layer")));
		}
	}
}

// kittytk_enable_miniaturize adds NSWindowStyleMaskMiniaturizable
// (1 << 2) to a borderless window's style mask: without it Cocoa
// silently refuses to miniaturize borderless windows, so torn-off
// windows couldn't go to the Dock.
static void kittytk_enable_miniaturize(void *nswindow) {
	id win = (id)nswindow;
	if (!win) {
		return;
	}
	unsigned long mask = ((unsigned long (*)(id, SEL))objc_msgSend)(
		win, sel_registerName("styleMask"));
	((void (*)(id, SEL, unsigned long))objc_msgSend)(
		win, sel_registerName("setStyleMask:"), mask|(1UL<<2));
}
*/
import "C"

import (
	"unsafe"

	sdl2 "github.com/veandco/go-sdl2/sdl"
)

// platformPerPixelAlpha: macOS composites per-pixel window alpha via
// the Cocoa shim, so rounded borderless surfaces skip SDL's shaped-
// window machinery entirely.
const platformPerPixelAlpha = true

// makeWindowTransparent enables per-pixel window alpha on macOS.
// Call after the renderer exists (its layer must be reachable).
func makeWindowTransparent(win *sdl2.Window) bool {
	cocoa := cocoaWindow(win)
	if cocoa == nil {
		return false
	}
	C.kittytk_make_window_transparent(cocoa)
	return true
}

// makeWindowMiniaturizable lets a borderless window go to the Dock.
func makeWindowMiniaturizable(win *sdl2.Window) {
	if cocoa := cocoaWindow(win); cocoa != nil {
		C.kittytk_enable_miniaturize(cocoa)
	}
}

func cocoaWindow(win *sdl2.Window) unsafe.Pointer {
	info, err := win.GetWMInfo()
	if err != nil {
		return nil
	}
	cocoa := info.GetCocoaInfo()
	if cocoa == nil {
		return nil
	}
	return cocoa.Window
}

// getMetalLayer retrieves the CAMetalLayer from an SDL Metal window
func getMetalLayer(win *sdl2.Window) unsafe.Pointer {
	// First try using SDL's Metal functions directly
	layer := C.kittytk_create_metal_view((*C.SDL_Window)(unsafe.Pointer(win)))
	if layer != nil {
		return layer
	}
	
	// Fallback: try to find an existing Metal layer
	cocoa := cocoaWindow(win)
	if cocoa == nil {
		return nil
	}
	return C.kittytk_get_metal_layer(cocoa)
}
