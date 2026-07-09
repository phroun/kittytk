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

// A ListView's default width is "30 characters"; that must track
// font_size, not stay a fixed 240 units. Render at 6pt and 12pt and
// assert the box width scales with the cell width (so it is ~30 cells at
// either size, not a huge box around tiny text).
func TestListViewWidthTracksFontSize(t *testing.T) {
	t.Cleanup(func() { core.SetTextMeasurer(nil) })
	dir := os.Getenv("KITTYTK_PROOF_DIR")
	if dir == "" {
		dir = t.TempDir()
	}
	widthCells := map[int]int{}
	for _, size := range []int{6, 12} {
		b, err := raster.New(600, 260)
		if err != nil {
			t.Fatal(err)
		}
		m := raster.CellMetricsForFontSize(size)
		b.SetCellMetrics(m)

		d := NewDesktop()
		d.SetBackend(b)
		d.SetFont(&core.Font{Name: "ui-text", Size: size})

		lv := NewListView()
		lv.SetParent(d)
		for i := 1; i <= 12; i++ {
			lv.AddItem(NewListItem("Item " + strconv.Itoa(i)))
		}
		hint := lv.SizeHint()
		widthCells[size] = int(hint.Width / m.CellWidth)

		lv.SetBounds(core.UnitRect{Width: hint.Width, Height: 10 * m.CellHeight})
		b.Clear(style.DefaultStyle())
		lv.Paint(core.NewPainter(b))
		out := filepath.Join(dir, "listview_"+strconv.Itoa(size)+".png")
		if err := b.WritePNG(out); err != nil {
			t.Fatal(err)
		}
		t.Logf("font_size=%d cell=%+v hint.Width=%d (%d cells) -> %s",
			size, m, hint.Width, widthCells[size], out)
	}
	// ~30 cells wide at both sizes (not a fixed unit width).
	if widthCells[6] != widthCells[12] {
		t.Errorf("ListView width in cells changed with font_size: 6pt=%d 12pt=%d cells",
			widthCells[6], widthCells[12])
	}
	if widthCells[12] != 30 {
		t.Errorf("ListView width = %d cells, want 30", widthCells[12])
	}
}
