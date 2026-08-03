# GPU Compositor — Status

Updated 2026-08-03 after the bug-fix + hardening session. The original
August 2 handoff described four open bugs and a half-finished renderer
split; all four are fixed, the renderer abstraction is now real, and the
behavior is locked in by headless tests.

## Layering contract (design of record)

Per OS window, bottom to top:

1. **Base layer** — the surface's own content: desktop background, menu
   bar, status bar, dock (a torn-off window draws its own chrome, menu
   bar included, on its base layer).
2. **Child windows** — each on its own GPU texture; MDI children
   composite *within* their parent window's texture because the window
   paints its whole subtree.
3. **Menu dropdowns** — the menu bar's open dropdown.
4. **Popups** — combo box lists, context menus. Topmost, so a popup
   opened from a menu paints above the menu that spawned it.

Torn-off windows currently present as plain single surfaces (base layer
only, everything painted into it — correct visually). Giving each its
own child-window/popup layer stack later only requires its surface
handler to implement `platform.WindowProvider` + `BaseLayerPainter`;
the platform plumbing is already per-surface.

## How presents work now

`Platform.presentWindow` (sdl/platform_sdl.go) is the ONE path to the
screen, used by the main loop, live resize, and font zoom alike:

- Compositing renderer + handler with child windows →
  `RenderFrameWithChildWindows` (the 4-layer pipeline above). The base
  layer is painted through `platform.BaseLayerPainter.FrameBase`
  (chrome only).
- Otherwise → `paintAndPresent`: full-scene `Frame` into the backend,
  then `Renderer.Present` (software: SDL streaming texture; the
  legacy WebGPU blit path also remains for GPU windows without child
  windows).

`SurfaceHandler.Frame` ALWAYS paints the complete scene (windows
included). Only the compositor asks for the chrome-only `FrameBase`.
This split is what fixed windows vanishing during live resize and the
blank-window software path.

## Fixed this session (regression tests in parentheses)

- **Black triangle after resize** — the legacy present drew 3 vertices
  against the 6-vertex quad shader and wrote only 12 of the 32 uniform
  bytes; every resize present painted half the window. Fixed; resize
  path also funnels through `presentWindow`
  (`TestSDLLiveResizeRepaintsFully`).
- **Windows disappeared during desktop resize** — live resize used the
  non-compositing present while `Frame` painted chrome-only. Frame
  paints everything again; resize composites
  (`TestDesktopFramePaintsWindowsFrameBaseDoesNot`).
- **Wrong window scale after desktop resize** — per-window NDC uniforms
  were baked at texture creation against the then-current surface size.
  Uniforms are now rewritten every frame from current bounds + surface
  size (`TestWindowNDCTracksSurfaceResize`); window moves no longer
  recreate GPU resources at all. Child backends also inherit the parent
  backend's font size, so content density matches at any `font_size`.
- **Menu scroll buttons let clicks fall through** — title hit-testing
  had no right boundary, so titles extending beneath `[<][>]`/the clock
  still activated on press, hover, and drag
  (`TestMenuBarScrollButtonsConsumeMouse`).
- **Menu/popup z-order** — dropdowns now render below popups (shared
  `drawOverlay` helper replaced the two duplicated reflection blobs).
- **Elided menu title drew monospace** — the last partially-visible
  title now measures AND paints in the bar's proportional font
  (`TestElidedTitlePrefixMeasuresProportionally`).
- **Software renderer path restored** — createWindow/sizeFramebuffer/
  paintAndPresent no longer hard-require the WebGPU chain; the headless
  dummy-driver test runs the whole loop again
  (`TestSDLPlatformHeadless`).
- **Linux build** — `getMetalLayer` is behind a darwin seam;
  X11/Windows handles come from SDL WM info (`sdl/gpusurface_*.go`).
- **Leaks** — closed child windows now release their compositor
  surfaces (eviction each frame + on shutdown).
- **CLI renderer override** — `--webgpu`, `--software` (alias `--sdl`),
  `--renderer=NAME` override `kittytk.ini` in the host binary, parsed
  with `github.com/phroun/argwild` (host app only, not the library).

- **Rotation demo rewritten for the compositor** — the `r` key works in
  both GPU modes again. The blit vertex shader rigidly rotates/shrinks
  every layer's quad around the surface center (aspect-corrected, so the
  scene doesn't skew), and the spinning content-textured cube draws over
  the composited scene. The cube pipeline and all effect easing now live
  in WebGPURenderer (Platform borrows them via exposeWebGPUObjects); the
  Platform keeps only its animation clock for mouse-coordinate rotation
  compensation.

## Known gaps (not regressions)
- **Text caret in compositor mode**: windows paint on their own layers,
  so a focused terminal's caret request cannot reach the OS surface
  through the base-layer painter. Needs caret plumbing from the
  per-window paint to the platform (pre-existing gap, now documented).
- **Compositor repaints every window every frame** — per-window damage
  is tracked (`surf.dirty`) but not yet honored. Optimization, not
  correctness.
- **`windowSurfaces` keys** truncate the child-window pointer to
  uint32; collision is unlikely but a stable window ID would be nicer.
- **Linux `-tags "sdl webgpu"` final link** currently fails in goffi
  (`unhandled relocation … SDYNIMPORT`) — packages compile and vet
  clean; macOS unaffected. Upstream (go-webgpu/goffi) issue.

## Build & test matrix

```
go build ./...                    # core, TUI, tools
go build -tags sdl ./...          # SDL host, software renderer
go vet  -tags "sdl webgpu" ./sdl  # WebGPU renderer type-checks
go test ./...                     # includes untagged compositor math tests
go test -tags sdl ./...           # + headless SDL loop/resize tests
```
