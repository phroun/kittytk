//go:build sdl && webgpu

package sdl

import (
	"fmt"
	"time"

	wgpu "github.com/gogpu/wgpu"
	gputypes "github.com/gogpu/gputypes"
	_ "github.com/gogpu/wgpu/hal/allbackends"

	"github.com/phroun/kittytk/backend/raster"
)

// WindowSurface holds GPU resources for a single window's off-screen rendering
type WindowSurface struct {
	texture     *wgpu.Texture
	textureView *wgpu.TextureView
	bindGroup   *wgpu.BindGroup
	width       uint32
	height      uint32
	
	// Transform state for compositing
	translateX  float32
	translateY  float32
	rotation    float32 // radians
	scaleX      float32
	scaleY      float32
	opacity     float32
	
	dirty       bool // needs re-render
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

	// Rotation/scale effect state
	rotationStartTime           time.Time
	rotationActivationTime      time.Time
	rotationDeactivationTime    time.Time
	rotationAngleAtDeactivation float64
	rotationEnabled             bool
	
	// Per-window surfaces for compositing
	windowSurfaces map[uint32]*WindowSurface // windowID -> surface

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
		vsync:          vsync,
		windowSurfaces: make(map[uint32]*WindowSurface),
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
	// TODO: Extract WebGPU presentation logic from platform_sdl.go
	// This includes uploading backend pixels, running blit/cube passes, presenting
	return fmt.Errorf("WebGPURenderer.Present not yet implemented")
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

	// Create pipeline layout
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
