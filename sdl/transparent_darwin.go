//go:build sdl && darwin

package sdl

/*
#cgo LDFLAGS: -framework Cocoa -framework QuartzCore
#include <objc/runtime.h>
#include <objc/message.h>

// tuitk_make_window_transparent flips an NSWindow (and its backing
// layers) to non-opaque with a clear background so the framebuffer's
// alpha channel composites against whatever is behind the window.
// SDL2 has no portable per-pixel window alpha; this is the standard
// Cocoa-side arrangement for it.
static void tuitk_make_window_transparent(void *nswindow) {
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

	// The content view's backing layer - and SDL's Metal/GL sublayer
	// beneath it - must stop declaring themselves opaque too.
	id view = ((id (*)(id, SEL))objc_msgSend)(win, sel_registerName("contentView"));
	if (!view) {
		return;
	}
	id layer = ((id (*)(id, SEL))objc_msgSend)(view, sel_registerName("layer"));
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
*/
import "C"

import (
	sdl2 "github.com/veandco/go-sdl2/sdl"
)

// makeWindowTransparent enables per-pixel window alpha on macOS.
// Call after the renderer exists (its layer must be reachable).
// Returns false when the native window is unreachable; the caller
// falls back to shaped windows.
func makeWindowTransparent(win *sdl2.Window) bool {
	info, err := win.GetWMInfo()
	if err != nil {
		return false
	}
	cocoa := info.GetCocoaInfo()
	if cocoa == nil || cocoa.Window == nil {
		return false
	}
	C.tuitk_make_window_transparent(cocoa.Window)
	return true
}
