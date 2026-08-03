//go:build sdl

// Package sdlcompat presents the small slice of SDL that KittyTK's
// graphical host uses, implemented over SDL3.
//
// Why a shim rather than an in-place rewrite: the host's SDL surface is
// ~110 symbols concentrated in a few hundred lines of an otherwise
// SDL-agnostic 2000-line platform layer. Putting every SDL2/SDL3
// difference HERE keeps those differences reviewable in one file,
// makes the runtime switch atomic, and leaves the platform layer free
// to shed the SDL2-shaped names incrementally rather than in one
// high-risk sweep.
//
// The names follow SDL2's spelling because that is what the platform
// layer already calls; the behavior is SDL3's. The important thing
// SDL3 brings is WindowTransparent: a window whose framebuffer alpha
// actually composites, which SDL2 could not express at all and which
// no amount of Cocoa poking could retrofit.
package sdlcompat

import (
	"fmt"
	"os"
	"runtime"
	"unsafe"

	sdl3 "github.com/Zyko0/go-sdl3/sdl"
	"github.com/ebitengine/purego"
)

// --- init / lifecycle ---

const (
	INIT_VIDEO  = sdl3.INIT_VIDEO
	INIT_EVENTS = sdl3.INIT_EVENTS
)

// Init loads libSDL3 and initializes the requested subsystems. The
// binding is purego-based: nothing is linked at build time, so the
// library has to be opened before any SDL call — otherwise the first
// one dereferences an unregistered function pointer. Doing it here
// keeps that requirement from leaking into the platform layer.
func Init(flags sdl3.InitFlags) error {
	if err := loadLibrary(); err != nil {
		return err
	}
	return sdl3.Init(flags)
}

var libLoaded bool

// loadLibrary opens libSDL3, trying the plain library name first and
// then the usual install prefixes.
//
// The bare name alone is not enough: dyld searches /usr/lib and the
// dyld cache, but Homebrew installs to /opt/homebrew (Apple Silicon)
// or /usr/local (Intel), and neither is on the default search path. A
// correctly installed SDL3 would otherwise fail to load with a
// confusing "no such file". KITTYTK_SDL3 overrides the search outright
// for an SDL built somewhere else.
func loadLibrary() error {
	if libLoaded {
		return nil
	}

	// An embedded build carries its own SDL3 and never searches.
	if embeddedSDL {
		if err := loadEmbedded(); err != nil {
			return err
		}
		libLoaded = true
		return nil
	}

	candidates := libraryCandidates()

	var firstErr error
	for _, path := range candidates {
		if path == "" {
			continue
		}
		if err := sdl3.LoadLibrary(path); err == nil {
			libLoaded = true
			return nil
		} else if firstErr == nil {
			firstErr = err
		}
	}
	return fmt.Errorf("sdl3: could not load libSDL3 (tried %v): %w"+
		"\n\tinstall it (macOS: brew install sdl3) or set KITTYTK_SDL3 to its path",
		candidates, firstErr)
}

// libraryCandidates lists where libSDL3 might live, most specific
// first. dyld and ld.so search neither Homebrew prefix by default, so
// a correctly installed SDL3 is invisible to a bare library name.
func libraryCandidates() []string {
	var c []string
	if custom := os.Getenv("KITTYTK_SDL3"); custom != "" {
		c = append(c, custom)
	}
	c = append(c, sdl3.Path())
	switch runtime.GOOS {
	case "darwin":
		c = append(c,
			"/opt/homebrew/lib/libSDL3.dylib", // Homebrew, Apple Silicon
			"/opt/homebrew/opt/sdl3/lib/libSDL3.dylib",
			"/usr/local/lib/libSDL3.dylib", // Homebrew, Intel
			"/opt/local/lib/libSDL3.dylib", // MacPorts
		)
	case "linux":
		c = append(c,
			"libSDL3.so",
			"/usr/local/lib/libSDL3.so.0",
			"/usr/lib/x86_64-linux-gnu/libSDL3.so.0",
			"/usr/lib/aarch64-linux-gnu/libSDL3.so.0",
		)
	}
	return c
}
func Quit()                           { sdl3.Quit() }
func Delay(ms uint32)                 { sdl3.Delay(ms) }

func SetHint(name, value string) error { return sdl3.SetHint(name, value) }

// --- windows ---

// WINDOWPOS_CENTERED asks for a centered window. SDL3 keeps the same
// sentinel, but position is no longer a creation argument, so
// CreateWindow applies it after the window exists.
const WINDOWPOS_CENTERED = 0x2FFF0000

type WindowFlags = sdl3.WindowFlags

const (
	WINDOW_SHOWN      WindowFlags = 0 // SDL3 shows by default; HIDDEN is the opt-out
	WINDOW_HIDDEN                 = sdl3.WINDOW_HIDDEN
	WINDOW_RESIZABLE              = sdl3.WINDOW_RESIZABLE
	WINDOW_BORDERLESS             = sdl3.WINDOW_BORDERLESS
	WINDOW_MINIMIZED              = sdl3.WINDOW_MINIMIZED
	WINDOW_METAL                  = sdl3.WINDOW_METAL

	// WINDOW_TRANSPARENT is the SDL3-only flag this whole migration was
	// for: the window's framebuffer alpha composites with what is
	// behind it. It must be set at CREATION - it cannot be applied to
	// an existing window.
	WINDOW_TRANSPARENT = sdl3.WINDOW_TRANSPARENT
)

// Window wraps an SDL3 window with the method names the platform layer
// already uses.
type Window struct {
	w *sdl3.Window
}

// Raw exposes the underlying SDL3 window for code that needs the real
// thing (the macOS layer shim).
func (w *Window) Raw() *sdl3.Window { return w.w }

// CreateWindow creates a window at a position, SDL2-style. SDL3 dropped
// the position arguments, so they are applied immediately after.
func CreateWindow(title string, x, y int32, width, height int, flags WindowFlags) (*Window, error) {
	sw, err := sdl3.CreateWindow(title, width, height, flags)
	if err != nil {
		return nil, err
	}
	w := &Window{w: sw}
	if x != WINDOWPOS_CENTERED || y != WINDOWPOS_CENTERED {
		w.SetPosition(x, y)
	} else {
		w.SetPosition(WINDOWPOS_CENTERED, WINDOWPOS_CENTERED)
	}
	return w, nil
}

// CreateTransparentWindow creates a window whose framebuffer alpha
// composites — SDL3's replacement for SDL2's shaped windows, and the
// mechanism rounded corners now rely on.
func CreateTransparentWindow(title string, x, y int32, width, height int, flags WindowFlags) (*Window, error) {
	return CreateWindow(title, x, y, width, height, flags|WINDOW_TRANSPARENT)
}

func (w *Window) Destroy()               { w.w.Destroy() }
func (w *Window) SetTitle(title string)  { _ = w.w.SetTitle(title) }
func (w *Window) SetPosition(x, y int32) { _ = w.w.SetPosition(int32(x), int32(y)) }

func (w *Window) GetPosition() (int32, int32) {
	x, y, err := w.w.Position()
	if err != nil {
		return 0, 0
	}
	return x, y
}

func (w *Window) GetSize() (int32, int32) {
	width, height, err := w.w.Size()
	if err != nil {
		return 0, 0
	}
	return width, height
}

func (w *Window) SetSize(width, height int32) { _ = w.w.SetSize(width, height) }

func (w *Window) GetID() (uint32, error) {
	id, err := w.w.ID()
	return uint32(id), err
}

func (w *Window) GetFlags() WindowFlags       { return w.w.Flags() }
func (w *Window) Minimize()                   { _ = w.w.Minimize() }
func (w *Window) Restore()                    { _ = w.w.Restore() }
func (w *Window) Raise()                      { _ = w.w.Raise() }
func (w *Window) SetBordered(bordered bool)   { _ = w.w.SetBordered(bordered) }
func (w *Window) SetWindowOpacity(o float32) error { return w.w.SetOpacity(o) }

// GetDisplayIndex reports the display this window is on. SDL3 returns a
// DisplayID rather than an index; callers only use it to look the
// display's bounds back up, so the ID serves the same purpose.
func (w *Window) GetDisplayIndex() (int, error) {
	id := sdl3.GetDisplayForWindow(w.w)
	return int(id), nil
}

// --- displays ---

type Rect = sdl3.Rect

// GetDisplayUsableBounds returns the work area of a display (the screen
// minus the menu bar and Dock), keyed by the value GetDisplayIndex
// returned.
func GetDisplayUsableBounds(display int) (Rect, error) {
	r, err := sdl3.DisplayID(display).UsableBounds()
	if err != nil {
		return Rect{}, err
	}
	return *r, nil
}

// --- surfaces (shape masks) ---

type Surface = sdl3.Surface

const (
	PIXELFORMAT_ARGB8888 = sdl3.PIXELFORMAT_ARGB8888
	PIXELFORMAT_ABGR8888 = sdl3.PIXELFORMAT_ABGR8888
)

// CreateRGBSurfaceWithFormat keeps SDL2's name; SDL3 dropped the unused
// flags and depth arguments.
func CreateRGBSurfaceWithFormat(flags uint32, width, height, depth int32, format sdl3.PixelFormat) (*Surface, error) {
	return sdl3.CreateSurface(int(width), int(height), format)
}

// FreeSurface releases a surface. SDL3 renamed SDL_FreeSurface, so the
// host calls this rather than a method that no longer exists.
func FreeSurface(s *Surface) { s.Destroy() }

// SetShape applies an alpha mask to a transparent window. SDL3 requires
// the window to have been created with WINDOW_TRANSPARENT.
func (w *Window) SetShape(shape *Surface) error {
	return w.w.SetShape(shape)
}

// --- mouse / cursors ---

type Cursor = sdl3.Cursor
type SystemCursor = sdl3.SystemCursor

const (
	SYSTEM_CURSOR_ARROW    = sdl3.SYSTEM_CURSOR_DEFAULT
	SYSTEM_CURSOR_IBEAM    = sdl3.SYSTEM_CURSOR_TEXT
	SYSTEM_CURSOR_SIZEWE   = sdl3.SYSTEM_CURSOR_EW_RESIZE
	SYSTEM_CURSOR_SIZENS   = sdl3.SYSTEM_CURSOR_NS_RESIZE
	SYSTEM_CURSOR_SIZENWSE = sdl3.SYSTEM_CURSOR_NWSE_RESIZE
	SYSTEM_CURSOR_SIZENESW = sdl3.SYSTEM_CURSOR_NESW_RESIZE
)

func CreateSystemCursor(id SystemCursor) (*Cursor, error) { return sdl3.CreateSystemCursor(id) }
func SetCursor(c *Cursor) error                           { return sdl3.SetCursor(c) }
func CaptureMouse(enabled bool) error                     { return sdl3.CaptureMouse(enabled) }

const (
	BUTTON_LEFT   = 1
	BUTTON_MIDDLE = 2
	BUTTON_RIGHT  = 3

	// ButtonLMask is the held-buttons bit for the left button.
	ButtonLMask = 1 << 0
)

// GetGlobalMouseState reports the pointer in desktop coordinates. SDL3
// returns floats; the platform layer works in whole pixels.
func GetGlobalMouseState() (int32, int32, uint32) {
	state, x, y := sdl3.GetGlobalMouseState()
	return int32(x), int32(y), uint32(state)
}

// GetMouseState reports the pointer relative to the focused window.
func GetMouseState() (int32, int32, uint32) {
	state, x, y := sdl3.GetMouseState()
	return int32(x), int32(y), uint32(state)
}

// --- keyboard ---

type Keycode = sdl3.Keycode

// Keysym keeps SDL2's shape: SDL3 puts the key and modifiers directly
// on the event, so this is assembled when the event is translated.
type Keysym struct {
	Sym Keycode
	Mod uint16
}

const (
	KMOD_LSHIFT = uint16(sdl3.KMOD_LSHIFT)
	KMOD_SHIFT  = uint16(sdl3.KMOD_SHIFT)
	KMOD_LCTRL  = uint16(sdl3.KMOD_LCTRL)
	KMOD_CTRL   = uint16(sdl3.KMOD_CTRL)
	KMOD_LALT   = uint16(sdl3.KMOD_LALT)
	KMOD_ALT    = uint16(sdl3.KMOD_ALT)
	KMOD_LGUI   = uint16(sdl3.KMOD_LGUI)
	KMOD_RGUI   = uint16(sdl3.KMOD_RGUI)
	KMOD_GUI    = uint16(sdl3.KMOD_GUI)
)

func GetModState() uint16 { return uint16(sdl3.GetModState()) }

const (
	K_RETURN    = sdl3.K_RETURN
	K_ESCAPE    = sdl3.K_ESCAPE
	K_BACKSPACE = sdl3.K_BACKSPACE
	K_TAB       = sdl3.K_TAB
	K_DELETE    = sdl3.K_DELETE
	K_INSERT    = sdl3.K_INSERT
	K_HOME      = sdl3.K_HOME
	K_END       = sdl3.K_END
	K_PAGEUP    = sdl3.K_PAGEUP
	K_PAGEDOWN  = sdl3.K_PAGEDOWN
	K_UP        = sdl3.K_UP
	K_DOWN      = sdl3.K_DOWN
	K_LEFT      = sdl3.K_LEFT
	K_RIGHT     = sdl3.K_RIGHT
	K_EQUALS    = sdl3.K_EQUALS
	K_PLUS      = sdl3.K_PLUS
	K_MINUS     = sdl3.K_MINUS
	K_0         = sdl3.K_0
	K_a         = sdl3.K_A
	K_r         = sdl3.K_R
	K_KP_0      = sdl3.K_KP_0
	K_KP_PLUS   = sdl3.K_KP_PLUS
	K_KP_MINUS  = sdl3.K_KP_MINUS
	K_KP_ENTER  = sdl3.K_KP_ENTER
	K_F1        = sdl3.K_F1
	K_F2        = sdl3.K_F2
	K_F3        = sdl3.K_F3
	K_F4        = sdl3.K_F4
	K_F5        = sdl3.K_F5
	K_F6        = sdl3.K_F6
	K_F7        = sdl3.K_F7
	K_F8        = sdl3.K_F8
	K_F9        = sdl3.K_F9
	K_F10       = sdl3.K_F10
	K_F11       = sdl3.K_F11
	K_F12       = sdl3.K_F12
)

// StartTextInput enables text events. SDL3 scopes it to a window.
func StartTextInput(w *Window) error { return w.w.StartTextInput() }

// --- clipboard ---

func SetClipboardText(text string) error { return sdl3.SetClipboardText(text) }
func GetClipboardText() (string, error)  { return sdl3.GetClipboardText() }

// --- events ---
//
// SDL3 replaced SDL2's WINDOWEVENT-with-a-subtype scheme with distinct
// event types. The platform layer still switches on SDL2's shapes, so
// PollEvent translates: a resized/focus/leave event becomes a
// WindowEvent carrying the matching SDL2 subtype.

const (
	KEYDOWN         = 1
	KEYUP           = 2
	MOUSEBUTTONDOWN = 3

	WINDOWEVENT_SIZE_CHANGED = 1
	WINDOWEVENT_FOCUS_GAINED = 2
	WINDOWEVENT_FOCUS_LOST   = 3
	WINDOWEVENT_LEAVE        = 4
)

// Event is any translated SDL event.
type Event interface{ isEvent() }

type QuitEvent struct{}

type WindowEvent struct {
	WindowID     uint32
	Event        uint8
	Data1, Data2 int32
}

type KeyboardEvent struct {
	Type     uint32
	WindowID uint32
	Keysym   Keysym
}

type TextInputEvent struct {
	WindowID uint32
	text     string
}

func (e *TextInputEvent) GetText() string { return e.text }

type MouseButtonEvent struct {
	Type     uint32
	WindowID uint32
	Button   uint8
	State    uint8
	X, Y     int32
}

type MouseMotionEvent struct {
	WindowID uint32
	X, Y     int32
	State    uint32
}

type MouseWheelEvent struct {
	WindowID         uint32
	X, Y             int32
	PreciseX         float32
	PreciseY         float32
}

func (*QuitEvent) isEvent()        {}
func (*WindowEvent) isEvent()      {}
func (*KeyboardEvent) isEvent()    {}
func (*TextInputEvent) isEvent()   {}
func (*MouseButtonEvent) isEvent() {}
func (*MouseMotionEvent) isEvent() {}
func (*MouseWheelEvent) isEvent()  {}

// PollEvent returns the next translated event, or nil when the queue is
// empty.
func PollEvent() Event {
	var ev sdl3.Event
	if !sdl3.PollEvent(&ev) {
		return nil
	}
	return translate(&ev)
}

// translate maps one SDL3 event onto the SDL2-shaped value the platform
// layer switches on. Unhandled event types return nil, which the caller
// treats as "nothing for me" rather than "queue empty" — PollEvent's
// contract is preserved because the platform loop drains until nil and
// SDL3 delivers many more event types than SDL2 did.
func translate(ev *sdl3.Event) Event {
	switch ev.Type {
	case sdl3.EVENT_QUIT:
		return &QuitEvent{}

	case sdl3.EVENT_WINDOW_RESIZED, sdl3.EVENT_WINDOW_PIXEL_SIZE_CHANGED:
		w := ev.WindowEvent()
		return &WindowEvent{
			WindowID: uint32(w.WindowID),
			Event:    WINDOWEVENT_SIZE_CHANGED,
			Data1:    w.Data1,
			Data2:    w.Data2,
		}
	case sdl3.EVENT_WINDOW_FOCUS_GAINED:
		w := ev.WindowEvent()
		return &WindowEvent{WindowID: uint32(w.WindowID), Event: WINDOWEVENT_FOCUS_GAINED}
	case sdl3.EVENT_WINDOW_FOCUS_LOST:
		w := ev.WindowEvent()
		return &WindowEvent{WindowID: uint32(w.WindowID), Event: WINDOWEVENT_FOCUS_LOST}
	case sdl3.EVENT_WINDOW_MOUSE_LEAVE:
		w := ev.WindowEvent()
		return &WindowEvent{WindowID: uint32(w.WindowID), Event: WINDOWEVENT_LEAVE}

	case sdl3.EVENT_KEY_DOWN, sdl3.EVENT_KEY_UP:
		k := ev.KeyboardEvent()
		typ := uint32(KEYDOWN)
		if ev.Type == sdl3.EVENT_KEY_UP {
			typ = KEYUP
		}
		return &KeyboardEvent{
			Type:     typ,
			WindowID: uint32(k.WindowID),
			Keysym:   Keysym{Sym: k.Key, Mod: uint16(k.Mod)},
		}

	case sdl3.EVENT_TEXT_INPUT:
		t := ev.TextInputEvent()
		return &TextInputEvent{WindowID: uint32(t.WindowID), text: t.Text}

	case sdl3.EVENT_MOUSE_BUTTON_DOWN, sdl3.EVENT_MOUSE_BUTTON_UP:
		m := ev.MouseButtonEvent()
		state := uint8(0)
		if ev.Type == sdl3.EVENT_MOUSE_BUTTON_DOWN {
			state = 1
		}
		return &MouseButtonEvent{
			Type:     uint32(ev.Type),
			WindowID: uint32(m.WindowID),
			Button:   m.Button,
			State:    state,
			X:        int32(m.X),
			Y:        int32(m.Y),
		}

	case sdl3.EVENT_MOUSE_MOTION:
		m := ev.MouseMotionEvent()
		return &MouseMotionEvent{
			WindowID: uint32(m.WindowID),
			X:        int32(m.X),
			Y:        int32(m.Y),
			State:    uint32(m.State),
		}

	case sdl3.EVENT_MOUSE_WHEEL:
		m := ev.MouseWheelEvent()
		return &MouseWheelEvent{
			WindowID: uint32(m.WindowID),
			X:        int32(m.X),
			Y:        int32(m.Y),
			PreciseX: m.X,
			PreciseY: m.Y,
		}
	}
	return nil
}

// AddEventWatchFunc installs a callback run as events arrive, which is
// how the host keeps painting during macOS's modal resize loop.
func AddEventWatchFunc(fn func(Event, interface{}) bool, userdata interface{}) {
	// SDL3's event filter is a raw C function pointer, so the Go
	// callback has to be trampolined. purego.NewCallback allocates a
	// permanent trampoline, which suits a watch installed once for the
	// process's lifetime.
	cb := purego.NewCallback(func(_ uintptr, ev *sdl3.Event) uintptr {
		if translated := translate(ev); translated != nil {
			fn(translated, userdata)
		}
		return 1
	})
	_ = sdl3.AddEventWatch(sdl3.EventFilter(cb))
}

// --- renderer / textures ---
//
// SDL3 reworked this API: the renderer is created by driver NAME rather
// than by flags (vsync is a separate call), textures blit through
// RenderTexture with float rects, and "accelerated vs software" is a
// driver choice rather than a flag bit.

type Renderer struct {
	r *sdl3.Renderer
}

type Texture struct {
	t *sdl3.Texture
	w int32
	h int32
	f sdl3.PixelFormat
}

// Renderer creation flags kept for call-site compatibility. SDL3 has no
// flag word: ACCELERATED is the default driver, and vsync is applied
// separately, so these only carry intent.
const (
	RENDERER_ACCELERATED  = 1 << 0
	RENDERER_PRESENTVSYNC = 1 << 1
	RENDERER_SOFTWARE     = 1 << 2
)

const TEXTUREACCESS_STREAMING = sdl3.TEXTUREACCESS_STREAMING

// CreateRenderer picks a driver for the window. index is ignored (SDL2
// used -1 for "first supporting the flags"); PRESENTVSYNC in flags
// switches vsync on afterwards, and SOFTWARE asks for SDL3's "software"
// driver by name.
func CreateRenderer(w *Window, index int, flags uint32) (*Renderer, error) {
	name := ""
	if flags&RENDERER_SOFTWARE != 0 {
		name = "software"
	}
	r, err := w.w.CreateRenderer(name)
	if err != nil {
		return nil, err
	}
	if flags&RENDERER_PRESENTVSYNC != 0 {
		_ = r.SetVSync(1)
	}
	return &Renderer{r: r}, nil
}

func (r *Renderer) Destroy() { r.r.Destroy() }

func (r *Renderer) CreateTexture(format sdl3.PixelFormat, access sdl3.TextureAccess, w, h int32) (*Texture, error) {
	t, err := r.r.CreateTexture(format, access, int(w), int(h))
	if err != nil {
		return nil, err
	}
	return &Texture{t: t, w: w, h: h, f: format}, nil
}

func (r *Renderer) SetDrawColor(red, g, b, a uint8) error {
	return r.r.SetDrawColor(red, g, b, a)
}

func (r *Renderer) Clear() error   { return r.r.Clear() }
func (r *Renderer) Present() error { return r.r.Present() }

// Copy blits a whole texture over the whole render target — the only
// form the host uses. SDL3's rects are floats.
func (r *Renderer) Copy(t *Texture, src, dst *Rect) error {
	return r.r.RenderTexture(t.t, nil, nil)
}

func (t *Texture) Destroy() { t.t.Destroy() }

// Update uploads pixels into a streaming texture. The host passes the
// whole surface, so the rect is always nil.
func (t *Texture) Update(rect *Rect, pixels []byte, pitch int) error {
	return t.t.Update(nil, pixels, int32(pitch))
}

// Query reports the texture's creation parameters, SDL2-style. SDL3
// exposes size as floats and drops the combined query, so the values
// are remembered at creation.
func (t *Texture) Query() (uint32, int, int32, int32, error) {
	return uint32(t.f), int(sdl3.TEXTUREACCESS_STREAMING), t.w, t.h, nil
}

// --- native handles ---
//
// SDL3 replaced SDL_GetWindowWMInfo with typed window properties.

// MetalLayer returns the CAMetalLayer for a window, creating the
// backing Metal view on first use (macOS/iOS only; nil elsewhere).
func (w *Window) MetalLayer() unsafe.Pointer {
	view := w.w.Metal_CreateView()
	if view == 0 {
		return nil
	}
	return metalGetLayer(uintptr(view))
}

// CocoaWindow returns the NSWindow* for a window, or nil off macOS.
func (w *Window) CocoaWindow() unsafe.Pointer {
	props, err := w.w.Properties()
	if err != nil {
		return nil
	}
	return unsafe.Pointer(props.PointerProperty("SDL.window.cocoa.window", nil))
}

// X11Handles returns the X11 Display* and Window id, or (0,0) when the
// window is not an X11 window.
func (w *Window) X11Handles() (uintptr, uintptr) {
	props, err := w.w.Properties()
	if err != nil {
		return 0, 0
	}
	display := uintptr(unsafe.Pointer(props.PointerProperty("SDL.window.x11.display", nil)))
	window := uintptr(props.NumberProperty("SDL.window.x11.window", 0))
	return display, window
}

// Win32HWND returns the HWND for a window, or 0 off Windows.
func (w *Window) Win32HWND() uintptr {
	props, err := w.w.Properties()
	if err != nil {
		return 0
	}
	return uintptr(unsafe.Pointer(props.PointerProperty("SDL.window.win32.hwnd", nil)))
}
