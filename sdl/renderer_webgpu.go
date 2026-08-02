//go:build sdl && webgpu

package sdl

import (
	"fmt"
	"reflect"
	"time"
	"unsafe"

	wgpu "github.com/gogpu/wgpu"
	gputypes "github.com/gogpu/gputypes"
	_ "github.com/gogpu/wgpu/hal/allbackends"

	"github.com/phroun/kittytk/backend/raster"
	"github.com/phroun/kittytk/core"
	"github.com/phroun/kittytk/platform"
)

// WindowSurface holds GPU resources for a single window's off-screen rendering
type WindowSurface struct {
	texture     *wgpu.Texture
	textureView *wgpu.TextureView
	bindGroup   *wgpu.BindGroup
	width       uint32
	height      uint32
	
	// Per-window positioning (NDC coordinates)
	posUniformBuffer    *wgpu.Buffer
	posUniformBindGroup *wgpu.BindGroup
	
	// Transform state for compositing
	translateX  float32
	translateY  float32
	rotation    float32 // radians
	scaleX      float32
	scaleY      float32
	opacity     float32
	
	dirty       bool // needs re-render
	
	// UI Window compositor support (for child windows within an OS window)
	uiWindow    interface{}      // UI Window trinket (interface{} to avoid import cycle)
	backend     *raster.Backend  // Per-window raster backend for UI windows
	zOrder      int              // Z-order for compositing (higher = on top)
}

// WebGPURenderer implements GPU-accelerated rendering with WebGPU.
// Supports 2D transforms (rotation, scale), 3D effects, and compositing.
type WebGPURenderer struct {
	vsync bool

	// WebGPU core objects
	instance *wgpu.Instance
	adapter  *wgpu.Adapter
	device   *wgpu.Device
	queue    *wgpu.Queue

	// 2D blit pipeline (for presenting raster backend)
	blitPipeline         *wgpu.RenderPipeline
	blitSampler          *wgpu.Sampler
	blitLayout           *wgpu.BindGroupLayout
	blitUniformBuffer    *wgpu.Buffer
	blitUniformLayout    *wgpu.BindGroupLayout
	blitUniformBindGroup *wgpu.BindGroup
	blitPosLayout        *wgpu.BindGroupLayout // Per-window position uniforms

	// Rotation/scale effect state
	rotationStartTime           time.Time
	rotationActivationTime      time.Time
	rotationDeactivationTime    time.Time
	rotationAngleAtDeactivation float64
	rotationEnabled             bool
	
	// Per-window surfaces for compositing
	windowSurfaces        map[uint32]*WindowSurface // windowID -> surface
	firstCompositorFrame  bool                       // Track first compositor call

	// 3D cube rendering
	cubePipeline         *wgpu.RenderPipeline
	cubeVertexBuffer     *wgpu.Buffer
	cubeIndexBuffer      *wgpu.Buffer
	cubeUniformBuffer    *wgpu.Buffer
	cubeUniformLayout    *wgpu.BindGroupLayout
	cubeUniformBindGroup *wgpu.BindGroup
}

// NewWebGPURenderer creates a GPU-accelerated renderer
func NewWebGPURenderer(vsync bool) (Renderer, error) {
	r := &WebGPURenderer{
		vsync:                vsync,
		windowSurfaces:       make(map[uint32]*WindowSurface),
		firstCompositorFrame: true,
	}
	return r, nil
}

// Initialize sets up WebGPU context
func (r *WebGPURenderer) Initialize() error {
	// Create WebGPU instance
	instance, err := wgpu.CreateInstance(nil)
	if err != nil {
		return fmt.Errorf("failed to create WebGPU instance: %w", err)
	}
	r.instance = instance

	// Request adapter
	adapter, err := instance.RequestAdapter(&wgpu.RequestAdapterOptions{
		PowerPreference: gputypes.PowerPreferenceHighPerformance,
	})
	if err != nil {
		instance.Release()
		return fmt.Errorf("failed to request WebGPU adapter: %w", err)
	}
	r.adapter = adapter

	// Request device and queue
	device, err := adapter.RequestDevice(&wgpu.DeviceDescriptor{
		Label: "KittyTK Device",
	})
	if err != nil {
		adapter.Release()
		instance.Release()
		return fmt.Errorf("failed to request WebGPU device: %w", err)
	}
	r.device = device
	r.queue = device.Queue()

	// Initialize blit pipeline (for 2D rendering)
	if err := r.initBlitPipeline(); err != nil {
		r.Shutdown()
		return fmt.Errorf("failed to initialize blit pipeline: %w", err)
	}

	// Initialize cube pipeline (for 3D effects)
	if err := r.initCubePipeline(); err != nil {
		r.Shutdown()
		return fmt.Errorf("failed to initialize cube pipeline: %w", err)
	}

	r.rotationStartTime = time.Now()
	return nil
}

// Shutdown cleans up WebGPU resources
func (r *WebGPURenderer) Shutdown() {
	// Clean up cube resources
	if r.cubeUniformBindGroup != nil {
		r.cubeUniformBindGroup.Release()
	}
	if r.cubeUniformBuffer != nil {
		r.cubeUniformBuffer.Release()
	}
	if r.cubeIndexBuffer != nil {
		r.cubeIndexBuffer.Release()
	}
	if r.cubeVertexBuffer != nil {
		r.cubeVertexBuffer.Release()
	}
	if r.cubePipeline != nil {
		r.cubePipeline.Release()
	}
	if r.cubeUniformLayout != nil {
		r.cubeUniformLayout.Release()
	}

	// Clean up blit resources
	if r.blitUniformBindGroup != nil {
		r.blitUniformBindGroup.Release()
	}
	if r.blitUniformBuffer != nil {
		r.blitUniformBuffer.Release()
	}
	if r.blitUniformLayout != nil {
		r.blitUniformLayout.Release()
	}
	if r.blitLayout != nil {
		r.blitLayout.Release()
	}
	if r.blitSampler != nil {
		r.blitSampler.Release()
	}
	if r.blitPipeline != nil {
		r.blitPipeline.Release()
	}

	// Clean up core objects
	if r.device != nil {
		r.device.Release()
	}
	if r.adapter != nil {
		r.adapter.Release()
	}
	if r.instance != nil {
		r.instance.Release()
	}
}

// CreateWindowRenderer creates WebGPU surface and resources for a window
func (r *WebGPURenderer) CreateWindowRenderer(w *nativeWin, pxW, pxH int) error {
	// Create off-screen texture for this window
	texture, err := r.device.CreateTexture(&wgpu.TextureDescriptor{
		Size:          wgpu.Extent3D{Width: uint32(pxW), Height: uint32(pxH), DepthOrArrayLayers: 1},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     wgpu.TextureDimension2D,
		Format:        wgpu.TextureFormatBGRA8Unorm,
		Usage:         wgpu.TextureUsageRenderAttachment | wgpu.TextureUsageTextureBinding,
	})
	if err != nil {
		return fmt.Errorf("failed to create window texture: %w", err)
	}

	// Create texture view
	textureView, err := r.device.CreateTextureView(texture, nil)
	if err != nil {
		texture.Release()
		return fmt.Errorf("failed to create texture view: %w", err)
	}

	// Create bind group for compositing
	bindGroup, err := r.device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Layout: r.blitLayout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, TextureView: textureView},
			{Binding: 1, Sampler: r.blitSampler},
		},
	})
	if err != nil {
		textureView.Release()
		texture.Release()
		return fmt.Errorf("failed to create bind group: %w", err)
	}

	// Create WindowSurface and store it
	surf := &WindowSurface{
		texture:     texture,
		textureView: textureView,
		bindGroup:   bindGroup,
		width:       uint32(pxW),
		height:      uint32(pxH),
		translateX:  0,
		translateY:  0,
		rotation:    0,
		scaleX:      1.0,
		scaleY:      1.0,
		opacity:     1.0,
		dirty:       true,
	}

	r.windowSurfaces[w.id] = surf
	return nil
}

// DestroyWindowRenderer cleans up WebGPU resources for a window
func (r *WebGPURenderer) DestroyWindowRenderer(w *nativeWin) {
	surf, ok := r.windowSurfaces[w.id]
	if !ok {
		return
	}

	// Release GPU resources
	if surf.bindGroup != nil {
		surf.bindGroup.Release()
	}
	if surf.textureView != nil {
		surf.textureView.Release()
	}
	if surf.texture != nil {
		surf.texture.Release()
	}

	delete(r.windowSurfaces, w.id)
}

// ResizeWindowRenderer adjusts WebGPU resources when window size changes
func (r *WebGPURenderer) ResizeWindowRenderer(w *nativeWin, pxW, pxH int) error {
	// Destroy old resources
	r.DestroyWindowRenderer(w)

	// Create new resources at new size
	return r.CreateWindowRenderer(w, pxW, pxH)
}

// Present renders using WebGPU pipeline
func (r *WebGPURenderer) Present(w *nativeWin, backend *raster.Backend) error {
	img := backend.Image()
	if img == nil {
		return fmt.Errorf("backend image is nil")
	}
	
	// Upload backend to temporary texture and blit to screen
	texture, textureView, bindGroup, err := r.uploadBackendToTexture(backend)
	if err != nil {
		return err
	}
	// NOTE: Don't defer release - must stay alive until after GPU Submit()
	
	// Create fullscreen position uniforms
	posBuffer, posBindGroup, err := r.createFullscreenPositionUniforms()
	if err != nil {
		texture.Release()
		textureView.Release()
		bindGroup.Release()
		return err
	}
	// NOTE: Don't defer release - must stay alive until after GPU Submit()
	
	// Get surface texture
	surfaceTexture, _, err := w.gpuSurface.GetCurrentTexture()
	if err != nil {
		posBindGroup.Release()
		posBuffer.Release()
		bindGroup.Release()
		textureView.Release()
		texture.Release()
		return err
	}
	
	surfaceView, err := surfaceTexture.CreateView(nil)
	if err != nil {
		posBindGroup.Release()
		posBuffer.Release()
		bindGroup.Release()
		textureView.Release()
		texture.Release()
		return err
	}
	
	// Create command encoder
	encoder, err := r.device.CreateCommandEncoder(nil)
	if err != nil {
		surfaceView.Release()
		posBindGroup.Release()
		posBuffer.Release()
		bindGroup.Release()
		textureView.Release()
		texture.Release()
		return err
	}
	
	// Begin render pass
	renderPass, _ := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		ColorAttachments: []wgpu.RenderPassColorAttachment{
			{
				View:    surfaceView,
				LoadOp:  gputypes.LoadOpClear,
				StoreOp: gputypes.StoreOpStore,
				ClearValue: wgpu.Color{R: 0.0, G: 0.0, B: 0.0, A: 1.0},
			},
		},
	})
	
	// Draw fullscreen quad with backend texture
	renderPass.SetPipeline(r.blitPipeline)
	renderPass.SetBindGroup(0, bindGroup, nil)
	renderPass.SetBindGroup(1, r.blitUniformBindGroup, nil)
	renderPass.SetBindGroup(2, posBindGroup, nil)
	renderPass.Draw(6, 1, 0, 0) // Draw quad
	renderPass.End()
	
	// Submit and present
	cmdBuffer, _ := encoder.Finish()
	_, err = r.queue.Submit(cmdBuffer)
	
	// NOW we can release resources after GPU has the commands
	surfaceView.Release()
	posBindGroup.Release()
	posBuffer.Release()
	bindGroup.Release()
	textureView.Release()
	texture.Release()
	
	if err != nil {
		return err
	}
	w.gpuSurface.Present(surfaceTexture)
	
	return nil
}

// ApplyWindowShape applies rounded corners (WebGPU uses fragment shader clipping)
func (r *WebGPURenderer) ApplyWindowShape(w *nativeWin, radiusPx int, transparent bool) error {
	// WebGPU doesn't need OS-level window shaping - handled in fragment shader
	return nil
}

// SupportsFeature checks WebGPU renderer capabilities
func (r *WebGPURenderer) SupportsFeature(feature RendererFeature) bool {
	switch feature {
	case FeatureRotation, FeatureScale, Feature3DCube, FeatureCompositing:
		return true
	default:
		return false
	}
}

// initBlitPipeline creates the 2D rendering pipeline
func (r *WebGPURenderer) initBlitPipeline() error {
	// Create shader modules
	vertexShader, err := r.device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label: "Blit Vertex Shader",
		WGSL:  blitVertexShader,
	})
	if err != nil {
		return fmt.Errorf("failed to create vertex shader: %w", err)
	}
	defer vertexShader.Release()

	fragmentShader, err := r.device.CreateShaderModule(&wgpu.ShaderModuleDescriptor{
		Label: "Blit Fragment Shader",
		WGSL:  blitFragmentShader,
	})
	if err != nil {
		return fmt.Errorf("failed to create fragment shader: %w", err)
	}
	defer fragmentShader.Release()

	// Create sampler
	r.blitSampler, err = r.device.CreateSampler(&wgpu.SamplerDescriptor{
		AddressModeU: 2, // ClampToEdge
		AddressModeV: 2,
		MagFilter:    1, // Linear
		MinFilter:    1,
	})
	if err != nil {
		return fmt.Errorf("failed to create sampler: %w", err)
	}

	// Create uniform buffer for rotation angle + enabled flag + scale
	r.blitUniformBuffer, err = r.device.CreateBuffer(&wgpu.BufferDescriptor{
		Size:  12,
		Usage: wgpu.BufferUsageUniform | wgpu.BufferUsageCopyDst,
	})
	if err != nil {
		return fmt.Errorf("failed to create uniform buffer: %w", err)
	}

	// Create uniform bind group layout
	r.blitUniformLayout, err = r.device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Entries: []gputypes.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: wgpu.ShaderStageFragment,
				Buffer: &gputypes.BufferBindingLayout{
					Type:             0, // Uniform
					MinBindingSize:   12,
					HasDynamicOffset: false,
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create uniform bind group layout: %w", err)
	}

	// Create bind group layout for texture + sampler
	r.blitLayout, err = r.device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Entries: []gputypes.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: wgpu.ShaderStageFragment,
				Texture: &gputypes.TextureBindingLayout{
					SampleType:    1, // Float
					ViewDimension: 2, // 2D
				},
			},
			{
				Binding:    1,
				Visibility: wgpu.ShaderStageFragment,
				Sampler: &gputypes.SamplerBindingLayout{
					Type: 1, // Filtering
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create bind group layout: %w", err)
	}

	// Create bind group layout for per-window position uniforms (NDC coordinates)
	r.blitPosLayout, err = r.device.CreateBindGroupLayout(&wgpu.BindGroupLayoutDescriptor{
		Entries: []gputypes.BindGroupLayoutEntry{
			{
				Binding:    0,
				Visibility: wgpu.ShaderStageVertex,
				Buffer: &gputypes.BufferBindingLayout{
					Type:             0, // Uniform
					MinBindingSize:   16, // vec4: x, y, width, height in NDC
					HasDynamicOffset: false,
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create position uniform layout: %w", err)
	}

	// Create pipeline layout with 3 bind groups: texture, effects, position
	pipelineLayout, err := r.device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		BindGroupLayouts: []*wgpu.BindGroupLayout{r.blitLayout, r.blitUniformLayout, r.blitPosLayout},
	})
	if err != nil {
		return fmt.Errorf("failed to create pipeline layout: %w", err)
	}
	defer pipelineLayout.Release()

	// Create render pipeline
	r.blitPipeline, err = r.device.CreateRenderPipeline(&wgpu.RenderPipelineDescriptor{
		Layout: pipelineLayout,
		Vertex: wgpu.VertexState{
			Module:     vertexShader,
			EntryPoint: "vs_main",
		},
		Fragment: &wgpu.FragmentState{
			Module:     fragmentShader,
			EntryPoint: "fs_main",
			Targets: []wgpu.ColorTargetState{
				{
					Format:    wgpu.TextureFormatBGRA8Unorm,
					WriteMask: 0xF,
				},
			},
		},
		Primitive: wgpu.PrimitiveState{
			Topology: 3, // TriangleList
		},
		Multisample: wgpu.MultisampleState{
			Count: 1,
			Mask:  0xFFFFFFFF,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create render pipeline: %w", err)
	}

	// Create uniform bind group
	r.blitUniformBindGroup, err = r.device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Layout: r.blitUniformLayout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, Buffer: r.blitUniformBuffer, Size: 12},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create uniform bind group: %w", err)
	}

	return nil
}

// initCubePipeline creates the 3D cube rendering pipeline
func (r *WebGPURenderer) initCubePipeline() error {
	// TODO: Implement cube pipeline initialization
	// For now, return nil to allow WebGPU renderer to initialize
	// Cube rendering can be added back later
	return nil
}

// SetRotationEnabled toggles the 2D rotation effect
func (r *WebGPURenderer) SetRotationEnabled(enabled bool) {
	if enabled == r.rotationEnabled {
		return
	}

	now := time.Now()
	if enabled {
		r.rotationActivationTime = now
	} else {
		// Store angle at deactivation for smooth ease-out
		elapsed := now.Sub(r.rotationActivationTime).Seconds()
		if elapsed > 0.5 { // After ease-in completes
			elapsedAfterEaseIn := elapsed - 0.5
			r.rotationAngleAtDeactivation = elapsedAfterEaseIn * 0.1
		}
		r.rotationDeactivationTime = now
	}
	r.rotationEnabled = enabled
}

// SetWindowTransform sets the transform for a window
func (r *WebGPURenderer) SetWindowTransform(windowID uint32, translateX, translateY, rotation, scaleX, scaleY, opacity float32) {
	surf, ok := r.windowSurfaces[windowID]
	if !ok {
		return
	}

	surf.translateX = translateX
	surf.translateY = translateY
	surf.rotation = rotation
	surf.scaleX = scaleX
	surf.scaleY = scaleY
	surf.opacity = opacity
}

// RotationEnabled returns whether rotation effect is active
func (r *WebGPURenderer) RotationEnabled() bool {
	return r.rotationEnabled
}


// RenderFrame implements the Renderer interface for per-window compositing.
// For now, this is a simple implementation that just renders and presents.
func (r *WebGPURenderer) RenderFrame(w *nativeWin, windows []*nativeWin, renderWindow func(*nativeWin)) error {
	// Call the render callback for this window
	renderWindow(w)
	
	// Present the rendered content
	return r.Present(w, w.backend)
}


// RenderFrameWithChildWindows implements per-child-window GPU compositing.
// Each UI child window is rendered to its own GPU texture, then all textures
// are composited together with Z-ordering, transforms, and effects.
func (r *WebGPURenderer) RenderFrameWithChildWindows(
	osWindow *nativeWin,
	childWindowList *platform.ChildWindowList,
	scale int,
	renderWindow func(*nativeWin),
) error {
	if r.firstCompositorFrame {
		if childWindowList != nil {
			fmt.Printf("🎨 GPU Compositor Active - compositing %d UI child windows\n", len(childWindowList.Windows))
		}
		r.firstCompositorFrame = false
	}
	
	if childWindowList == nil || len(childWindowList.Windows) == 0 {
		// No child windows, just render normally
		renderWindow(osWindow)
		return r.Present(osWindow, osWindow.backend)
	}
	
	// Step 0: Render Desktop base layer (background, menu, status, dock - NOT windows)
	// The desktopSurfaceHandler.Frame() will call Desktop.Paint() when windows exist
	renderWindow(osWindow)
	
	// Step 1: Render each child window to its own texture
	type WindowLike interface {
		Bounds() core.UnitRect
		Paint(*core.Painter)
	}
	
	for _, childIface := range childWindowList.Windows {
		win, ok := childIface.(WindowLike)
		if !ok {
			continue
		}
		
		bounds := win.Bounds()
		if bounds.Width <= 0 || bounds.Height <= 0 {
			continue
		}
		
		// Calculate pixel dimensions
		backendImg := osWindow.backend.Image()
		if backendImg == nil {
			continue
		}
		backendSize := osWindow.backend.Size()
		metrics := osWindow.backend.Metrics()
		
		backendBounds := backendImg.Bounds()
		pixelsPerUnitW := float64(backendBounds.Dx()) / float64(backendSize.Width)
		pixelsPerUnitH := float64(backendBounds.Dy()) / float64(backendSize.Height)
		
		widthPx := int(float64(bounds.Width) * pixelsPerUnitW)
		heightPx := int(float64(bounds.Height) * pixelsPerUnitH)
		
		if widthPx <= 0 || heightPx <= 0 {
			continue
		}
		
		// Get stable window ID
		winValue := reflect.ValueOf(childIface)
		windowID := uint32(winValue.Pointer())
		surf, ok := r.windowSurfaces[windowID]
		
		if !ok || surf.backend == nil {
			// Create new surface
			backend, err := raster.NewScaled(widthPx, heightPx, scale)
			if err != nil {
				continue
			}
			backend.SetCellMetrics(metrics)
			
			// Create GPU texture
			texture, err := r.device.CreateTexture(&wgpu.TextureDescriptor{
				Usage: wgpu.TextureUsageTextureBinding | wgpu.TextureUsageCopyDst,
				Dimension: wgpu.TextureDimension2D,
				Size: wgpu.Extent3D{
					Width:              uint32(widthPx),
					Height:             uint32(heightPx),
					DepthOrArrayLayers: 1,
				},
				Format:        wgpu.TextureFormatBGRA8Unorm,
				MipLevelCount: 1,
				SampleCount:   1,
			})
			if err != nil {
				continue
			}
			
			textureView, err := r.device.CreateTextureView(texture, nil)
			if err != nil {
				texture.Release()
				continue
			}
			
			bindGroup, err := r.device.CreateBindGroup(&wgpu.BindGroupDescriptor{
				Layout: r.blitLayout,
				Entries: []wgpu.BindGroupEntry{
					{Binding: 0, TextureView: textureView},
					{Binding: 1, Sampler: r.blitSampler},
				},
			})
			if err != nil {
				textureView.Release()
				texture.Release()
				continue
			}
			
			// Create position uniforms
			surfaceSize := osWindow.backend.Size()
			posBuffer, posBindGroup, err := r.createWindowPositionUniforms(bounds, surfaceSize)
			if err != nil {
				bindGroup.Release()
				textureView.Release()
				texture.Release()
				continue
			}
			
			surf = &WindowSurface{
				texture:             texture,
				textureView:         textureView,
				bindGroup:           bindGroup,
				posUniformBuffer:    posBuffer,
				posUniformBindGroup: posBindGroup,
				width:               uint32(widthPx),
				height:              uint32(heightPx),
				uiWindow:            childIface,
				backend:             backend,
				dirty:               true,
				scaleX:              1.0,
				scaleY:              1.0,
				opacity:             1.0,
			}
			r.windowSurfaces[windowID] = surf
		}
		
		// Check if window was resized or moved
		needsUpdate := int(surf.width) != widthPx || int(surf.height) != heightPx
		if !needsUpdate {
			// Check if position changed
			if storedWin, ok := surf.uiWindow.(WindowLike); ok {
				storedBounds := storedWin.Bounds()
				needsUpdate = storedBounds.X != bounds.X || storedBounds.Y != bounds.Y
			}
		}
		
		if needsUpdate {
			surf.backend, _ = raster.NewScaled(widthPx, heightPx, scale)
			surf.backend.SetCellMetrics(metrics)
			
			if surf.texture != nil {
				surf.texture.Release()
			}
			if surf.textureView != nil {
				surf.textureView.Release()
			}
			if surf.bindGroup != nil {
				surf.bindGroup.Release()
			}
			
			texture, err := r.device.CreateTexture(&wgpu.TextureDescriptor{
				Usage: wgpu.TextureUsageTextureBinding | wgpu.TextureUsageCopyDst,
				Dimension: wgpu.TextureDimension2D,
				Size: wgpu.Extent3D{
					Width:              uint32(widthPx),
					Height:             uint32(heightPx),
					DepthOrArrayLayers: 1,
				},
				Format:        wgpu.TextureFormatBGRA8Unorm,
				MipLevelCount: 1,
				SampleCount:   1,
			})
			if err != nil {
				continue
			}
			
			textureView, err := r.device.CreateTextureView(texture, nil)
			if err != nil {
				texture.Release()
				continue
			}
			
			bindGroup, err := r.device.CreateBindGroup(&wgpu.BindGroupDescriptor{
				Layout: r.blitLayout,
				Entries: []wgpu.BindGroupEntry{
					{Binding: 0, TextureView: textureView},
					{Binding: 1, Sampler: r.blitSampler},
				},
			})
			if err != nil {
				textureView.Release()
				texture.Release()
				continue
			}
			
			// Recreate position uniforms
			if surf.posUniformBuffer != nil {
				surf.posUniformBuffer.Release()
			}
			if surf.posUniformBindGroup != nil {
				surf.posUniformBindGroup.Release()
			}
			
			surfaceSize := osWindow.backend.Size()
			posBuffer, posBindGroup, err := r.createWindowPositionUniforms(bounds, surfaceSize)
			if err != nil {
				bindGroup.Release()
				textureView.Release()
				texture.Release()
				continue
			}
			
			surf.texture = texture
			surf.textureView = textureView
			surf.bindGroup = bindGroup
			surf.posUniformBuffer = posBuffer
			surf.posUniformBindGroup = posBindGroup
			surf.width = uint32(widthPx)
			surf.height = uint32(heightPx)
			surf.dirty = true
		}
		
		// Render window to its backend
		surf.backend.BeginFrame()
		painter := core.NewPainter(surf.backend)
		win.Paint(painter)
		surf.backend.EndFrame()
		
		// Upload to GPU texture
		img := surf.backend.Image()
		if img != nil {
			bounds := img.Bounds()
			imgWidth := uint32(bounds.Dx())
			imgHeight := uint32(bounds.Dy())
			
			bytesPerPixel := uint32(4)
			bytesPerRow := imgWidth * bytesPerPixel
			alignment := uint32(256)
			alignedBytesPerRow := ((bytesPerRow + alignment - 1) / alignment) * alignment
			
			pixelData := make([]byte, alignedBytesPerRow*imgHeight)
			
			for y := uint32(0); y < imgHeight; y++ {
				srcOffset := y * uint32(img.Stride)
				dstOffset := y * alignedBytesPerRow
				
				for x := uint32(0); x < imgWidth; x++ {
					srcIdx := srcOffset + x*4
					dstIdx := dstOffset + x*4
					
					pixelData[dstIdx+0] = img.Pix[srcIdx+2] // B
					pixelData[dstIdx+1] = img.Pix[srcIdx+1] // G
					pixelData[dstIdx+2] = img.Pix[srcIdx+0] // R
					pixelData[dstIdx+3] = img.Pix[srcIdx+3] // A
				}
			}
			
			r.queue.WriteTexture(
				&wgpu.ImageCopyTexture{
					Texture:  surf.texture,
					MipLevel: 0,
					Origin:   wgpu.Origin3D{X: 0, Y: 0, Z: 0},
					Aspect:   0,
				},
				pixelData,
				&wgpu.ImageDataLayout{
					Offset:       0,
					BytesPerRow:  alignedBytesPerRow,
					RowsPerImage: imgHeight,
				},
				&wgpu.Extent3D{
					Width:              imgWidth,
					Height:             imgHeight,
					DepthOrArrayLayers: 1,
				},
			)
		}
		
		surf.dirty = false
	}
	
	// Step 2: Upload Desktop base layer
	desktopTexture, desktopView, desktopBindGroup, err := r.uploadBackendToTexture(osWindow.backend)
	if err != nil {
		return fmt.Errorf("failed to upload Desktop base: %w", err)
	}
	// DON'T defer - must stay alive until after Submit
	
	// Create fullscreen position uniforms for Desktop
	desktopPosBuffer, desktopPosBindGroup, err := r.createFullscreenPositionUniforms()
	if err != nil {
		desktopBindGroup.Release()
		desktopView.Release()
		desktopTexture.Release()
		return fmt.Errorf("failed to create Desktop position uniforms: %w", err)
	}
	// DON'T defer - must stay alive until after Submit
	
	// Step 3: Composite all textures
	surfaceTexture, _, err := osWindow.gpuSurface.GetCurrentTexture()
	if err != nil {
		return err
	}
	
	surfaceView, err := surfaceTexture.CreateView(nil)
	if err != nil {
		desktopPosBindGroup.Release()
		desktopPosBuffer.Release()
		desktopBindGroup.Release()
		desktopView.Release()
		desktopTexture.Release()
		return err
	}
	// DON'T defer - release after Submit
	
	encoder, err := r.device.CreateCommandEncoder(nil)
	if err != nil {
		return err
	}
	
	renderPass, _ := encoder.BeginRenderPass(&wgpu.RenderPassDescriptor{
		ColorAttachments: []wgpu.RenderPassColorAttachment{
			{
				View:    surfaceView,
				LoadOp:  gputypes.LoadOpClear,
				StoreOp: gputypes.StoreOpStore,
				ClearValue: wgpu.Color{R: 0.2, G: 0.2, B: 0.2, A: 1.0},
			},
		},
	})
	
	renderPass.SetPipeline(r.blitPipeline)
	
	// Draw Desktop base first (fullscreen)
	renderPass.SetBindGroup(0, desktopBindGroup, nil)
	renderPass.SetBindGroup(1, r.blitUniformBindGroup, nil)
	renderPass.SetBindGroup(2, desktopPosBindGroup, nil)
	renderPass.Draw(6, 1, 0, 0) // Draw quad (6 vertices)
	
	// Draw each child window at its position
	for _, childIface := range childWindowList.Windows {
		winValue := reflect.ValueOf(childIface)
		windowID := uint32(winValue.Pointer())
		surf, ok := r.windowSurfaces[windowID]
		if !ok {
			continue
		}
		
		// Bind texture, effects uniforms, and position uniforms
		renderPass.SetBindGroup(0, surf.bindGroup, nil)
		renderPass.SetBindGroup(1, r.blitUniformBindGroup, nil)
		renderPass.SetBindGroup(2, surf.posUniformBindGroup, nil)
		renderPass.Draw(6, 1, 0, 0) // Draw quad at window position
	}
	
	renderPass.End()
	
	cmdBuffer, _ := encoder.Finish()
	_, err = r.queue.Submit(cmdBuffer)
	
	// Release temporary resources after GPU has the commands
	surfaceView.Release()
	desktopPosBindGroup.Release()
	desktopPosBuffer.Release()
	desktopBindGroup.Release()
	desktopView.Release()
	desktopTexture.Release()
	
	if err != nil {
		return err
	}
	osWindow.gpuSurface.Present(surfaceTexture)
	
	return nil
}


// uploadBackendToTexture creates a GPU texture from a raster backend.
func (r *WebGPURenderer) uploadBackendToTexture(backend *raster.Backend) (*wgpu.Texture, *wgpu.TextureView, *wgpu.BindGroup, error) {
	img := backend.Image()
	if img == nil {
		return nil, nil, nil, fmt.Errorf("backend has no image")
	}
	
	bounds := img.Bounds()
	width := uint32(bounds.Dx())
	height := uint32(bounds.Dy())
	
	texture, err := r.device.CreateTexture(&wgpu.TextureDescriptor{
		Size: wgpu.Extent3D{
			Width:              width,
			Height:             height,
			DepthOrArrayLayers: 1,
		},
		MipLevelCount: 1,
		SampleCount:   1,
		Dimension:     wgpu.TextureDimension2D,
		Format:        wgpu.TextureFormatBGRA8Unorm,
		Usage:         wgpu.TextureUsageTextureBinding | wgpu.TextureUsageCopyDst,
	})
	if err != nil {
		return nil, nil, nil, err
	}
	
	bytesPerPixel := uint32(4)
	bytesPerRow := width * bytesPerPixel
	alignment := uint32(256)
	alignedBytesPerRow := ((bytesPerRow + alignment - 1) / alignment) * alignment
	
	pixelData := make([]byte, alignedBytesPerRow*height)
	
	for y := uint32(0); y < height; y++ {
		srcOffset := y * uint32(img.Stride)
		dstOffset := y * alignedBytesPerRow
		
		for x := uint32(0); x < width; x++ {
			srcIdx := srcOffset + x*4
			dstIdx := dstOffset + x*4
			
			pixelData[dstIdx+0] = img.Pix[srcIdx+2] // B
			pixelData[dstIdx+1] = img.Pix[srcIdx+1] // G
			pixelData[dstIdx+2] = img.Pix[srcIdx+0] // R
			pixelData[dstIdx+3] = img.Pix[srcIdx+3] // A
		}
	}
	
	r.queue.WriteTexture(
		&wgpu.ImageCopyTexture{
			Texture:  texture,
			MipLevel: 0,
			Origin:   wgpu.Origin3D{X: 0, Y: 0, Z: 0},
			Aspect:   0,
		},
		pixelData,
		&wgpu.ImageDataLayout{
			Offset:       0,
			BytesPerRow:  alignedBytesPerRow,
			RowsPerImage: height,
		},
		&wgpu.Extent3D{
			Width:              width,
			Height:             height,
			DepthOrArrayLayers: 1,
		},
	)
	
	textureView, err := r.device.CreateTextureView(texture, nil)
	if err != nil {
		texture.Release()
		return nil, nil, nil, err
	}
	
	bindGroup, err := r.device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Layout: r.blitLayout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, TextureView: textureView},
			{Binding: 1, Sampler: r.blitSampler},
		},
	})
	if err != nil {
		textureView.Release()
		texture.Release()
		return nil, nil, nil, err
	}
	
	return texture, textureView, bindGroup, nil
}


// createWindowPositionUniforms creates position uniforms for a window at the given bounds.
func (r *WebGPURenderer) createWindowPositionUniforms(bounds core.UnitRect, surfaceSize core.UnitSize) (*wgpu.Buffer, *wgpu.BindGroup, error) {
	// Convert unit coordinates to NDC (-1 to 1)
	// NDC: (-1, -1) is bottom-left, (1, 1) is top-right
	ndcX := (float32(bounds.X) / float32(surfaceSize.Width)) * 2.0 - 1.0
	ndcY := 1.0 - (float32(bounds.Y) / float32(surfaceSize.Height)) * 2.0 // Flip Y
	ndcWidth := (float32(bounds.Width) / float32(surfaceSize.Width)) * 2.0
	ndcHeight := (float32(bounds.Height) / float32(surfaceSize.Height)) * 2.0
	
	// Adjust Y for bottom-left origin
	uniformData := []float32{ndcX, ndcY - ndcHeight, ndcWidth, ndcHeight}
	
	buffer, err := r.device.CreateBuffer(&wgpu.BufferDescriptor{
		Size:  16,
		Usage: wgpu.BufferUsageUniform | wgpu.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, nil, err
	}
	
	// Convert float32 slice to bytes
	uniformBytes := (*[16]byte)(unsafe.Pointer(&uniformData[0]))[:]
	r.queue.WriteBuffer(buffer, 0, uniformBytes)
	
	bindGroup, err := r.device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Layout: r.blitPosLayout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, Buffer: buffer, Size: 16},
		},
	})
	if err != nil {
		buffer.Release()
		return nil, nil, err
	}
	
	return buffer, bindGroup, nil
}

// createFullscreenPositionUniforms creates uniforms for fullscreen rendering.
func (r *WebGPURenderer) createFullscreenPositionUniforms() (*wgpu.Buffer, *wgpu.BindGroup, error) {
	uniformData := []float32{-1.0, -1.0, 2.0, 2.0}
	
	buffer, err := r.device.CreateBuffer(&wgpu.BufferDescriptor{
		Size:  16,
		Usage: wgpu.BufferUsageUniform | wgpu.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, nil, err
	}
	
	uniformBytes := (*[16]byte)(unsafe.Pointer(&uniformData[0]))[:]
	r.queue.WriteBuffer(buffer, 0, uniformBytes)
	
	bindGroup, err := r.device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Layout: r.blitPosLayout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, Buffer: buffer, Size: 16},
		},
	})
	if err != nil {
		buffer.Release()
		return nil, nil, err
	}
	
	return buffer, bindGroup, nil
}
