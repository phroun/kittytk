package window

import (
	"testing"

	"github.com/phroun/tuitk/core"
)

// The tear handle sits in the title focus order between the maximize
// button and the title, and only when the window is tearable.
func TestTearHandleFocusOrder(t *testing.T) {
	win := NewWindow("w")
	win.SetTearable(true)

	if got := win.nextTitleFocus(TitleFocusMaximize); got != TitleFocusTear {
		t.Errorf("after maximize = %v, want Tear", got)
	}
	if got := win.nextTitleFocus(TitleFocusTear); got != TitleFocusTitle {
		t.Errorf("after tear = %v, want Title", got)
	}
	if got := win.prevTitleFocus(TitleFocusTitle); got != TitleFocusTear {
		t.Errorf("before title = %v, want Tear", got)
	}

	// Not tearable: the handle is skipped.
	plain := NewWindow("p")
	if got := plain.nextTitleFocus(TitleFocusMaximize); got != TitleFocusTitle {
		t.Errorf("non-tearable after maximize = %v, want Title", got)
	}
}

// The handle occupies the button-width slot after [x][.][^]; clicks
// there hit-test to the tear button and activate the tear callback.
func TestTearHandleHitTestAndActivate(t *testing.T) {
	win := NewWindow("w")
	win.SetTearable(true)
	win.SetBounds(core.UnitRect{Width: 300, Height: 120})

	// Local x in [80,104) is the handle slot (8px cells).
	if got := win.buttonAtPosition(88, 4); got != TitleButtonTear {
		t.Errorf("hit-test at handle = %v, want TitleButtonTear", got)
	}
	if got := win.buttonAtPosition(64, 4); got == TitleButtonTear {
		t.Error("maximize slot hit-tested as tear")
	}

	// Keyboard activation on the focused handle fires the callback.
	torn := false
	win.SetOnTearRequest(func() { torn = true })
	win.SetTitleFocus(TitleFocusTear)
	win.handleTitleBarKey(core.KeyPressEvent{Key: "Enter"})
	if !torn {
		t.Error("Enter on the focused handle did not request tear")
	}

	// Detached state flips the reported glyph state.
	win.SetDetached(true)
	if !win.IsDetached() {
		t.Error("SetDetached not reflected")
	}
}
