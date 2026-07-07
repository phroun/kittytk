package widgets

import (
	"testing"

	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/window"
)

// Desktop.FocusedWidget reaches through the active window to its
// focused widget - the source the Edit menu commands use so they act
// on the same target as an edit box's context menu (crucial on cell
// surfaces, where the context menu doesn't exist).
func TestDesktopFocusedWidgetReachesActiveWindow(t *testing.T) {
	t.Cleanup(func() { core.SetTextMeasurer(nil) })
	d := NewDesktop()
	d.SetBackend(&nullBackend{})

	input := NewTextInput()
	input.SetText("hello world")
	win := window.NewWindow("host")
	win.SetContent(input)
	d.WindowManager().AddWindow(win)
	win.SetBounds(core.UnitRect{X: 0, Y: 0, Width: 200, Height: 120})
	win.Layout()
	d.WindowManager().ActivateWindow(win)
	input.SetFocus()

	if got := d.FocusedWidget(); got != input.Self() && got != core.Widget(input) {
		t.Fatalf("FocusedWidget = %v, want the text input", got)
	}

	// The Edit-menu-style path: select all, then Copy through the
	// focused-widget interface, must match the direct method.
	ea, ok := d.FocusedWidget().(interface {
		Copy()
		SelectAll()
	})
	if !ok {
		t.Fatal("focused widget does not expose the edit actions")
	}
	ea.SelectAll()
	if input.SelectedText() != "hello world" {
		t.Errorf("SelectAll via focused widget selected %q", input.SelectedText())
	}
}
