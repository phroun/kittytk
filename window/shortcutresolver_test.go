package window

import (
	"testing"

	"github.com/phroun/tuitk/core"
)

// A window with no chrome of its own (a torn-off child) services its
// app's shortcuts through the resolver the desktop installs, checked
// before the focused widget sees the key.
func TestShortcutResolverHandlesKey(t *testing.T) {
	win := NewWindow("child")

	got := ""
	win.SetShortcutResolver(func(ev core.KeyPressEvent) bool {
		if ev.Key == "C-x" {
			got = ev.Key
			return true
		}
		return false
	})

	if !win.HandleKeyPress(core.KeyPressEvent{Key: "C-x"}) {
		t.Fatal("resolver key was not consumed")
	}
	if got != "C-x" {
		t.Errorf("resolver saw %q, want C-x", got)
	}

	// A key the resolver rejects is not consumed by it.
	if win.HandleKeyPress(core.KeyPressEvent{Key: "C-q"}) {
		t.Error("unrelated key was wrongly consumed by the resolver")
	}

	// Clearing the resolver stops it servicing keys.
	win.SetShortcutResolver(nil)
	got = ""
	win.HandleKeyPress(core.KeyPressEvent{Key: "C-x"})
	if got != "" {
		t.Error("cleared resolver still fired")
	}
}
