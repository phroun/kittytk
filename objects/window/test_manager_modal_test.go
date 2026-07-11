package window

import "testing"

// A window in the manager is modally blocked when a modal sits above it: any
// modal blocks a non-modal window, and a later modal blocks an earlier one.
// The top modal is never blocked, and closing a modal unblocks down the stack.
func TestModalStackBlocks(t *testing.T) {
	m := NewWindowManager()
	a := NewWindow("A")
	b := NewWindow("B")
	m.AddWindow(a)
	m.AddWindow(b)

	if m.isModalBlocked(a) || m.isModalBlocked(b) {
		t.Fatal("no modal yet: nothing should be blocked")
	}

	modal := NewWindow("Modal")
	m.ShowModal(modal)
	if !m.isModalBlocked(a) || !m.isModalBlocked(b) {
		t.Error("a and b should be blocked by the modal")
	}
	if m.isModalBlocked(modal) {
		t.Error("the top modal must not be blocked")
	}

	// A second modal blocks the first.
	modal2 := NewWindow("Modal2")
	m.ShowModal(modal2)
	if !m.isModalBlocked(modal) {
		t.Error("a lower modal must be blocked by a later one")
	}
	if m.isModalBlocked(modal2) {
		t.Error("the top modal must not be blocked")
	}

	// Closing the top modal makes the previous one top (unblocked) again.
	m.CloseModal()
	if m.isModalBlocked(modal) {
		t.Error("modal is top again after CloseModal, must not be blocked")
	}
	if !m.isModalBlocked(a) {
		t.Error("a stays blocked while any modal remains")
	}

	// Closing the last modal unblocks everything.
	m.CloseModal()
	if m.isModalBlocked(a) || m.isModalBlocked(b) {
		t.Error("no modals left: nothing should be blocked")
	}
}

// A detached (torn-off) window lives on its own surface and is never blocked
// by the desktop manager's modal stack.
func TestDetachedWindowNotModalBlocked(t *testing.T) {
	m := NewWindowManager()
	torn := NewWindow("Torn")
	torn.SetDetached(true)
	m.AddWindow(torn)
	m.ShowModal(NewWindow("Modal"))

	if m.isModalBlocked(torn) {
		t.Error("a detached window must not be blocked by the desktop modal stack")
	}
}

// Adding a window to the desktop while a modal is up must leave the modal on
// top with focus, not the newly added window.
func TestAddWindowKeepsModalOnTop(t *testing.T) {
	m := NewWindowManager()
	modal := NewWindow("Modal")
	m.ShowModal(modal)

	later := NewWindow("Later")
	m.AddWindow(later)

	if m.ActiveWindow() != modal {
		t.Errorf("active window = %v, want the modal to stay on top", m.ActiveWindow())
	}
}

// Adding the modal itself (the ShowModal path) must not demote it: the raise
// is exempt when the added window is the top modal.
func TestShowModalActivatesTheModal(t *testing.T) {
	m := NewWindowManager()
	base := NewWindow("Base")
	m.AddWindow(base)

	modal := NewWindow("Modal")
	m.ShowModal(modal)

	if m.ActiveWindow() != modal {
		t.Errorf("active window = %v, want the freshly shown modal", m.ActiveWindow())
	}
}

// A click on a modally-blocked window or the wallpaper restores the top modal
// when it is minimized, firing the restore callback (dock removal) just like a
// dock-item click.
func TestRestoreMinimizedTopModal(t *testing.T) {
	m := NewWindowManager()
	modal := NewWindow("Modal")
	m.ShowModal(modal)

	restored := 0
	m.SetOnWindowRestored(func(*Window) { restored++ })
	m.MinimizeWindow(modal)
	if !modal.IsMinimized() {
		t.Fatal("modal should be minimized")
	}

	if !m.restoreMinimizedTopModal() {
		t.Fatal("restoreMinimizedTopModal should report it restored the modal")
	}
	if modal.IsMinimized() {
		t.Error("modal should be restored (not minimized)")
	}
	if restored != 1 {
		t.Errorf("restore callback fired %d times, want 1", restored)
	}

	// No modal minimized now: nothing to restore.
	if m.restoreMinimizedTopModal() {
		t.Error("restoreMinimizedTopModal should be a no-op when the modal is not minimized")
	}
}

// RaiseTopModalOver does nothing while the top modal is minimized - a click,
// not an automatic raise, is what surfaces a minimized modal.
func TestRaiseTopModalOverSkipsMinimized(t *testing.T) {
	m := NewWindowManager()
	modal := NewWindow("Modal")
	m.ShowModal(modal)
	m.MinimizeWindow(modal)

	later := NewWindow("Later")
	m.AddWindow(later)

	if !modal.IsMinimized() {
		t.Error("a minimized modal must not be auto-restored when a new window is added")
	}
}
