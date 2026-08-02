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
	
	// Per-window uniform buffer (for positioning)
	uniformBuffer    *wgpu.Buffer
	uniformBindGroup *wgpu.BindGroup
	
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
	lastBounds  core.UnitRect    // Last known position/size for change detection
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
	
	// Write combined uniforms: effects (angle=0, enabled=0, scale=1, padding=0) + position (fullscreen: -1,-1,2,2)
	combinedUniforms := []float32{
		0.0, 0.0, 1.0, 0.0,  // angle, enabled, scale, padding
		-1.0, -1.0, 2.0, 2.0,  // pos_x, pos_y, size_w, size_h (fullscreen)
	}
	uniformBytes := (*[32]byte)(unsafe.Pointer(&combinedUniforms[0]))[:]
	r.queue.WriteBuffer(r.blitUniformBuffer, 0, uniformBytes)
	
	// Get surface texture
	surfaceTexture, _, err := w.gpuSurface.GetCurrentTexture()
	if err != nil {
		bindGroup.Release()
		textureView.Release()
		texture.Release()
		return err
	}
	
	surfaceView, err := surfaceTexture.CreateView(nil)
	if err != nil {
		bindGroup.Release()
		textureView.Release()
		texture.Release()
		return err
	}
	
	// Create command encoder
	encoder, err := r.device.CreateCommandEncoder(nil)
	if err != nil {
		surfaceView.Release()
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
	
	// Draw fullscreen triangle with backend texture
	renderPass.SetPipeline(r.blitPipeline)
	renderPass.SetBindGroup(0, bindGroup, nil)
	renderPass.SetBindGroup(1, r.blitUniformBindGroup, nil)
	renderPass.Draw(6, 1, 0, 0) // Draw quad (6 vertices)
	renderPass.End()
	
	// Submit and present
	cmdBuffer, _ := encoder.Finish()
	_, err = r.queue.Submit(cmdBuffer)
	
	// NOW we can release resources after GPU has the commands
	surfaceView.Release()
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

	// Create uniform buffer for combined uniforms (effects + position)
	// 8 floats: angle, enabled, scale, padding, pos_x, pos_y, size_w, size_h
	r.blitUniformBuffer, err = r.device.CreateBuffer(&wgpu.BufferDescriptor{
		Size:  32,  // 8 floats = 32 bytes
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
				Visibility: wgpu.ShaderStageVertex | wgpu.ShaderStageFragment,  // Both stages
				Buffer: &gputypes.BufferBindingLayout{
					Type:             0, // Uniform
					MinBindingSize:   32, // 8 floats
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
	if r.blitPosLayout == nil {
		return fmt.Errorf("blitPosLayout is nil after creation!")
	}
	fmt.Printf("✅ Created blitPosLayout: %p\n", r.blitPosLayout)

	// Create pipeline layout with 2 bind groups: texture, combined uniforms
	pipelineLayout, err := r.device.CreatePipelineLayout(&wgpu.PipelineLayoutDescriptor{
		BindGroupLayouts: []*wgpu.BindGroupLayout{r.blitLayout, r.blitUniformLayout},
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
					Blend: &gputypes.BlendState{
						Color: gputypes.BlendComponent{
							SrcFactor: gputypes.BlendFactorSrcAlpha,
							DstFactor: gputypes.BlendFactorOneMinusSrcAlpha,
							Operation: gputypes.BlendOperationAdd,
						},
						Alpha: gputypes.BlendComponent{
							SrcFactor: gputypes.BlendFactorOne,
							DstFactor: gputypes.BlendFactorOneMinusSrcAlpha,
							Operation: gputypes.BlendOperationAdd,
						},
					},
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
			{Binding: 0, Buffer: r.blitUniformBuffer, Size: 32},
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
			
			// Create per-window uniform buffer with position
			surfaceSize := osWindow.backend.Size()
			uniformBuffer, uniformBindGroup, err := r.createWindowUniformBuffer(bounds, surfaceSize)
			if err != nil {
				bindGroup.Release()
				textureView.Release()
				texture.Release()
				continue
			}
			
			fmt.Printf("✨ Created WindowSurface for window at (%d,%d) %dx%d\n", 
				bounds.X, bounds.Y, bounds.Width, bounds.Height)
			
			surf = &WindowSurface{
				texture:          texture,
				textureView:      textureView,
				bindGroup:        bindGroup,
				uniformBuffer:    uniformBuffer,
				uniformBindGroup: uniformBindGroup,
				width:            uint32(widthPx),
				height:           uint32(heightPx),
				uiWindow:         childIface,
				backend:          backend,
				dirty:            true,
				scaleX:           1.0,
				scaleY:           1.0,
				opacity:          1.0,
				lastBounds:       bounds, // Store initial position
			}
			r.windowSurfaces[windowID] = surf
		}
		
		// Check if window was resized or moved
		needsUpdate := int(surf.width) != widthPx || int(surf.height) != heightPx
		if !needsUpdate {
			// Check if position changed by comparing to last known bounds
			needsUpdate = surf.lastBounds.X != bounds.X || surf.lastBounds.Y != bounds.Y
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
			
			// Recreate uniform buffer with new position
			if surf.uniformBuffer != nil {
				surf.uniformBuffer.Release()
			}
			if surf.uniformBindGroup != nil {
				surf.uniformBindGroup.Release()
			}
			
			surfaceSize := osWindow.backend.Size()
			uniformBuffer, uniformBindGroup, err := r.createWindowUniformBuffer(bounds, surfaceSize)
			if err != nil {
				bindGroup.Release()
				textureView.Release()
				texture.Release()
				continue
			}
			
			surf.texture = texture
			surf.textureView = textureView
			surf.bindGroup = bindGroup
			surf.uniformBuffer = uniformBuffer
			surf.uniformBindGroup = uniformBindGroup
			surf.width = uint32(widthPx)
			surf.height = uint32(heightPx)
			surf.lastBounds = bounds  // Update stored position
			surf.dirty = true
			fmt.Printf("🔄 Updated WindowSurface position to (%d,%d) %dx%d\n",
				bounds.X, bounds.Y, bounds.Width, bounds.Height)
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
	
	// Step 3: Composite all textures
	surfaceTexture, _, err := osWindow.gpuSurface.GetCurrentTexture()
	if err != nil {
		return err
	}
	
	surfaceView, err := surfaceTexture.CreateView(nil)
	if err != nil {
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
				ClearValue: wgpu.Color{R: 0.0, G: 0.0, B: 0.0, A: 1.0},
			},
		},
	})
	
	renderPass.SetPipeline(r.blitPipeline)
	
	// Draw Desktop base first (fullscreen)
	// Write combined uniforms for fullscreen
	desktopUniforms := []float32{
		0.0, 0.0, 1.0, 0.0,   // angle, enabled, scale, padding
		-1.0, -1.0, 2.0, 2.0,  // pos_x, pos_y, size_w, size_h (fullscreen)
	}
	desktopUniformBytes := (*[32]byte)(unsafe.Pointer(&desktopUniforms[0]))[:]
	r.queue.WriteBuffer(r.blitUniformBuffer, 0, desktopUniformBytes)
	
	renderPass.SetBindGroup(0, desktopBindGroup, nil)
	renderPass.SetBindGroup(1, r.blitUniformBindGroup, nil)
	renderPass.Draw(6, 1, 0, 0) // Draw quad
	
	// Draw each child window at its position
	// Draw each child window with its pre-baked uniforms
	windowCount := 0
	fmt.Printf("🔍 childWindowList has %d windows, windowSurfaces has %d\n", 
		len(childWindowList.Windows), len(r.windowSurfaces))
	
	for _, childIface := range childWindowList.Windows {
		winValue := reflect.ValueOf(childIface)
		windowID := uint32(winValue.Pointer())
		surf, ok := r.windowSurfaces[windowID]
		if !ok {
			fmt.Printf("⚠️  Window ID %d in childWindowList but not in windowSurfaces!\n", windowID)
			continue
		}
		
		// Bind texture and per-window uniforms, then draw
		renderPass.SetBindGroup(0, surf.bindGroup, nil)
		renderPass.SetBindGroup(1, surf.uniformBindGroup, nil)  // Per-window uniforms!
		renderPass.Draw(6, 1, 0, 0) // Draw quad at window position
		windowCount++
	}
	fmt.Printf("🎨 Compositor drew %d windows (total surfaces: %d)\n", windowCount, len(r.windowSurfaces))
	
	// Step 4: Render popups on top of everything
	if childWindowList.Popups != nil && len(childWindowList.Popups) > 0 {
		fmt.Printf("🎯 Rendering %d popups\n", len(childWindowList.Popups))
		
		for popupIdx, popupIface := range childWindowList.Popups {
			fmt.Printf("  [%d] Processing popup...\n", popupIdx)
			
			// Use reflection to access PopupOverlay fields (can't import window package - circular dep)
			popupValue := reflect.ValueOf(popupIface)
			if popupValue.Kind() == reflect.Ptr {
				popupValue = popupValue.Elem()
			}
			
			// Get ID for debugging
			idField := popupValue.FieldByName("ID")
			popupID := "unknown"
			if idField.IsValid() {
				if id, ok := idField.Interface().(string); ok {
					popupID = id
				}
			}
			fmt.Printf("  [%d] Popup ID: %s\n", popupIdx, popupID)
			
			fmt.Printf("  [%d] Popup type: %v, kind: %v\n", popupIdx, popupValue.Type(), popupValue.Kind())
			
			boundsField := popupValue.FieldByName("Bounds")
			paintField := popupValue.FieldByName("Paint")
			
			if !boundsField.IsValid() || !paintField.IsValid() {
				fmt.Printf("⚠️  [%d] Popup missing Bounds or Paint field (boundsValid=%v, paintValid=%v)\n", 
					popupIdx, boundsField.IsValid(), paintField.IsValid())
				continue
			}
			
			bounds, ok := boundsField.Interface().(core.UnitRect)
			if !ok {
				fmt.Printf("⚠️  [%d] Bounds field is not UnitRect\n", popupIdx)
				continue
			}
			
			fmt.Printf("  [%d] Popup bounds: (%d,%d) %dx%d\n", popupIdx, bounds.X, bounds.Y, bounds.Width, bounds.Height)
			
			if bounds.Width <= 0 || bounds.Height <= 0 {
				fmt.Printf("⚠️  [%d] Invalid bounds size\n", popupIdx)
				continue
			}
			
			paintFunc, ok := paintField.Interface().(func(*core.Painter))
			if !ok || paintFunc == nil {
				fmt.Printf("⚠️  [%d] Paint field is not valid function\n", popupIdx)
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
			
			fmt.Printf("  [%d] Popup pixel size: %dx%d\n", popupIdx, widthPx, heightPx)
			
			if widthPx <= 0 || heightPx <= 0 {
				fmt.Printf("⚠️  [%d] Invalid pixel size\n", popupIdx)
				continue
			}
			
			// Create temporary backend for popup
			fmt.Printf("  [%d] Creating backend...\n", popupIdx)
			popupBackend, err := raster.NewScaled(widthPx, heightPx, scale)
			if err != nil {
				fmt.Printf("❌ [%d] Failed to create backend: %v\n", popupIdx, err)
				continue
			}
			popupBackend.SetCellMetrics(metrics)
			
			// Render popup to backend
			fmt.Printf("  [%d] Calling Paint function...\n", popupIdx)
			popupBackend.BeginFrame()
			
			// CRITICAL: The popup's Paint function expects a painter at screen origin (0,0)
			// and will call WithOffset(popupBounds.X, popupBounds.Y) to position itself.
			// But our backend is SIZED to the popup, starting at (0,0).
			// Solution: Offset the painter NEGATIVELY so when Paint adds the offset, it ends at (0,0)
			painter := core.NewPainter(popupBackend)
			offsetPainter := painter.WithOffset(-bounds.X, -bounds.Y)
			
			paintFunc(offsetPainter)
			popupBackend.EndFrame()
			fmt.Printf("  [%d] Paint complete\n", popupIdx)
			
			// Upload popup to temporary texture
			popupImg := popupBackend.Image()
			if popupImg == nil {
				fmt.Printf("❌ [%d] Backend returned nil image\n", popupIdx)
				continue
			}
			
			imgBounds := popupImg.Bounds()
			imgWidth := uint32(imgBounds.Dx())
			imgHeight := uint32(imgBounds.Dy())
			
			// Check if popup has any visible pixels (non-transparent)
			visiblePixels := 0
			for i := 3; i < len(popupImg.Pix); i += 4 {
				if popupImg.Pix[i] > 0 { // Check alpha channel
					visiblePixels++
				}
			}
			fmt.Printf("  [%d] Popup image: %dx%d, %d/%d visible pixels\n", 
				popupIdx, imgWidth, imgHeight, visiblePixels, imgWidth*imgHeight)
			
			popupTexture, err := r.device.CreateTexture(&wgpu.TextureDescriptor{
				Usage: wgpu.TextureUsageTextureBinding | wgpu.TextureUsageCopyDst,
				Dimension: wgpu.TextureDimension2D,
				Size: wgpu.Extent3D{
					Width:              imgWidth,
					Height:             imgHeight,
					DepthOrArrayLayers: 1,
				},
				Format:        wgpu.TextureFormatBGRA8Unorm,
				MipLevelCount: 1,
				SampleCount:   1,
			})
			if err != nil {
				continue
			}
			
			popupTextureView, err := r.device.CreateTextureView(popupTexture, nil)
			if err != nil {
				popupTexture.Release()
				continue
			}
			
			popupBindGroup, err := r.device.CreateBindGroup(&wgpu.BindGroupDescriptor{
				Layout: r.blitLayout,
				Entries: []wgpu.BindGroupEntry{
					{Binding: 0, TextureView: popupTextureView},
					{Binding: 1, Sampler: r.blitSampler},
				},
			})
			if err != nil {
				popupTextureView.Release()
				popupTexture.Release()
				continue
			}
			
			// Upload BGRA pixels to texture
			bytesPerPixel := uint32(4)
			bytesPerRow := imgWidth * bytesPerPixel
			alignment := uint32(256)
			alignedBytesPerRow := ((bytesPerRow + alignment - 1) / alignment) * alignment
			
			pixelData := make([]byte, alignedBytesPerRow*imgHeight)
			
			for y := uint32(0); y < imgHeight; y++ {
				srcOffset := y * uint32(popupImg.Stride)
				dstOffset := y * alignedBytesPerRow
				
				for x := uint32(0); x < imgWidth; x++ {
					srcIdx := srcOffset + x*4
					dstIdx := dstOffset + x*4
					
					pixelData[dstIdx+0] = popupImg.Pix[srcIdx+2] // B
					pixelData[dstIdx+1] = popupImg.Pix[srcIdx+1] // G
					pixelData[dstIdx+2] = popupImg.Pix[srcIdx+0] // R
					pixelData[dstIdx+3] = popupImg.Pix[srcIdx+3] // A
				}
			}
			
			r.queue.WriteTexture(
				&wgpu.ImageCopyTexture{
					Texture:  popupTexture,
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
			
			// Create uniform buffer with popup position
			popupUniformBuffer, popupUniformBindGroup, err := r.createWindowUniformBuffer(bounds, backendSize)
			if err != nil {
				fmt.Printf("❌ [%d] Failed to create uniform buffer: %v\n", popupIdx, err)
				popupBindGroup.Release()
				popupTextureView.Release()
				popupTexture.Release()
				continue
			}
			
			// Debug: Show what NDC coordinates we calculated
			ndcX := (float32(bounds.X) / float32(backendSize.Width)) * 2.0 - 1.0
			ndcY := 1.0 - (float32(bounds.Y) / float32(backendSize.Height)) * 2.0
			ndcWidth := (float32(bounds.Width) / float32(backendSize.Width)) * 2.0
			ndcHeight := (float32(bounds.Height) / float32(backendSize.Height)) * 2.0
			fmt.Printf("  [%d] NDC coords: x=%f, y=%f, w=%f, h=%f (backendSize=%dx%d)\n", 
				popupIdx, ndcX, ndcY-ndcHeight, ndcWidth, ndcHeight, backendSize.Width, backendSize.Height)
			
			// Draw popup
			renderPass.SetBindGroup(0, popupBindGroup, nil)
			renderPass.SetBindGroup(1, popupUniformBindGroup, nil)
			renderPass.Draw(6, 1, 0, 0)
			
			fmt.Printf("✅ Drew popup at (%d,%d) %dx%d\n", bounds.X, bounds.Y, bounds.Width, bounds.Height)
			
			// DON'T release yet - must stay alive until after Submit()
			// Store for cleanup after GPU submission
			defer func() {
				popupUniformBindGroup.Release()
				popupUniformBuffer.Release()
				popupBindGroup.Release()
				popupTextureView.Release()
				popupTexture.Release()
			}()
		}
	}
	
	// Step 5: Render menu dropdown if active (on top of popups for proper z-order)
	if childWindowList.MenuDropdown != nil {
		fmt.Printf("🍔 Rendering menu dropdown\n")
		
		// Extract Paint function via reflection
		menuValue := reflect.ValueOf(childWindowList.MenuDropdown)
		if menuValue.Kind() == reflect.Ptr {
			menuValue = menuValue.Elem()
		}
		
		paintField := menuValue.FieldByName("Paint")
		if paintField.IsValid() {
			if paintFunc, ok := paintField.Interface().(func(*core.Painter)); ok && paintFunc != nil {
				// Menu dropdowns paint themselves - we need to render the whole Desktop
				// layer again but only the menu part will show
				// TODO: This is inefficient - we should get menu bounds and render just that region
				// For now, just log that we detected it
				fmt.Printf("🍔 Menu dropdown detected but not yet rendered as separate layer\n")
			}
		}
	}
	
	renderPass.End()
	
	cmdBuffer, _ := encoder.Finish()
	_, err = r.queue.Submit(cmdBuffer)
	
	// Release temporary resources after GPU has the commands
	surfaceView.Release()
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



// createWindowUniformBuffer creates a uniform buffer for a window with position baked in.
func (r *WebGPURenderer) createWindowUniformBuffer(bounds core.UnitRect, surfaceSize core.UnitSize) (*wgpu.Buffer, *wgpu.BindGroup, error) {
	// Calculate NDC position
	ndcX := (float32(bounds.X) / float32(surfaceSize.Width)) * 2.0 - 1.0
	ndcY := 1.0 - (float32(bounds.Y) / float32(surfaceSize.Height)) * 2.0
	ndcWidth := (float32(bounds.Width) / float32(surfaceSize.Width)) * 2.0
	ndcHeight := (float32(bounds.Height) / float32(surfaceSize.Height)) * 2.0
	
	// Combined uniforms: effects + position
	uniformData := []float32{
		0.0, 0.0, 1.0, 0.0,                       // angle, enabled, scale, padding
		ndcX, ndcY - ndcHeight, ndcWidth, ndcHeight,  // pos_x, pos_y, size_w, size_h
	}
	
	buffer, err := r.device.CreateBuffer(&wgpu.BufferDescriptor{
		Size:  32,
		Usage: wgpu.BufferUsageUniform | wgpu.BufferUsageCopyDst,
	})
	if err != nil {
		return nil, nil, err
	}
	
	uniformBytes := (*[32]byte)(unsafe.Pointer(&uniformData[0]))[:]
	r.queue.WriteBuffer(buffer, 0, uniformBytes)
	
	bindGroup, err := r.device.CreateBindGroup(&wgpu.BindGroupDescriptor{
		Layout: r.blitUniformLayout,
		Entries: []wgpu.BindGroupEntry{
			{Binding: 0, Buffer: buffer, Size: 32},
		},
	})
	if err != nil {
		buffer.Release()
		return nil, nil, err
	}
	
	return buffer, bindGroup, nil
}
