// Package core provides fundamental types for the TUI toolkit.
package core

import (
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
	X, Y   Unit        // Position in units
	Button MouseButton // Which button
}

func (MousePressEvent) isEvent() {}

// MouseReleaseEvent represents a mouse button release.
type MouseReleaseEvent struct {
	X, Y   Unit
	Button MouseButton
}

func (MouseReleaseEvent) isEvent() {}

// MouseMoveEvent represents mouse movement.
type MouseMoveEvent struct {
	X, Y    Unit
	Buttons MouseButton // Buttons currently held
}

func (MouseMoveEvent) isEvent() {}

// MouseWheelEvent represents mouse wheel scrolling.
type MouseWheelEvent struct {
	X, Y   Unit
	DeltaX int // Horizontal scroll
	DeltaY int // Vertical scroll (positive = up)
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
	return &Painter{
		backend:   p.backend,
		transform: t.Compose(p.transform),
		clip:      p.clip,
		metrics:   p.metrics,
	}
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
	newClip := p.clip.Intersection(screenClip)
	return &Painter{
		backend:   p.backend,
		transform: p.transform,
		clip:      newClip,
		metrics:   p.metrics,
	}
}

// Clip returns the current clip rectangle in local coordinates.
func (p *Painter) Clip() UnitRect {
	inv := p.transform.Inverse()
	return inv.ApplyRect(p.clip)
}

// applyClip sets the backend clip to our current clip.
func (p *Painter) applyClip() {
	p.backend.SetClip(p.clip)
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
