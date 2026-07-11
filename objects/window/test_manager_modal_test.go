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
