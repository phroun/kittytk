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

// Ending a run commits the landing spot to the MRU front, so the next
// independent run starts from there.
func TestCycleWindowsCommitsMRUOnSessionEnd(t *testing.T) {
	m, _ := newFourWindowManager(t)

	// Cycle backward once to C, then end the run (a non-cycle key).
	m.CycleWindows(false)
	if got := activeTitle(m); got != "C" {
		t.Fatalf("landed on %q, want C", got)
	}
	m.HandleKeyPress(core.KeyPressEvent{Key: "x"}) // any non-cycle key ends the run

	// C is now MRU-front (active). A fresh backward run steps to the window
	// just before it in the committed order: [A, B, D, C] -> back from C is D.
	m.CycleWindows(false)
	if got := activeTitle(m); got != "D" {
		t.Errorf("after commit, backward from C = %q, want D", got)
	}
}
