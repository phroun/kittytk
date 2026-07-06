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
	"os"

	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gomono"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"

	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/style"
)

// Backend renders tuitk drawing primitives into an RGBA image.
type Backend struct {
	img  *image.RGBA
	w, h int // pixels (== units at scale 1)

	clip    core.UnitRect
	hasClip bool

	face   font.Face
	ascent int

	defaultFg color.RGBA
	defaultBg color.RGBA
}

// New creates a framebuffer backend of the given pixel size.
func New(widthPx, heightPx int) (*Backend, error) {
	ft, err := opentype.Parse(gomono.TTF)
	if err != nil {
		return nil, err
	}
	// Size 10pt at 96dpi = 13.33px em; Go Mono's advance is 0.6em =
	// 8px - exactly Monday's per-character unit width, so measured
	// layout and rasterized glyphs agree by construction.
	face, err := opentype.NewFace(ft, &opentype.FaceOptions{
		Size: 10, DPI: 96, Hinting: font.HintingFull,
	})
	if err != nil {
		return nil, err
	}
	m := face.Metrics()

	b := &Backend{
		img:       image.NewRGBA(image.Rect(0, 0, widthPx, heightPx)),
		w:         widthPx,
		h:         heightPx,
		face:      face,
		ascent:    m.Ascent.Ceil(),
		defaultFg: color.RGBA{220, 220, 220, 255},
		defaultBg: color.RGBA{16, 16, 24, 255},
	}
	return b, nil
}

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
	return core.UnitSize{Width: core.Unit(b.w), Height: core.Unit(b.h)}
}

func (b *Backend) BeginFrame() {}
func (b *Backend) EndFrame()   {}

func (b *Backend) SetClip(clip core.UnitRect) {
	b.clip = clip
	b.hasClip = !clip.IsEmpty()
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
		cx0, cy0 := int(b.clip.X), int(b.clip.Y)
		cx1, cy1 := int(b.clip.X+b.clip.Width), int(b.clip.Y+b.clip.Height)
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
	b.hasClip = false
	b.fillPx(0, 0, b.w, b.h, bg)
	b.clip, b.hasClip = saved, savedHas
}

// --- Text ---

// runeAdvance mirrors core.Font.MeasureText for one rune.
func runeAdvance(f *core.Font, ch rune) core.Unit {
	if f == nil {
		f = core.DefaultFont()
	}
	return f.MeasureText(string(ch))
}

// drawRune paints one glyph inside its advance box: background fill,
// then the glyph centered horizontally, baseline-aligned.
func (b *Backend) drawRune(x, y core.Unit, ch rune, adv core.Unit, fg, bg color.RGBA, underline bool) {
	b.fillPx(int(x), int(y), int(x+adv), int(y)+16, bg)

	if ch != ' ' && ch != 0 {
		d := font.Drawer{
			Dst:  &clippedRGBA{b: b},
			Src:  image.NewUniform(fg),
			Face: b.face,
		}
		gw, _ := b.face.GlyphAdvance(ch)
		pad := (int(adv) - gw.Ceil()) / 2
		if pad < 0 {
			pad = 0
		}
		d.Dot = fixed.P(int(x)+pad, int(y)+b.ascent)
		d.DrawString(string(ch))
	}
	if underline {
		b.fillPx(int(x), int(y)+14, int(x+adv), int(y)+15, fg)
	}
}

// clippedRGBA adapts the framebuffer for font.Drawer with clip.
type clippedRGBA struct{ b *Backend }

func (c *clippedRGBA) ColorModel() color.Model { return c.b.img.ColorModel() }
func (c *clippedRGBA) Bounds() image.Rectangle { return c.b.img.Bounds() }
func (c *clippedRGBA) At(x, y int) color.Color { return c.b.img.At(x, y) }
func (c *clippedRGBA) Set(x, y int, col color.Color) {
	if c.b.hasClip {
		cl := c.b.clip
		if x < int(cl.X) || y < int(cl.Y) || x >= int(cl.X+cl.Width) || y >= int(cl.Y+cl.Height) {
			return
		}
	}
	c.b.img.Set(x, y, col)
}

func (b *Backend) DrawCell(x, y core.Unit, ch rune, s style.CellStyle) {
	fg, bg := b.styleColors(s)
	b.drawRune(x, y, ch, runeAdvance(nil, ch), fg, bg, s.Attrs&style.StyleUnderline != 0)
}

func (b *Backend) DrawText(x, y core.Unit, text string, s style.CellStyle, f *core.Font) core.Unit {
	fg, bg := b.styleColors(s)
	underline := s.Attrs&style.StyleUnderline != 0
	pen := x
	for _, ch := range text {
		adv := runeAdvance(f, ch)
		b.drawRune(pen, y, ch, adv, fg, bg, underline)
		pen += adv
	}
	return pen - x
}

func (b *Backend) DrawTextAligned(bounds core.UnitRect, text string, hAlign, vAlign core.Alignment, s style.CellStyle, f *core.Font) {
	if f == nil {
		f = core.DefaultFont()
	}
	w := f.MeasureText(text)
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
		y += (bounds.Height - f.LineHeight()) / 2
	case core.AlignBottom:
		y += bounds.Height - f.LineHeight()
	}
	b.DrawText(x, y, text, s, f)
}

// --- Fills & lines: REAL pixels, not box runes (D23) ---

// shadeAlpha maps the classic shade runes to fg-over-bg blends.
var shadeAlpha = map[rune]float64{
	'░': 0.25, '▒': 0.5, '▓': 0.75, '█': 1.0,
}

func (b *Backend) FillRect(r core.UnitRect, ch rune, s style.CellStyle) {
	fg, bg := b.styleColors(s)
	switch {
	case ch == ' ' || ch == 0:
		b.fillPx(int(r.X), int(r.Y), int(r.X+r.Width), int(r.Y+r.Height), bg)
	default:
		if a, ok := shadeAlpha[ch]; ok {
			b.fillPx(int(r.X), int(r.Y), int(r.X+r.Width), int(r.Y+r.Height), blend(fg, bg, a))
			return
		}
		// Arbitrary fill character: tile the glyph.
		for y := r.Y; y < r.Y+r.Height; y += 16 {
			for x := r.X; x < r.X+r.Width; x += 8 {
				b.drawRune(x, y, ch, 8, fg, bg, false)
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

	left := int(r.X) + 4
	right := int(r.X+r.Width) - 4
	top := int(r.Y) + 8
	bottom := int(r.Y+r.Height) - 8

	stroke := func(x0, y0, x1, y1 int) {
		b.fillPx(x0, y0, x1, y1, fg)
	}
	rect := func(l, t, rr, bt int) {
		stroke(l, t, rr, t+th)     // top
		stroke(l, bt-th, rr, bt)   // bottom
		stroke(l, t, l+th, bt)     // left
		stroke(rr-th, t, rr, bt)   // right
	}
	rect(left, top, right, bottom)
	if dbl {
		rect(left-3, top-3, right+3, bottom+3)
	}
}

func (b *Backend) DrawHLine(x, y, width core.Unit, ch rune, s style.CellStyle) {
	fg, _ := b.styleColors(s)
	b.fillPx(int(x), int(y)+8, int(x+width), int(y)+9, fg)
}

func (b *Backend) DrawVLine(x, y, height core.Unit, ch rune, s style.CellStyle) {
	fg, _ := b.styleColors(s)
	b.fillPx(int(x)+4, int(y), int(x)+5, int(y+height), fg)
}

func (b *Backend) DrawBox(r core.UnitRect, bs style.BorderStyle, title string, s style.CellStyle) {
	b.DrawRect(r, bs, s)
	if title != "" {
		b.DrawText(r.X+16, r.Y, " "+title+" ", s, nil)
	}
}

// --- Input & misc: the substrate's job; headless stubs here ---

func (b *Backend) PollEvent() core.Event                  { return nil }
func (b *Backend) WaitEvent() core.Event                  { return nil }
func (b *Backend) SetCursorVisible(bool)                  {}
func (b *Backend) SetCursorPosition(core.Unit, core.Unit) {}
func (b *Backend) SupportsColor() bool                    { return true }
func (b *Backend) SupportsMouse() bool                    { return true }
func (b *Backend) SupportsUnicode() bool                  { return true }
func (b *Backend) ColorDepth() int                        { return 1 << 24 }
func (b *Backend) GetClipboard() string                   { return "" }
func (b *Backend) SetClipboard(string)                    {}
func (b *Backend) Beep()                                  {}
