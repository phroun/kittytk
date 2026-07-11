package window

import (
	"strings"
	"testing"

	"github.com/phroun/kittytk/core"
)

// activeTitle returns the active window's title, or "" when the dock (nil) is
// the current selection.
func activeTitle(m *WindowManager) string {
	if w := m.ActiveWindow(); w != nil {
		return w.Title()
	}
	return ""
}

// cycleSeq runs n cycle steps in the given direction and records the active
// window title after each step.
func cycleSeq(m *WindowManager, forward bool, n int) []string {
	seq := make([]string, 0, n)
	for i := 0; i < n; i++ {
		m.CycleWindows(forward)
		seq = append(seq, activeTitle(m))
	}
	return seq
}

func newFourWindowManager(t *testing.T) (*WindowManager, [4]*Window) {
	t.Helper()
	m := NewWindowManager()
	var ws [4]*Window
	for i, name := range []string{"A", "B", "C", "D"} {
		ws[i] = NewWindow(name)
		m.AddWindow(ws[i])
	}
	m.ActivateWindow(ws[3]) // known start: cycleOrder [A,B,C,D], active D
	return m, ws
}

// Backward cycling must walk the full window set in reverse, not ping-pong
// between the two most-recent windows. This is the regression: promoting the
// selection to the MRU front on every step used to make M-S-Tab oscillate.
func TestCycleWindowsBackwardTraversesAll(t *testing.T) {
	m, _ := newFourWindowManager(t)

	got := cycleSeq(m, false, 4)
	want := []string{"C", "B", "A", "D"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("backward cycle = %v, want %v", got, want)
	}
}

// Forward cycling walks the full set in order and wraps.
func TestCycleWindowsForwardTraversesAll(t *testing.T) {
	m, _ := newFourWindowManager(t)

	got := cycleSeq(m, true, 4)
	want := []string{"A", "B", "C", "D"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("forward cycle = %v, want %v", got, want)
	}
}

// A window added mid-run is picked up because the run reads the live cycle
// list, not a frozen snapshot: continued cycling still reaches every window,
// the newcomer included, with no window orphaned and no ping-pong. (Adding a
// window activates it, which also commits the run - so the newcomer is where
// the next steps continue from.)
func TestCycleWindowsPicksUpWindowAddedMidRun(t *testing.T) {
	m, _ := newFourWindowManager(t)

	// Step forward once, then add E while the run is live.
	m.CycleWindows(true)
	e := NewWindow("E")
	m.AddWindow(e)

	// Over a full lap, every window (including the newcomer E) is reachable.
	seen := map[string]bool{activeTitle(m): true}
	for _, title := range cycleSeq(m, true, 5) {
		seen[title] = true
	}
	for _, name := range []string{"A", "B", "C", "D", "E"} {
		if !seen[name] {
			t.Errorf("window %q was not reachable after a mid-run add; seen=%v", name, seen)
		}
	}
}

// A closed window drops out of the run: the live list no longer contains it.
func TestCycleWindowsSkipsWindowRemovedMidRun(t *testing.T) {
	m, ws := newFourWindowManager(t)

	// Step forward once: A. Order [A,B,C,D].
	m.CycleWindows(true)

	// Remove C. Live order becomes [A,B,D].
	m.RemoveWindow(ws[2])

	got := cycleSeq(m, true, 3)
	want := []string{"B", "D", "A"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("cycle after mid-run remove = %v, want %v", got, want)
	}
}

// A genuine window interaction ends a run and commits the landing spot to the
// MRU front, so the next independent run starts from there. (endCycleSession is
// the commit primitive; in the desktop it fires when the active window itself
// handles a key, or on a window click/activation - not on menu-bar keys.)
func TestCycleWindowsCommitsMRUOnSessionEnd(t *testing.T) {
	m, _ := newFourWindowManager(t)

	// Cycle backward once to C, then a genuine window interaction commits it.
	m.CycleWindows(false)
	if got := activeTitle(m); got != "C" {
		t.Fatalf("landed on %q, want C", got)
	}
	m.endCycleSession() // stands in for a window-handled key / click

	// C is now MRU-front (active). A fresh backward run steps to the window
	// just before it in the committed order: [A, B, D, C] -> back from C is D.
	m.CycleWindows(false)
	if got := activeTitle(m); got != "D" {
		t.Errorf("after commit, backward from C = %q, want D", got)
	}
}

// On surfaces that deliver key releases (SDL), a run commits the moment all
// modifiers rise: NotifyModifiersReleased promotes the landing spot.
func TestCycleWindowsCommitsOnModifiersReleased(t *testing.T) {
	m, _ := newFourWindowManager(t)
	m.SetModifierReleaseTracked(true)

	m.CycleWindows(false) // land on C
	if got := activeTitle(m); got != "C" {
		t.Fatalf("landed on %q, want C", got)
	}
	m.NotifyModifiersReleased() // all modifiers up: lock in C

	// Committed [A,B,D,C]: a fresh backward run from C steps to D.
	m.CycleWindows(false)
	if got := activeTitle(m); got != "D" {
		t.Errorf("after modifier release, backward from C = %q, want D", got)
	}
}

// On the TUI (no modifier-release), a cycle step long after the previous one
// starts a new gesture and locks the prior run in first (the idle timer).
func TestCycleWindowsIdleLockInCommitsPriorRun(t *testing.T) {
	m, _ := newFourWindowManager(t)
	// TUI default: modifier release not tracked, so the idle timer is active.

	m.CycleWindows(false) // land on C, cycling
	if got := activeTitle(m); got != "C" {
		t.Fatalf("landed on %q, want C", got)
	}

	// Simulate more than the lock-in timeout passing since that step.
	m.mu.Lock()
	m.lastCycleAt = m.lastCycleAt.Add(-2 * cycleCommitTimeout)
	m.mu.Unlock()

	// The next step is a new gesture: it locks C in first, then steps from the
	// committed order [A,B,D,C] -> backward from C is D.
	m.CycleWindows(false)
	if got := activeTitle(m); got != "D" {
		t.Errorf("after idle lock-in, backward from C = %q, want D", got)
	}
}

// With modifier-release tracking on (SDL), the idle timer is disabled: a late
// cycle step does NOT lock in the prior run - the run keeps going as one
// gesture until modifiers rise.
func TestCycleWindowsIdleTimerDisabledWhenModifiersTracked(t *testing.T) {
	m, _ := newFourWindowManager(t)
	m.SetModifierReleaseTracked(true)

	m.CycleWindows(false) // land on C
	m.mu.Lock()
	m.lastCycleAt = m.lastCycleAt.Add(-2 * cycleCommitTimeout)
	m.mu.Unlock()

	// No idle lock-in: the run continues from the frozen order [A,B,C,D],
	// backward from C is B (not D, which a committed C would give).
	m.CycleWindows(false)
	if got := activeTitle(m); got != "B" {
		t.Errorf("with modifier tracking, backward from C = %q, want B (no idle commit)", got)
	}
}

// Menu-bar keys must NOT commit the cycle order: a key that the active window
// declines (falling through to the desktop/menu bar) leaves the MRU frozen, so
// the run's landing spot is not promoted.
func TestCycleWindowsMenuKeyDoesNotCommit(t *testing.T) {
	m, _ := newFourWindowManager(t)

	// Cycle backward to C. The MRU stays [A,B,C,D] (frozen during the run).
	m.CycleWindows(false)
	if got := activeTitle(m); got != "C" {
		t.Fatalf("landed on %q, want C", got)
	}

	// A key the active window declines (a bare window handles no "F9") falls
	// through to the desktop/menu bar and must not commit the MRU.
	m.HandleKeyPress(core.KeyPressEvent{Key: "F9"})

	// MRU uncommitted: a fresh backward run from C walks the ORIGINAL order
	// [A,B,C,D] -> back from C is B (not D, which is what a committed C gives).
	m.CycleWindows(false)
	if got := activeTitle(m); got != "B" {
		t.Errorf("after a menu-bar key, backward from C = %q, want B (MRU uncommitted)", got)
	}
}
