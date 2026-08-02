# GPU Compositor Implementation - Handoff Document

## What We Built (Successfully Working)

### 1. Per-Window GPU Compositor ✅
**Location**: `sdl/renderer_webgpu.go` - `RenderFrameWithChildWindows()`

**Architecture**:
- Each UI window renders to its own GPU texture
- Desktop base layer (background, menu bar, status bar, dock) renders separately
- All textures composited together with proper Z-ordering and alpha blending

**Rendering Pipeline (4 layers)**:
1. **Desktop Base** - Background, menu bar, dock, status bar
2. **Windows** - Each window at its own position via NDC coordinates
3. **Menu Dropdowns** - File/Edit/etc menus (separate from popups)
4. **Popups** - Combo boxes, context menus on top

**Key Features Working**:
- ✅ Multiple windows render simultaneously with correct positioning
- ✅ Window dragging updates positions correctly (via `lastBounds` tracking)
- ✅ Alpha blending for transparent rounded corners on all layers
- ✅ Combo box dropdowns visible and interactive
- ✅ Context menus visible and interactive
- ✅ Menu bar dropdowns visible and interactive
- ✅ Outer strokes on popups/menus (2px padding for thicker experimental lines)
- ✅ Torn-out OS windows have transparent rounded corners (blend state added to platform_sdl.go)

**Technical Details**:
- Per-window uniform buffers with NDC coordinates baked in
- Negative painter offset for popups (they paint at screen coords, we offset to 0,0)
- Texture padding (4px total, 2px each side) for outer strokes
- BGRA pixel format with 256-byte row alignment for GPU upload
- Position change detection via `lastBounds` comparison

### 2. Key Architecture Components

**`platform/platform.go`**:
- `ChildWindowList` struct with Windows, Popups, MenuDropdown fields
- `WindowProvider` interface for Desktop to expose child windows

**`objects/trinkets/desktop.go`**:
- `Desktop.GetChildWindows()` returns windows, popups, and active menu dropdown
- `Desktop.Paint()` renders ONLY chrome (not windows) in compositor mode
- Comment documents that popups/menus handled separately

**`objects/window/manager.go`**:
- `GetPopups()` returns popup overlays for compositor
- `PaintPopups()` helper for rendering popup layer
- WindowManager is the PopupController that combo boxes register with

**Shader** (`sdl/platform_sdl.go` lines 25-75):
- Combined uniforms in group 1 (angle, enabled, scale, padding, pos_x, pos_y, size_w, size_h)
- Proper quad generation (6 vertices, 2 triangles)
- Position scaling/translation from uniforms

## Critical Bugs Still Present

### Bug #1: Black Triangle on Window Resize ⚠️ CRITICAL
**Symptoms**:
- When resizing ANY window (SDL Desktop or child windows), half the window shows correct content, half is black
- Diagonal line from top-left to bottom-right divides the two triangles
- Black triangle persists until mouse move or other event triggers repaint
- User provided screenshot: `half.png`

**Root Cause Analysis**:
- This is a **regular window rendering path** issue, NOT compositor
- Happens on SDL Desktop window when NO child windows are open (non-compositor path)
- Backend is recreated in `sizeFramebuffer()` (platform_sdl.go line ~1027)
- New backend has zero-initialized (black) pixel buffer
- Surface handler (Desktop/WindowManager) has damage tracking - doesn't know backend was recreated
- Handler thinks everything is "clean" and doesn't repaint unpainted areas
- Result: Half the window never gets painted, shows black uninitialized pixels

**Failed Attempted Fixes**:
1. ❌ Deferred resource cleanup (caused windows to disappear, broke metrics)
2. ❌ Skip drawing dirty windows (dirty flag never clears without repaint)
3. ❌ Gray background fill (bandaid, not acceptable quality)
4. ❌ Compositor window invalidation (wrong code path - this is regular window issue)
5. ❌ Added `surface.Invalidate()` after backend creation (line ~1038) - **NO MESSAGES IN CONSOLE, NOT BEING CALLED**

**Current State**:
- Added debug at line 1038 in platform_sdl.go: should print "🔄 Invalidating surface"
- Debug does NOT print, meaning `sizeFramebuffer()` is not being called during resize
- OR the resize is going through a different code path entirely

**Next Steps to Debug**:
1. Find WHERE resize actually creates new backend (not sizeFramebuffer?)
2. Check if `liveResize()` is even being called (add debug at line 1494)
3. Verify resize events are being processed (SDL WINDOWEVENT_RESIZED)
4. The diagonal split means ONE triangle paints, one doesn't - classic two-triangle corruption
5. Must invalidate handler's damage tracking when backend recreates

### Bug #2: Windows Disappear During Desktop Resize ⚠️
**Symptoms**:
- When resizing SDL Desktop window, all child windows inside disappear
- Windows reappear after resize completes
- Happens during the resize drag operation

**Potential Causes**:
- Compositor not being called during resize?
- Window surfaces being destroyed/recreated incorrectly?
- Resource cleanup happening at wrong time?
- Check if compositor path is even active during resize (maybe falling back to non-compositor?)

### Bug #3: Wrong Metrics/Scale After Resize ⚠️
**Symptoms**:
- After resizing Desktop window, child windows have wrong zoom/scale
- Text/content size is incorrect
- Persists after resize completes

**Root Cause**:
- Metrics not being updated when backend changes size
- Check `SetCellMetrics()` calls in resize path
- Verify `osWindow.backend.Metrics()` returns correct values after resize

### Bug #4: Mouse Events Fall Through Menu Bar Buttons
**Status**: Not a compositor bug
**Location**: MenuBar event handling (objects/trinkets/menu.go)
**Issue**: Scroll buttons [<] [>] on menu bar don't consume mouse events, causing menu items beneath to activate
**Fix Location**: `MenuBar.HandleMouseMove()` hit testing

## Code Locations Reference

### Main Compositor Code
- `sdl/renderer_webgpu.go` lines 606-1400: `RenderFrameWithChildWindows()`
  - Line 641: Desktop base layer render
  - Line 645: Per-window texture creation loop
  - Line 762: Window resize/move detection (`needsUpdate`)
  - Line 768: Window invalidation (compositor path - doesn't help regular windows)
  - Line 907: Desktop texture upload
  - Line 960: Window composite draw loop
  - Line 987: Popup rendering
  - Line 1220: Menu dropdown rendering

### Regular Window Rendering (WHERE BLACK TRIANGLE BUG IS)
- `sdl/platform_sdl.go` lines 1026-1095: `sizeFramebuffer()` - Backend recreation
  - Line 1027: `raster.NewScaled()` creates new empty backend
  - Line 1030: Backend assigned to window
  - Line ~1038: Added `surface.Invalidate()` - **NOT BEING CALLED**
- `sdl/platform_sdl.go` lines 1494-1509: `liveResize()` - Resize handler
  - Line 1503: Calls `sizeFramebuffer()`
  - Line 1504: Calls `handler.Resized()`
  - Line 1505: Calls `paintAndPresent(w, true)` - true forces full paint
- `sdl/platform_sdl.go` lines 1115-1200: `paintAndPresent()` - Paint and upload

### Window/Popup Management
- `objects/window/manager.go` line 1716: `RegisterPopup()` - Popups register here
- `objects/window/manager.go` line 2986: `PaintMenuDropdown()` - Old non-compositor path
- `objects/window/manager.go` line 3015: `GetPopups()` - Returns popups for compositor

## Known Working Commits

- `bdf4574` - Stroke padding and torn-out window alpha blending
- `d13840d` - Menu dropdown rendering in compositor  
- `450af7a` - Popup coordinate offset fix (negative offset)
- `7155ce6` - Window position tracking with lastBounds
- `d6df02b` - Alpha blending for transparent corners
- `65464ed` - Popup resource lifetime (deferred cleanup)

## Known Breaking Changes

- Commit `809213f` - Deferred cleanup for "triangle fix" - **BROKE METRICS AND CAUSED DISAPPEARING WINDOWS**
  - Reverted in commit `56b2ae5`

## Debug Flags Currently Active

**`sdl/renderer_webgpu.go`**:
- Line ~766: "🔍 Window size check: surf(%dx%d) vs new(%dx%d), needsUpdate=%v"
- Line ~771: "🔧 Window needs update (resize/move detected)"
- Line ~775: "✅ Calling window.Invalidate()" or "❌ Window doesn't implement Invalidate()"
- Line ~847: "🔄 Updated WindowSurface position"
- Line ~895: "⚠️ Size mismatch! Texture: %dx%d, Backend: %dx%d"
- Line ~964: "🔍 childWindowList has %d windows, windowSurfaces has %d"
- Line ~980: "🎨 Compositor drew %d windows"
- Line ~995: "🎯 Rendering %d popups"

**`sdl/platform_sdl.go`**:
- Line ~1038: "🔄 Invalidating surface after backend resize" - **NOT PRINTING**

## Architecture Notes

### Why Compositor Has Two Texture Formats
- Window textures: BGRA8Unorm (native GPU format)
- Backend pixels: RGBA (what raster backend produces)
- Conversion loop swaps R↔B during upload (lines ~875-881 in renderer_webgpu.go)

### Why Popups Use Negative Offset
- Popup Paint() functions expect painter at screen origin (0,0)
- They call `WithOffset(bounds.X, bounds.Y)` to position themselves
- Our backend is sized to popup bounds starting at (0,0)
- Solution: Give painter `WithOffset(-bounds.X, -bounds.Y)`
- When Paint adds its offset: `-bounds.X + bounds.X = 0` ✓

### Why Z-Order Is Wrong (Documented, Not Fixed)
- Current order: Desktop → Windows → Popups → Menus
- Should be: Desktop → Windows → Menus → Popups
- Old WindowManager.Paint() had correct order
- TODO: Swap Step 4 and Step 5 in compositor (complex code move)

## Recommendations for Next Session

### PRIORITY 1: Fix Black Triangle Bug
1. Add debug to `liveResize()` line 1494 to see if it's called
2. Search for OTHER functions that might create backends during resize
3. Check SDL event handling for WINDOWEVENT_RESIZED
4. If `sizeFramebuffer()` IS being called but invalidate isn't printing, surface might be NULL
5. Consider: Clear backend to background color right after creation as nuclear option
6. Real fix: Find where handler's damage rect is stored and clear it on backend recreation

### PRIORITY 2: Fix Disappearing Windows
1. Check if `RenderFrameWithChildWindows()` is called during Desktop resize
2. Add debug to compositor entry point (line 628)
3. Verify `GetChildWindows()` returns windows during resize
4. Check if windowSurfaces are being destroyed during parent resize

### PRIORITY 3: Fix Metrics After Resize
1. Check `SetCellMetrics()` calls in resize path
2. Verify metrics propagate to child window surfaces
3. Check if scale factor changes during resize

### PRIORITY 4: Swap Menu/Popup Z-Order
- Move popup rendering code (line ~987) before menu rendering (line ~1220)
- Update step numbers in comments

## Test Cases

### Working
- ✅ Open multiple windows, drag them around
- ✅ Click combo boxes, select items from dropdown
- ✅ Right-click for context menu
- ✅ Click menu bar (File/Edit/etc), select menu items
- ✅ Tear out window to OS level, see rounded corners
- ✅ Windows have outer strokes on borders

### Failing
- ❌ Resize SDL Desktop window → black triangle appears
- ❌ Resize SDL Desktop window → child windows disappear during resize
- ❌ Resize SDL Desktop window → child window metrics wrong afterward
- ❌ Menu bar scroll buttons [<] [>] don't block mouse events

## Notes from User

- "Gray is a stupid fix, not acceptable quality in this project" - No workarounds, proper fixes only
- "We know every maximize/restore or drag operation since we are the one initiating it" - We control timing, should invalidate proactively
- Black triangle "stays there for as long as I want, as long as I don't release the mouse button or move the cursor after the resize" - Damage tracking issue
- "this has been the regular window path the entire time" - Not compositor code
- "I see no messages during a resize" - sizeFramebuffer/invalidate not being called

## Files to Review

If debugging the black triangle:
1. `sdl/platform_sdl.go` - sizeFramebuffer, liveResize, paintAndPresent
2. `backend/raster/raster.go` - NewScaled, BeginFrame, Image
3. `platform/platform.go` - Surface.Invalidate, damage tracking
4. SDL event handling for WINDOWEVENT_RESIZED

The debug message not appearing is the key clue - find where resize ACTUALLY happens.
