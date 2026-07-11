package trinkets

import (
	"strings"
	"testing"

	"github.com/phroun/kittytk/backend/raster"
	"github.com/phroun/kittytk/core"
	"github.com/phroun/kittytk/style"
)

// Scheme ledger/header defaults: cyan on black (odd), silver on dark
// gray (even), silver on dark yellow (header).
func TestSchemeLedgerHeaderDefaults(t *testing.T) {
	s := &style.Scheme{}
	if got := s.GetLedgerOdd(); got.Fg != style.ColorCyan || got.Bg != style.ColorBlack {
		t.Errorf("LedgerOdd default = %v/%v", got.Fg, got.Bg)
	}
	if got := s.GetLedgerEven(); got.Fg != style.ColorWhite || got.Bg != style.ColorBrightBlack {
		t.Errorf("LedgerEven default = %v/%v", got.Fg, got.Bg)
	}
	if got := s.GetHeader(); got.Fg != style.ColorWhite || got.Bg != style.ColorYellow {
		t.Errorf("Header default = %v/%v", got.Fg, got.Bg)
	}
}

// ellipsizeText replaces a cut tail with an ellipsis, rune-safely, and
// leaves fitting text alone.
func TestEllipsizeText(t *testing.T) {
	b, _ := raster.New(320, 32)
	d := NewDesktop()
	d.SetBackend(b)
	p := core.NewPainter(b)
	font := d.EffectiveFont()

	if got := ellipsizeText(p, font, "short", 400); got != "short" {
		t.Errorf("fitting text changed: %q", got)
	}
	long := "a rather long caption"
	got := ellipsizeText(p, font, long, 60)
	if !strings.HasSuffix(got, "…") {
		t.Errorf("graphical ellipsis missing: %q", got)
	}
	if font.MeasureText(got) > 60 {
		t.Errorf("ellipsized text still overflows: %q", got)
	}
	// Rune safety: multibyte text must not split mid-rune.
	got = ellipsizeText(p, font, "ααααααααααααααα", 40)
	for _, r := range got {
		if r == '�' {
			t.Errorf("mid-rune split: %q", got)
		}
	}
}

// Pixel surfaces reserve NO divider cell: spans run edge to edge, with
// the hairline on the boundary. The TUI keeps its one-cell divider.
func TestTreeColumnNoDividerCellOnPixels(t *testing.T) {
	b, _ := raster.New(640, 240)
	d := NewDesktop()
	d.SetBackend(b)
	tv := newColumnsTree(60, 10)
	tv.SetParent(d) // desktop backend => smooth positioning

	lay := tv.columnLayout()
	for i := 0; i+1 < len(lay.spans); i++ {
		if got := lay.spans[i+1].x - (lay.spans[i].x + lay.spans[i].w); got != 0 {
			t.Errorf("span %d..%d gap = %d units, want 0 on pixels", i, i+1, got)
		}
		if lay.spans[i].divX != lay.spans[i].x+lay.spans[i].w {
			t.Errorf("divider %d not on the boundary", i)
		}
	}

	// Same tree without a smooth ancestor: one divider cell between spans.
	tv2 := newColumnsTree(60, 10)
	lay2 := tv2.columnLayout()
	cw := tv2.EffectiveCellMetrics().CellWidth
	if got := lay2.spans[1].x - (lay2.spans[0].x + lay2.spans[0].w); got != cw {
		t.Errorf("TUI divider gap = %d, want one cell (%d)", got, cw)
	}
}

// The last pinned-left column's boundary hairline must survive the
// horizontal-scroll edge fade: the left fade starts exactly on that
// hairline, so it is repainted OVER the fade - otherwise scrolling
// erases it and the pinned column visually merges with the scrolled
// content.
func TestTreePinnedDividerAboveFade(t *testing.T) {
	b, _ := raster.New(640, 240)
	d := NewDesktop()
	d.SetBackend(b)
	tv := newColumnsTree(60, 10)
	tv.SetParent(d)
	tv.SetFitWidth(false)
	tv.SetKeyWidth(15)
	tv.SetFixedColumns(1, 0)
	tv.scrollHorizontally(4) // panned right: the left fade paints
	b.Clear(style.DefaultStyle())
	tv.Paint(core.NewPainter(b))

	lay := tv.columnLayout()
	if !lay.spans[0].fixed || lay.spans[0].divX != lay.scrollL {
		t.Fatalf("precondition: pinned key divider at scrollL (divX=%d scrollL=%d)",
			lay.spans[0].divX, lay.scrollL)
	}
	// Sample the divider pixel in a plain row band (row 2; row 0 holds
	// the selection): the fade there is the list background, and the
	// hairline must still stand out from it.
	bgR, bgG, bgB := tv.GetScheme().GetListBG().RGBComponents()
	c := b.Image().RGBAAt(int(lay.spans[0].divX), 16+2*16+8)
	abs := func(n int) int {
		if n < 0 {
			return -n
		}
		return n
	}
	diff := abs(int(c.R)-int(bgR)) + abs(int(c.G)-int(bgG)) + abs(int(c.B)-int(bgB))
	if diff < 20 {
		t.Errorf("pinned divider vanished under the fade: pixel %d,%d,%d ~= list bg %d,%d,%d",
			c.R, c.G, c.B, bgR, bgG, bgB)
	}
}

// Ledger banding: non-selected rows alternate LedgerOdd/LedgerEven,
// selection keeps the selection colors, and the blank area below the
// last row keeps the plain list background.
func TestTreeLedgerRows(t *testing.T) {
	b, _ := raster.New(480, 160)
	d := NewDesktop()
	d.SetBackend(b)
	tv := NewTreeView()
	tv.SetParent(d)
	tv.SetShowHeader(true)
	tv.AddColumn(NewTreeColumn("size", "Size", 10))
	for _, name := range []string{"aaa", "bbb", "ccc"} {
		tv.AddRootItem(NewTreeItem(name))
	}
	tv.SetLedger(true)
	tv.SetCurrentIndex(0) // rows 1 (even) and 2 (odd) show both bands
	tv.SetBounds(core.UnitRect{Width: 480, Height: 160})
	b.Clear(style.DefaultStyle())
	tv.Paint(core.NewPainter(b))

	scheme := tv.GetScheme()
	oddR, oddG, oddB := scheme.GetLedgerOdd().Bg.RGBComponents()
	evenR, evenG, evenB := scheme.GetLedgerEven().Bg.RGBComponents()
	listR, listG, listB := scheme.GetListBG().RGBComponents()

	// Sample a background pixel mid-row, away from text (x=300).
	at := func(y int) (uint8, uint8, uint8) {
		c := b.Image().RGBAAt(300, y)
		return c.R, c.G, c.B
	}
	// Row 0 is selected: NOT a ledger color.
	if r, g, bl := at(16 + 8); r == evenR && g == evenG && bl == evenB {
		t.Errorf("selected row painted in a ledger color (%d,%d,%d)", r, g, bl)
	}
	// Row 1 (index 1, 1-based row 2): ledger EVEN (dark gray).
	if r, g, bl := at(32 + 8); r != evenR || g != evenG || bl != evenB {
		t.Errorf("row 1 bg = %d,%d,%d want LedgerEven %d,%d,%d", r, g, bl, evenR, evenG, evenB)
	}
	// Row 2 (index 2, 1-based row 3): ledger ODD (black).
	if r, g, bl := at(48 + 8); r != oddR || g != oddG || bl != oddB {
		t.Errorf("row 2 bg = %d,%d,%d want LedgerOdd %d,%d,%d", r, g, bl, oddR, oddG, oddB)
	}
	// Blank space below the rows: the plain list background.
	if r, g, bl := at(120); r != listR || g != listG || bl != listB {
		t.Errorf("blank area bg = %d,%d,%d want list %d,%d,%d", r, g, bl, listR, listG, listB)
	}
	// Header band: the scheme Header background.
	hR, hG, hB := scheme.GetHeader().Bg.RGBComponents()
	if r, g, bl := at(8); r != hR || g != hG || bl != hB {
		t.Errorf("header bg = %d,%d,%d want Header %d,%d,%d", r, g, bl, hR, hG, hB)
	}
}
