// Package core provides fundamental types for the TUI toolkit.
package core

import (
	"image"

	"github.com/phroun/tuitk/style"
)

// RenderBackend abstracts the rendering target.
// Implementations exist for text terminals, and could be added for
// graphics (SDL, OpenGL, Canvas, WebGL, etc.).
type RenderBackend interface {
	// Lifecycle
	Init() error
	Shutdown()

	// Size returns the current size in abstract units.
	Size() UnitSize

	// CellMetrics returns the metrics for this backend.
	// For TUI, this defines how units map to character cells.
	// For GUI, this might be 1:1 with pixels or scaled.
	Metrics() CellMetrics

	// BeginFrame starts a new frame for rendering.
	BeginFrame()

	// EndFrame completes the frame and presents it.
	EndFrame()

	// Clear fills the entire surface with a style.
	Clear(s style.CellStyle)

	// SetClip sets the clipping rectangle. All drawing operations
	// will be clipped to this region. Pass empty rect to disable clipping.
	SetClip(clip UnitRect)

	// Drawing primitives (all coordinates in abstract units)

	// DrawCell draws a single character at the given position.
	DrawCell(x, y Unit, ch rune, s style.CellStyle)

	// DrawText draws a string starting at the given position using the given font.
	// If font is nil, uses DefaultFont().
	// Returns the width consumed in units.
	DrawText(x, y Unit, text string, s style.CellStyle, font *Font) Unit

	// DrawTextAligned draws text aligned within a box using the given font.
	// If font is nil, uses DefaultFont().
	DrawTextAligned(bounds UnitRect, text string, hAlign, vAlign Alignment, s style.CellStyle, font *Font)

	// FillRect fills a rectangle with a character and style.
	FillRect(r UnitRect, ch rune, s style.CellStyle)

	// DrawRect draws just the border of a rectangle.
	DrawRect(r UnitRect, border style.BorderStyle, s style.CellStyle)

	// DrawHLine draws a horizontal line using border style.
	DrawHLine(x, y, width Unit, ch rune, s style.CellStyle)

	// DrawVLine draws a vertical line using border style.
	DrawVLine(x, y, height Unit, ch rune, s style.CellStyle)

	// DrawBox draws a box with optional title.
	DrawBox(r UnitRect, border style.BorderStyle, title string, s style.CellStyle)

	// Input handling

	// PollEvent returns the next input event, or nil if none available.
	// This is non-blocking.
	PollEvent() Event

	// WaitEvent blocks until an event is available.
	WaitEvent() Event

	// SetCursorVisible shows or hides the cursor.
	SetCursorVisible(visible bool)

	// SetCursorPosition positions the cursor (for text input feedback).
	SetCursorPosition(x, y Unit)

	// Capabilities

	// SupportsColor returns whether the backend supports color.
	SupportsColor() bool

	// SupportsMouse returns whether the backend supports mouse input.
	SupportsMouse() bool

	// SupportsUnicode returns whether the backend supports Unicode.
	SupportsUnicode() bool

	// ColorDepth returns the number of colors supported (2, 16, 256, or 16777216 for true color).
	ColorDepth() int

	// Clipboard operations

	// GetClipboard returns the current clipboard contents.
	GetClipboard() string

	// SetClipboard sets the clipboard contents.
	SetClipboard(text string)

	// System

	// Beep produces an audible alert.
	Beep()
}

// SmoothPositioner is an optional RenderBackend capability: true
// when the surface can place window chrome at arbitrary unit
// positions (pixel surfaces). Cell-only surfaces (terminals) omit it
// - their painting quantizes to the cell grid, so drag/resize must
// snap to keep hit-testing and pixels aligned.
type SmoothPositioner interface {
	SmoothPositioning() bool
}

// SmoothPositioningProvider is the widget-side carrier of the same
// capability: window-manager hosts stamp it onto the windows they
// manage, and nested window hosts (MDI panes) discover it by walking
// their ancestry with FindSmoothPositioning.
type SmoothPositioningProvider interface {
	SmoothWindowPositioning() bool
}

// RoundedRectDrawer is an optional RenderBackend capability: pixel
// surfaces paint a filled, stroked rounded rectangle in a single
// pass - the fill in the style's background color, the stroke in its
// foreground, with the stroke weight taken from the border style
// (2 device pixels for double, 1 for single). Window frames use this
// as their entire graphical surface; cell surfaces omit it and
// frames fall back to box-drawing runes.
//
// StrokeRoundedRect paints only the stroke, leaving the interior
// untouched: window frames re-stroke over their content, because
// graphical content extends to the window edge (only the titlebar
// reserves a full row - the hairline border shares its boundary
// pixels with the content beneath it).
type RoundedRectDrawer interface {
	DrawRoundedRect(r UnitRect, radius Unit, border style.BorderStyle, s style.CellStyle)
	StrokeRoundedRect(r UnitRect, radius Unit, border style.BorderStyle, s style.CellStyle)
}

// TranslucentPixelFiller is an optional RenderBackend capability: fill a
// device-pixel rectangle with a color at partial opacity, blended over
// the existing pixels and respecting the clip (including a rounded clip
// region). The resize-edge hover highlight uses it. Cell surfaces omit it.
type TranslucentPixelFiller interface {
	FillRectPxAlpha(xPx, yPx, wPx, hPx int, r, g, b uint8, alpha float64)
}

// ArcWedgeDrawer is an optional RenderBackend capability: fill the
// part of a rect lying outside the quarter ellipse inscribed in it
// and centered on the chosen corner - antialiased - painting the fill
// in the style's background and an optional stroke of the given
// weight along the arc in its foreground (0 = no stroke). The tab
// strip's silhouette corners use this; cell surfaces omit it and
// callers fall back to scanline fills.
type ArcWedgeDrawer interface {
	DrawArcWedge(r UnitRect, centerRight, centerBottom bool, strokeW Unit, s style.CellStyle)
}

// ImageDrawer is an optional RenderBackend capability: composite a
// raster image onto the surface. The image is in DEVICE pixels
// (callers render at the surface's scale); alpha is honored
// (Porter-Duff over). DrawImage anchors at a unit position;
// DrawImagePx anchors at a device pixel for sub-unit placement
// (sprite fine positioning, animation offsets). The carrier for
// PurfecTerm's sprites and custom glyphs, and any widget with image
// content.
type ImageDrawer interface {
	DrawImage(x, y Unit, img image.Image)
	DrawImagePx(xPx, yPx int, img image.Image)
}

// DeviceScaler is an optional RenderBackend capability reporting how
// many device pixels one unit covers (the raster backend's scale).
type DeviceScaler interface {
	Scale() int
}

// GraphicalModer is the D1 mode query: a backend reports true when
// it paints pixels rather than character cells. Widgets branch their
// rendering on Painter.Graphical() - e.g. label-type text passes
// style.ColorTransparent backgrounds only on graphical targets,
// where glyphs can blend over existing pixels.
type GraphicalModer interface {
	GraphicalMode() bool
}

// PatternFiller is an optional RenderBackend capability: tile an 8x8
// two-color bitmap pattern across a rect (classic MacOS desktop
// style). Each pattern bit covers chunkPx x chunkPx device pixels
// (set = foreground, clear = background); the pattern is anchored at
// the surface origin so it does not swim as rects move. Cell
// surfaces omit it and callers fall back to rune fills.
type PatternFiller interface {
	FillPattern(r UnitRect, pattern [8]uint8, chunkPx int, s style.CellStyle)
}

// RoundedClipper is an optional RenderBackend capability: an
// additional clip constraint shaped as a rounded rectangle,
// composing with the rectangular SetClip (a pixel paints only if it
// passes both). A zero rect clears it. Window frames confine their
// edge-to-edge content with this so nothing paints past the rounded
// corners.
type RoundedClipper interface {
	SetRoundedClip(r UnitRect, radius Unit)
}

// ResizeGripProvider reports the window resize-grip thickness (in
// units) for graphical frames: only the outer sliver of a window
// edge acts as a resize handle (a quarter of a layout column, never
// less than 4 device pixels), so widgets living at the window's
// edge remain clickable. Zero means the cell-frame behavior: the
// whole border row/column is the grip (it IS the frame there).
// The desktop provides it; window hosts discover it by ancestry
// with FindResizeGrip.
type ResizeGripProvider interface {
	GraphicalResizeGrip() Unit
}

// FindResizeGrip walks up the widget tree for a ResizeGripProvider.
// Default (no provider found): 0 - classic full-cell grip zones.
func FindResizeGrip(w Widget) Unit {
	for current := Widget(w); current != nil; {
		if p, ok := current.(ResizeGripProvider); ok {
			return p.GraphicalResizeGrip()
		}
		parent := current.Parent()
		if parent == nil {
			return 0
		}
		current = parent
	}
	return 0
}

// GraphicalFrameProvider is the widget-side carrier of the frame
// mode: the desktop reports true when its backend paints rounded
// window frames, and windows discover it by walking their ancestry
// with FindGraphicalFrames. It governs the client-area contract: on
// graphical frames the content area extends to the window's left,
// right, and bottom edges (only the titlebar reserves a full row);
// on cell frames the border occupies a full cell on every side.
type GraphicalFrameProvider interface {
	GraphicalWindowFrames() bool
}

// FindGraphicalFrames walks up the widget tree for a
// GraphicalFrameProvider. Default (no provider found): false - the
// cell-frame client area, the only always-safe answer.
func FindGraphicalFrames(w Widget) bool {
	for current := Widget(w); current != nil; {
		if p, ok := current.(GraphicalFrameProvider); ok {
			return p.GraphicalWindowFrames()
		}
		parent := current.Parent()
		if parent == nil {
			return false
		}
		current = parent
	}
	return false
}

// CaretDrawer is an optional RenderBackend capability: pixel surfaces
// draw the text-insertion caret as a thin vertical bar sitting at the
// left edge of the glyph box at (x, y) - where the next character
// would be output. Callers pass the same style they would use for a
// block cursor; the backend renders the bar in the color that block
// would appear (the style's background). Cell surfaces omit the
// capability and widgets fall back to their cell-idiom caret
// (reverse-video block).
type CaretDrawer interface {
	DrawCaret(x, y, height Unit, s style.CellStyle)
}

// PixelRectFiller is an optional RenderBackend capability: fill a
// rectangle whose anchor is given in units but whose position and size
// are refined in device pixels, for hairline separators and 1-pixel
// gutter strokes that a whole-unit FillRect is too coarse to express.
// The style's background color fills the rect. Cell surfaces omit it.
type PixelRectFiller interface {
	FillRectPx(xPx, yPx, wPx, hPx int, s style.CellStyle)
}

// FindSmoothPositioning walks up the widget tree for a
// SmoothPositioningProvider. Default (no provider found): false -
// snap to cells, the only always-safe answer.
func FindSmoothPositioning(w Widget) bool {
	for current := Widget(w); current != nil; {
		if p, ok := current.(SmoothPositioningProvider); ok {
			return p.SmoothWindowPositioning()
		}
		parent := current.Parent()
		if parent == nil {
			return false
		}
		current = parent
	}
	return false
}

// Event is the base interface for all input events.
type Event interface {
	isEvent()
}

// KeyPressEvent represents a key press.
type KeyPressEvent struct {
	Key       string       // Key name from direct-key-handler
	Modifiers KeyModifiers // Active modifiers
	Text      string       // Printable text if any
}

func (KeyPressEvent) isEvent() {}

// KeyReleaseEvent represents a key release.
// Note: Not all terminals support key release events.
type KeyReleaseEvent struct {
	Key       string       // Key name
	Modifiers KeyModifiers // Active modifiers
}

func (KeyReleaseEvent) isEvent() {}

// MousePressEvent represents a mouse button press.
type MousePressEvent struct {
	X, Y      Unit         // Position in units
	Button    MouseButton  // Which button
	Modifiers KeyModifiers // Active keyboard modifiers
}

func (MousePressEvent) isEvent() {}

// MouseReleaseEvent represents a mouse button release.
type MouseReleaseEvent struct {
	X, Y      Unit
	Button    MouseButton
	Modifiers KeyModifiers
}

func (MouseReleaseEvent) isEvent() {}

// MouseMoveEvent represents mouse movement.
type MouseMoveEvent struct {
	X, Y      Unit
	Buttons   MouseButton  // Buttons currently held
	Modifiers KeyModifiers // Active keyboard modifiers
}

func (MouseMoveEvent) isEvent() {}

// MouseWheelEvent represents mouse wheel scrolling.
type MouseWheelEvent struct {
	X, Y   Unit
	DeltaX int // Horizontal scroll
	DeltaY int // Vertical scroll (positive = up)
	// Precise deltas (trackpad two-finger pan); zero when the source
	// only reports whole notches. Sign convention matches DeltaX/Y.
	PreciseX, PreciseY float64
	// Screen-space position, stamped once at the top of routing and
	// preserved through coordinate translation (wheel-gesture latch).
	ScreenX, ScreenY Unit
	Modifiers        KeyModifiers // Active keyboard modifiers
}

func (MouseWheelEvent) isEvent() {}

// ResizeEvent indicates the terminal/window was resized.
type ResizeEvent struct {
	Width, Height Unit // New size in units
	Cols, Rows    int  // New size in cells (for TUI)
}

func (ResizeEvent) isEvent() {}

// FocusEvent indicates focus gained or lost.
type FocusEvent struct {
	Focused bool
}

func (FocusEvent) isEvent() {}

// QuitEvent indicates the user requested to quit.
type QuitEvent struct{}

func (QuitEvent) isEvent() {}

// PasteEvent contains pasted text.
type PasteEvent struct {
	Text string
}

func (PasteEvent) isEvent() {}

// Painter provides drawing operations with automatic coordinate translation.
// Widgets receive a Painter configured with their local coordinate system.
type Painter struct {
	backend   RenderBackend
	transform Transform
	clip      UnitRect
	metrics   CellMetrics

	// Rounded clip region (screen coordinates; zero rect = none): an
	// additional constraint beyond the rectangular clip, honored by
	// backends implementing RoundedClipper. Window frames set it so
	// edge-to-edge content cannot paint past the frame's rounded
	// corners.
	roundClip       UnitRect
	roundClipRadius Unit
}

// NewPainter creates a painter for a backend.
func NewPainter(backend RenderBackend) *Painter {
	size := backend.Size()
	return &Painter{
		backend:   backend,
		transform: IdentityTransform(),
		clip:      UnitRect{Width: size.Width, Height: size.Height},
		metrics:   backend.Metrics(),
	}
}

// Metrics returns the cell metrics.
func (p *Painter) Metrics() CellMetrics {
	return p.metrics
}

// WithTransform returns a new Painter with an additional transform
// applied. The new transform maps into the current local space: local
// coordinates pass through t first, then the existing transform. (With
// translations only the order is immaterial; once scales are involved
// it is not.)
func (p *Painter) WithTransform(t Transform) *Painter {
	np := *p
	np.transform = t.Compose(p.transform)
	return &np
}

// WithDenomination returns a Painter whose local coordinates are
// denominated in `child` metrics, given the current space is
// denominated in `parent` metrics. Used when descending into a
// container that carries a grid-metrics override: the same number of
// rows/columns, re-expressed, so re-denomination is visually invariant.
// Identity when the denominations match.
func (p *Painter) WithDenomination(parent, child CellMetrics) *Painter {
	if parent == child || child.CellWidth <= 0 || child.CellHeight <= 0 {
		return p
	}
	return p.WithTransform(Transform{
		ScaleX: float64(parent.CellWidth) / float64(child.CellWidth),
		ScaleY: float64(parent.CellHeight) / float64(child.CellHeight),
	})
}

// WithOffset returns a new Painter offset by the given amount.
func (p *Painter) WithOffset(dx, dy Unit) *Painter {
	return p.WithTransform(NewTranslation(dx, dy))
}

// WithClip returns a new Painter with clipping applied.
// The clip rect is intersected with any existing clip.
func (p *Painter) WithClip(clip UnitRect) *Painter {
	// Transform clip to screen coordinates
	screenClip := p.transform.ApplyRect(clip)
	// Intersect with existing clip
	np := *p
	np.clip = p.clip.Intersection(screenClip)
	return &np
}

// WithRoundedClipRegion returns a Painter whose drawing is
// additionally confined to a rounded rectangle (in current local
// coordinates). It composes with the rectangular clip chain: a pixel
// paints only if it passes both. Backends without RoundedClipper
// ignore it (cell surfaces have no rounded geometry to protect).
func (p *Painter) WithRoundedClipRegion(r UnitRect, radius Unit) *Painter {
	np := *p
	np.roundClip = p.transform.ApplyRect(r)
	np.roundClipRadius = radius
	return &np
}

// Clip returns the current clip rectangle in local coordinates.
func (p *Painter) Clip() UnitRect {
	inv := p.transform.Inverse()
	return inv.ApplyRect(p.clip)
}

// applyClip sets the backend clip to our current clip.
func (p *Painter) applyClip() {
	p.backend.SetClip(p.clip)
	if rc, ok := p.backend.(RoundedClipper); ok {
		rc.SetRoundedClip(p.roundClip, p.roundClipRadius)
	}
}

// toScreen transforms local coordinates to screen coordinates.
func (p *Painter) toScreen(x, y Unit) (Unit, Unit) {
	pt := p.transform.Apply(UnitPoint{X: x, Y: y})
	return pt.X, pt.Y
}

// DrawCell draws a single character.
func (p *Painter) DrawCell(x, y Unit, ch rune, s style.CellStyle) {
	sx, sy := p.toScreen(x, y)
	p.applyClip()
	p.backend.DrawCell(sx, sy, ch, s)
}

// DrawRoundedRect paints a filled, stroked rounded rectangle when
// the backend supports it (see RoundedRectDrawer). Returns false on
// cell surfaces; the caller then falls back to its cell-idiom
// rendering (box-drawing runes).
func (p *Painter) DrawRoundedRect(r UnitRect, radius Unit, border style.BorderStyle, s style.CellStyle) bool {
	rd, ok := p.backend.(RoundedRectDrawer)
	if !ok {
		return false
	}
	screenRect := p.transform.ApplyRect(r)
	p.applyClip()
	rd.DrawRoundedRect(screenRect, radius, border, s)
	return true
}

// DrawArcWedge paints an antialiased quarter-arc wedge when the
// backend supports it (see ArcWedgeDrawer). strokeW is in screen
// units. Returns false on cell surfaces; the caller then falls back
// to its scanline rendering.
func (p *Painter) DrawArcWedge(r UnitRect, centerRight, centerBottom bool, strokeW Unit, s style.CellStyle) bool {
	ad, ok := p.backend.(ArcWedgeDrawer)
	if !ok {
		return false
	}
	screenRect := p.transform.ApplyRect(r)
	p.applyClip()
	ad.DrawArcWedge(screenRect, centerRight, centerBottom, strokeW, s)
	return true
}

// DrawImage composites a device-pixel image at a unit position when
// the backend supports it (see ImageDrawer). Returns false on cell
// surfaces.
func (p *Painter) DrawImage(x, y Unit, img image.Image) bool {
	id, ok := p.backend.(ImageDrawer)
	if !ok {
		return false
	}
	sx, sy := p.toScreen(x, y)
	p.applyClip()
	id.DrawImage(sx, sy, img)
	return true
}

// DrawImageOffset composites a device-pixel image anchored at a unit
// position plus a device-pixel nudge - for content that needs
// sub-unit placement (sprite fine positioning, wave animation).
// Returns false on cell surfaces.
func (p *Painter) DrawImageOffset(x, y Unit, offXPx, offYPx int, img image.Image) bool {
	id, ok := p.backend.(ImageDrawer)
	if !ok {
		return false
	}
	sx, sy := p.toScreen(x, y)
	scale := p.DeviceScale()
	p.applyClip()
	id.DrawImagePx(int(sx)*scale+offXPx, int(sy)*scale+offYPx, img)
	return true
}

// FillRectPixels fills, in device pixels, a rectangle anchored at unit
// (x, y) plus a device-pixel offset: wPx x hPx device pixels in the
// style's background color. For hairline separators and 1-pixel gutter
// strokes that whole-unit FillRect can't express. Returns false on cell
// surfaces (the caller falls back to a cell-idiom line).
func (p *Painter) FillRectPixels(x, y Unit, offXPx, offYPx, wPx, hPx int, s style.CellStyle) bool {
	pf, ok := p.backend.(PixelRectFiller)
	if !ok {
		return false
	}
	sx, sy := p.toScreen(x, y)
	scale := p.DeviceScale()
	p.applyClip()
	pf.FillRectPx(int(sx)*scale+offXPx, int(sy)*scale+offYPx, wPx, hPx, s)
	return true
}

// FillRectPixelsAlpha blends, in device pixels, the rectangle anchored at
// unit (x, y) plus a device-pixel offset (wPx x hPx device pixels) with
// the given RGB at alpha (0..1), over the existing pixels and respecting
// the clip - including any rounded clip region, so a fill along a window
// edge follows its corner curve. Returns false on backends that can't
// blend (e.g. cell surfaces).
func (p *Painter) FillRectPixelsAlpha(x, y Unit, offXPx, offYPx, wPx, hPx int, r, g, b uint8, alpha float64) bool {
	tf, ok := p.backend.(TranslucentPixelFiller)
	if !ok {
		return false
	}
	sx, sy := p.toScreen(x, y)
	scale := p.DeviceScale()
	p.applyClip()
	tf.FillRectPxAlpha(int(sx)*scale+offXPx, int(sy)*scale+offYPx, wPx, hPx, r, g, b, alpha)
	return true
}

// DeviceScale reports how many device pixels one unit covers on this
// target (1 on cell surfaces and unscaled pixel surfaces).
func (p *Painter) DeviceScale() int {
	if ds, ok := p.backend.(DeviceScaler); ok {
		if s := ds.Scale(); s > 0 {
			return s
		}
	}
	return 1
}

// Graphical reports whether the target paints pixels rather than
// character cells (the D1 mode query). Widgets use it to select
// their graphical rendering material - cell targets get the cell
// idiom, always.
func (p *Painter) Graphical() bool {
	gm, ok := p.backend.(GraphicalModer)
	return ok && gm.GraphicalMode()
}

// FillPattern tiles an 8x8 two-color bitmap across the rect when the
// backend supports it (see PatternFiller). Returns false on cell
// surfaces; the caller then falls back to its rune fill.
func (p *Painter) FillPattern(r UnitRect, pattern [8]uint8, chunkPx int, s style.CellStyle) bool {
	pf, ok := p.backend.(PatternFiller)
	if !ok {
		return false
	}
	screenRect := p.transform.ApplyRect(r)
	p.applyClip()
	pf.FillPattern(screenRect, pattern, chunkPx, s)
	return true
}

// StrokeRoundedRect paints only the rounded rectangle's stroke,
// leaving the interior untouched, when the backend supports it.
// Returns false on cell surfaces.
func (p *Painter) StrokeRoundedRect(r UnitRect, radius Unit, border style.BorderStyle, s style.CellStyle) bool {
	rd, ok := p.backend.(RoundedRectDrawer)
	if !ok {
		return false
	}
	screenRect := p.transform.ApplyRect(r)
	p.applyClip()
	rd.StrokeRoundedRect(screenRect, radius, border, s)
	return true
}

// DrawCaret draws a text-insertion caret at the left edge of the
// glyph box at (x, y) when the backend supports bar carets (see
// CaretDrawer). Returns false on cell surfaces; the caller then
// falls back to its cell-idiom caret (reverse-video block).
func (p *Painter) DrawCaret(x, y, height Unit, s style.CellStyle) bool {
	cd, ok := p.backend.(CaretDrawer)
	if !ok {
		return false
	}
	sx, sy := p.toScreen(x, y)
	p.applyClip()
	cd.DrawCaret(sx, sy, height, s)
	return true
}

// ScreenHeightToLocal converts a screen-space height into local
// units under the painter's current transform. Font metrics
// (LineHeight, MeasureText) are screen-space: glyph rasters ignore
// denomination scaling, so layout math inside re-denominated
// interiors must convert them before mixing with local coordinates.
func (p *Painter) ScreenHeightToLocal(h Unit) Unit {
	r := p.transform.Inverse().ApplyRect(UnitRect{Height: h})
	return r.Height
}

// ScreenWidthToLocal is ScreenHeightToLocal for the X axis.
func (p *Painter) ScreenWidthToLocal(w Unit) Unit {
	r := p.transform.Inverse().ApplyRect(UnitRect{Width: w})
	return r.Width
}

// DrawText draws a string using the specified font.
// If font is nil, uses DefaultFont().
func (p *Painter) DrawText(x, y Unit, text string, s style.CellStyle, font *Font) Unit {
	sx, sy := p.toScreen(x, y)
	p.applyClip()
	return p.backend.DrawText(sx, sy, text, s, font)
}

// DrawTextAligned draws text aligned within a box using the specified font.
// If font is nil, uses DefaultFont().
func (p *Painter) DrawTextAligned(bounds UnitRect, text string, hAlign, vAlign Alignment, s style.CellStyle, font *Font) {
	screenBounds := p.transform.ApplyRect(bounds)
	p.applyClip()
	p.backend.DrawTextAligned(screenBounds, text, hAlign, vAlign, s, font)
}

// FillRect fills a rectangle.
func (p *Painter) FillRect(r UnitRect, ch rune, s style.CellStyle) {
	screenRect := p.transform.ApplyRect(r)
	p.applyClip()
	p.backend.FillRect(screenRect, ch, s)
}

// DrawRect draws a rectangle border.
func (p *Painter) DrawRect(r UnitRect, border style.BorderStyle, s style.CellStyle) {
	screenRect := p.transform.ApplyRect(r)
	p.applyClip()
	p.backend.DrawRect(screenRect, border, s)
}

// DrawHLine draws a horizontal line.
func (p *Painter) DrawHLine(x, y, width Unit, ch rune, s style.CellStyle) {
	sx, sy := p.toScreen(x, y)
	p.applyClip()
	p.backend.DrawHLine(sx, sy, width, ch, s)
}

// DrawVLine draws a vertical line.
func (p *Painter) DrawVLine(x, y, height Unit, ch rune, s style.CellStyle) {
	sx, sy := p.toScreen(x, y)
	p.applyClip()
	p.backend.DrawVLine(sx, sy, height, ch, s)
}

// DrawBox draws a box with optional title.
func (p *Painter) DrawBox(r UnitRect, border style.BorderStyle, title string, s style.CellStyle) {
	screenRect := p.transform.ApplyRect(r)
	p.applyClip()
	p.backend.DrawBox(screenRect, border, title, s)
}

// Clear fills a rectangle with space characters.
func (p *Painter) Clear(r UnitRect, s style.CellStyle) {
	p.FillRect(r, ' ', s)
}

// TextWidth returns the width needed for text in units using the specified font.
// If font is nil, uses DefaultFont().
func (p *Painter) TextWidth(text string, font *Font) Unit {
	if font == nil {
		font = DefaultFont()
	}
	return font.MeasureText(text)
}

// Size returns a size in units for the given cell dimensions.
func (p *Painter) Size(cols, rows int) UnitSize {
	return p.metrics.CellsToUnits(cols, rows)
}
