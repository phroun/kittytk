//go:build sdl

package sdl

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	sdl2 "github.com/veandco/go-sdl2/sdl"

	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/platform"
	"github.com/phroun/tuitk/raster"
)

// Platform runs tuitk in an SDL2 window: the raster backend paints,
// SDL presents and feeds input. All callbacks on the OS-locked main
// thread per D21.
type Platform struct {
	title    string
	wPx, hPx int
	scale    int // pixels per unit; see SetScale

	mu     sync.Mutex
	posts  []func()
	timers []timerEntry

	quitting atomic.Bool
	exitCode atomic.Int32

	window   *sdl2.Window
	renderer *sdl2.Renderer
	texture  *sdl2.Texture
	backend  *raster.Backend

	surface *sdlSurface
}

type timerEntry struct {
	due time.Time
	fn  func()
}

// New creates an SDL platform for one window of the given pixel size.
func New(title string, widthPx, heightPx int) *Platform {
	return &Platform{title: title, wPx: widthPx, hPx: heightPx, scale: 1}
}

// SetScale sets how many window pixels one abstract unit covers.
// The raster backend renders glyphs at the scaled size (crisp, not
// upsampled) and input coordinates are converted back to units. Call
// before Run/EnsureBackend. Stopgap until DPI-derived scaling lands.
func (p *Platform) SetScale(scale int) {
	if scale < 1 {
		scale = 1
	}
	p.scale = scale
}

// Backend returns the raster backend (valid after Run starts; used
// by embedders that must seed desktop metrics before RunOn).
func (p *Platform) Backend() *raster.Backend { return p.backend }

// EnsureBackend creates the framebuffer early (before Run) so
// Desktop.SetBackend can seed metrics from it.
func (p *Platform) EnsureBackend() (*raster.Backend, error) {
	if p.backend == nil {
		b, err := raster.NewScaled(p.wPx, p.hPx, p.scale)
		if err != nil {
			return nil, err
		}
		p.backend = b
	}
	return p.backend, nil
}

// Run implements platform.Platform.
func (p *Platform) Run(init func(platform.Platform)) int {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if err := sdl2.Init(sdl2.INIT_VIDEO | sdl2.INIT_EVENTS); err != nil {
		return 1
	}
	defer sdl2.Quit()

	var err error
	p.window, err = sdl2.CreateWindow(p.title,
		sdl2.WINDOWPOS_CENTERED, sdl2.WINDOWPOS_CENTERED,
		int32(p.wPx), int32(p.hPx),
		sdl2.WINDOW_SHOWN|sdl2.WINDOW_RESIZABLE)
	if err != nil {
		return 1
	}
	defer p.window.Destroy()

	p.renderer, err = sdl2.CreateRenderer(p.window, -1, sdl2.RENDERER_ACCELERATED|sdl2.RENDERER_PRESENTVSYNC)
	if err != nil {
		p.renderer, err = sdl2.CreateRenderer(p.window, -1, 0)
		if err != nil {
			return 1
		}
	}
	defer p.renderer.Destroy()

	if err := p.recreateFramebuffer(p.wPx, p.hPx); err != nil {
		return 1
	}
	defer func() {
		if p.texture != nil {
			p.texture.Destroy()
		}
	}()

	sdl2.StartTextInput()

	if init != nil {
		init(p)
	}

	for !p.quitting.Load() {
		p.drainPosts()
		p.fireDueTimers()
		if p.quitting.Load() {
			break
		}

		delivered := p.pumpEvents()

		if s := p.surface; s != nil && s.dirty.Swap(false) {
			p.paintAndPresent()
		}

		if !delivered {
			sdl2.Delay(5)
		}
	}
	return int(p.exitCode.Load())
}

// recreateFramebuffer sizes the raster backend and streaming texture.
func (p *Platform) recreateFramebuffer(wPx, hPx int) error {
	b, err := raster.NewScaled(wPx, hPx, p.scale)
	if err != nil {
		return err
	}
	p.backend = b
	p.wPx, p.hPx = wPx, hPx

	if p.texture != nil {
		p.texture.Destroy()
	}
	// Go's image.RGBA stores bytes R,G,B,A; on little-endian that is
	// SDL's ABGR8888 packed format.
	p.texture, err = p.renderer.CreateTexture(
		sdl2.PIXELFORMAT_ABGR8888, sdl2.TEXTUREACCESS_STREAMING,
		int32(wPx), int32(hPx))
	return err
}

// paintAndPresent runs the handler frame into the raster backend and
// blits it.
func (p *Platform) paintAndPresent() {
	s := p.surface
	if s == nil || s.handler == nil {
		return
	}
	p.backend.BeginFrame()
	s.handler.Frame(core.NewPainter(p.backend))
	p.backend.EndFrame()

	img := p.backend.Image()
	_ = p.texture.Update(nil, unsafe.Pointer(&img.Pix[0]), img.Stride)
	_ = p.renderer.Clear()
	_ = p.renderer.Copy(p.texture, nil, nil)
	p.renderer.Present()
}

// pumpEvents drains SDL's queue into the surface handler.
func (p *Platform) pumpEvents() bool {
	s := p.surface
	delivered := false
	for {
		ev := sdl2.PollEvent()
		if ev == nil {
			return delivered
		}
		delivered = true
		if s == nil || s.handler == nil {
			continue
		}
		switch e := ev.(type) {
		case *sdl2.QuitEvent:
			s.handler.Event(core.QuitEvent{})
		case *sdl2.WindowEvent:
			if e.Event == sdl2.WINDOWEVENT_SIZE_CHANGED {
				if err := p.recreateFramebuffer(int(e.Data1), int(e.Data2)); err == nil {
					s.handler.Resized(p.backend.Size())
					s.Invalidate(core.UnitRect{})
				}
			}
		case *sdl2.TextInputEvent:
			text := e.GetText()
			for _, ch := range text {
				s.handler.Event(core.KeyPressEvent{
					Key:  string(ch),
					Text: string(ch),
				})
			}
		case *sdl2.KeyboardEvent:
			if e.Type == sdl2.KEYDOWN {
				if key := translateKey(e.Keysym); key != "" {
					mods, name := core.ParseKeyModifiers(key)
					text := ""
					if len(name) == 1 && name[0] >= 32 && name[0] < 127 {
						text = name
					}
					s.handler.Event(core.KeyPressEvent{Key: key, Modifiers: mods, Text: text})
				}
			}
		case *sdl2.MouseButtonEvent:
			btn := mapButton(e.Button)
			x, y := p.toUnits(e.X, e.Y)
			if e.Type == sdl2.MOUSEBUTTONDOWN {
				s.handler.Event(core.MousePressEvent{X: x, Y: y, Button: btn})
			} else {
				s.handler.Event(core.MouseReleaseEvent{X: x, Y: y, Button: btn})
			}
		case *sdl2.MouseMotionEvent:
			var held core.MouseButton
			if e.State&sdl2.ButtonLMask() != 0 {
				held = core.LeftButton
			}
			x, y := p.toUnits(e.X, e.Y)
			s.handler.Event(core.MouseMoveEvent{X: x, Y: y, Buttons: held})
		case *sdl2.MouseWheelEvent:
			mx, my, _ := sdl2.GetMouseState()
			x, y := p.toUnits(mx, my)
			s.handler.Event(core.MouseWheelEvent{
				X: x, Y: y,
				DeltaX: int(e.X), DeltaY: int(e.Y),
			})
		}
	}
}

// toUnits converts window-pixel mouse coordinates to abstract units.
func (p *Platform) toUnits(x, y int32) (core.Unit, core.Unit) {
	return core.Unit(int(x) / p.scale), core.Unit(int(y) / p.scale)
}

func mapButton(b uint8) core.MouseButton {
	switch b {
	case sdl2.BUTTON_LEFT:
		return core.LeftButton
	case sdl2.BUTTON_MIDDLE:
		return core.MiddleButton
	case sdl2.BUTTON_RIGHT:
		return core.RightButton
	}
	return core.NoButton
}

// specialKeys maps SDL keycodes to D3 key names (spellings match
// core/keybindings.go).
var specialKeys = map[sdl2.Keycode]string{
	sdl2.K_RETURN:    "Enter",
	sdl2.K_KP_ENTER:  "Enter",
	sdl2.K_TAB:       "Tab",
	sdl2.K_ESCAPE:    "Escape",
	sdl2.K_BACKSPACE: "Backspace",
	sdl2.K_DELETE:    "Delete",
	sdl2.K_INSERT:    "Insert",
	sdl2.K_HOME:      "Home",
	sdl2.K_END:       "End",
	sdl2.K_PAGEUP:    "PageUp",
	sdl2.K_PAGEDOWN:  "PageDown",
	sdl2.K_UP:        "Up",
	sdl2.K_DOWN:      "Down",
	sdl2.K_LEFT:      "Left",
	sdl2.K_RIGHT:     "Right",
	sdl2.K_F1:        "F1",
	sdl2.K_F2:        "F2",
	sdl2.K_F3:        "F3",
	sdl2.K_F4:        "F4",
	sdl2.K_F5:        "F5",
	sdl2.K_F6:        "F6",
	sdl2.K_F7:        "F7",
	sdl2.K_F8:        "F8",
	sdl2.K_F9:        "F9",
	sdl2.K_F10:       "F10",
	sdl2.K_F11:       "F11",
	sdl2.K_F12:       "F12",
}

// translateKey produces the D3 key string for a KEYDOWN, or "" when
// the TextInput path owns it (plain printable characters).
func translateKey(sym sdl2.Keysym) string {
	ctrl := sym.Mod&sdl2.KMOD_CTRL != 0
	alt := sym.Mod&sdl2.KMOD_ALT != 0
	shift := sym.Mod&sdl2.KMOD_SHIFT != 0

	if name, ok := specialKeys[sym.Sym]; ok {
		prefix := ""
		if alt {
			prefix += "M-"
		}
		if ctrl {
			prefix += "C-"
		}
		if shift {
			prefix += "S-"
		}
		return prefix + name
	}

	// Letters and printable symbols.
	if sym.Sym >= 32 && sym.Sym < 127 {
		ch := rune(sym.Sym)
		isLetter := ch >= 'a' && ch <= 'z'
		switch {
		case ctrl && isLetter && !shift:
			base := "^" + string(ch-'a'+'A')
			if alt {
				return "M-" + base
			}
			return base
		case ctrl:
			prefix := ""
			if alt {
				prefix += "M-"
			}
			prefix += "C-"
			if shift {
				prefix += "S-"
			}
			return prefix + string(ch)
		case alt:
			return "M-" + string(ch)
		default:
			// Plain (possibly shifted) printable: TextInput delivers it.
			return ""
		}
	}
	return ""
}

func (p *Platform) drainPosts() {
	for {
		p.mu.Lock()
		if len(p.posts) == 0 {
			p.mu.Unlock()
			return
		}
		fns := p.posts
		p.posts = nil
		p.mu.Unlock()
		for _, fn := range fns {
			fn()
		}
	}
}

func (p *Platform) fireDueTimers() {
	now := time.Now()
	p.mu.Lock()
	var due []func()
	var rest []timerEntry
	for _, t := range p.timers {
		if !t.due.After(now) {
			due = append(due, t.fn)
		} else {
			rest = append(rest, t)
		}
	}
	p.timers = rest
	p.mu.Unlock()
	for _, fn := range due {
		fn()
	}
}

// Post implements platform.Platform.
func (p *Platform) Post(fn func()) {
	p.mu.Lock()
	p.posts = append(p.posts, fn)
	p.mu.Unlock()
}

// PostAfter implements platform.Platform.
func (p *Platform) PostAfter(d time.Duration, fn func()) {
	p.mu.Lock()
	p.timers = append(p.timers, timerEntry{due: time.Now().Add(d), fn: fn})
	p.mu.Unlock()
}

// Quit implements platform.Platform.
func (p *Platform) Quit(code int) {
	p.exitCode.Store(int32(code))
	p.quitting.Store(true)
}

// CreateSurface implements platform.Platform: the SDL window is the
// one surface (per-window native surfaces arrive with G4 granting).
func (p *Platform) CreateSurface(opts platform.SurfaceOptions) (platform.Surface, error) {
	if p.surface != nil {
		return nil, fmt.Errorf("sdl platform: surface already created")
	}
	p.surface = &sdlSurface{platform: p}
	return p.surface, nil
}

// Clipboard implements platform.Platform.
func (p *Platform) Clipboard() string {
	s, _ := sdl2.GetClipboardText()
	return s
}

// SetClipboard implements platform.Platform.
func (p *Platform) SetClipboard(text string) { _ = sdl2.SetClipboardText(text) }

// Beep implements platform.Platform.
func (p *Platform) Beep() {}

// sdlSurface is the SDL window as a platform.Surface.
type sdlSurface struct {
	platform *Platform
	handler  platform.SurfaceHandler
	dirty    atomic.Bool
}

func (s *sdlSurface) Size() core.UnitSize {
	return s.platform.backend.Size()
}
func (s *sdlSurface) Metrics() core.CellMetrics {
	return s.platform.backend.Metrics()
}
func (s *sdlSurface) SetHandler(h platform.SurfaceHandler) { s.handler = h }
func (s *sdlSurface) Invalidate(core.UnitRect)             { s.dirty.Store(true) }
func (s *sdlSurface) SetCursorVisible(bool)                {}
func (s *sdlSurface) SetCursorPosition(x, y core.Unit)     {}
