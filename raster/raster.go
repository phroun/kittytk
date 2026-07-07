// Package raster is the pixel implementation of tuitk's rendering
// primitives (D23): the same core.RenderBackend interface the TUI
// speaks, drawn onto an RGBA framebuffer with real font glyphs and
// real lines. There is no glyph-grid emulation stage - DrawRect
// draws lines, not box runes; DrawText rasterizes a TTF; the whole
// desktop renders graphically through the existing widget paint
// paths.
//
// Substrates (SDL first, per D23) present this framebuffer in a
// window and feed input back; the package itself is substrate-free
// and cgo-free, so it also serves headless rendering and tests.
//
// Units: 1 unit = 1 pixel at scale 1 (D23 bring-up default). The
// root CellMetrics derive from the default font - Monday's 8x16 -
// so layout is identical to the TUI until mode-aware paint paths
// (D1) start exploiting sub-cell precision.
package raster

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"sync"

	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/style"
	"github.com/phroun/tuitk/text"
)

// Backend renders tuitk drawing primitives into an RGBA image.
type Backend struct {
	img   *image.RGBA
	w, h  int // pixels
	scale int // pixels per unit (framebuffer is w/scale x h/scale units)

	clip    core.UnitRect
	hasClip bool

	// Rounded clip (core.RoundedClipper): an additional constraint on
	// top of the rectangular clip. Window frames confine their
	// edge-to-edge content with it.
	roundClip       core.UnitRect
	roundClipRadius core.Unit
	hasRoundClip    bool

	defaultFg color.RGBA
	defaultBg color.RGBA

	// clipboard holds the local clipboard for headless use; SDL and
	// other substrates may sync it with the system clipboard.
	clipboard string
}

// New creates a framebuffer backend of the given pixel size at scale 1
// (one unit = one pixel).
func New(widthPx, heightPx int) (*Backend, error) {
	return NewScaled(widthPx, heightPx, 1)
}

// NewScaled creates a framebuffer backend where each abstract unit
// covers scale x scale pixels. Glyphs are rasterized by the shared
// text engine at the scaled size (not upsampled), so they stay sharp
// at any scale.
func NewScaled(widthPx, heightPx, scale int) (*Backend, error) {
	if scale < 1 {
		scale = 1
	}
	b := &Backend{
		img:       image.NewRGBA(image.Rect(0, 0, widthPx, heightPx)),
		w:         widthPx,
		h:         heightPx,
		scale:     scale,
		defaultFg: color.RGBA{220, 220, 220, 255},
		defaultBg: color.RGBA{16, 16, 24, 255},
	}
	return b, nil
}

// px converts an abstract-unit coordinate to framebuffer pixels.
func (b *Backend) px(u core.Unit) int { return int(u) * b.scale }

// Scale reports pixels per unit.
func (b *Backend) Scale() int { return b.scale }

// Image exposes the framebuffer (substrates blit it; tests read it).
func (b *Backend) Image() *image.RGBA { return b.img }

// WritePNG saves the framebuffer.
func (b *Backend) WritePNG(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, b.img)
}

// --- RenderBackend: lifecycle & geometry ---

func (b *Backend) Init() error { return nil }
func (b *Backend) Shutdown()   {}

// Metrics: the root cell denomination derives from the default font
// (Monday: 8x16), per D23/D8'.
func (b *Backend) Metrics() core.CellMetrics {
	return core.CellMetrics{CellWidth: 8, CellHeight: 16}
}

func (b *Backend) Size() core.UnitSize {
	return core.UnitSize{Width: core.Unit(b.w / b.scale), Height: core.Unit(b.h / b.scale)}
}

func (b *Backend) BeginFrame() {}
func (b *Backend) EndFrame()   {}

func (b *Backend) SetClip(clip core.UnitRect) {
	b.clip = clip
	// An empty clip clips EVERYTHING (a window squeezed until its
	// client area vanishes must not spill its content); "unclipped"
	// is only ever the state before the first SetClip.
	b.hasClip = true
}

// SetRoundedClip implements core.RoundedClipper: a zero rect clears
// the constraint.
func (b *Backend) SetRoundedClip(r core.UnitRect, radius core.Unit) {
	b.roundClip = r
	b.roundClipRadius = radius
	b.hasRoundClip = !r.IsEmpty()
}

// pointVisible applies every active clip constraint to a device
// pixel: framebuffer bounds, the rectangular clip, and the rounded
// clip's rect and corner arcs (hard-edged, pixel centers).
func (b *Backend) pointVisible(x, y int) bool {
	if x < 0 || y < 0 || x >= b.w || y >= b.h {
		return false
	}
	if b.hasClip {
		if x < b.px(b.clip.X) || y < b.px(b.clip.Y) ||
			x >= b.px(b.clip.X+b.clip.Width) || y >= b.px(b.clip.Y+b.clip.Height) {
			return false
		}
	}
	if b.hasRoundClip {
		rx0, ry0 := b.px(b.roundClip.X), b.px(b.roundClip.Y)
		rx1, ry1 := b.px(b.roundClip.X+b.roundClip.Width), b.px(b.roundClip.Y+b.roundClip.Height)
		if x < rx0 || y < ry0 || x >= rx1 || y >= ry1 {
			return false
		}
		rad := int(b.roundClipRadius) * b.scale
		if m := min(rx1-rx0, ry1-ry0) / 2; rad > m {
			rad = m
		}
		if rad > 0 {
			// Inside a corner box: require the pixel center within
			// the corner arc.
			cx, cy := 0.0, 0.0
			inCorner := false
			switch {
			case x < rx0+rad && y < ry0+rad:
				cx, cy, inCorner = float64(rx0+rad), float64(ry0+rad), true
			case x >= rx1-rad && y < ry0+rad:
				cx, cy, inCorner = float64(rx1-rad), float64(ry0+rad), true
			case x < rx0+rad && y >= ry1-rad:
				cx, cy, inCorner = float64(rx0+rad), float64(ry1-rad), true
			case x >= rx1-rad && y >= ry1-rad:
				cx, cy, inCorner = float64(rx1-rad), float64(ry1-rad), true
			}
			if inCorner {
				dx := float64(x) + 0.5 - cx
				dy := float64(y) + 0.5 - cy
				if dx*dx+dy*dy > float64(rad)*float64(rad) {
					return false
				}
			}
		}
	}
	return true
}

// --- Color resolution ---

// ansi16 is the classic palette (0-7 normal, 8-15 bright).
var ansi16 = [16]color.RGBA{
	{0, 0, 0, 255}, {205, 49, 49, 255}, {13, 188, 121, 255}, {229, 229, 16, 255},
	{36, 114, 200, 255}, {188, 63, 188, 255}, {17, 168, 205, 255}, {229, 229, 229, 255},
	{102, 102, 102, 255}, {241, 76, 76, 255}, {35, 209, 139, 255}, {245, 245, 67, 255},
	{59, 142, 234, 255}, {214, 112, 214, 255}, {41, 184, 219, 255}, {255, 255, 255, 255},
}

func (b *Backend) rgba(c style.Color, isFg bool) color.RGBA {
	switch {
	case c == style.ColorDefault:
		if isFg {
			return b.defaultFg
		}
		return b.defaultBg
	case c >= 0 && c < 16:
		return ansi16[c]
	case c >= 256:
		v := uint32(c - 256)
		return color.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}
	default:
		if isFg {
			return b.defaultFg
		}
		return b.defaultBg
	}
}

func (b *Backend) styleColors(s style.CellStyle) (fg, bg color.RGBA) {
	fg = b.rgba(s.Fg, true)
	bg = b.rgba(s.Bg, false)
	if s.Attrs&style.StyleReverse != 0 {
		fg, bg = bg, fg
	}
	return fg, bg
}

// --- Pixel helpers (clip-aware) ---

func (b *Backend) fillPx(x0, y0, x1, y1 int, c color.RGBA) {
	if b.hasClip {
		cx0, cy0 := b.px(b.clip.X), b.px(b.clip.Y)
		cx1, cy1 := b.px(b.clip.X+b.clip.Width), b.px(b.clip.Y+b.clip.Height)
		if x0 < cx0 {
			x0 = cx0
		}
		if y0 < cy0 {
			y0 = cy0
		}
		if x1 > cx1 {
			x1 = cx1
		}
		if y1 > cy1 {
			y1 = cy1
		}
	}
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > b.w {
		x1 = b.w
	}
	if y1 > b.h {
		y1 = b.h
	}
	if b.hasRoundClip {
		// Rounded clip active: per-pixel test (corner arcs).
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				if b.pointVisible(x, y) {
					b.img.SetRGBA(x, y, c)
				}
			}
		}
		return
	}
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			b.img.SetRGBA(x, y, c)
		}
	}
}

func blend(over, under color.RGBA, alpha float64) color.RGBA {
	mix := func(a, b uint8) uint8 {
		return uint8(float64(a)*alpha + float64(b)*(1-alpha))
	}
	return color.RGBA{mix(over.R, under.R), mix(over.G, under.G), mix(over.B, under.B), 255}
}

// Clear fills the entire surface.
func (b *Backend) Clear(s style.CellStyle) {
	_, bg := b.styleColors(s)
	saved, savedHas := b.clip, b.hasClip
	savedRound := b.hasRoundClip
	b.hasClip, b.hasRoundClip = false, false
	b.fillPx(0, 0, b.w, b.h, bg)
	b.clip, b.hasClip, b.hasRoundClip = saved, savedHas, savedRound
}

// --- Text ---

// engine is the shared text engine (D5/D6): one per process, shared
// by every raster backend (SDL recreates backends on resize; the
// engine and its font caches survive). The same engine answers
// core.TextMeasurer, so measurement and painting cannot disagree.
var (
	engineOnce sync.Once
	engineInst *text.Engine
)

func engine() *text.Engine {
	engineOnce.Do(func() {
		engineInst = text.NewEngine()
		// Well-known OS faces join the TAIL of the fallback chain:
		// the embedded faces stay authoritative, system fonts only
		// catch runes nothing embedded covers.
		engineInst.LoadSystemFallbacks()
	})
	return engineInst
}

// Engine exposes the shared text engine (e.g. to register fonts).
func (b *Backend) Engine() *text.Engine { return engine() }

// MeasureText implements core.TextMeasurer: the graphical target's
// measurement is the shaper's, matching the proportional render
// exactly. (The text-based system keeps its own cell-arithmetic
// answer; this never applies there.)
func (b *Backend) MeasureText(f *core.Font, s string) core.Unit {
	return engine().Measure(f, s)
}

// LineHeight implements core.TextMeasurer: Size * 4/3 units by the
// engine's denomination (12pt = 16 units = one default cell row).
func (b *Backend) LineHeight(f *core.Font) core.Unit {
	return engine().LineHeight(f)
}

// cellAdvance is the terminal-region advance for one rune: exactly
// one cell of layout units (two for wide characters). Used only by
// the cell primitives (DrawCell, glyph tiling), never by DrawText.
func cellAdvance(ch rune) core.Unit {
	adv := core.Unit(8)
	if isWide(ch) {
		adv += 8
	}
	return adv
}

func isWide(ch rune) bool {
	return (ch >= 0x4E00 && ch <= 0x9FFF) || (ch >= 0x3400 && ch <= 0x4DBF) ||
		(ch >= 0xAC00 && ch <= 0xD7AF) || (ch >= 0x3040 && ch <= 0x30FF) ||
		(ch >= 0xFF00 && ch <= 0xFFEF)
}

// drawRune paints one glyph inside its advance box: background fill,
// then the glyph centered horizontally, baseline-aligned.
// cellFont is the face for the CELL primitive (DrawCell): the
// monospace cell font at the size whose line box equals one cell
// row. Shaping through the engine gives chrome glyphs (checkmarks,
// markers, box drawing) the same per-rune fallback chain as text.
var cellFont = &core.Font{Name: "Monday", Size: 12}

func (b *Backend) drawRune(x, y core.Unit, ch rune, adv core.Unit, fg, bg color.RGBA, underline, transparentBg bool) {
	xPx, yPx, advPx := b.px(x), b.px(y), b.px(adv)
	if !transparentBg {
		b.fillPx(xPx, yPx, xPx+advPx, yPx+16*b.scale, bg)
	}

	if ch != ' ' && ch != 0 {
		ti := b.cachedTextImage(cellFont, string(ch), fg, color.RGBA{}, false, false)
		pad := (advPx - ti.img.Rect.Dx()) / 2
		if pad < 0 {
			pad = 0
		}
		b.compositeRGBA(xPx+pad, yPx, ti.img)
	}
	if underline {
		b.fillPx(xPx, yPx+14*b.scale, xPx+advPx, yPx+15*b.scale, fg)
	}
}

// DrawCell is the CELL primitive: terminal-style regions (D23
// carve-out - PurfecTerm and friends) paint through it, so its
// advance is exactly one cell of layout units regardless of any
// font. This is not a second text path: UI text goes through
// DrawText's shaped path below.
func (b *Backend) DrawCell(x, y core.Unit, ch rune, s style.CellStyle) {
	fg, bg := b.styleColors(s)
	b.drawRune(x, y, ch, cellAdvance(ch), fg, bg, s.Attrs&style.StyleUnderline != 0,
		s.Bg == style.ColorTransparent)
}

// textImage is one cached rendered string at the backend's scale,
// ready to blit. Opaque images include the background; transparent
// ones (style.ColorTransparent background) carry alpha and are
// composited over the framebuffer.
type textImage struct {
	img    *image.RGBA
	width  core.Unit
	opaque bool
}

// textImageCache caches rendered strings across frames (and across
// backends - SDL recreates the backend on resize; scale is in the
// key). Same two-generation scheme as the engine's shape cache.
// Without it every frame re-rasterizes every glyph outline of every
// visible string.
var textImageCache = struct {
	sync.Mutex
	epoch     uint64
	cur, prev map[string]textImage
}{cur: map[string]textImage{}, prev: map[string]textImage{}}

const textImageCacheMax = 512

func textImageKey(f *core.Font, s string, fg, bg color.RGBA, underline bool, scale int) string {
	if f == nil {
		f = core.DefaultFont()
	}
	u := byte('-')
	if underline {
		u = 'u'
	}
	return f.Name + "\x00" + string([]byte{
		byte(f.Style), byte(f.Style >> 8), byte(f.Size), byte(scale), u,
		fg.R, fg.G, fg.B, bg.R, bg.G, bg.B,
	}) + "\x00" + s
}

// cachedTextImage returns the cached render of one string (see
// textImageCache), rasterizing on a miss.
func (b *Backend) cachedTextImage(f *core.Font, s string, fg, bg color.RGBA, underline, opaque bool) textImage {
	key := textImageKey(f, s, fg, bg, underline, b.scale)
	if !opaque {
		key = "t\x00" + key
	}
	textImageCache.Lock()
	defer textImageCache.Unlock()
	if e := engine().Epoch(); e != textImageCache.epoch {
		// Font set changed: shaped output may differ - flush.
		textImageCache.epoch = e
		textImageCache.cur = map[string]textImage{}
		textImageCache.prev = map[string]textImage{}
	}
	ti, ok := textImageCache.cur[key]
	if !ok {
		if ti, ok = textImageCache.prev[key]; ok {
			textImageCache.cur[key] = ti // keep the working set warm
		}
	}
	if !ok {
		ti = b.renderTextImage(f, s, fg, bg, underline, opaque)
		if len(textImageCache.cur) >= textImageCacheMax {
			textImageCache.prev = textImageCache.cur
			textImageCache.cur = map[string]textImage{}
		}
		textImageCache.cur[key] = ti
	}
	return ti
}

// renderTextImage rasterizes one string into a fresh image: opaque
// (background filled) or transparent (alpha only, for compositing).
func (b *Backend) renderTextImage(f *core.Font, s string, fg, bg color.RGBA, underline, opaque bool) textImage {
	sp := engine().ShapeRun(f, s)
	w := sp.Width()
	h := engine().LineHeight(f)
	img := image.NewRGBA(image.Rect(0, 0, b.px(w), b.px(h)))
	if opaque {
		for i := range img.Pix {
			switch i % 4 {
			case 0:
				img.Pix[i] = bg.R
			case 1:
				img.Pix[i] = bg.G
			case 2:
				img.Pix[i] = bg.B
			case 3:
				img.Pix[i] = 255
			}
		}
	}
	text.Render(img, sp, 0, 0, b.scale, fg)
	if underline && len(sp.Lines) > 0 {
		uy := b.px(sp.Lines[0].Baseline) + b.scale
		for py := uy; py < uy+b.scale && py < img.Rect.Max.Y; py++ {
			for px := 0; px < img.Rect.Max.X; px++ {
				img.SetRGBA(px, py, fg)
			}
		}
	}
	return textImage{img: img, width: w, opaque: opaque}
}

// compositeRGBA alpha-blends a transparent text image over the
// framebuffer (source pixels are alpha-premultiplied, as image.RGBA
// defines), honoring every active clip.
func (b *Backend) compositeRGBA(xPx, yPx int, src *image.RGBA) {
	sw, sh := src.Rect.Dx(), src.Rect.Dy()
	for row := 0; row < sh; row++ {
		for col := 0; col < sw; col++ {
			s := src.RGBAAt(col, row)
			if s.A == 0 {
				continue
			}
			dx, dy := xPx+col, yPx+row
			if !b.pointVisible(dx, dy) {
				continue
			}
			if s.A == 255 {
				b.img.SetRGBA(dx, dy, s)
				continue
			}
			d := b.img.RGBAAt(dx, dy)
			inv := uint32(255 - s.A)
			b.img.SetRGBA(dx, dy, color.RGBA{
				R: uint8(uint32(s.R) + uint32(d.R)*inv/255),
				G: uint8(uint32(s.G) + uint32(d.G)*inv/255),
				B: uint8(uint32(s.B) + uint32(d.B)*inv/255),
				A: 255,
			})
		}
	}
}

// blitRGBA copies a rendered image to (xPx, yPx), honoring every
// active clip. Fully visible blits take a per-row copy.
func (b *Backend) blitRGBA(xPx, yPx int, src *image.RGBA) {
	sw, sh := src.Rect.Dx(), src.Rect.Dy()
	if sw <= 0 || sh <= 0 {
		return
	}
	fullyVisible := !b.hasRoundClip &&
		b.pointVisible(xPx, yPx) && b.pointVisible(xPx+sw-1, yPx) &&
		b.pointVisible(xPx, yPx+sh-1) && b.pointVisible(xPx+sw-1, yPx+sh-1)
	if fullyVisible {
		for row := 0; row < sh; row++ {
			so := src.PixOffset(0, row)
			do := b.img.PixOffset(xPx, yPx+row)
			copy(b.img.Pix[do:do+sw*4], src.Pix[so:so+sw*4])
		}
		return
	}
	for row := 0; row < sh; row++ {
		for col := 0; col < sw; col++ {
			if b.pointVisible(xPx+col, yPx+row) {
				b.img.SetRGBA(xPx+col, yPx+row, src.RGBAAt(col, row))
			}
		}
	}
}

// DrawText is the graphical renderer's one text path: fully shaped,
// proportional (D6). Choosing a monospaced family yields the
// cell-gridded look; there is no separate grid-quantized path here.
// Measurement (core.TextMeasurer, answered by this backend) uses the
// same engine, so the returned advance always equals the painted one.
// Rendered strings are cached: a repeated frame blits pixels instead
// of re-shaping and re-rasterizing.
func (b *Backend) DrawText(x, y core.Unit, s string, st style.CellStyle, f *core.Font) core.Unit {
	if s == "" {
		return 0
	}
	fg, bg := b.styleColors(st)
	underline := st.Attrs&style.StyleUnderline != 0
	opaque := st.Bg != style.ColorTransparent

	ti := b.cachedTextImage(f, s, fg, bg, underline, opaque)

	if ti.opaque {
		b.blitRGBA(b.px(x), b.px(y), ti.img)
	} else {
		b.compositeRGBA(b.px(x), b.px(y), ti.img)
	}
	return ti.width
}

func (b *Backend) DrawTextAligned(bounds core.UnitRect, s string, hAlign, vAlign core.Alignment, st style.CellStyle, f *core.Font) {
	w := engine().Measure(f, s)
	h := engine().LineHeight(f)
	x := bounds.X
	switch hAlign {
	case core.AlignCenter:
		x += (bounds.Width - w) / 2
	case core.AlignRight:
		x += bounds.Width - w
	}
	y := bounds.Y
	switch vAlign {
	case core.AlignMiddle:
		y += (bounds.Height - h) / 2
	case core.AlignBottom:
		y += bounds.Height - h
	}
	b.DrawText(x, y, s, st, f)
}

// --- Fills & lines: REAL pixels, not box runes (D23) ---

// shadeAlpha maps the classic shade runes to fg-over-bg blends.
var shadeAlpha = map[rune]float64{
	'░': 0.25, '▒': 0.5, '▓': 0.75, '█': 1.0,
}

func (b *Backend) FillRect(r core.UnitRect, ch rune, s style.CellStyle) {
	fg, bg := b.styleColors(s)
	transparent := s.Bg == style.ColorTransparent
	switch {
	case ch == ' ' || ch == 0:
		if transparent {
			return // nothing to paint: the background shows through
		}
		b.fillPx(b.px(r.X), b.px(r.Y), b.px(r.X+r.Width), b.px(r.Y+r.Height), bg)
	default:
		if a, ok := shadeAlpha[ch]; ok {
			if transparent {
				// Blend the shade's foreground over what is beneath.
				for y := b.px(r.Y); y < b.px(r.Y+r.Height); y++ {
					for x := b.px(r.X); x < b.px(r.X+r.Width); x++ {
						b.blendPx(x, y, fg, a)
					}
				}
				return
			}
			b.fillPx(b.px(r.X), b.px(r.Y), b.px(r.X+r.Width), b.px(r.Y+r.Height), blend(fg, bg, a))
			return
		}
		// Arbitrary fill character: tile the glyph.
		for y := r.Y; y < r.Y+r.Height; y += 16 {
			for x := r.X; x < r.X+r.Width; x += 8 {
				b.drawRune(x, y, ch, 8, fg, bg, false, s.Bg == style.ColorTransparent)
			}
		}
	}
}

// borderWeight classifies a BorderStyle into line rendering.
func borderWeight(bs style.BorderStyle) (thickness int, double bool) {
	switch bs {
	case style.BorderDouble:
		return 1, true
	case style.BorderHeavy:
		return 2, false
	default:
		return 1, false
	}
}

// DrawRect draws a real rectangle outline, centered in the border
// cell band so it aligns with TUI-era layout (borders occupy one
// 8x16 cell ring).
func (b *Backend) DrawRect(r core.UnitRect, bs style.BorderStyle, s style.CellStyle) {
	fg, _ := b.styleColors(s)
	th, dbl := borderWeight(bs)
	th *= b.scale

	left := b.px(r.X) + 4*b.scale
	right := b.px(r.X+r.Width) - 4*b.scale
	top := b.px(r.Y) + 8*b.scale
	bottom := b.px(r.Y+r.Height) - 8*b.scale

	stroke := func(x0, y0, x1, y1 int) {
		b.fillPx(x0, y0, x1, y1, fg)
	}
	rect := func(l, t, rr, bt int) {
		stroke(l, t, rr, t+th)   // top
		stroke(l, bt-th, rr, bt) // bottom
		stroke(l, t, l+th, bt)   // left
		stroke(rr-th, t, rr, bt) // right
	}
	rect(left, top, right, bottom)
	if dbl {
		rect(left-3*b.scale, top-3*b.scale, right+3*b.scale, bottom+3*b.scale)
	}
}

// strokePx maps a border style to a stroke weight in device pixels:
// 2 for double (and heavy), 1 for single - hairlines, deliberately
// not multiplied by the unit scale.
func strokePx(bs style.BorderStyle) int {
	if bs == style.BorderDouble || bs == style.BorderHeavy {
		return 2
	}
	return 1
}

// inClip reports whether a device pixel is inside every active clip.
func (b *Backend) inClip(x, y int) bool {
	return b.pointVisible(x, y)
}

// blendPx composites c over the existing pixel with coverage a.
func (b *Backend) blendPx(x, y int, c color.RGBA, a float64) {
	if a <= 0 || !b.inClip(x, y) {
		return
	}
	if a >= 1 {
		b.img.SetRGBA(x, y, c)
		return
	}
	b.img.SetRGBA(x, y, blend(c, b.img.RGBAAt(x, y), a))
}

// DrawArcWedge implements core.ArcWedgeDrawer: per-pixel coverage
// against the quarter ellipse inscribed in r and centered on the
// chosen corner, so silhouette curves come out antialiased. The part
// of r outside the arc fills with the style's background; a stroke of
// the given weight follows the arc in its foreground (0 = no stroke).
func (b *Backend) DrawArcWedge(r core.UnitRect, centerRight, centerBottom bool, strokeW core.Unit, s style.CellStyle) {
	fg, bg := b.styleColors(s)
	x0, y0 := b.px(r.X), b.px(r.Y)
	x1, y1 := b.px(r.X+r.Width), b.px(r.Y+r.Height)
	w, h := x1-x0, y1-y0
	if w <= 0 || h <= 0 {
		return
	}
	rx, ry := float64(w), float64(h)
	cx, cy := float64(x0), float64(y0)
	if centerRight {
		cx = float64(x1)
	}
	if centerBottom {
		cy = float64(y1)
	}
	th := float64(int(strokeW) * b.scale)
	clampCov := func(v float64) float64 {
		if v < 0 {
			return 0
		}
		if v > 1 {
			return 1
		}
		return v
	}
	for py := y0; py < y1; py++ {
		for px := x0; px < x1; px++ {
			nx := (float64(px) + 0.5 - cx) / rx
			ny := (float64(py) + 0.5 - cy) / ry
			nd := math.Hypot(nx, ny)
			// Signed pixel distance outside the arc: the normalized
			// distance converted back through the local radial
			// gradient (exactly d - r for a circle).
			pd := -math.Min(rx, ry)
			if nd > 0 {
				pd = (nd - 1) * nd / math.Hypot(nx/rx, ny/ry)
			}
			cov := clampCov(pd + th + 0.5) // stroke band + wedge fill
			fillCov := clampCov(pd + 0.5)  // wedge fill only
			if th <= 0 {
				cov = fillCov
			}
			if cov <= 0 {
				continue
			}
			src := bg
			if fillCov < cov {
				src = blend(bg, fg, fillCov/cov)
			}
			b.blendPx(px, py, src, cov)
		}
	}
}

// DrawRoundedRect implements core.RoundedRectDrawer: one pass paints
// the fill (style background) and the stroke (style foreground, at
// strokePx weight) with anti-aliased corners. Window frames on pixel
// surfaces are exactly one of these.
func (b *Backend) DrawRoundedRect(r core.UnitRect, radius core.Unit, bs style.BorderStyle, s style.CellStyle) {
	b.roundedRect(r, radius, bs, s, true)
}

// StrokeRoundedRect implements the stroke-only variant: the interior
// is left untouched. Window frames re-stroke over their edge-to-edge
// content with this.
func (b *Backend) StrokeRoundedRect(r core.UnitRect, radius core.Unit, bs style.BorderStyle, s style.CellStyle) {
	b.roundedRect(r, radius, bs, s, false)
}

func (b *Backend) roundedRect(r core.UnitRect, radius core.Unit, bs style.BorderStyle, s style.CellStyle, fill bool) {
	fg, bg := b.styleColors(s)
	x0, y0 := b.px(r.X), b.px(r.Y)
	x1, y1 := b.px(r.X+r.Width), b.px(r.Y+r.Height)
	w, h := x1-x0, y1-y0
	if w <= 0 || h <= 0 {
		return
	}
	th := strokePx(bs)
	rad := int(radius) * b.scale
	if m := min(w, h) / 2; rad > m {
		rad = m
	}

	if rad <= 0 {
		if fill {
			b.fillPx(x0+th, y0+th, x1-th, y1-th, bg)
		}
		b.fillPx(x0, y0, x1, y0+th, fg)
		b.fillPx(x0, y1-th, x1, y1, fg)
		b.fillPx(x0, y0, x0+th, y1, fg)
		b.fillPx(x1-th, y0, x1, y1, fg)
		return
	}

	// Straight bands (everything outside the four corner boxes).
	b.fillPx(x0+rad, y0, x1-rad, y0+th, fg) // top stroke
	b.fillPx(x0+rad, y1-th, x1-rad, y1, fg) // bottom stroke
	b.fillPx(x0, y0+rad, x0+th, y1-rad, fg) // left stroke
	b.fillPx(x1-th, y0+rad, x1, y1-rad, fg) // right stroke
	if fill {
		b.fillPx(x0+th, y0+rad, x1-th, y1-rad, bg)  // center
		b.fillPx(x0+rad, y0+th, x1-rad, y0+rad, bg) // top band
		b.fillPx(x0+rad, y1-rad, x1-rad, y1-th, bg) // bottom band
	}

	// Corners: per-pixel coverage against the corner circle, blended
	// for smooth edges. cx/cy is the circle center in continuous
	// pixel coordinates; sx/sy the corner box origin.
	corners := [4]struct {
		cx, cy float64
		sx, sy int
	}{
		{float64(x0 + rad), float64(y0 + rad), x0, y0},             // top-left
		{float64(x1 - rad), float64(y0 + rad), x1 - rad, y0},       // top-right
		{float64(x0 + rad), float64(y1 - rad), x0, y1 - rad},       // bottom-left
		{float64(x1 - rad), float64(y1 - rad), x1 - rad, y1 - rad}, // bottom-right
	}
	clampCov := func(v float64) float64 {
		if v < 0 {
			return 0
		}
		if v > 1 {
			return 1
		}
		return v
	}
	for _, c := range corners {
		for py := c.sy; py < c.sy+rad; py++ {
			for px := c.sx; px < c.sx+rad; px++ {
				d := math.Hypot(float64(px)+0.5-c.cx, float64(py)+0.5-c.cy)
				outer := clampCov(float64(rad) - d + 0.5)
				if outer <= 0 {
					continue
				}
				inner := clampCov(float64(rad-th) - d + 0.5)
				if !fill {
					// Stroke only: paint just the band between the
					// inner and outer boundaries.
					if cov := outer - inner; cov > 0 {
						b.blendPx(px, py, fg, cov)
					}
					continue
				}
				// Source color: fill inside the inner boundary, stroke
				// in the band between inner and outer.
				src := fg
				if inner >= outer {
					src = bg
				} else if inner > 0 {
					src = blend(bg, fg, inner/outer)
				}
				b.blendPx(px, py, src, outer)
			}
		}
	}
}

func (b *Backend) DrawHLine(x, y, width core.Unit, ch rune, s style.CellStyle) {
	fg, _ := b.styleColors(s)
	b.fillPx(b.px(x), b.px(y)+8*b.scale, b.px(x+width), b.px(y)+8*b.scale+b.scale, fg)
}

func (b *Backend) DrawVLine(x, y, height core.Unit, ch rune, s style.CellStyle) {
	fg, _ := b.styleColors(s)
	b.fillPx(b.px(x)+4*b.scale, b.px(y), b.px(x)+4*b.scale+b.scale, b.px(y+height), fg)
}

func (b *Backend) DrawBox(r core.UnitRect, bs style.BorderStyle, title string, s style.CellStyle) {
	b.DrawRect(r, bs, s)
	if title != "" {
		b.DrawText(r.X+16, r.Y, " "+title+" ", s, nil)
	}
}

// --- Input & misc: the substrate's job; headless stubs here ---

// SmoothPositioning implements core.SmoothPositioner: pixel surfaces
// place window chrome at any unit position.
func (b *Backend) SmoothPositioning() bool { return true }

// GraphicalMode implements core.GraphicalModer (the D1 mode query):
// this backend paints pixels.
func (b *Backend) GraphicalMode() bool { return true }

// DrawImage implements core.ImageDrawer: composite a device-pixel
// image (alpha honored) at a unit position, respecting every active
// clip.
func (b *Backend) DrawImage(x, y core.Unit, img image.Image) {
	b.DrawImagePx(b.px(x), b.px(y), img)
}

// DrawImagePx implements core.ImageDrawer's device-pixel anchor for
// sub-unit placement (sprite fine positioning, animation offsets).
func (b *Backend) DrawImagePx(xPx, yPx int, img image.Image) {
	if rgba, ok := img.(*image.RGBA); ok {
		b.compositeRGBA(xPx, yPx, rgba)
		return
	}
	// Generic path for other image types.
	bnds := img.Bounds()
	for row := 0; row < bnds.Dy(); row++ {
		for col := 0; col < bnds.Dx(); col++ {
			dx, dy := xPx+col, yPx+row
			if !b.pointVisible(dx, dy) {
				continue
			}
			r, g, bl, a := img.At(bnds.Min.X+col, bnds.Min.Y+row).RGBA()
			if a == 0 {
				continue
			}
			if a == 0xffff {
				b.img.SetRGBA(dx, dy, color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(bl >> 8), 255})
				continue
			}
			d := b.img.RGBAAt(dx, dy)
			inv := 0xffff - a
			b.img.SetRGBA(dx, dy, color.RGBA{
				R: uint8((r + uint32(d.R)*0x101*inv/0xffff) >> 8),
				G: uint8((g + uint32(d.G)*0x101*inv/0xffff) >> 8),
				B: uint8((bl + uint32(d.B)*0x101*inv/0xffff) >> 8),
				A: 255,
			})
		}
	}
}

// FillPattern implements core.PatternFiller: an 8x8 two-color bitmap
// tiled across the rect, each pattern bit chunked to chunkPx x
// chunkPx device pixels, anchored at the surface origin.
func (b *Backend) FillPattern(r core.UnitRect, pattern [8]uint8, chunkPx int, s style.CellStyle) {
	if chunkPx < 1 {
		chunkPx = 1
	}
	fg, bg := b.styleColors(s)
	x0, y0 := b.px(r.X), b.px(r.Y)
	x1, y1 := b.px(r.X+r.Width), b.px(r.Y+r.Height)

	// Walk whole pattern blocks (origin-anchored), filling each with
	// its color; fillPx clips each block to the rect and clips.
	for by := y0 - mod(y0, chunkPx); by < y1; by += chunkPx {
		row := pattern[(by/chunkPx)%8]
		for bx := x0 - mod(x0, chunkPx); bx < x1; bx += chunkPx {
			c := bg
			if row&(0x80>>uint((bx/chunkPx)%8)) != 0 {
				c = fg
			}
			cx0, cy0, cx1, cy1 := bx, by, bx+chunkPx, by+chunkPx
			if cx0 < x0 {
				cx0 = x0
			}
			if cy0 < y0 {
				cy0 = y0
			}
			if cx1 > x1 {
				cx1 = x1
			}
			if cy1 > y1 {
				cy1 = y1
			}
			b.fillPx(cx0, cy0, cx1, cy1, c)
		}
	}
}

func mod(a, m int) int {
	r := a % m
	if r < 0 {
		r += m
	}
	return r
}

// DrawCaret implements core.CaretDrawer: a one-unit-wide vertical bar
// at the left edge of the glyph box - where the next character would
// start. Drawn in the color a block cursor of style s would show
// (its background), so widgets pass their block-cursor style
// unchanged.
func (b *Backend) DrawCaret(x, y, height core.Unit, s style.CellStyle) {
	_, bar := b.styleColors(s)
	b.fillPx(b.px(x), b.px(y), b.px(x)+b.scale, b.px(y+height), bar)
}

func (b *Backend) PollEvent() core.Event                  { return nil }
func (b *Backend) WaitEvent() core.Event                  { return nil }
func (b *Backend) SetCursorVisible(bool)                  {}
func (b *Backend) SetCursorPosition(core.Unit, core.Unit) {}
func (b *Backend) SupportsColor() bool                    { return true }
func (b *Backend) SupportsMouse() bool                    { return true }
func (b *Backend) SupportsUnicode() bool                  { return true }
func (b *Backend) ColorDepth() int                        { return 1 << 24 }
func (b *Backend) GetClipboard() string                   { return b.clipboard }
func (b *Backend) SetClipboard(s string)                  { b.clipboard = s }
func (b *Backend) Beep()                                  {}
