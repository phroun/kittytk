//go:build sdl && darwin

package sdl

/*
#cgo LDFLAGS: -framework Cocoa -framework QuartzCore -framework Metal
#cgo darwin CFLAGS: -I/opt/homebrew/include -I/usr/local/include
#cgo darwin LDFLAGS: -L/opt/homebrew/lib -L/usr/local/lib -lSDL2
#include <stdio.h>
#include <objc/runtime.h>
#include <objc/message.h>
#include <CoreGraphics/CoreGraphics.h>
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
	// An opaque backgroundColor defeats setOpaque:NO - drop it too.
	((void (*)(id, SEL, void*))objc_msgSend)(
		layer, sel_registerName("setBackgroundColor:"), NULL);
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
			((void (*)(id, SEL, void*))objc_msgSend)(
				sub, sel_registerName("setBackgroundColor:"), NULL);
		}
	}
}

// kittytk_noop_drawrect replaces SDL's content-view drawRect:. SDL2's
// SDLView fills every dirty rect with opaque BLACK (its white-flash
// guard) - and that fill sits BENEATH the Metal layer, so a transparent
// framebuffer composites against SDL's black instead of the desktop
// behind the window. Drawing nothing leaves those pixels genuinely
// clear.
static void kittytk_noop_drawrect(id self, SEL _cmd, CGRect rect) {
	(void)self; (void)_cmd; (void)rect;
}

// kittytk_view_clear_fill swaps ONE view onto a dynamic subclass whose
// drawRect: is a no-op (per-instance isa swizzle - other windows keep
// SDL's stock black-fill behavior), then forces a redisplay so any
// black already drawn is replaced by nothing.
static void kittytk_view_clear_fill(id view) {
	if (!view) {
		return;
	}
	Class cur = object_getClass(view);
	Class sub = objc_getClass("KittyTKClearFillView");
	if (!sub) {
		sub = objc_allocateClassPair(cur, "KittyTKClearFillView", 0);
		if (!sub) {
			return;
		}
		class_addMethod(sub, sel_registerName("drawRect:"),
			(IMP)kittytk_noop_drawrect, "v@:{CGRect={CGPoint=dd}{CGSize=dd}}");
		objc_registerClassPair(sub);
	}
	if (cur != sub && class_getSuperclass(sub) == cur) {
		object_setClass(view, sub);
	}
	((void (*)(id, SEL, signed char))objc_msgSend)(
		view, sel_registerName("setNeedsDisplay:"), 1);
}

// kittytk_debug_layer prints one layer's compositing-relevant state.
static void kittytk_debug_layer(const char *label, id layer) {
	if (!layer) {
		fprintf(stderr, "kittytk-alpha:   %s: no layer\n", label);
		return;
	}
	signed char op = ((signed char (*)(id, SEL))objc_msgSend)(
		layer, sel_registerName("isOpaque"));
	void *bg = ((void *(*)(id, SEL))objc_msgSend)(
		layer, sel_registerName("backgroundColor"));
	fprintf(stderr, "kittytk-alpha:   %s: layer=%s opaque=%d bg=%s\n",
		label, class_getName(object_getClass(layer)), (int)op, bg ? "SET" : "nil");
}

// kittytk_debug_window_alpha dumps the window/view/layer opacity chain
// to stderr (KITTYTK_ALPHA_DEBUG=1), before and after the transparency
// arrangement, so a failed transparent window can be diagnosed from a
// single run's output.
static void kittytk_debug_window_alpha(void *nswindow, int phase) {
	id win = (id)nswindow;
	if (!win) {
		return;
	}
	signed char wop = ((signed char (*)(id, SEL))objc_msgSend)(
		win, sel_registerName("isOpaque"));
	fprintf(stderr, "kittytk-alpha: %s: window opaque=%d\n",
		phase ? "after" : "before", (int)wop);
	id view = ((id (*)(id, SEL))objc_msgSend)(win, sel_registerName("contentView"));
	if (!view) {
		fprintf(stderr, "kittytk-alpha:   no contentView\n");
		return;
	}
	fprintf(stderr, "kittytk-alpha:   contentView=%s\n",
		class_getName(object_getClass(view)));
	kittytk_debug_layer("contentView", ((id (*)(id, SEL))objc_msgSend)(
		view, sel_registerName("layer")));
	id subviews = ((id (*)(id, SEL))objc_msgSend)(view, sel_registerName("subviews"));
	if (subviews) {
		unsigned long n = ((unsigned long (*)(id, SEL))objc_msgSend)(
			subviews, sel_registerName("count"));
		unsigned long i;
		for (i = 0; i < n; i++) {
			id sv = ((id (*)(id, SEL, unsigned long))objc_msgSend)(
				subviews, sel_registerName("objectAtIndex:"), i);
			fprintf(stderr, "kittytk-alpha:   subview[%lu]=%s\n",
				i, class_getName(object_getClass(sv)));
			kittytk_debug_layer("subview", ((id (*)(id, SEL))objc_msgSend)(
				sv, sel_registerName("layer")));
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
	kittytk_view_clear_fill(view);
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

// kittytk_reassert_layer_alpha re-applies non-opacity to a CAMetalLayer
// (and its window/view chain) directly. Configuring a surface calls
// setDrawableSize:, which "resets internal CAMetalLayer state on some
// macOS versions" (wgpu's own Metal HAL says so and re-applies its
// settings for the same reason) — and an opaque layer discards the
// alpha channel no matter what the renderer clears to. Cheap enough to
// run per present.
static void kittytk_reassert_layer_alpha(void *metalLayer, void *nswindow) {
	if (metalLayer) {
		id layer = (id)metalLayer;
		((void (*)(id, SEL, signed char))objc_msgSend)(
			layer, sel_registerName("setOpaque:"), 0);
		((void (*)(id, SEL, void*))objc_msgSend)(
			layer, sel_registerName("setBackgroundColor:"), NULL);
	}
	if (nswindow) {
		id win = (id)nswindow;
		((void (*)(id, SEL, signed char))objc_msgSend)(
			win, sel_registerName("setOpaque:"), 0);
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
	"os"
	"unsafe"

	sdl2 "github.com/veandco/go-sdl2/sdl"
)

// platformPerPixelAlpha: macOS composites per-pixel window alpha via
// the Cocoa shim, so rounded borderless surfaces skip SDL's shaped-
// window machinery entirely.
const platformPerPixelAlpha = true

// makeWindowTransparent enables per-pixel window alpha on macOS.
// Call after the renderer exists (its layer must be reachable).
// KITTYTK_ALPHA_DEBUG=1 dumps the window/view/layer opacity chain to
// stderr before and after the arrangement.
func makeWindowTransparent(win *sdl2.Window) bool {
	cocoa := cocoaWindow(win)
	if cocoa == nil {
		return false
	}
	debug := os.Getenv("KITTYTK_ALPHA_DEBUG") != ""
	if debug {
		C.kittytk_debug_window_alpha(cocoa, 0)
	}
	C.kittytk_make_window_transparent(cocoa)
	if debug {
		C.kittytk_debug_window_alpha(cocoa, 1)
	}
	return true
}

// reassertWindowAlpha re-applies non-opacity to a transparent window's
// Metal layer and NSWindow. Surface configuration (window creation,
// every resize) can reset CAMetalLayer state, and an opaque layer
// discards alpha regardless of what the renderer clears to — so the
// present path calls this each frame for transparent windows.
func reassertWindowAlpha(win *sdl2.Window) {
	if win == nil {
		return
	}
	cocoa := cocoaWindow(win)
	if cocoa == nil {
		return
	}
	// The non-creating lookup: find the existing Metal layer under the
	// window's content view (kittytk_create_metal_view would make a new
	// view every frame).
	C.kittytk_reassert_layer_alpha(C.kittytk_get_metal_layer(cocoa), cocoa)
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
