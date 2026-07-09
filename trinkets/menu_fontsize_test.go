package trinkets

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/phroun/kittytk/core"
	"github.com/phroun/kittytk/raster"
	"github.com/phroun/kittytk/style"
)

// A menu-bar dropdown is not parented into the trinket tree, so without
// the opener handing down its grid and font the popup would fall back to
// the built-in 8x16 / 12pt defaults and ignore the host's font_size.
// OpenMenu must give the dropdown the bar's effective metrics and font.
func TestMenuDropdownInheritsFontSize(t *testing.T) {
	cases := []struct {
		metrics core.CellMetrics
		size    int
	}{
		{core.CellMetrics{CellWidth: 8, CellHeight: 16}, 12},
		{core.CellMetrics{CellWidth: 12, CellHeight: 24}, 18},
	}
	for _, tc := range cases {
		mb := NewMenuBar()
		mb.SetHideCalendar(true)
		// Stand in for the desktop the bar would inherit from.
		m := tc.metrics
		mb.SetCellMetrics(&m)
		mb.SetFont(&core.Font{Name: "ui-text", Size: tc.size})

		edit := NewMenu("Edit")
		edit.AddItem(NewMenuItem("Undo"))
		mb.AddMenu(edit)
		idx := len(mb.menus) - 1

		mb.SetBounds(core.UnitRect{Width: 400, Height: tc.metrics.CellHeight})
		mb.OpenMenu(idx)

		if got := edit.EffectiveCellMetrics(); got != tc.metrics {
			t.Errorf("size %d: dropdown metrics = %+v, want %+v", tc.size, got, tc.metrics)
		}
		if got := edit.EffectiveFont(); got == nil || got.Size != tc.size {
			t.Errorf("size %d: dropdown font = %+v, want size %d", tc.size, got, tc.size)
		}
		// A submenu opened from this dropdown inherits the same context.
		sub := NewMenu("More")
		sub.AddItem(NewMenuItem("Deeper"))
		parentItem := NewMenuItem("More")
		parentItem.SubMenu = sub
		edit.openSubMenu(parentItem)
		if got := sub.EffectiveCellMetrics(); got != tc.metrics {
			t.Errorf("size %d: submenu metrics = %+v, want %+v", tc.size, got, tc.metrics)
		}
		if got := sub.EffectiveFont(); got == nil || got.Size != tc.size {
			t.Errorf("size %d: submenu font = %+v, want size %d", tc.size, got, tc.size)
		}

		mb.CloseMenu()
	}
}

// PNG proof: the same dropdown rendered at 12pt and 18pt. The 18pt
// dropdown is physically taller (its rows track the enlarged cell), so
// it covers more painted pixels below the bar.
func TestMenuDropdownFontSizeProof(t *testing.T) {
	t.Cleanup(func() { core.SetTextMeasurer(nil) })
	dir := t.TempDir()
	if env := os.Getenv("KITTYTK_PROOF_DIR"); env != "" {
		dir = env
	}

	var heights [2]core.Unit
	for i, size := range []int{12, 18} {
		b, err := raster.New(360, 240)
		if err != nil {
			t.Fatal(err)
		}
		b.SetCellMetrics(raster.CellMetricsForFontSize(size))
		metrics := b.Metrics()

		d := NewDesktop()
		d.SetBackend(b)
		d.SetFont(&core.Font{Name: "ui-text", Size: size})

		mb := NewMenuBar()
		mb.SetHideCalendar(true)
		mb.SetCellMetrics(&metrics)
		mb.SetFont(&core.Font{Name: "ui-text", Size: size})
		file := NewMenu("File")
		for _, it := range []string{"New", "Open", "Save", "Quit"} {
			file.AddItem(NewMenuItem(it))
		}
		mb.AddMenu(file)
		mb.SetBounds(core.UnitRect{Width: 360, Height: metrics.CellHeight})
		mb.OpenMenu(0)

		heights[i] = file.DropdownBounds().Height

		b.Clear(style.DefaultStyle())
		mb.Paint(core.NewPainter(b))
		mb.PaintDropdown(core.NewPainter(b))
		out := filepath.Join(dir, "menu_fontsize_"+strconv.Itoa(size)+".png")
		if err := b.WritePNG(out); err != nil {
			t.Fatalf("WritePNG: %v", err)
		}
		t.Logf("font_size=%d -> cell %+v, dropdown height %d units, png %s",
			size, metrics, heights[i], out)
		mb.CloseMenu()
	}

	if heights[1] <= heights[0] {
		t.Errorf("dropdown did not grow with font_size: 12pt=%d 18pt=%d", heights[0], heights[1])
	}
}
