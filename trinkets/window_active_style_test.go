package trinkets

import (
	"testing"

	"github.com/phroun/kittytk/window"
)

// A detached window lives on its own OS surface; the desktop must not mark
// it passive (which would force the single/heavy border) just because it is
// the manager's remembered previous window while no in-surface window is
// active.
func TestDetachedWindowNotPassive(t *testing.T) {
	d := NewDesktop()
	d.windowManager = window.NewWindowManager()

	w := window.NewWindow("Main")
	d.windowManager.AddWindow(w)
	d.windowManager.ActivateWindow(w)
	d.windowManager.DeactivateActiveWindow() // now previous; active == nil

	if !d.IsWindowPassive(w) {
		t.Fatal("in-surface remembered window should be passive while menu holds focus")
	}
	w.SetDetached(true)
	if d.IsWindowPassive(w) {
		t.Error("detached window must not be reported passive by the desktop")
	}
}

// quasiActivateExclusive lights exactly one top-level window; any other torn
// window still carrying a lit/heavy style is returned to inactive.
func TestQuasiActivateExclusive(t *testing.T) {
	d := NewDesktop()
	d.windowManager = window.NewWindowManager()

	a := window.NewWindow("A")
	b := window.NewWindow("B")
	// Simulate two torn windows both left quasi-active.
	a.SetQuasiActive(true)
	b.SetQuasiActive(true)
	d.windowManager.AddWindow(a)
	d.windowManager.AddWindow(b)

	d.quasiActivateExclusive(b)

	if a.IsQuasiActive() || a.IsActive() {
		t.Errorf("A should be inactive after B becomes exclusively quasi-active")
	}
	if !b.IsQuasiActive() {
		t.Errorf("B should be quasi-active")
	}
}
