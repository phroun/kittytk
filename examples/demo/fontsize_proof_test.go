package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/phroun/kittytk/core"
	"github.com/phroun/kittytk/protocol"
	"github.com/phroun/kittytk/raster"
	"github.com/phroun/kittytk/style"
	"github.com/phroun/kittytk/trinkets"
	"github.com/phroun/kittytk/window"
)

// buildDemoWindow parses the shared main-window script onto a fresh
// desktop whose backend is denominated for the given UI point size,
// the same wiring kittytk-sdl performs for cfg.FontSize.
func buildDemoWindow(t *testing.T, fontSize int) (*raster.Backend, *window.Window) {
	t.Helper()
	t.Cleanup(func() { core.SetTextMeasurer(nil) })

	b, err := raster.New(1024, 768)
	if err != nil {
		t.Fatal(err)
	}
	b.SetCellMetrics(raster.CellMetricsForFontSize(fontSize))

	d := trinkets.NewDesktop()
	d.SetBackend(b)
	d.SetFont(&core.Font{Name: "ui-text", Size: fontSize})
	d.SetBounds(core.UnitRect{Width: 1024, Height: 768})
	d.WindowManager().SetScreenBounds(core.UnitRect{Width: 1024, Height: 768})

	ctx := &protocol.BindContext{Dispatch: func(string) {}}
	factory := &idCaptureFactory{
		inner: protocol.NewRegistryFactory(ctx),
		byID:  make(map[uint64]any),
	}
	script, err := protocol.Parse(mainWindowScript())
	if err != nil {
		t.Fatal(err)
	}
	reply, err := protocol.NewSession().Execute(script, factory)
	if err != nil {
		t.Fatal(err)
	}
	win := factory.byID[reply.IDs["w"]].(*window.Window)
	d.WindowManager().AddWindow(win)
	win.SetBounds(core.UnitRect{X: 0, Y: 0, Width: 1024, Height: 768})
	win.SetActive(true)
	win.Layout()
	return b, win
}

// FontSize scales the whole desktop's type: the demo window paints its
// chrome intact at a larger point size, and the same title measures
// physically wider (real font pixels, not a re-denominated grid). This
// is the kittytk-sdl font_size knob end to end; at 12pt it is the
// historical default. PNGs are written for eyeballing.
func TestFontSizeScalesDesktop(t *testing.T) {
	dir := t.TempDir()
	if env := os.Getenv("KITTYTK_PROOF_DIR"); env != "" {
		dir = env
	}

	var widths [2]core.Unit
	for i, size := range []int{12, 18} {
		b, win := buildDemoWindow(t, size)

		// A larger font_size grows the root cell so chrome still fits.
		if size == 12 {
			if m := b.Metrics(); m.CellWidth != 8 || m.CellHeight != 16 {
				t.Fatalf("12pt root cell = %+v, want 8x16", m)
			}
		}

		// The installed measurer answers in real font pixels: the title
		// gets wider as the point size grows.
		widths[i] = (&core.Font{Name: "ui-text", Size: size}).MeasureText("KittyTK Demo")

		b.Clear(style.DefaultStyle())
		win.Paint(core.NewPainter(b))
		out := filepath.Join(dir, "fontsize_"+itoa(size)+".png")
		if err := b.WritePNG(out); err != nil {
			t.Fatalf("WritePNG: %v", err)
		}
		t.Logf("font_size=%d -> root cell %+v, title width %d units, png %s",
			size, b.Metrics(), widths[i], out)
	}

	if widths[1] <= widths[0] {
		t.Errorf("title did not grow with font_size: 12pt=%d 18pt=%d", widths[0], widths[1])
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [4]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
