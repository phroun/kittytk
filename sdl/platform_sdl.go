//go:build sdl

package sdl

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	sdl2 "github.com/veandco/go-sdl2/sdl"
	wgpu "github.com/gogpu/wgpu" // Native, zero-cgo WebGPU dependency
	gputypes "github.com/gogpu/gputypes"
    _ "github.com/gogpu/wgpu/hal/allbackends"

	"github.com/phroun/kittytk/backend/raster"
	"github.com/phroun/kittytk/core"
	"github.com/phroun/kittytk/platform"
)

// WGSL shaders for blitting the UI texture to the screen
const blitVertexShader = `
// Combined uniforms in group 1 (since group 2 doesn't work)
struct CombinedUniforms {
    angle: f32,      // rotation (from effects)
    enabled: f32,    // effects enabled
    scale: f32,      // effects scale (2.0 = scene at 50%)
    aspect: f32,     // surface width/height, for undistorted rotation
    pos_x: f32,      // window x in NDC
    pos_y: f32,      // window y in NDC
    size_w: f32,     // window width in NDC
    size_h: f32,     // window height in NDC
}

@group(1) @binding(0) var<uniform> uniforms: CombinedUniforms;

struct VertexOutput {
    @builtin(position) position: vec4<f32>,
    @location(0) texCoord: vec2<f32>,
}

@vertex
fn vs_main(@builtin(vertex_index) vertex_index: u32) -> VertexOutput {
    // Generate a proper quad (2 triangles = 6 vertices)
    var pos = array<vec2<f32>, 6>(
        vec2<f32>(0.0, 0.0),  // Triangle 1: bottom-left
        vec2<f32>(1.0, 0.0),  //            bottom-right
        vec2<f32>(0.0, 1.0),  //            top-left
        vec2<f32>(1.0, 0.0),  // Triangle 2: bottom-right
        vec2<f32>(1.0, 1.0),  //            top-right
        vec2<f32>(0.0, 1.0)   //            top-left
    );

    var texCoords = array<vec2<f32>, 6>(
        vec2<f32>(0.0, 1.0),  // Flipped Y for texture
        vec2<f32>(1.0, 1.0),
        vec2<f32>(0.0, 0.0),
        vec2<f32>(1.0, 1.0),
        vec2<f32>(1.0, 0.0),
        vec2<f32>(0.0, 0.0)
    );

    // Apply position and size from uniforms
    let scaled_pos = pos[vertex_index] * vec2<f32>(uniforms.size_w, uniforms.size_h);
    var final_pos = vec2<f32>(uniforms.pos_x, uniforms.pos_y) + scaled_pos;

    // Rotation demo: rigidly rotate and shrink the whole scene around the
    // surface center. Every layer (desktop base, windows, menus, popups)
    // carries the same effect values, so the composited scene turns as
    // one. Rotation happens in aspect-corrected space so circles stay
    // circles on non-square surfaces.
    if (uniforms.enabled > 0.5) {
        let inv = 1.0 / max(uniforms.scale, 0.001);
        let aspect = max(uniforms.aspect, 0.001);
        var q = vec2<f32>(final_pos.x * aspect, final_pos.y) * inv;
        let ang = -uniforms.angle;
        let c = cos(ang);
        let s = sin(ang);
        q = vec2<f32>(q.x * c - q.y * s, q.x * s + q.y * c);
        final_pos = vec2<f32>(q.x / aspect, q.y);
    }

    var output: VertexOutput;
    output.position = vec4<f32>(final_pos, 0.0, 1.0);
    output.texCoord = texCoords[vertex_index];
    return output;
}
`

const blitFragmentShader = `
struct CombinedUniforms {
    angle: f32,
    enabled: f32,
    scale: f32,
    aspect: f32,
    pos_x: f32,
    pos_y: f32,
    size_w: f32,
    size_h: f32,
}

@group(1) @binding(0) var<uniform> uniforms: CombinedUniforms;
@group(0) @binding(0) var ui_texture: texture_2d<f32>;
@group(0) @binding(1) var ui_sampler: sampler;

@fragment
fn fs_main(@builtin(position) fragPos: vec4<f32>, @location(0) texCoord: vec2<f32>) -> @location(0) vec4<f32> {
    // Sample texture
    return textureSample(ui_texture, ui_sampler, texCoord);
}
`

// 3D cube shaders
const cubeVertexShader = `
struct CubeUniforms {
    mvp: mat4x4<f32>,  // Model-View-Projection matrix
}

@group(1) @binding(0) var<uniform> cube_uniforms: CubeUniforms;

struct VertexInput {
    @location(0) position: vec3<f32>,
    @location(1) texCoord: vec2<f32>,
}

struct VertexOutput {
    @builtin(position) position: vec4<f32>,
    @location(0) texCoord: vec2<f32>,
}

@vertex
fn vs_main(in: VertexInput) -> VertexOutput {
    var out: VertexOutput;
    out.position = cube_uniforms.mvp * vec4<f32>(in.position, 1.0);
    out.texCoord = in.texCoord;
    return out;
}
`

const cubeFragmentShader = `
@group(0) @binding(0) var cube_texture: texture_2d<f32>;
@group(0) @binding(1) var cube_sampler: sampler;

@fragment
fn fs_main(@location(0) texCoord: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSample(cube_texture, cube_sampler, texCoord);
}
`

// Platform runs KittyTK over SDL2 windows with pluggable renderer backend.
// All callbacks execute on the OS-locked main thread.
type Platform struct {
	title    string
	appName  string // OS application name (macOS menu bar / task switcher); "" = SDL default
	wPx, hPx int
	scale    int              // device zoom: pixels per unit at 12pt; see SetScale
	fontSize int              // UI point size that sets the cell pixel size (0 = 12pt base)
	metrics  core.CellMetrics // root cell denomination for every surface (0 = raster default 8x16)

	// defaultFontSize is the CONFIGURED font size (the SetFontSize value)
	defaultFontSize int

	mu     sync.Mutex
	posts  []func()
	timers []timerEntry

	quitting atomic.Bool
	exitCode atomic.Int32

	backend *raster.Backend // main window's framebuffer
	seed    *raster.Backend

	// Rendering backend (software or WebGPU)
	renderer Renderer

	// WebGPU-specific fields (only used when renderer is WebGPURenderer)
	// These will eventually be moved entirely into WebGPURenderer
	gpuInstance *wgpu.Instance
	gpuAdapter  *wgpu.Adapter
	gpuDevice   *wgpu.Device
	gpuQueue    *wgpu.Queue
	blitPipeline  *wgpu.RenderPipeline
	blitSampler   *wgpu.Sampler
	blitLayout    *wgpu.BindGroupLayout
	blitUniformBuffer *wgpu.Buffer
	blitUniformLayout *wgpu.BindGroupLayout
	blitUniformBindGroup *wgpu.BindGroup
	cubePipeline *wgpu.RenderPipeline
	cubeVertexBuffer *wgpu.Buffer
	cubeIndexBuffer *wgpu.Buffer
	cubeUniformBuffer *wgpu.Buffer
	cubeUniformLayout *wgpu.BindGroupLayout
	cubeUniformBindGroup *wgpu.BindGroup
	cubeLayout *wgpu.BindGroupLayout
	rotationStartTime time.Time
	rotationEnabled atomic.Bool
	rotationActivationTime time.Time
	rotationDeactivationTime time.Time
	rotationAngleAtDeactivation float64

	main *nativeWin
	wins map[uint32]*nativeWin // by SDL window ID, main included

	// System mouse cursors, created on demand and cached by shape.
	cursors   map[core.CursorShape]*sdl2.Cursor
	cursorSet bool

	// FPS overlay in the OS title bar
	showFPS   bool
	fpsFrames int
	fpsSince  time.Time

	// vsync selects whether presents sync to the display refresh.
	vsync bool
}

// SetShowFPS enables the render frame-rate readout in the main window's OS title bar.
func (p *Platform) SetShowFPS(on bool) { p.showFPS = on }

// SetVSync selects whether presents sync to the display refresh.
func (p *Platform) SetVSync(on bool) { p.vsync = on }

// nativeWin bundles one OS window with its GoGPU hardware presentation chain.
type nativeWin struct {
	window   *sdl2.Window
	
	// WebGPU rendering (when using webgpu renderer)
	gpuSurface    *wgpu.Surface
	config   *wgpu.SurfaceConfiguration
	uiTexture *wgpu.Texture       // VRAM container matching your framebuffer dimensions
	depthTexture *wgpu.Texture    // Depth buffer for 3D rendering
	depthView *wgpu.TextureView   // View for depth texture
	
	// Software rendering (when using software renderer)
	renderer *sdl2.Renderer
	texture  *sdl2.Texture
	
	// Common fields
	backend  *raster.Backend
	uiBuffer *wgpu.Buffer        // Staging buffer (WebGPU only, unused now)
	surface  *sdlSurface
	id       uint32

	// shapeRadiusPx > 0 shapes the OS window with rounded corners
	shapeRadiusPx int

	// transparent marks a window with real per-pixel alpha (macOS)
	transparent bool

	// layerRadiusPx > 0 rounds the window by clipping its Core
	// Animation layer; re-applied per present because surface
	// configuration can reset layer state.
	layerRadiusPx int
}

type timerEntry struct {
	due time.Time
	fn  func()
}

// New creates an SDL + WebGPU composite platform.
// New creates an SDL platform with the specified renderer backend.
// rendererType should be "software" or "webgpu".
func New(title string, widthPx, heightPx int, rendererType string) (*Platform, error) {
	// Create renderer
	renderer, err := NewRenderer(rendererType, true) // vsync default true
	if err != nil {
		return nil, fmt.Errorf("failed to create %s renderer: %w", rendererType, err)
	}

	return &Platform{
		title:             title,
		wPx:               widthPx,
		hPx:               heightPx,
		scale:             1,
		vsync:             true,
		renderer:          renderer,
		wins:              map[uint32]*nativeWin{},
		cursors:           map[core.CursorShape]*sdl2.Cursor{},
		rotationStartTime: time.Now(),
	}, nil
}

func (p *Platform) SetAppName(name string) {
	p.appName = name
}

var macAboutHandler func()

func (p *Platform) SetAboutHandler(fn func()) {
	macAboutHandler = func() {
		// Enable rotation effect when About is clicked
		if !p.rotationEnabled.Load() {
			p.rotationEnabled.Store(true)
			p.rotationStartTime = time.Now() // Reset start time
		}
		// Call the original handler
		if fn != nil {
			fn()
		}
	}
}

func (p *Platform) SetScale(scale int) {
	if scale < 1 {
		scale = 1
	}
	p.scale = scale
}

func (p *Platform) SetCellMetrics(m core.CellMetrics) {
	p.metrics = m
}

func (p *Platform) SetFontSize(size int) {
	if size > 0 {
		size = clampFontPt(size)
	}
	p.fontSize = size
	p.defaultFontSize = size
}

func (p *Platform) applyMetrics(b *raster.Backend) {
	if p.metrics.CellWidth > 0 && p.metrics.CellHeight > 0 {
		b.SetCellMetrics(p.metrics)
	}
	if p.fontSize > 0 {
		b.SetFontSize(p.fontSize)
	}
}

func (p *Platform) Backend() *raster.Backend { return p.backend }

func (p *Platform) EnsureBackend() (*raster.Backend, error) {
	if p.backend == nil {
		b, err := raster.NewScaled(p.wPx, p.hPx, p.scale)
		if err != nil {
			return nil, err
		}
		p.applyMetrics(b)
		p.backend = b
		p.seed = b
	}
	return p.backend, nil
}

// The dynamic size is bounded to 4..100pt.
const (
	minFontPt = 4
	maxFontPt = 100
)

// alphaPresentTest (KITTYTK_ALPHA_TEST=1) is a diagnostic: transparent
// (per-pixel alpha) windows present a bare alpha-0 clear with no
// content. A torn-off window that still shows as a black rectangle
// proves the compositing chain below the renderer (CAMetalLayer / SDL
// content view / NSWindow) is discarding alpha; a window that vanishes
// entirely proves the chain honors alpha and any remaining opacity
// comes from painted content.
var alphaPresentTest = os.Getenv("KITTYTK_ALPHA_TEST") != ""

// roundedCornerMechanism selects how a rounded borderless window gets
// its corners cut, via KITTYTK_WINDOW_SHAPE:
//
//	"layer"    Core Animation clips the Metal layer itself with
//	           cornerRadius + masksToBounds. The corner pixels are
//	           never composited, so this does not depend on the
//	           framebuffer's alpha surviving the swapchain - and CA
//	           antialiases the curve. Requires a Metal layer, so it
//	           applies to the WebGPU renderer only.
//	"shape"    SDL's shaped-window alpha mask. SDL masks the window's
//	           CONTENT VIEW, which is where SDL's OWN renderer draws -
//	           so this is the mechanism for the software renderer. A
//	           Metal-rendered window draws through a separate subview
//	           layer on top of that view, which the mask cannot clip.
//	"perpixel" the Cocoa route alone: a non-opaque NSWindow
//	           compositing the framebuffer's alpha channel, with no
//	           geometric clipping. Kept for experimentation; it has
//	           never produced transparent corners here.
//
// The default follows the renderer: "layer" when a GPU device gives us
// a Metal layer, else "shape". Off macOS there is no per-pixel window
// alpha and no Metal layer, so the shaped window is the only mechanism.
func (p *Platform) roundedCornerMechanism() string {
	switch os.Getenv("KITTYTK_WINDOW_SHAPE") {
	case "perpixel":
		return "perpixel"
	case "shape":
		return "shape"
	case "layer":
		return "layer"
	}
	if platformPerPixelAlpha && p.gpuDevice != nil {
		return "layer"
	}
	return "shape"
}

// clampFontPt bounds a point size to the dynamic zoom range.
func clampFontPt(size int) int {
	if size < minFontPt {
		return minFontPt
	}
	if size > maxFontPt {
		return maxFontPt
	}
	return size
}

// Run implements platform.Platform.
func (p *Platform) Run(init func(platform.Platform)) int {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if p.appName != "" {
		_ = sdl2.SetHint("SDL_APP_NAME", p.appName)
	}

	if err := sdl2.Init(sdl2.INIT_VIDEO | sdl2.INIT_EVENTS); err != nil {
		return 1
	}
	defer sdl2.Quit()

	_ = sdl2.SetHint("SDL_MOUSE_FOCUS_CLICKTHROUGH", "1")

	// Initialize the renderer (WebGPU setup or software renderer setup)
	if err := p.renderer.Initialize(); err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to initialize renderer: %v\n", err)
		return 1
	}
	defer p.renderer.Shutdown()

	// For WebGPU renderer, expose GPU objects to Platform temporarily
	// TODO: Remove this once full extraction is complete
	p.exposeWebGPUObjects()
	

	// 2. Create Master System UI Window Viewport Surface
	win, err := p.createWindow(p.title, sdl2.WINDOWPOS_CENTERED, sdl2.WINDOWPOS_CENTERED,
		p.wPx, p.hPx, sdl2.WINDOW_SHOWN|sdl2.WINDOW_RESIZABLE, 0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Failed to create window: %v\n", err)
		return 1
	}
	p.main = win
	p.backend = win.backend
	defer func() {
		for _, w := range p.wins {
			w.destroy()
		}
	}()

	if macAboutHandler != nil {
		installAboutMenuHandler()
	}

	sdl2.StartTextInput()

	// Event watch hooks handle continuous redraw requests during active modal resize loops
	sdl2.AddEventWatchFunc(func(ev sdl2.Event, _ interface{}) bool {
		if e, ok := ev.(*sdl2.WindowEvent); ok && e.Event == sdl2.WINDOWEVENT_SIZE_CHANGED {
			p.drainPosts()
			if !p.liveResize(e.WindowID, int(e.Data1), int(e.Data2)) {
				// Same-size event: liveResize didn't present, so keep the
				// modal loop fed with a fresh frame ourselves.
				if w, ok := p.wins[e.WindowID]; ok && w.window != nil {
					p.presentWindow(w, true)
				}
			}
		}
		return true
	}, nil)

	if init != nil {
		init(p)
	}

	// Main Display Server Loop Execution Pipeline
	for !p.quitting.Load() {
		p.drainPosts()
		p.fireDueTimers()
		if p.quitting.Load() {
			break
		}

		delivered := p.pumpEvents()
		p.reassertCursor()

		// Check if any windows need rendering
		anyDirty := false
		for _, w := range p.wins {
			s := w.surface
			if s == nil {
				continue
			}
			if s.dirty.Load() || (p.showFPS && w == p.main) {
				anyDirty = true
				break
			}
		}

		if anyDirty {
			for _, w := range p.wins {
				s := w.surface
				if s == nil {
					continue
				}
				dirty := s.dirty.Swap(false)
				burn := p.showFPS && w == p.main
				if dirty || burn {
					p.presentWindow(w, burn)
				}
			}
		}

		if p.showFPS {
			p.updateFPSTitle()
		}

		// Always add a small delay to prevent event loop starvation
		// Even when continuously rendering (rotation effects), we need to process events
		if !delivered {
			sdl2.Delay(5)
		} else {
			// Yield briefly even when events are flowing to prevent UI freeze
			sdl2.Delay(1)
		}
	}
	return int(p.exitCode.Load())
}

// createWindow builds one OS window with its presentation chain.
func (p *Platform) createWindow(title string, x, y int32, wPx, hPx int, flags uint32, shapeRadiusPx int) (*nativeWin, error) {
	w := &nativeWin{shapeRadiusPx: shapeRadiusPx}
	var err error

	if shapeRadiusPx > 0 && (!platformPerPixelAlpha || p.roundedCornerMechanism() == "shape") {
		// SDL's own shaped window: created shaped, masked in applyShape.
		w.window, err = sdl2.CreateShapedWindow(title, 0, 0, uint32(wPx), uint32(hPx), flags)
		if err == nil {
			w.window.SetPosition(x, y)
		} else {
			w.shapeRadiusPx = 0
		}
	}

	if w.window == nil {
		winFlags := flags
		if p.gpuDevice != nil && runtime.GOOS == "darwin" {
			// 0x00020000 is the hardcoded bitmask for SDL_WINDOW_METAL across all SDL2 distributions
			winFlags |= 0x00020000
		}
		w.window, err = sdl2.CreateWindow(title, x, y, int32(wPx), int32(hPx), winFlags)
	}

	if err != nil {
		return nil, err
	}

	// The WebGPU presentation chain binds directly to the native window.
	// The software renderer presents through SDL textures instead
	// (Renderer.CreateWindowRenderer below) and skips all of this.
	if p.gpuDevice != nil {
		// 1. Native Surface Integration: Map the raw SDL window handle directly to WebGPU
		// macOS hands over the CAMetalLayer; X11/Windows hand over display/window handles.
		displayHandle, windowHandle, err := nativeSurfaceHandles(w.window)
		if err != nil {
			w.window.Destroy()
			return nil, err
		}

		w.gpuSurface, err = p.gpuInstance.CreateSurface(displayHandle, windowHandle)
		if err != nil || w.gpuSurface == nil {
			w.window.Destroy()
			return nil, fmt.Errorf("failed to bind WebGPU hardware surface to window context: %w", err)
		}

		// The standard format supported natively by both macOS Metal and Windows 11
		surfaceFormat := wgpu.TextureFormatBGRA8Unorm

		presentMode := wgpu.PresentModeFifo
		if !p.vsync {
			presentMode = wgpu.PresentModeImmediate
		}

		// A shaped window on a per-pixel-alpha platform (macOS) will be
		// made transparent below; its surface must publish the alpha
		// channel or the rounded corners composite as opaque black.
		alphaMode := gputypes.CompositeAlphaModeOpaque
		if w.shapeRadiusPx > 0 && platformPerPixelAlpha {
			alphaMode = gputypes.CompositeAlphaModePremultiplied
		}

		w.config = &wgpu.SurfaceConfiguration{
			Format:      surfaceFormat,
			Usage:       wgpu.TextureUsageRenderAttachment | wgpu.TextureUsageCopyDst,
			AlphaMode:   alphaMode,
			Width:       uint32(wPx),
			Height:      uint32(hPx),
			PresentMode: presentMode,
		}

		err = w.gpuSurface.Configure(p.gpuDevice, w.config)
		if err != nil {
			w.gpuSurface.Release()
			w.window.Destroy()
			return nil, fmt.Errorf("failed to configure surface: %w", err)
		}

		// 2. Initialize offscreen VRAM texture buffers for software framebuffer blitting
		// Use BGRA format to match the surface format for direct copying
		w.uiTexture, err = p.gpuDevice.CreateTexture(&wgpu.TextureDescriptor{
			Size:          wgpu.Extent3D{Width: uint32(wPx), Height: uint32(hPx), DepthOrArrayLayers: 1},
			MipLevelCount: 1,
			SampleCount:   1,
			Dimension:     wgpu.TextureDimension2D,
			Format:        wgpu.TextureFormatBGRA8Unorm,
			Usage:         wgpu.TextureUsageCopySrc | wgpu.TextureUsageCopyDst | wgpu.TextureUsageRenderAttachment | wgpu.TextureUsageTextureBinding,
		})
		if err != nil {
			w.gpuSurface.Release()
			w.window.Destroy()
			return nil, err
		}

		// Size staging buffers to transfer raw pixel arrays from the raster framework to VRAM
		paddedBytesPerRow := (wPx * 4) // RGBA format pixel scaling
		w.uiBuffer, err = p.gpuDevice.CreateBuffer(&wgpu.BufferDescriptor{
			Size:  uint64(paddedBytesPerRow * hPx),
			Usage: wgpu.BufferUsageCopySrc | wgpu.BufferUsageMapWrite,
		})
	}

	if err := p.sizeFramebuffer(w, wPx, hPx); err != nil {
		if w.uiTexture != nil {
			w.uiTexture.Release()
		}
		if w.gpuSurface != nil {
			w.gpuSurface.Release()
		}
		w.window.Destroy()
		return nil, err
	}

	if shapeRadiusPx > 0 && os.Getenv("KITTYTK_ALPHA_DEBUG") != "" {
		fmt.Fprintf(os.Stderr,
			"kittytk-alpha: rounding request: radius=%dpx mechanism=%q perPixelAlpha=%v shapedWindow=%v\n",
			shapeRadiusPx, p.roundedCornerMechanism(), platformPerPixelAlpha, w.shapeRadiusPx > 0)
	}

	if w.shapeRadiusPx > 0 && platformPerPixelAlpha {
		switch p.roundedCornerMechanism() {
		case "layer":
			// Core Animation clips the Metal layer itself. The window
			// must still be non-opaque for the clipped-away corners to
			// show what is behind them.
			transparent := makeWindowTransparent(w.window)
			rounded := roundWindowLayer(w.window, w.shapeRadiusPx)
			if os.Getenv("KITTYTK_ALPHA_DEBUG") != "" {
				fmt.Fprintf(os.Stderr,
					"kittytk-alpha: layer rounding: transparent=%v layerFound=%v\n",
					transparent, rounded)
			}
			if transparent && rounded {
				w.transparent = true
				w.layerRadiusPx = w.shapeRadiusPx
				w.shapeRadiusPx = 0 // geometry comes from the layer, not a shape mask
			}
		case "perpixel":
			// The drawn frame's own alpha cuts the corners.
			if makeWindowTransparent(w.window) {
				w.transparent = true
				w.shapeRadiusPx = 0
			}
		}
	}
	w.id, _ = w.window.GetID()
	p.wins[w.id] = w

	// Create renderer resources for this window (compositor texture)
	if err := p.renderer.CreateWindowRenderer(w, wPx, hPx); err != nil {
		// Non-fatal for software renderer, but log it
		fmt.Fprintf(os.Stderr, "WARNING: Failed to create window renderer resources: %v\n", err)
	}

	// Shape AFTER the renderer exists, matching the order in the
	// known-good SDL shaped-window sequence (create shaped window,
	// create renderer, then SetShape).
	w.applyShape()

	return w, nil
}

// destroy tears down one window's hardware presentation chain.
func (w *nativeWin) destroy() {
	// Note: We can't call p.renderer.DestroyWindowRenderer here because we don't have access to Platform
	// The renderer cleanup will happen when Platform shuts down

	if w.texture != nil {
		w.texture.Destroy()
		w.texture = nil
	}
	if w.renderer != nil {
		w.renderer.Destroy()
		w.renderer = nil
	}
	if w.uiTexture != nil {
		w.uiTexture.Release()
		w.uiTexture = nil
	}
	if w.uiBuffer != nil {
		w.uiBuffer.Release()
		w.uiBuffer = nil
	}
	if w.gpuSurface != nil {
		w.gpuSurface.Release()
		w.gpuSurface = nil
	}
	if w.window != nil {
		w.window.Destroy()
		w.window = nil
	}
}

// sizeFramebuffer sizes one window's raster backend and streaming WebGPU textures.
func (p *Platform) sizeFramebuffer(w *nativeWin, wPx, hPx int) error {
	b, err := raster.NewScaled(wPx, hPx, p.scale)
	if err != nil {
		return err
	}
	p.applyMetrics(b)
	w.backend = b
	if w == p.main || p.main == nil {
		p.backend = b
		p.wPx, p.hPx = wPx, hPx
	}
	
	// The fresh backend starts zero-filled, so the surface's damage
	// tracking must be reset: without this the handler repaints only what
	// it thinks changed, and the untouched area presents as black.
	if w.surface != nil {
		w.surface.Invalidate(core.UnitRect{}) // Empty rect = invalidate all
	}

	if w.gpuSurface == nil {
		// Software renderer: re-size the SDL streaming texture instead of
		// the WebGPU chain.
		return p.renderer.ResizeWindowRenderer(w, wPx, hPx)
	}

	// Clean up old WebGPU texture if this is a resize event
	if w.uiTexture != nil {
		w.uiTexture.Release()
	}
	if w.depthTexture != nil {
		w.depthTexture.Release()
	}
	if w.depthView != nil {
		w.depthView.Release()
	}

	// 1. Re-configure the physical Window Surface Swapchain size limits
	w.config.Width = uint32(wPx)
	w.config.Height = uint32(hPx)
	w.gpuSurface.Configure(p.gpuDevice, w.config)

	// 2. Re-allocate the intermediate GPU backing texture layout
	w.uiTexture, err = p.gpuDevice.CreateTexture(&wgpu.TextureDescriptor{
		Size:          wgpu.Extent3D{Width: uint32(wPx), Height: uint32(hPx), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     wgpu.TextureDimension2D,
		Format:        wgpu.TextureFormatBGRA8Unorm,
		Usage:         wgpu.TextureUsageCopySrc | wgpu.TextureUsageCopyDst | wgpu.TextureUsageRenderAttachment | wgpu.TextureUsageTextureBinding,
	})
	if err != nil {
		return err
	}

	// 3. Create depth texture for 3D rendering
	w.depthTexture, err = p.gpuDevice.CreateTexture(&wgpu.TextureDescriptor{
		Size:          wgpu.Extent3D{Width: uint32(wPx), Height: uint32(hPx), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     wgpu.TextureDimension2D,
		Format:        wgpu.TextureFormatDepth24Plus,
		Usage:         wgpu.TextureUsageRenderAttachment,
	})
	if err != nil {
		return err
	}

	w.depthView, err = p.gpuDevice.CreateTextureView(w.depthTexture, nil)
	if err != nil {
		return err
	}

	// Note: We'll create a transient view for each frame since textures don't have permanent views
	// The bind group will be created per-frame with the texture

	return nil
}

// createMVPMatrix creates a model-view-projection matrix for 3D cube rendering
func createMVPMatrix(aspectRatio float32, rotationAngle float32, scale float32, floatY float32) [16]float32 {
	// Simple rotation matrices
	sinY := float32(math.Sin(float64(rotationAngle)))
	cosY := float32(math.Cos(float64(rotationAngle)))
	sinX := float32(math.Sin(float64(rotationAngle * 0.7)))
	cosX := float32(math.Cos(float64(rotationAngle * 0.7)))
	
	// Use passed-in scale (will be eased from 0 to 1.5)
	translateZ := float32(0.5) // Move it forward so it's in front of clip plane
	
	return [16]float32{
		// Column 0 (X axis after transform)
		scale * cosY / aspectRatio, scale * sinX * sinY / aspectRatio, scale * cosX * sinY / aspectRatio, 0,
		// Column 1 (Y axis after transform)
		0, scale * cosX, -scale * sinX, 0,
		// Column 2 (Z axis after transform)
		-scale * sinY / aspectRatio, scale * sinX * cosY / aspectRatio, scale * cosX * cosY / aspectRatio, 0,
		// Column 3 (translation with floating Y motion)
		0, floatY, translateZ, 1,
	}
}

// paintBackend runs the handler frame into the window's raster backend,
// honoring the surface's damage region unless forceFull demands a
// complete repaint. baseOnly selects the handler's chrome-only base
// layer (BaseLayerPainter) when the compositor renders child windows,
// menus, and popups on their own layers.
func (p *Platform) paintBackend(w *nativeWin, forceFull, baseOnly bool) {
	s := w.surface
	if s == nil || s.handler == nil || w.backend == nil {
		return
	}

	full, dmg := s.takeDamage()
	if forceFull {
		full = true
	}
	if !full && (dmg.Width <= 0 || dmg.Height <= 0) {
		// Dirty with no bounded region recorded: repaint everything.
		full = true
	}

	frame := s.handler.Frame
	if baseOnly {
		if base, ok := s.handler.(platform.BaseLayerPainter); ok {
			frame = base.FrameBase
		}
	}

	w.backend.BeginFrame()
	if full {
		frame(core.NewPainter(w.backend))
	} else {
		// Clip the whole tree to the damaged region
		frame(core.NewPainter(w.backend).WithClip(dmg))
	}
	w.backend.EndFrame()
}

// presentWindow is the ONE way a window reaches the screen: it paints the
// backend and presents through the active renderer, compositing child
// windows when the renderer and the surface handler both support it.
// Every present path — main loop, live resize, font zoom — funnels here so
// resize frames cannot diverge from steady-state frames.
func (p *Platform) presentWindow(w *nativeWin, forceFull bool) {
	s := w.surface
	if s == nil || s.handler == nil {
		return
	}

	if p.renderer.SupportsFeature(FeatureCompositing) {
		if provider, ok := s.handler.(platform.WindowProvider); ok {
			if childWindowList := provider.GetChildWindows(); childWindowList != nil && len(childWindowList.Windows) > 0 {
				err := p.renderer.RenderFrameWithChildWindows(w, childWindowList, p.scale, func(win *nativeWin) {
					p.paintBackend(win, forceFull, true)
				})
				if err == nil {
					p.scheduleAnimationFrame(s)
					return
				}
				fmt.Fprintf(os.Stderr, "Child window compositor error on window %d: %v\n", w.id, err)
				// Fall through to the plain present so the frame still lands.
			}
		}
	}

	p.paintAndPresent(w, forceFull)
}

// scheduleAnimationFrame keeps frames coming while the rotation demo is
// running (or easing out) — the animation must not stall waiting for
// input events.
func (p *Platform) scheduleAnimationFrame(s *sdlSurface) {
	if s == nil {
		return
	}
	animating := p.rotationEnabled.Load()
	if !animating {
		animating = time.Since(p.rotationDeactivationTime).Seconds() < 0.5
	}
	if animating {
		s.Invalidate(core.UnitRect{})
	}
}

// paintAndPresent runs the handler frame into the window's raster backend and
// presents it as a single surface (no child window compositing).
func (p *Platform) paintAndPresent(w *nativeWin, forceFull bool) {
	frameStart := time.Now()

	s := w.surface
	if s == nil || s.handler == nil || w.backend == nil {
		return
	}

	p.paintBackend(w, forceFull, false)

	// Without a GPU presentation chain (software renderer), the renderer
	// blits the backend through SDL textures instead.
	if p.gpuDevice == nil || w.uiTexture == nil {
		if err := p.renderer.Present(w, w.backend); err != nil {
			fmt.Fprintf(os.Stderr, "Present error on window %d: %v\n", w.id, err)
		}
		return
	}

	img := w.backend.Image()
	if img == nil {
		return
	}

	// 3. Upload pixels to intermediate texture, then render it to surface
	bounds := img.Bounds()
	wPx := uint32(bounds.Dx())
	hPx := uint32(bounds.Dy())
	
	// Convert RGBA to BGRA
	pixelData := make([]byte, len(img.Pix))
	for i := 0; i < len(img.Pix); i += 4 {
		pixelData[i+0] = img.Pix[i+2] // B
		pixelData[i+1] = img.Pix[i+1] // G
		pixelData[i+2] = img.Pix[i+0] // R
		pixelData[i+3] = img.Pix[i+3] // A
	}
	
	// Write to intermediate UI texture
	err := p.gpuQueue.WriteTexture(
		&wgpu.ImageCopyTexture{
			Texture:  w.uiTexture,
			MipLevel: 0,
			Origin:   wgpu.Origin3D{X: 0, Y: 0, Z: 0},
			Aspect:   0,
		},
		pixelData,
		&wgpu.ImageDataLayout{
			Offset:       0,
			BytesPerRow:  uint32(img.Stride),
			RowsPerImage: hPx,
		},
		&wgpu.Extent3D{
			Width:              wPx,
			Height:             hPx,
			DepthOrArrayLayers: 1,
		},
	)
	if err != nil {
		return
	}
	
	// Surface configuration can reset the Metal layer's opacity state,
	// and an opaque layer discards alpha whatever we clear to.
	if w.transparent {
		reassertWindowAlpha(w.window)
	}
	if w.layerRadiusPx > 0 {
		roundWindowLayer(w.window, w.layerRadiusPx)
	}

	// Get surface texture
	surfaceTexture, _, err := w.gpuSurface.GetCurrentTexture()
	if err != nil {
		return
	}
	
	// Create texture view for UI texture (needed for bind group)
	uiTextureView, err := p.gpuDevice.CreateTextureView(w.uiTexture, nil)
	if err != nil {
		return
	}
	defer uiTextureView.Release()
	
	// Calculate current rotation angle and scale with easing
	var angle float32
	var enabled float32
	var scale float32 = 1.0 // Default to no scaling
	
	if p.rotationEnabled.Load() {
		// Easing IN
		timeSinceActivation := time.Since(p.rotationActivationTime).Seconds()
		
		// Easing function: ease-out cubic for smooth deceleration
		easeOutCubic := func(t float64) float64 {
			t = math.Min(t, 1.0) // Clamp to 1.0
			return 1.0 - math.Pow(1.0-t, 3.0)
		}
		
		// Scale eases in over 0.5 seconds from 1.0 to 2.0
		scaleProgress := timeSinceActivation / 0.5
		scaleEased := easeOutCubic(scaleProgress)
		scale = float32(1.0 + scaleEased*1.0) // 1.0 -> 2.0
		
		// Rotation eases in over 1.0 seconds from 0 to full speed
		rotationProgress := timeSinceActivation / 1.0
		rotationEased := easeOutCubic(rotationProgress)
		
		// After easing completes, continue rotating at full speed
		elapsedRotation := time.Since(p.rotationStartTime).Seconds()
		angle = float32(elapsedRotation * 0.1 * rotationEased) // Gradually reach full rotation speed
		
		enabled = 1.0
	} else {
		// Easing OUT - return to normal, but check if we're still animating
		timeSinceDeactivation := time.Since(p.rotationDeactivationTime).Seconds()
		
		if timeSinceDeactivation < 0.5 {
			// Still easing out
			easeOutCubic := func(t float64) float64 {
				t = math.Min(t, 1.0)
				return 1.0 - math.Pow(1.0-t, 3.0)
			}
			
			// Ease scale back from 2.0 to 1.0
			scaleProgress := timeSinceDeactivation / 0.5
			scaleEased := easeOutCubic(scaleProgress)
			scale = float32(2.0 - scaleEased*1.0) // 2.0 -> 1.0
			
			// Continue rotating FORWARD but speed up to snap to 0 (next 2π)
			currentAngle := p.rotationAngleAtDeactivation
			twoPi := 2.0 * math.Pi
			
			// Calculate how much further to rotate to reach next 0 (always forward/clockwise)
			normalizedAngle := math.Mod(currentAngle, twoPi)
			if normalizedAngle < 0 {
				normalizedAngle += twoPi
			}
			angleRemaining := twoPi - normalizedAngle
			
			// Continue at normal speed PLUS accelerated catch-up to reach 0 in 0.5 seconds
			elapsedSinceDeactivation := timeSinceDeactivation
			normalRotation := elapsedSinceDeactivation * 0.1 // Normal rotation speed
			
			// Add accelerated rotation to catch up with ease-out at the end
			// Use ease-in-out for smooth acceleration and deceleration
			catchUpProgress := math.Min(timeSinceDeactivation / 0.5, 1.0)
			
			// Ease-in-out cubic: accelerate, then decelerate as it approaches target
			easeInOutCubic := func(t float64) float64 {
				if t < 0.5 {
					return 4.0 * t * t * t
				}
				return 1.0 - math.Pow(-2.0*t+2.0, 3.0)/2.0
			}
			catchUpEased := easeInOutCubic(catchUpProgress)
			catchUpRotation := angleRemaining * catchUpEased
			
			// Cap at target angle (next 2π boundary)
			targetAngle := currentAngle + angleRemaining
			currentRotatedAngle := currentAngle + normalRotation + catchUpRotation
			if currentRotatedAngle > targetAngle {
				currentRotatedAngle = targetAngle
			}
			
			angle = float32(currentRotatedAngle)
			
			enabled = 1.0 // Keep effects enabled during ease-out
		} else {
			// Fully deactivated
			angle = 0.0
			enabled = 0.0
			scale = 1.0
		}
	}
	
	// Update the full 32-byte CombinedUniforms block the blit shader reads:
	// effects (angle, enabled, scale, padding) plus a fullscreen quad
	// position. Writing only the effects half would leave the position half
	// at whatever a previous frame put there.
	aspect := float32(1)
	if hPx > 0 {
		aspect = float32(wPx) / float32(hPx)
	}
	uniformData := make([]byte, 32)
	binary.LittleEndian.PutUint32(uniformData[0:4], math.Float32bits(angle))
	binary.LittleEndian.PutUint32(uniformData[4:8], math.Float32bits(enabled))
	binary.LittleEndian.PutUint32(uniformData[8:12], math.Float32bits(scale))
	binary.LittleEndian.PutUint32(uniformData[12:16], math.Float32bits(aspect))
	binary.LittleEndian.PutUint32(uniformData[16:20], math.Float32bits(-1.0)) // pos_x
	binary.LittleEndian.PutUint32(uniformData[20:24], math.Float32bits(-1.0)) // pos_y
	binary.LittleEndian.PutUint32(uniformData[24:28], math.Float32bits(2.0))  // size_w
	binary.LittleEndian.PutUint32(uniformData[28:32], math.Float32bits(2.0))  // size_h
	p.gpuQueue.WriteBuffer(p.blitUniformBuffer, 0, uniformData)
	
	// Use cached uniform bind group (no need to recreate)
	
	// Create bind group for texture + sampler (still need per-frame because texture view changes)
	bindGroup, err := p.gpuDevice.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Layout: p.blitLayout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, TextureView: uiTextureView},
			{Binding: 1, Sampler: p.blitSampler},
		},
	})
	if err != nil {
		return
	}
	defer bindGroup.Release()
	
	// Create surface view for rendering
	surfaceView, _ := surfaceTexture.CreateView(nil)
	defer surfaceView.Release()
	
	// Create command encoder
	encoder, _ := p.gpuDevice.CreateCommandEncoder(nil)
	
	// Render pass that draws the UI texture to the surface. A transparent
	// (per-pixel alpha) window must clear to alpha 0 or its rounded
	// corners composite as opaque black.
	clearAlpha := 1.0
	if w.transparent {
		clearAlpha = 0.0
	}
	renderPass, _ := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		ColorAttachments: []wgpu.RenderPassColorAttachment{
			{
				View:    surfaceView,
				LoadOp:  gputypes.LoadOpClear,
				StoreOp: gputypes.StoreOpStore,
				ClearValue: wgpu.Color{R: 0.0, G: 0.0, B: 0.0, A: clearAlpha},
			},
		},
	})
	
	// Set pipeline and bind groups, then draw. The blit vertex shader
	// generates a two-triangle quad, so this MUST draw all 6 vertices:
	// drawing 3 paints exactly half the window and leaves the other half
	// as the clear color — the diagonal "black triangle" resize artifact.
	// (Under KITTYTK_ALPHA_TEST a transparent window presents the bare
	// alpha-0 clear instead, isolating the compositing chain from the
	// painted content.)
	if !(alphaPresentTest && w.transparent) {
		renderPass.SetPipeline(p.blitPipeline)
		renderPass.SetBindGroup(0, bindGroup, nil)                  // Texture + sampler (per-frame)
		renderPass.SetBindGroup(1, p.blitUniformBindGroup, nil)    // Rotation uniform (cached)
		renderPass.Draw(6, 1, 0, 0)
	}
	renderPass.End()
	
	// Render 3D cube when rotation is active OR during ease-out
	shouldRenderCube := p.rotationEnabled.Load() && p.cubePipeline != nil
	if !shouldRenderCube {
		// Check if we're in ease-out phase
		timeSinceDeactivation := time.Since(p.rotationDeactivationTime).Seconds()
		if timeSinceDeactivation < 0.5 {
			shouldRenderCube = true
		}
	}
	
	if shouldRenderCube {
		// Calculate rotation angle for cube
		elapsed := time.Since(p.rotationStartTime).Seconds()
		cubeAngle := float32(elapsed * 0.5) // Rotate faster than the background
		
		// Add floating motion (sine wave)
		floatOffset := float32(math.Sin(elapsed * 1.5) * 0.15)
		
		// Calculate cube scale based on current state
		var cubeScale float32
		easeOutCubic := func(t float64) float64 {
			t = math.Min(t, 1.0)
			return 1.0 - math.Pow(1.0-t, 3.0)
		}
		
		if p.rotationEnabled.Load() {
			// Easing IN
			timeSinceActivation := time.Since(p.rotationActivationTime).Seconds()
			scaleProgress := timeSinceActivation / 0.5
			scaleEased := easeOutCubic(scaleProgress)
			cubeScale = float32(scaleEased * 1.5) // Ease from 0 to 1.5
			
			// Add slight scale pulsing after initial ease
			if scaleProgress >= 1.0 {
				pulse := float32(math.Sin(elapsed*2.0)*0.05 + 1.0)
				cubeScale *= pulse
			}
		} else {
			// Easing OUT
			timeSinceDeactivation := time.Since(p.rotationDeactivationTime).Seconds()
			scaleProgress := timeSinceDeactivation / 0.5
			scaleEased := easeOutCubic(scaleProgress)
			cubeScale = float32((1.0 - scaleEased) * 1.5) // Ease from 1.5 to 0
		}
		
		// Create MVP matrix with eased scale and floating motion
		aspectRatio := float32(wPx) / float32(hPx)
		mvp := createMVPMatrix(aspectRatio, cubeAngle, cubeScale, floatOffset)
		
		// Update uniform buffer
		mvpBytes := (*[64]byte)(unsafe.Pointer(&mvp[0]))[:]
		p.gpuQueue.WriteBuffer(p.cubeUniformBuffer, 0, mvpBytes)
		
		// Create bind group for cube texture (using UI texture)
		cubeBindGroup, err := p.gpuDevice.CreateBindGroup(&wgpu.BindGroupDescriptor{
			Layout: p.cubeLayout,
			Entries: []wgpu.BindGroupEntry{
				{Binding: 0, TextureView: uiTextureView},
				{Binding: 1, Sampler: p.blitSampler},
			},
		})
		if err == nil {
			defer cubeBindGroup.Release()
			
			// Render cube with back-face culling only (no depth buffer needed)
			cubePass, _ := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
				ColorAttachments: []wgpu.RenderPassColorAttachment{
					{
						View:       surfaceView,
						LoadOp:     gputypes.LoadOpLoad, // Keep existing content
						StoreOp:    gputypes.StoreOpStore,
						ClearValue: wgpu.Color{R: 0.0, G: 0.0, B: 0.0, A: 0.0},
					},
				},
			})
			
			cubePass.SetPipeline(p.cubePipeline)
			cubePass.SetBindGroup(0, cubeBindGroup, nil)
			cubePass.SetBindGroup(1, p.cubeUniformBindGroup, nil)
			cubePass.SetVertexBuffer(0, p.cubeVertexBuffer, 0)
			cubePass.SetIndexBuffer(p.cubeIndexBuffer, gputypes.IndexFormatUint16, 0)
			cubePass.DrawIndexed(36, 1, 0, 0, 0) // 36 indices for 12 triangles
			cubePass.End()
		}
	}
	
	// Submit and present
	cmdBuffer, _ := encoder.Finish()
	_, err = p.gpuQueue.Submit(cmdBuffer)
	if err != nil {
		return
	}
	w.gpuSurface.Present(surfaceTexture)

	// Enforce minimum frame time to prevent event starvation
	// Target 60 FPS = 16.67ms per frame
	frameDuration := time.Since(frameStart)
	minFrameTime := 16 * time.Millisecond
	if frameDuration < minFrameTime {
		time.Sleep(minFrameTime - frameDuration)
	}
	
	if frameDuration > 30*time.Millisecond {
		fmt.Printf("SLOW FRAME: %v (%.1f FPS)\n", frameDuration, 1000.0/float64(frameDuration.Milliseconds()))
	}

	// Continuous rotation requires continuous repaints
	p.scheduleAnimationFrame(s)

	if p.showFPS && w == p.main {
		p.fpsFrames++
	}
}

// damageDevicePx returns the device-pixel sub-rectangle to re-upload for a bounded repaint.
func damageDevicePx(b *raster.Backend, full bool, dmg core.UnitRect) (x0, y0, x1, y1 int, ok bool) {
	if full {
		return 0, 0, 0, 0, false
	}
	return b.DevicePxRect(dmg)
}

// updateFPSTitle rewrites the main window's OS title with the measured frame rate.
func (p *Platform) updateFPSTitle() {
	now := time.Now()
	if p.fpsSince.IsZero() {
		p.fpsSince = now
		return
	}
	elapsed := now.Sub(p.fpsSince)
	if elapsed < time.Second {
		return
	}
	fps := int(float64(p.fpsFrames)/elapsed.Seconds() + 0.5)
	if p.main != nil && p.main.window != nil {
		p.main.window.SetTitle(fmt.Sprintf("%s - %d fps", p.title, fps))
	}
	p.fpsFrames = 0
	p.fpsSince = now
}

// liveResize re-sizes one window's framebuffer, re-lays out its handler, and
// presents immediately. It reports whether it presented a frame, so the
// caller knows if a same-size event still needs a present of its own.
func (p *Platform) liveResize(id uint32, wPx, hPx int) bool {
	w, ok := p.wins[id]
	if !ok || wPx <= 0 || hPx <= 0 {
		return false
	}
	if img := w.backend.Image(); img != nil &&
		img.Bounds().Dx() == wPx && img.Bounds().Dy() == hPx {
		return false
	}
	if err := p.sizeFramebuffer(w, wPx, hPx); err != nil {
		return false
	}
	w.applyShape()
	s := w.surface
	if s == nil || s.handler == nil {
		return false
	}
	s.handler.Resized(w.backend.Size())
	p.presentWindow(w, true)
	s.dirty.Store(false)
	return true
}

// zoomTarget resolves a Command/Meta zoom chord to the font size it asks
// for: "+"/"=" (same key) steps up a point, "-" steps down, "0" returns to
// the configured default; the keypad's +/-/0 count too. ok is false for any
// other key: Ctrl or Alt in the chord makes it an ordinary key combination,
// but Shift rides along freely — "+" IS Shift+"=" on common layouts.
func zoomTarget(sym sdl2.Keysym, cur, def int) (int, bool) {
	if sym.Mod&sdl2.KMOD_GUI == 0 || sym.Mod&(sdl2.KMOD_CTRL|sdl2.KMOD_ALT) != 0 {
		return 0, false
	}
	switch sym.Sym {
	case sdl2.K_EQUALS, sdl2.K_PLUS, sdl2.K_KP_PLUS:
		return clampFontPt(cur + 1), true
	case sdl2.K_MINUS, sdl2.K_KP_MINUS:
		return clampFontPt(cur - 1), true
	case sdl2.K_0, sdl2.K_KP_0:
		return clampFontPt(def), true
	}
	return 0, false
}

// fontZoomKey consumes a KEYDOWN when it is one of the zoom chords, applying
// the resulting size live; the key never reaches the surface handler.
func (p *Platform) fontZoomKey(sym sdl2.Keysym) bool {
	cur, def := p.fontSize, p.defaultFontSize
	if cur < 1 {
		cur = 12 // the raster base when no size was ever configured
	}
	if def < 1 {
		def = 12
	}
	size, ok := zoomTarget(sym, cur, def)
	if !ok {
		return false
	}
	p.applyFontSize(size)
	return true
}

// applyFontSize changes the live font_size on every open window at once.
// When w.window.SetSize executes, it triggers an OS window resize event.
// Because Part 2 wires the watch function to catch this size shift, our refactored
// WebGPU swapchain code adapts dynamically with zero structural friction.
func (p *Platform) applyFontSize(size int) {
	cur := p.fontSize
	if cur < 1 {
		cur = 12
	}
	if size == cur {
		return
	}
	p.fontSize = size
	if p.seed != nil {
		p.seed.SetFontSize(size)
	}
	for _, w := range p.wins {
		if w.backend == nil {
			continue
		}
		keepPx := w == p.main
		if s := w.surface; !keepPx && s != nil && s.handler != nil {
			if a, ok := s.handler.(platform.PixelAnchoredOnFontZoom); ok {
				keepPx = a.KeepPixelSizeOnFontZoom()
			}
		}
		if keepPx {
			w.backend.SetFontSize(size)
			if s := w.surface; s != nil && s.handler != nil {
				s.handler.Resized(w.backend.Size())
				p.presentWindow(w, true)
				s.dirty.Store(false)
			}
			continue
		}
		units := w.backend.Size()
		w.backend.SetFontSize(size)
		wPx := w.backend.UnitToPxX(units.Width)
		hPx := w.backend.UnitToPxY(units.Height)
		if w.window != nil {
			w.window.SetSize(int32(wPx), int32(hPx))
		}
		if err := p.sizeFramebuffer(w, wPx, hPx); err != nil {
			continue
		}
		w.applyShape()
		if s := w.surface; s != nil && s.handler != nil {
			s.handler.Resized(w.backend.Size())
			p.presentWindow(w, true)
			s.dirty.Store(false)
		}
	}
}

// surfaceFor routes an event's window ID to its surface mapping wrapper.
func (p *Platform) surfaceFor(id uint32) *sdlSurface {
	if w, ok := p.wins[id]; ok {
		return w.surface
	}
	return nil
}

// pumpEvents drains SDL's hardware queue into the per-window surface handlers.
// It maps system pixel dimensions to abstract core toolkit units dynamically.
func (p *Platform) pumpEvents() bool {
	delivered := false
	for {
		ev := sdl2.PollEvent()
		if ev == nil {
			return delivered
		}
		delivered = true
		switch e := ev.(type) {
		case *sdl2.QuitEvent:
			if s := p.mainSurface(); s != nil && s.handler != nil {
				s.handler.Event(core.QuitEvent{})
			}
		case *sdl2.WindowEvent:
			s := p.surfaceFor(e.WindowID)
			if s == nil || s.handler == nil {
				continue
			}
			switch e.Event {
			case sdl2.WINDOWEVENT_SIZE_CHANGED:
				// Handled automatically via our event watch hook in Part 2.
				// This acts as a reliable, idempotent backstop fallback.
				p.liveResize(e.WindowID, int(e.Data1), int(e.Data2))
			case sdl2.WINDOWEVENT_FOCUS_GAINED:
				s.handler.Event(core.FocusEvent{Focused: true})
				s.Invalidate(core.UnitRect{})
			case sdl2.WINDOWEVENT_FOCUS_LOST:
				s.handler.Event(core.FocusEvent{Focused: false})
				s.Invalidate(core.UnitRect{})
			case sdl2.WINDOWEVENT_LEAVE:
				// Pointer left the active boundary box: clear hover affordances.
				s.handler.Event(core.MouseLeaveEvent{})
				s.Invalidate(core.UnitRect{})
			}
		case *sdl2.TextInputEvent:
			s := p.surfaceFor(e.WindowID)
			if s == nil || s.handler == nil {
				continue
			}
			text := e.GetText()
			for _, ch := range text {
				// On macOS, handle native Option key shortcuts by mapping them
				// back into clear "M-key" syntax to ensure uniformity across environments.
				if runtime.GOOS == "darwin" {
					if decoded, ok := decodeMacOSOptionChar(ch); ok {
						mods, name := core.ParseKeyModifiers(decoded)
						t := ""
						if len(name) == 1 && name[0] >= 32 && name[0] < 127 {
							t = name
						}
						s.handler.Event(core.KeyPressEvent{Key: decoded, Modifiers: mods, Text: t})
						continue
					}
				}
				s.handler.Event(core.KeyPressEvent{
					Key:  string(ch),
					Text: string(ch),
				})
			}
		case *sdl2.KeyboardEvent:
			s := p.surfaceFor(e.WindowID)
			if s == nil || s.handler == nil {
				continue
			}
			if e.Type == sdl2.KEYDOWN {
				// Check for rotation trigger (R key) - toggles on/off.
				// Only supported by renderers with rotation capability
				// (WebGPU); works in plain-present AND compositor modes.
				if e.Keysym.Sym == sdl2.K_r && p.renderer.SupportsFeature(FeatureRotation) {
					enabled := !p.rotationEnabled.Load()
					p.rotationEnabled.Store(enabled)

					// The Platform keeps its own copy of the animation clock
					// for mouse-coordinate rotation compensation.
					if enabled {
						p.rotationActivationTime = time.Now()
						p.rotationStartTime = time.Now()
					} else {
						// Store current angle for smooth ease-out
						elapsed := time.Since(p.rotationStartTime).Seconds()
						timeSinceActivation := time.Since(p.rotationActivationTime).Seconds()
						if timeSinceActivation > 1.0 { // After ease-in completes
							easeOutCubic := func(t float64) float64 {
								t = math.Min(t, 1.0)
								return 1.0 - math.Pow(1.0-t, 3.0)
							}
							rotationProgress := timeSinceActivation / 1.0
							rotationEased := easeOutCubic(rotationProgress)
							p.rotationAngleAtDeactivation = elapsed * 0.1 * rotationEased
						}
						p.rotationDeactivationTime = time.Now()
					}

					p.renderer.SetRotationEnabled(enabled)

					// The animation needs frames even while input is idle.
					s.Invalidate(core.UnitRect{})
				}
				
				if p.fontZoomKey(e.Keysym) {
					continue // Consumed by host zoom controller, skip dispatching
				}
				if key := translateKey(e.Keysym); key != "" {
					mods, name := core.ParseKeyModifiers(key)
					text := ""
					if len(name) == 1 && name[0] >= 32 && name[0] < 127 {
						text = name
					}
					s.handler.Event(core.KeyPressEvent{Key: key, Modifiers: mods, Text: text})
				}
			} else if e.Type == sdl2.KEYUP {
				// Report release actions back to tracking vectors using the modifier
				// states parsed immediately AFTER the key release event completes.
				s.handler.Event(core.KeyReleaseEvent{
					Key:       translateKey(e.Keysym),
					Modifiers: currentKeyModifiers(),
				})
			}
		case *sdl2.MouseButtonEvent:
			s := p.surfaceFor(e.WindowID)
			if s == nil || s.handler == nil {
				continue
			}
			btn := mapButton(e.Button)
			x, y := p.toUnits(e.X, e.Y, e.WindowID)
			mods := currentKeyModifiers()
			if e.Type == sdl2.MOUSEBUTTONDOWN {
				// Enable pointer capturing so dragging actions extend beyond window borders
				// to allow continuous, lag-free native widget tear-out gestures.
				_ = sdl2.CaptureMouse(true)
				s.handler.Event(core.MousePressEvent{X: x, Y: y, Button: btn, Modifiers: mods})
			} else {
				_ = sdl2.CaptureMouse(false)
				s.handler.Event(core.MouseReleaseEvent{X: x, Y: y, Button: btn, Modifiers: mods})
			}
		case *sdl2.MouseMotionEvent:
			s := p.surfaceFor(e.WindowID)
			if s == nil || s.handler == nil {
				continue
			}
			var held core.MouseButton
			if e.State&sdl2.ButtonLMask() != 0 {
				held = core.LeftButton
			}
			x, y := p.toUnits(e.X, e.Y, e.WindowID)
			s.handler.Event(core.MouseMoveEvent{X: x, Y: y, Buttons: held, Modifiers: currentKeyModifiers()})
		case *sdl2.MouseWheelEvent:
			s := p.surfaceFor(e.WindowID)
			if s == nil || s.handler == nil {
				continue
			}
			mx, my, _ := sdl2.GetMouseState()
			x, y := p.toUnits(mx, my, e.WindowID)
			s.handler.Event(core.MouseWheelEvent{
				X: x, Y: y,
				// Invert raw SDL wheel vectors to match standard toolkit scroll conventions
				DeltaX: int(e.X), DeltaY: -int(e.Y),
				PreciseX:  float64(e.PreciseX),
				PreciseY:  -float64(e.PreciseY),
				Modifiers: currentKeyModifiers(),
			})
		}
	}
}

// mainSurface retrieves the abstract surface binding associated with the primary workspace window.
func (p *Platform) mainSurface() *sdlSurface {
	if p.main != nil {
		return p.main.surface
	}
	return nil
}

// rootDenomination is the platform's root cell denomination (units per
// cell), matching what every surface's backend reports: the configured
// override or the default 8x16.
func (p *Platform) rootDenomination() (int, int) {
	w, h := int(p.metrics.CellWidth), int(p.metrics.CellHeight)
	if w < 1 {
		w = 8
	}
	if h < 1 {
		h = 16
	}
	return w, h
}

// cellPx is the exact integer pixel size of one root cell along an axis -
// the same value the raster backend paints with (denomination base scaled
// by font_size, ceil'd so a cell contains its glyph, then the integer
// device zoom). Must match raster.Backend.cellPx or hit-testing drifts.
func (p *Platform) cellPx(denom int) int {
	fs := p.fontSize
	if fs < 1 {
		fs = 12
	}
	n := (denom*fs + 11) / 12 // ceil(denom * fontSize/12)
	if n < 1 {
		n = 1
	}
	return n * p.scale
}

// pxToUnitAxis inverts the backend's cell-snapped forward mapping on one
// axis: whole cells map back from exact cellPx multiples, the sub-cell
// remainder from its rounded fraction. Floors toward negative infinity so
// captured-drag coordinates left/above the window stay strictly negative.
func pxToUnitAxis(px, denom, cellPx int) int {
	if cellPx < 1 {
		cellPx = 1
	}
	cells := px / cellPx
	rem := px - cells*cellPx
	if rem < 0 { // floor division for negative coordinates
		cells--
		rem += cellPx
	}
	sub := (rem*denom + cellPx/2) / cellPx // round(rem * denom/cellPx)
	return cells*denom + sub
}

// toUnits converts window-pixel mouse coordinates to abstract units,
// inverting the backend's font_size-aware, cell-snapped pixel mapping so
// hit-testing lands on the same grid the UI paints on at any font_size.
func (p *Platform) toUnits(x, y int32, windowID uint32) (core.Unit, core.Unit) {
	// Check if rotation effects are active (either easing in or easing out)
	isActive := p.rotationEnabled.Load()
	isEasingOut := false
	
	if !isActive {
		// Check if we're still in ease-out phase
		timeSinceDeactivation := time.Since(p.rotationDeactivationTime).Seconds()
		if timeSinceDeactivation < 0.5 {
			isEasingOut = true
			isActive = true // Treat as active for transformation purposes
		}
	}
	
	// Only apply rotation/scaling if enabled or easing out
	if !isActive {
		// Normal path - no transformation
		denomW, denomH := p.rootDenomination()
		ux := pxToUnitAxis(int(x), denomW, p.cellPx(denomW))
		uy := pxToUnitAxis(int(y), denomH, p.cellPx(denomH))
		return core.Unit(ux), core.Unit(uy)
	}
	
	// Get window for rotation pivot (needed for display rotation compensation)
	win, ok := p.wins[windowID]
	if !ok || win.window == nil {
		denomW, denomH := p.rootDenomination()
		ux := pxToUnitAxis(int(x), denomW, p.cellPx(denomW))
		uy := pxToUnitAxis(int(y), denomH, p.cellPx(denomH))
		return core.Unit(ux), core.Unit(uy)
	}
	
	w, h := win.window.GetSize()
	centerX := float64(w) / 2.0
	centerY := float64(h) / 2.0
	
	// Translate to center
	fx := float64(x) - centerX
	fy := float64(y) - centerY
	
	easeOutCubic := func(t float64) float64 {
		t = math.Min(t, 1.0)
		return 1.0 - math.Pow(1.0-t, 3.0)
	}
	
	easeInOutCubic := func(t float64) float64 {
		if t < 0.5 {
			return 4.0 * t * t * t
		}
		return 1.0 - math.Pow(-2.0*t+2.0, 3.0)/2.0
	}
	
	var currentScale float64
	var angle float64
	
	if isEasingOut {
		// Easing out - continue forward with speedup
		timeSinceDeactivation := time.Since(p.rotationDeactivationTime).Seconds()
		scaleProgress := timeSinceDeactivation / 0.5
		scaleEased := easeOutCubic(scaleProgress)
		currentScale = 2.0 - scaleEased*1.0 // 2.0 -> 1.0
		
		// Match shader: continue forward with accelerated catch-up, capped at target
		currentAngle := p.rotationAngleAtDeactivation
		twoPi := 2.0 * math.Pi
		normalizedAngle := math.Mod(currentAngle, twoPi)
		if normalizedAngle < 0 {
			normalizedAngle += twoPi
		}
		angleRemaining := twoPi - normalizedAngle
		
		normalRotation := timeSinceDeactivation * 0.1
		catchUpProgress := math.Min(timeSinceDeactivation / 0.5, 1.0)
		catchUpEased := easeInOutCubic(catchUpProgress)
		catchUpRotation := angleRemaining * catchUpEased
		
		targetAngle := currentAngle + angleRemaining
		currentRotatedAngle := currentAngle + normalRotation + catchUpRotation
		if currentRotatedAngle > targetAngle {
			currentRotatedAngle = targetAngle
		}
		
		angle = -currentRotatedAngle // Negative to match shader
	} else {
		// Easing in / active
		timeSinceActivation := time.Since(p.rotationActivationTime).Seconds()
		scaleProgress := timeSinceActivation / 0.5
		scaleEased := easeOutCubic(scaleProgress)
		currentScale = 1.0 + scaleEased*1.0 // 1.0 -> 2.0
		
		rotationProgress := timeSinceActivation / 1.0
		rotationEased := easeOutCubic(rotationProgress)
		elapsed := time.Since(p.rotationStartTime).Seconds()
		angle = -(elapsed * 0.1 * rotationEased) // Negative to match shader
	}
	
	// Scale by current scale to match the content scale
	fx *= currentScale
	fy *= currentScale
	
	// Rotate
	cosA := math.Cos(angle)
	sinA := math.Sin(angle)
	
	rotatedX := fx*cosA - fy*sinA
	rotatedY := fx*sinA + fy*cosA
	
	// Translate back
	finalX := rotatedX + centerX
	finalY := rotatedY + centerY
	
	denomW, denomH := p.rootDenomination()
	ux := pxToUnitAxis(int(finalX), denomW, p.cellPx(denomW))
	uy := pxToUnitAxis(int(finalY), denomH, p.cellPx(denomH))
	return core.Unit(ux), core.Unit(uy)
}

// currentKeyModifiers translates SDL's live modifier state for mouse
// events (Shift+click bypasses terminal mouse reporting, Shift+wheel
// scrolls horizontally).
func currentKeyModifiers() core.KeyModifiers {
	var mods core.KeyModifiers
	state := sdl2.GetModState()
	if state&sdl2.KMOD_SHIFT != 0 {
		mods |= core.ShiftModifier
	}
	if state&sdl2.KMOD_CTRL != 0 {
		mods |= core.ControlModifier
	}
	if state&sdl2.KMOD_ALT != 0 {
		mods |= core.AltModifier
	}
	if state&sdl2.KMOD_GUI != 0 {
		mods |= core.MetaModifier
	}
	return mods
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
	gui := sym.Mod&sdl2.KMOD_GUI != 0

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
		if gui {
			prefix += "s-"
		}
		return prefix + name
	}

	// Letters and printable symbols.
	if sym.Sym >= 32 && sym.Sym < 127 {
		ch := rune(sym.Sym)
		isLetter := ch >= 'a' && ch <= 'z'

		// Control-punctuation combinations that produce C0 control
		// bytes on a terminal keep their caret spellings so key
		// strings match the TUI backend (byte 0x1C = "^\\", etc.).
		// SDL keycodes are unshifted, hence the shifted trio for the
		// US-layout ^, _, and @ positions.
		if ctrl {
			name := ""
			switch {
			case ch == '\\':
				name = "^\\"
			case ch == ']':
				name = "^]"
			case ch == '[':
				name = "Escape"
			case ch == ' ':
				name = "^@"
			case shift && ch == '6':
				name = "^^"
			case shift && ch == '-':
				name = "^_"
			case ch == '/':
				name = "^_" // Ctrl+/ collapses onto ^_ (byte 0x1F), the terminal
				// convention (xterm), so Ctrl+/ reaches a terminal app instead of
				// being dropped. Without this the unshifted-punctuation path names
				// it "C-/", which purfecterm's key encoder has no byte for.
			case shift && ch == '2':
				name = "^@"
			}
			if name != "" {
				if alt {
					return "M-" + name
				}
				return name
			}
		}

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
			// On macOS a bare Option+printable composes a character that
			// SDL also delivers via TextInput, where we decode it back to
			// M-key (see the TextInputEvent handler). Defer to that path so
			// the shortcut fires exactly once; elsewhere Alt is a plain Meta
			// modifier and TextInput carries nothing, so emit M-key here.
			if runtime.GOOS == "darwin" {
				return ""
			}
			return "M-" + string(ch)
		case gui:
			// Command-modified printables never arrive via TextInput;
			// "s-" is the toolkit's Meta/Cmd prefix.
			prefix := ""
			if ctrl {
				prefix += "C-"
			}
			if shift {
				prefix += "S-"
			}
			return prefix + "s-" + string(ch)
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

// SupportsMultipleSurfaces implements platform.MultiSurfacePlatform.
func (p *Platform) SupportsMultipleSurfaces() bool { return true }

// GlobalPointerPx implements platform.GlobalPointerPlatform.
func (p *Platform) GlobalPointerPx() (int, int) {
	x, y, _ := sdl2.GetGlobalMouseState()
	return int(x), int(y)
}

// CreateSurface implements platform.Platform: the first surface binds
// the main window; each further call opens another OS window (G4
// granting - torn-off desktop windows, native-mode windows).
func (p *Platform) CreateSurface(opts platform.SurfaceOptions) (platform.Surface, error) {
	if p.main == nil {
		return nil, fmt.Errorf("sdl platform: not running")
	}
	if p.main.surface == nil {
		p.main.surface = &sdlSurface{platform: p, win: p.main}
		if opts.Title != "" {
			p.main.window.SetTitle(opts.Title)
		}
		return p.main.surface, nil
	}

	wPx, hPx := opts.WidthPx, opts.HeightPx
	if wPx <= 0 || hPx <= 0 {
		wPx, hPx = 640, 480
	}
	x, y := int32(opts.XPx), int32(opts.YPx)
	if opts.XPx == 0 && opts.YPx == 0 {
		x, y = sdl2.WINDOWPOS_CENTERED, sdl2.WINDOWPOS_CENTERED
	}
	flags := uint32(sdl2.WINDOW_SHOWN)
	if opts.Borderless {
		flags |= sdl2.WINDOW_BORDERLESS
	} else {
		flags |= sdl2.WINDOW_RESIZABLE
	}
	radius := 0
	if opts.Borderless {
		radius = opts.CornerRadiusPx
	}

	// Prevent secondary torn-out windows from stealing active focus mid-drag sessions
	_ = sdl2.SetHint("SDL_WINDOW_NO_ACTIVATION_WHEN_SHOWN", "1")

	// Spawns a native window and sets up its independent WebGPU surface swapchain contexts
	w, err := p.createWindow(opts.Title, x, y, wPx, hPx, flags, radius)
	if err != nil {
		return nil, err
	}
	w.surface = &sdlSurface{platform: p, win: w}
	if opts.Borderless {
		makeWindowMiniaturizable(w.window)
	}
	reassertCapture()
	return w.surface, nil
}

// reassertCapture re-enables mouse capture if a button is still held:
// SDL can silently drop capture when windows are created or destroyed
// mid-gesture, after which it CLAMPS motion coordinates to the window
// rect - the tear-off drag would fence itself in.
func reassertCapture() {
	if _, _, state := sdl2.GetGlobalMouseState(); state&sdl2.ButtonLMask() != 0 {
		_ = sdl2.CaptureMouse(true)
	}
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

// SetCursor implements platform.CursorController: set the application's
// system mouse cursor. System cursors are created on demand and cached;
// redundant sets (same shape) are skipped.
func (p *Platform) SetCursor(shape core.CursorShape) {
	if p.cursors == nil {
		p.cursors = map[core.CursorShape]*sdl2.Cursor{}
	}
	cur, ok := p.cursors[shape]
	if !ok {
		cur = sdl2.CreateSystemCursor(systemCursorID(shape))
		p.cursors[shape] = cur
	}
	if cur == nil {
		return
	}
	sdl2.SetCursor(cur)
	p.cursorSet = true
}

func (p *Platform) reassertCursor() {
	if p.cursorSet {
		sdl2.SetCursor(nil)
	}
}

// systemCursorID maps a core cursor shape to its SDL system cursor.
func systemCursorID(shape core.CursorShape) sdl2.SystemCursor {
	switch shape {
	case core.CursorText:
		return sdl2.SYSTEM_CURSOR_IBEAM
	case core.CursorResizeH:
		return sdl2.SYSTEM_CURSOR_SIZEWE
	case core.CursorResizeV:
		return sdl2.SYSTEM_CURSOR_SIZENS
	case core.CursorResizeNWSE:
		return sdl2.SYSTEM_CURSOR_SIZENWSE
	case core.CursorResizeNESW:
		return sdl2.SYSTEM_CURSOR_SIZENESW
	default:
		return sdl2.SYSTEM_CURSOR_ARROW
	}
}

// sdlSurface is one SDL window mapped onto a hardware GoGPU/WebGPU presentation layout.
type sdlSurface struct {
	platform *Platform
	win      *nativeWin
	handler  platform.SurfaceHandler
	dirty    atomic.Bool
	closed   bool

	// Damage accumulated since the last present: a full-surface flag (an empty
	// Invalidate, the default) or the union of bounded regions. A bounded frame
	// repaints (and re-uploads) only that rectangle.
	damageMu   sync.Mutex
	damageFull bool
	damageRect core.UnitRect
}

func (s *sdlSurface) Size() core.UnitSize {
	return s.win.backend.Size()
}
func (s *sdlSurface) Metrics() core.CellMetrics {
	return s.win.backend.Metrics()
}
func (s *sdlSurface) SetHandler(h platform.SurfaceHandler) { s.handler = h }

// Invalidate marks the surface dirty and accumulates damage regions for optimized sub-texture streaming.
func (s *sdlSurface) Invalidate(r core.UnitRect) {
	s.damageMu.Lock()
	if r.Width <= 0 || r.Height <= 0 {
		s.damageFull = true
	} else if !s.damageFull {
		s.damageRect = unionUnitRect(s.damageRect, r)
	}
	s.damageMu.Unlock()
	s.dirty.Store(true)
}

// takeDamage reads and clears the accumulated damage.
func (s *sdlSurface) takeDamage() (full bool, rect core.UnitRect) {
	s.damageMu.Lock()
	full, rect = s.damageFull, s.damageRect
	s.damageFull = false
	s.damageRect = core.UnitRect{}
	s.damageMu.Unlock()
	return full, rect
}

// unionUnitRect returns the smallest rectangle covering both; a degenerate
// operand is treated as the other.
func unionUnitRect(a, b core.UnitRect) core.UnitRect {
	if a.Width <= 0 || a.Height <= 0 {
		return b
	}
	if b.Width <= 0 || b.Height <= 0 {
		return a
	}
	x0, y0 := a.X, a.Y
	if b.X < x0 {
		x0 = b.X
	}
	if b.Y < y0 {
		y0 = b.Y
	}
	x1, y1 := a.X+a.Width, a.Y+a.Height
	if b.X+b.Width > x1 {
		x1 = b.X + b.Width
	}
	if b.Y+b.Height > y1 {
		y1 = b.Y + b.Height
	}
	return core.UnitRect{X: x0, Y: y0, Width: x1 - x0, Height: y1 - y0}
}
func (s *sdlSurface) SetCursorVisible(bool)            {}
func (s *sdlSurface) SetCursorPosition(x, y core.Unit) {}
func (s *sdlSurface) SetCursorStyle(int)               {}

// ScreenPositionPx implements platform.NativeSurface.
func (s *sdlSurface) ScreenPositionPx() (int, int) {
	if s.closed || s.win.window == nil {
		return 0, 0
	}
	x, y := s.win.window.GetPosition()
	return int(x), int(y)
}

// SetScreenPositionPx implements platform.NativeSurface.
func (s *sdlSurface) SetScreenPositionPx(x, y int) {
	if s.closed || s.win.window == nil {
		return
	}
	s.win.window.SetPosition(int32(x), int32(y))
}

// SetBordered implements platform.BorderToggler: toggle the OS title bar
// at runtime (solo mode strips the primary window's border so the app's
// own chrome is the only title bar).
func (s *sdlSurface) SetBordered(bordered bool) {
	if s.closed || s.win == nil || s.win.window == nil {
		return
	}
	s.win.window.SetBordered(bordered)
}

// ScreenSizePx implements platform.NativeSurface: the OS window's current
// pixel size, straight from SDL (no unit round-trip that would drift at
// fractional pixels-per-unit).
func (s *sdlSurface) ScreenSizePx() (int, int) {
	if s.closed || s.win.window == nil {
		return 0, 0
	}
	w, h := s.win.window.GetSize()
	return int(w), int(h)
}

// SetScreenSizePx implements platform.NativeSurface: the size change
// reports back through the WINDOWEVENT_SIZE_CHANGED path (framebuffer
// recreate, shape reapply, handler.Resized).
func (s *sdlSurface) SetScreenSizePx(w, h int) {
	if s.closed || s.win.window == nil || w <= 0 || h <= 0 {
		return
	}
	s.win.window.SetSize(int32(w), int32(h))
}

// WorkAreaPx implements platform.NativeSurface: the usable bounds of
// the display the window occupies (the macOS option-zoom target).
func (s *sdlSurface) WorkAreaPx() (int, int, int, int) {
	if s.closed || s.win.window == nil {
		return 0, 0, 0, 0
	}
	idx, err := s.win.window.GetDisplayIndex()
	if err != nil {
		idx = 0
	}
	r, err := sdl2.GetDisplayUsableBounds(idx)
	if err != nil {
		return 0, 0, 0, 0
	}
	return int(r.X), int(r.Y), int(r.W), int(r.H)
}

// applyShape rounds the OS window's corners with a binary alpha mask
// so the pixels outside the drawn roundrect frame are not opaque black.
func (w *nativeWin) applyShape() {
	if w.shapeRadiusPx <= 0 || w.window == nil {
		return
	}
	wPx, hPx := w.window.GetSize()
	if wPx <= 0 || hPx <= 0 {
		return
	}
	mask, err := sdl2.CreateRGBSurfaceWithFormat(0, wPx, hPx, 32, uint32(sdl2.PIXELFORMAT_ARGB8888))
	if err != nil {
		return
	}
	defer mask.Free()
	_ = mask.FillRect(nil, 0xffffffff)
	pix := mask.Pixels()
	pitch := int(mask.Pitch)
	r := w.shapeRadiusPx
	if m := int(min32(wPx, hPx)) / 2; r > m {
		r = m
	}
	rf := float64(r)
	clear := func(x, y int) {
		off := y*pitch + x*4
		pix[off], pix[off+1], pix[off+2], pix[off+3] = 0, 0, 0, 0
	}
	for j := 0; j < r; j++ {
		for i := 0; i < r; i++ {
			dx := rf - float64(i) - 0.5
			dy := rf - float64(j) - 0.5
			if dx*dx+dy*dy > rf*rf {
				clear(i, j)
				clear(int(wPx)-1-i, j)
				clear(i, int(hPx)-1-j)
				clear(int(wPx)-1-i, int(hPx)-1-j)
			}
		}
	}
	// BinarizeAlpha with a mid cutoff, matching the known-good SDL
	// shaped-window configuration (the mask is fully opaque inside and
	// fully clear outside, so the exact cutoff only needs to sit
	// between them).
	// A non-zero result is one of SDL's shape errors (most likely
	// NONSHAPEABLE_WINDOW: the window was not created shaped).
	if rc := w.window.SetShape(mask, sdl2.ShapeModeBinarizeAlpha{Cutoff: 1 << 6}); rc != 0 {
		fmt.Fprintf(os.Stderr, "WARNING: SetShape failed (rc=%d, window not shapeable?)\n", rc)
	}
}

func min32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

// Minimize implements platform.NativeSurface.
func (s *sdlSurface) Minimize() {
	if s.closed || s.win.window == nil {
		return
	}
	s.win.window.Minimize()
}

// Restore implements platform.NativeRestorer: un-minimizes (and unhides)
// the OS window so the desktop's "Show All" can bring torn windows back.
func (s *sdlSurface) Restore() {
	if s.closed || s.win.window == nil {
		return
	}
	s.win.window.Restore()
}

// Minimized implements platform.NativeSurface.
func (s *sdlSurface) Minimized() bool {
	if s.closed || s.win.window == nil {
		return true
	}
	flags := s.win.window.GetFlags()
	return flags&sdl2.WINDOW_MINIMIZED != 0 || flags&sdl2.WINDOW_HIDDEN != 0
}

// SetOpacity implements platform.NativeSurface.
func (s *sdlSurface) SetOpacity(opacity float64) {
	if s.closed || s.win.window == nil {
		return
	}
	_ = s.win.window.SetWindowOpacity(float32(opacity))
}

// Raise implements platform.NativeSurface: brings the OS window to the
// front and gives it input focus.
func (s *sdlSurface) Raise() {
	if s.closed || s.win.window == nil {
		return
	}
	s.win.window.Raise()
}

// Close implements platform.NativeSurface: destroys the OS window.
// It marshals the destruction onto the main thread via Post to avoid data races.
func (s *sdlSurface) Close() {
	if s.win == s.platform.main {
		return // Never destroy the primary manager shell layout
	}
	p := s.platform
	p.Post(func() {
		if s.closed {
			return
		}
		s.closed = true
		s.handler = nil
		if cur, ok := p.wins[s.win.id]; ok && cur == s.win {
			delete(p.wins, s.win.id)
		}
		s.win.destroy() // Calls our updated WebGPU FFI surface teardown macro smoothly
		reassertCapture()
	})
}
