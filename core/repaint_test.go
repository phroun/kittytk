package core

import (
	"testing"
	"time"

	"github.com/phroun/kittytk/style"
)

// trackerBox is a container that counts subtree repaint notifications.
// It is a Container only so far as SetParent needs — the walk asks for
// parents, never for children.
type trackerBox struct {
	TrinketBase
	children []Trinket
	notes    uint64
}

func (b *trackerBox) Children() []Trinket            { return b.children }
func (b *trackerBox) AddChild(c Trinket)             { b.children = append(b.children, c); c.SetParent(b) }
func (b *trackerBox) RemoveChild(Trinket)            {}
func (b *trackerBox) ChildAt(UnitPoint) Trinket      { return nil }
func (b *trackerBox) Layout()                        {}
func (b *trackerBox) LayoutManager() LayoutManager   { return nil }
func (b *trackerBox) SetLayoutManager(LayoutManager) {}

func newTrackerBox() *trackerBox {
	b := &trackerBox{}
	b.TrinketBase = *NewTrinketBase()
	b.Init(b)
	return b
}

func (b *trackerBox) NoteSubtreeRepaint()            { b.notes++ }
func (b *trackerBox) SubtreeRepaintRevision() uint64 { return b.notes }

// plainLeaf is a trinket that tracks nothing, so a walk must pass
// straight through it.
type plainLeaf struct{ TrinketBase }

func newPlainLeaf() *plainLeaf {
	l := &plainLeaf{}
	l.TrinketBase = *NewTrinketBase()
	l.Init(l)
	return l
}

// nest wires child into parent, the way a container would.
func nest(parent *trackerBox, child Trinket) {
	parent.AddChild(child)
}

// Update() on a deep trinket must reach EVERY tracker above it, not just
// the nearest. A window nested inside another paints into its ancestor's
// surface, so an ancestor that thought itself clean would never carry
// the change to the screen.
func TestUpdateNotifiesEveryAncestorTracker(t *testing.T) {
	outer := newTrackerBox()
	inner := newTrackerBox()
	leaf := newPlainLeaf()

	nest(outer, inner)
	nest(inner, leaf)

	leaf.Update()

	if outer.notes != 1 {
		t.Errorf("outer tracker got %d notifications, want 1", outer.notes)
	}
	if inner.notes != 1 {
		t.Errorf("inner tracker got %d notifications, want 1", inner.notes)
	}
}

// A tracker is notified when it is itself the trinket that changed.
func TestUpdateNotifiesSelf(t *testing.T) {
	box := newTrackerBox()
	box.Update()
	if box.notes != 1 {
		t.Errorf("tracker got %d notifications for its own Update, want 1", box.notes)
	}
}

// The revision has to move for the mutations that change what a trinket
// paints, not only for an explicit Update(). Each of these sets
// needsRepaint internally, and before the ancestor notification existed
// each one was a way for a cached container to miss a change entirely.
func TestPaintAffectingSettersNotifyAncestors(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func(*plainLeaf)
	}{
		{"SetBounds", func(l *plainLeaf) { l.SetBounds(UnitRect{Width: 10, Height: 10}) }},
		{"SetPos", func(l *plainLeaf) { l.SetPos(UnitPoint{X: 5, Y: 5}) }},
		{"SetSize", func(l *plainLeaf) { l.SetSize(UnitSize{Width: 20, Height: 20}) }},
		{"SetVisible", func(l *plainLeaf) { l.SetVisible(false) }},
		{"SetEnabled", func(l *plainLeaf) { l.SetEnabled(false) }},
		{"SetMargins", func(l *plainLeaf) { l.SetMargins(UnitMargins{Left: 2}) }},
		{"SetBackgroundColor", func(l *plainLeaf) { c := style.RGB(1, 2, 3); l.SetBackgroundColor(&c) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			box := newTrackerBox()
			leaf := newPlainLeaf()
			nest(box, leaf)

			before := box.SubtreeRepaintRevision()
			tc.mutate(leaf)
			if box.SubtreeRepaintRevision() == before {
				t.Errorf("%s did not move the ancestor's repaint revision; "+
					"a container caching rendered pixels would never learn of it", tc.name)
			}
		})
	}
}

// The walk must terminate even if the tree is cyclic — a hang inside
// Update() would freeze the whole UI.
func TestNoteSubtreeRepaintStopsOnCycle(t *testing.T) {
	a := newTrackerBox()
	b := newTrackerBox()
	nest(a, b)
	nest(b, a) // cycle

	done := make(chan struct{})
	go func() {
		noteSubtreeRepaint(a)
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("noteSubtreeRepaint did not terminate on a parent cycle")
	}
}
