package trinkets

import (
	"testing"

	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/window"
)

func TestMDIPaneInheritsSmoothPositioning(t *testing.T) {
	pane := NewMDIPane()
	win := window.NewWindow("host")
	win.SetContent(pane)

	if core.FindSmoothPositioning(pane.Self()) {
		t.Fatal("MDI pane should default to snapped positioning")
	}

	win.SetSmoothPositioning(true)
	if !core.FindSmoothPositioning(pane.Self()) {
		t.Fatal("MDI pane should inherit smooth positioning from its hosting window")
	}
}
