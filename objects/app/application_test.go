package app

import "testing"

// Every application gets a stable, unique, non-zero protocol identity on
// creation - the same ObjectID space windows and trinkets draw from - so a
// running app can be referred to (and, in time, set) over the protocol.
func TestApplicationObjectID(t *testing.T) {
	a := New(nil)
	b := New(nil)
	s := NewSecondary()

	if a.ObjectID() == 0 || b.ObjectID() == 0 || s.ObjectID() == 0 {
		t.Fatalf("object IDs must be non-zero: %d %d %d", a.ObjectID(), b.ObjectID(), s.ObjectID())
	}
	if a.ObjectID() == b.ObjectID() || a.ObjectID() == s.ObjectID() || b.ObjectID() == s.ObjectID() {
		t.Errorf("object IDs must be unique: %d %d %d", a.ObjectID(), b.ObjectID(), s.ObjectID())
	}
	if a.ObjectID() != a.ObjectID() {
		t.Error("ObjectID must be stable across calls")
	}
}
