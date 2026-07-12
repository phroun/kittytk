package garland

// Differential verification harness.
//
// A deliberately naive reference model (flat []byte + map of decoration
// positions + cursor offsets) is driven through the same randomized
// operation sequences as a real Garland, comparing content, all three
// counts, every decoration position, every cursor position, and the
// address-conversion functions after every single operation. Undo and
// fork navigation are replayed against a snapshot log of the model.
//
// The model encodes the semantics RULINGS from design review:
//   - Lines are 0-based; a newline is the last character of its line;
//     the position after a trailing newline is line N+1, rune 0.
//   - One decoration per key (map semantics).
//   - Marks are NEVER deleted by a range delete: they collapse to the
//     deletion point and are reported to the caller.
//   - Cursors are part of undo history (phase 1: observed and logged,
//     not hard-asserted, until the exact restore rule is pinned down).
//   - Invalid UTF-8 only needs to count consistently (phase 1 sticks
//     to valid UTF-8 content).
//
//   - insertBefore governs cursors exactly at the insert point the
//     same way it governs decorations (ruling: docs were right).

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"
	"unicode/utf8"
)

// ---------- reference model ----------

type refState struct {
	data    []byte
	decs    map[string]int64
	cursors []int64
}

func newRefState(data string, ncursors int) refState {
	return refState{
		data:    []byte(data),
		decs:    map[string]int64{},
		cursors: make([]int64, ncursors),
	}
}

func (s refState) clone() refState {
	c := refState{
		data:    append([]byte(nil), s.data...),
		decs:    make(map[string]int64, len(s.decs)),
		cursors: append([]int64(nil), s.cursors...),
	}
	for k, v := range s.decs {
		c.decs[k] = v
	}
	return c
}

func (s *refState) runeCount() int64 { return int64(utf8.RuneCount(s.data)) }

func (s *refState) lineCount() int64 {
	var n int64
	for _, b := range s.data {
		if b == '\n' {
			n++
		}
	}
	return n
}

// lineOf: line = newlines strictly before pos (the newline is the last
// character of its own line); runeInLine = runes since the last newline.
func (s *refState) lineOf(pos int64) (line, runeInLine int64) {
	lastNL := int64(-1)
	for i := int64(0); i < pos; i++ {
		if s.data[i] == '\n' {
			line++
			lastNL = i
		}
	}
	runeInLine = int64(utf8.RuneCount(s.data[lastNL+1 : pos]))
	return
}

// byteOfLineRune inverts lineOf.
func (s *refState) byteOfLineRune(line, runeInLine int64) int64 {
	pos := int64(0)
	for line > 0 && pos < int64(len(s.data)) {
		if s.data[pos] == '\n' {
			line--
		}
		pos++
	}
	for runeInLine > 0 && pos < int64(len(s.data)) {
		_, sz := utf8.DecodeRune(s.data[pos:])
		pos += int64(sz)
		runeInLine--
	}
	return pos
}

func (s *refState) byteToRune(pos int64) int64 {
	return int64(utf8.RuneCount(s.data[:pos]))
}

func (s *refState) runeToByte(runePos int64) int64 {
	pos := int64(0)
	for runePos > 0 && pos < int64(len(s.data)) {
		_, sz := utf8.DecodeRune(s.data[pos:])
		pos += int64(sz)
		runePos--
	}
	return pos
}

// byteOfRuneIndex returns the byte offset of the nth rune (n may equal
// runeCount, yielding len(data)).
func (s *refState) byteOfRuneIndex(n int64) int64 { return s.runeToByte(n) }

// insert applies an insertion. Decorations AND passive cursors at
// exactly pos slide right only when insertBefore (the ruling); the
// acting cursor lands after the insert.
func (s *refState) insert(actor int, pos int64, piece []byte, insertBefore bool) {
	n := int64(len(piece))
	s.data = append(s.data[:pos:pos], append(append([]byte(nil), piece...), s.data[pos:]...)...)
	for k, d := range s.decs {
		if d > pos || (d == pos && insertBefore) {
			s.decs[k] = d + n
		}
	}
	for i := range s.cursors {
		if i == actor {
			continue
		}
		if s.cursors[i] > pos || (s.cursors[i] == pos && insertBefore) {
			s.cursors[i] += n
		}
	}
	s.cursors[actor] = pos + n
}

// del applies a deletion of [pos, pos+n) and returns the keys that
// were IN the deleted range. RULING: marks are never deleted with a
// range - they collapse to the deletion point and survive; the
// returned list is a report for the caller.
func (s *refState) del(actor int, pos, n int64) []string {
	end := pos + n
	if end > int64(len(s.data)) {
		end = int64(len(s.data))
	}
	s.data = append(s.data[:pos:pos], s.data[end:]...)
	var removed []string
	for k, d := range s.decs {
		switch {
		case d >= pos && d < end:
			removed = append(removed, k)
			s.decs[k] = pos // collapse to the deletion point
		case d >= end:
			s.decs[k] = d - (end - pos)
		}
	}
	for i := range s.cursors {
		switch {
		case s.cursors[i] >= end:
			s.cursors[i] -= end - pos
		case s.cursors[i] > pos:
			s.cursors[i] = pos
		}
	}
	s.cursors[actor] = pos
	return removed
}

// ---------- harness ----------

type diffHarness struct {
	t       *testing.T
	g       *Garland
	curs    []*Cursor
	verify  *Cursor
	model   refState
	snaps   map[ForkRevision]refState
	parents map[ForkID]ForkRevision
	rnd     *rand.Rand
	oplog   []string
	soft    []string
	fork    ForkID
	rev     RevisionID

	// Fork-graph bookkeeping for prune/delete/fork-seek ops.
	// CONTRACT under test: destroying history on one fork (Prune,
	// DeleteFork) must never damage revisions another live fork can
	// still reach - shared data survives while anything depends on it.
	known  []ForkID              // every fork ever observed
	dead   map[ForkID]bool       // soft-deleted forks
	pruned map[ForkID]RevisionID // each fork's own PrunedUpTo watermark
}

func newDiffHarness(t *testing.T, seed int64, initial string) *diffHarness {
	lib, err := Init(LibraryOptions{})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	g, err := lib.Open(FileOptions{DataString: initial})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	h := &diffHarness{
		t:       t,
		g:       g,
		model:   newRefState(initial, 3),
		snaps:   map[ForkRevision]refState{},
		parents: map[ForkID]ForkRevision{},
		rnd:     rand.New(rand.NewSource(seed)),
		fork:    g.CurrentFork(),
		rev:     g.CurrentRevision(),
		dead:    map[ForkID]bool{},
		pruned:  map[ForkID]RevisionID{},
	}
	h.known = append(h.known, h.fork)
	for i := 0; i < 3; i++ {
		h.curs = append(h.curs, g.NewCursor())
	}
	h.verify = g.NewCursor()
	h.snaps[ForkRevision{h.fork, h.rev}] = h.model.clone()
	return h
}

func (h *diffHarness) logf(format string, args ...interface{}) {
	h.oplog = append(h.oplog, fmt.Sprintf(format, args...))
	if len(h.oplog) > 60 {
		h.oplog = h.oplog[len(h.oplog)-60:]
	}
}

func (h *diffHarness) fail(format string, args ...interface{}) {
	h.t.Helper()
	h.t.Errorf(format, args...)
	h.t.Logf("recent ops:\n  %s", joinLines(h.oplog))
	h.t.FailNow()
}

func joinLines(ss []string) string {
	out := ""
	for _, s := range ss {
		out += s + "\n  "
	}
	return out
}

// noteMutation records the reported (fork, revision), detecting fork
// branches, and snapshots the model.
func (h *diffHarness) noteMutation(res ChangeResult) {
	if res.Fork != h.fork {
		h.parents[res.Fork] = ForkRevision{h.fork, h.rev}
		h.known = append(h.known, res.Fork)
		// A fork inherits its parent's effective floor at branch time:
		// revisions the parent had already pruned are gone for good and
		// were never part of the child's reachable history.
		h.pruned[res.Fork] = h.pruned[h.fork]
		h.logf("  -> branched fork %d from (fork %d, rev %d)", res.Fork, h.fork, h.rev)
	}
	h.fork, h.rev = res.Fork, res.Revision
	h.snaps[ForkRevision{h.fork, h.rev}] = h.model.clone()
}

// expectedStateAt resolves a (fork, revision) through recorded lineage.
func (h *diffHarness) expectedStateAt(fork ForkID, rev RevisionID) (refState, bool) {
	for {
		if s, ok := h.snaps[ForkRevision{fork, rev}]; ok {
			return s, true
		}
		p, ok := h.parents[fork]
		if !ok {
			return refState{}, false
		}
		if rev > p.Revision {
			return refState{}, false
		}
		fork = p.Fork
	}
}

// check compares the full observable state against the model.
// cursorsHard controls whether cursor mismatches fail or are logged.
func (h *diffHarness) check(tag string, cursorsHard bool) {
	h.t.Helper()
	g, m := h.g, &h.model

	if err := h.verify.SeekByte(0); err != nil {
		h.fail("%s: verify seek: %v", tag, err)
	}
	got, err := h.verify.ReadBytes(g.ByteCount().Value)
	if err != nil {
		h.fail("%s: ReadBytes(%d): %v", tag, g.ByteCount().Value, err)
	}
	if !bytes.Equal(got, m.data) {
		h.fail("%s: content mismatch\n got %q\nwant %q", tag, got, m.data)
	}
	if bc := g.ByteCount(); bc.Value != int64(len(m.data)) {
		h.fail("%s: ByteCount = %d, want %d (content %q)", tag, bc.Value, len(m.data), got)
	}
	if rc := g.RuneCount(); rc.Value != m.runeCount() {
		h.fail("%s: RuneCount = %d, want %d (content %q)", tag, rc.Value, m.runeCount(), got)
	}
	if lc := g.LineCount(); lc.Value != m.lineCount() {
		h.fail("%s: LineCount = %d, want %d (content %q)", tag, lc.Value, m.lineCount(), got)
	}

	// Invariants: one live copy per key tree-wide, and no keys beyond
	// what the model says exist.
	if tree, err := g.GetDecorationsInByteRange(0, int64(len(m.data))+1); err == nil {
		seen := map[string]int64{}
		for _, e := range tree {
			if prev, dup := seen[e.Key]; dup {
				h.fail("%s: decoration %q duplicated in tree at %d and %d", tag, e.Key, prev, e.Address.Byte)
			}
			seen[e.Key] = e.Address.Byte
		}
		for k, p := range seen {
			if _, ok := m.decs[k]; !ok {
				h.fail("%s: stray decoration %q at byte %d (model has no such key; model decs %v)", tag, k, p, m.decs)
			}
		}
	}

	for k, want := range m.decs {
		addr, err := g.GetDecorationPosition(k)
		if err != nil {
			h.fail("%s: decoration %q missing (want byte %d): %v", tag, k, want, err)
		}
		if addr.Mode != ByteMode || addr.Byte != want {
			tree, _ := g.GetDecorationsInByteRange(0, int64(len(m.data))+1)
			var treeDump []string
			for _, e := range tree {
				treeDump = append(treeDump, fmt.Sprintf("%s@%d", e.Key, e.Address.Byte))
			}
			h.fail("%s: decoration %q at mode=%d byte=%d, want byte %d (model decs %v; tree %v)", tag, k, addr.Mode, addr.Byte, want, m.decs, treeDump)
		}
	}

	for i, c := range h.curs {
		// Model-independent coherence: a cursor's cached rune/line
		// coordinates must always agree with converting its own byte
		// position, no matter where the byte position ended up.
		gotB := c.BytePos()
		if gotB >= 0 && gotB <= int64(len(m.data)) {
			if r, err := g.ByteToRune(gotB); err == nil && c.RunePos() != r {
				h.fail("%s: cursor %d INCOHERENT runePos=%d but ByteToRune(%d)=%d", tag, i, c.RunePos(), gotB, r)
			}
			if cl, cr, err := g.ByteToLineRune(gotB); err == nil {
				if line, lr := c.LinePos(); line != cl || lr != cr {
					h.fail("%s: cursor %d INCOHERENT linePos=%d:%d but ByteToLineRune(%d)=%d:%d", tag, i, line, lr, gotB, cl, cr)
				}
			}
		}
		if got := gotB; got != m.cursors[i] {
			if cursorsHard {
				h.fail("%s: cursor %d bytePos = %d, want %d", tag, i, got, m.cursors[i])
			}
			h.soft = append(h.soft, fmt.Sprintf("%s: cursor %d bytePos = %d, want %d", tag, i, got, m.cursors[i]))
			m.cursors[i] = got // resync so later hard checks stay meaningful
			continue
		}
		if got := c.RunePos(); got != m.byteToRune(m.cursors[i]) {
			h.fail("%s: cursor %d runePos = %d, want %d", tag, i, got, m.byteToRune(m.cursors[i]))
		}
		wantLine, wantLR := m.lineOf(m.cursors[i])
		if line, lr := c.LinePos(); line != wantLine || lr != wantLR {
			cl, cr, cerr := h.g.ByteToLineRune(m.cursors[i])
			h.fail("%s: cursor %d linePos = %d:%d, want %d:%d (fresh conversion says %d:%d,%v; pos=%d content=%q)",
				tag, i, line, lr, wantLine, wantLR, cl, cr, cerr, m.cursors[i], m.data)
		}
	}

	// Spot-check address conversions at a random rune boundary.
	pos := m.byteOfRuneIndex(h.rnd.Int63n(m.runeCount() + 1))
	if r, err := g.ByteToRune(pos); err != nil || r != m.byteToRune(pos) {
		h.fail("%s: ByteToRune(%d) = %d,%v want %d", tag, pos, r, err, m.byteToRune(pos))
	}
	if b, err := g.RuneToByte(m.byteToRune(pos)); err != nil || b != pos {
		h.fail("%s: RuneToByte(%d) = %d,%v want %d", tag, m.byteToRune(pos), b, err, pos)
	}
	wl, wlr := m.lineOf(pos)
	if line, lr, err := g.ByteToLineRune(pos); err != nil || line != wl || lr != wlr {
		h.fail("%s: ByteToLineRune(%d) = %d:%d,%v want %d:%d", tag, pos, line, lr, err, wl, wlr)
	}
	if b, err := g.LineRuneToByte(wl, wlr); err != nil || b != pos {
		h.fail("%s: LineRuneToByte(%d,%d) = %d,%v want %d", tag, wl, wlr, b, err, pos)
	}
}

// ---------- operation generators ----------

var pieceRunes = []rune{'a', 'b', 'c', 'd', 'e', 'Z', 'é', '中', '🌊', '\n', '\n', '\r'}

func (h *diffHarness) randPiece() string {
	n := 1 + h.rnd.Intn(12)
	rs := make([]rune, n)
	for i := range rs {
		rs[i] = pieceRunes[h.rnd.Intn(len(pieceRunes))]
	}
	return string(rs)
}

func (h *diffHarness) randRunePos() int64 {
	return h.model.byteOfRuneIndex(h.rnd.Int63n(h.model.runeCount() + 1))
}

func (h *diffHarness) opInsert() {
	actor := h.rnd.Intn(len(h.curs))
	pos := h.randRunePos()
	piece := h.randPiece()
	before := h.rnd.Intn(2) == 0
	if err := h.curs[actor].SeekByte(pos); err != nil {
		h.fail("insert: seek(%d): %v", pos, err)
	}
	h.model.cursors[actor] = pos
	h.logf("c%d.InsertString(%q, before=%v) at %d", actor, piece, before, pos)
	res, err := h.curs[actor].InsertString(piece, nil, before)
	if err != nil {
		h.fail("InsertString: %v", err)
	}
	h.model.insert(actor, pos, []byte(piece), before)
	h.noteMutation(res)
	h.check("after insert", true)
}

func (h *diffHarness) opDelete() {
	if len(h.model.data) == 0 {
		return
	}
	actor := h.rnd.Intn(len(h.curs))
	rc := h.model.runeCount()
	startRune := h.rnd.Int63n(rc)
	maxRunes := rc - startRune
	nRunes := 1 + h.rnd.Int63n(min64(maxRunes, 10))
	start := h.model.byteOfRuneIndex(startRune)
	end := h.model.byteOfRuneIndex(startRune + nRunes)
	if err := h.curs[actor].SeekByte(start); err != nil {
		h.fail("delete: seek(%d): %v", start, err)
	}
	h.model.cursors[actor] = start
	asRunes := h.rnd.Intn(2) == 0
	var res ChangeResult
	var err error
	var got []RelativeDecoration
	if asRunes {
		h.logf("c%d.DeleteRunes(%d) at %d", actor, nRunes, start)
		got, res, err = h.curs[actor].DeleteRunes(nRunes, false)
	} else {
		h.logf("c%d.DeleteBytes(%d) at %d", actor, end-start, start)
		got, res, err = h.curs[actor].DeleteBytes(end-start, false)
	}
	if err != nil {
		h.fail("delete: %v", err)
	}
	removed := h.model.del(actor, start, end-start)
	if len(got) != len(removed) {
		var gotKeys []string
		for _, d := range got {
			gotKeys = append(gotKeys, d.Key)
		}
		h.fail("delete returned %d decorations (%v), model removed %d (%v)",
			len(got), gotKeys, len(removed), removed)
	}
	h.noteMutation(res)
	h.check("after delete", true)
}

func (h *diffHarness) opDecorate() {
	key := fmt.Sprintf("k%d", h.rnd.Intn(8))
	if _, exists := h.model.decs[key]; exists && h.rnd.Intn(3) == 0 {
		h.logf("Decorate remove %q", key)
		res, err := h.g.Decorate([]DecorationEntry{{Key: key, Address: nil}})
		if err != nil {
			h.fail("Decorate remove: %v", err)
		}
		if _, err := h.g.GetDecorationPosition(key); err == nil {
			h.fail("Decorate remove %q: decoration still present afterwards", key)
		}
		delete(h.model.decs, key)
		h.noteMutation(res)
	} else {
		pos := h.randRunePos()
		h.logf("Decorate %q at byte %d", key, pos)
		addr := ByteAddress(pos)
		res, err := h.g.Decorate([]DecorationEntry{{Key: key, Address: &addr}})
		if err != nil {
			h.fail("Decorate set: %v", err)
		}
		h.model.decs[key] = pos
		h.noteMutation(res)
	}
	h.check("after decorate", true)
}

func (h *diffHarness) opSeek() {
	actor := h.rnd.Intn(len(h.curs))
	switch h.rnd.Intn(3) {
	case 0:
		pos := h.randRunePos()
		h.logf("c%d.SeekByte(%d)", actor, pos)
		if err := h.curs[actor].SeekByte(pos); err != nil {
			h.fail("SeekByte(%d): %v", pos, err)
		}
		h.model.cursors[actor] = pos
	case 1:
		rp := h.rnd.Int63n(h.model.runeCount() + 1)
		h.logf("c%d.SeekRune(%d)", actor, rp)
		if err := h.curs[actor].SeekRune(rp); err != nil {
			h.fail("SeekRune(%d): %v", rp, err)
		}
		h.model.cursors[actor] = h.model.runeToByte(rp)
	case 2:
		pos := h.randRunePos()
		line, lr := h.model.lineOf(pos)
		h.logf("c%d.SeekLine(%d,%d) (byte %d)", actor, line, lr, pos)
		if err := h.curs[actor].SeekLine(line, lr); err != nil {
			h.fail("SeekLine(%d,%d): %v", line, lr, err)
		}
		h.model.cursors[actor] = pos
	}
	h.check("after seek", true)
}

func (h *diffHarness) opUndoRedo() {
	// Only the current fork's OWN watermark limits seeking; an
	// ancestor's prune must not affect this fork (contract).
	low := h.pruned[h.fork]
	if h.rev <= low {
		return
	}
	target := low + RevisionID(h.rnd.Int63n(int64(h.rev-low)))
	h.logf("UndoSeek(%d) from (fork %d, rev %d)", target, h.fork, h.rev)
	if err := h.g.UndoSeek(target); err != nil {
		h.fail("UndoSeek(%d): %v", target, err)
	}
	want, ok := h.expectedStateAt(h.g.CurrentFork(), target)
	if !ok {
		h.fail("harness: no snapshot for (fork %d, rev %d)", h.g.CurrentFork(), target)
	}
	h.fork, h.rev = h.g.CurrentFork(), target
	h.model = want.clone()
	h.check("after undo", false) // cursors soft: restore rule under observation

	// Half the time, seek forward again before the next edit.
	if h.rnd.Intn(2) == 0 {
		fi, err := h.g.GetForkInfo(h.fork)
		if err != nil {
			h.fail("GetForkInfo(%d): %v", h.fork, err)
		}
		if fi.HighestRevision > target {
			fwd := target + RevisionID(1+h.rnd.Int63n(int64(fi.HighestRevision-target)))
			h.logf("UndoSeek(%d) forward", fwd)
			if err := h.g.UndoSeek(fwd); err != nil {
				h.fail("redo UndoSeek(%d): %v", fwd, err)
			}
			want, ok := h.expectedStateAt(h.fork, fwd)
			if !ok {
				h.fail("harness: no snapshot for redo (fork %d, rev %d)", h.fork, fwd)
			}
			h.rev = fwd
			h.model = want.clone()
			h.check("after redo", false)
		}
	}
}

// liveOtherForks returns every known, non-deleted fork except the
// current one.
func (h *diffHarness) liveOtherForks() []ForkID {
	var out []ForkID
	for _, f := range h.known {
		if f != h.fork && !h.dead[f] {
			out = append(out, f)
		}
	}
	return out
}

func (h *diffHarness) opForkSeek() {
	candidates := h.liveOtherForks()
	if len(candidates) == 0 {
		return
	}
	target := candidates[h.rnd.Intn(len(candidates))]
	h.logf("ForkSeek(%d) from (fork %d, rev %d)", target, h.fork, h.rev)
	if err := h.g.ForkSeek(target); err != nil {
		if err == ErrRevisionNotFound {
			// Legitimate: everything reachable on the target lineage at
			// or below the common revision was pruned. Must be a clean
			// rejection - current state untouched.
			h.logf("  -> rejected: %v", err)
			h.check("after rejected forkseek", true)
			return
		}
		h.fail("ForkSeek(%d): %v", target, err)
	}
	f, r := h.g.CurrentFork(), h.g.CurrentRevision()
	h.logf("  -> landed (fork %d, rev %d), %d bytes", f, r, h.g.ByteCount().Value)
	if f != target {
		h.fail("ForkSeek(%d) landed on fork %d", target, f)
	}
	want, ok := h.expectedStateAt(f, r)
	if !ok {
		h.fail("harness: no snapshot for forkseek landing (fork %d, rev %d)", f, r)
	}
	h.fork, h.rev = f, r
	h.model = want.clone()
	h.check("after forkseek", false) // cursor restore policy observed, coherence hard
}

func (h *diffHarness) opPrune() {
	low := h.pruned[h.fork]
	if h.rev <= low {
		return
	}
	// keep in (low, rev]: always advances the watermark, never past the
	// current revision.
	keep := low + 1 + RevisionID(h.rnd.Int63n(int64(h.rev-low)))
	h.logf("Prune(%d) on (fork %d, rev %d)", keep, h.fork, h.rev)
	if err := h.g.Prune(keep); err != nil {
		h.fail("Prune(%d): %v", keep, err)
	}
	h.pruned[h.fork] = keep
	// Pruning history must not disturb any current observable state.
	h.check("after prune", true)

	// Negative probe: seeking below the watermark must fail and leave
	// the state untouched.
	if h.rnd.Intn(2) == 0 {
		bad := RevisionID(h.rnd.Int63n(int64(keep)))
		if err := h.g.UndoSeek(bad); err == nil {
			h.fail("UndoSeek(%d) below prune watermark %d succeeded", bad, keep)
		}
		h.check("after rejected undo", true)
	}
}

func (h *diffHarness) opDeleteFork() {
	candidates := h.liveOtherForks()
	if len(candidates) == 0 {
		return
	}
	target := candidates[h.rnd.Intn(len(candidates))]
	h.logf("DeleteFork(%d) while on (fork %d, rev %d)", target, h.fork, h.rev)
	if err := h.g.DeleteFork(target); err != nil {
		h.fail("DeleteFork(%d): %v", target, err)
	}
	h.dead[target] = true
	// Navigation to the deleted fork must now be rejected.
	if err := h.g.ForkSeek(target); err == nil {
		h.fail("ForkSeek(%d) to deleted fork succeeded", target)
	}
	// Deleting another fork must not disturb the current fork's state.
	h.check("after deletefork", true)
}

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// ---------- the test ----------

func TestDifferentialRandomOps(t *testing.T) {
	seeds := make([]int64, 64)
	for i := range seeds {
		seeds[i] = int64(i + 1)
	}
	_ = seeds
	for _, seed := range seeds {
		seed := seed
		t.Run(fmt.Sprintf("seed%d", seed), func(t *testing.T) {
			h := newDiffHarness(t, seed, "Hello, World!\nSecond line\n中文 αβγ\n")
			h.check("initial", true)
			for i := 0; i < 400; i++ {
				if len(h.model.data) > 8192 {
					h.opDelete()
					continue
				}
				switch h.rnd.Intn(17) {
				case 0, 1, 2:
					h.opInsert()
				case 3, 4:
					h.opDelete()
				case 5, 6:
					h.opDecorate()
				case 7, 8:
					h.opSeek()
				case 9:
					h.opUndoRedo()
				case 10:
					h.opOverwrite()
				case 11, 12:
					h.opMoveCopy()
				case 13:
					h.opTransaction()
				case 14:
					h.opPrune()
				case 15:
					h.opDeleteFork()
				case 16:
					h.opForkSeek()
				}
			}
			if len(h.soft) > 0 {
				max := len(h.soft)
				if max > 10 {
					max = 10
				}
				t.Logf("soft divergences (%d total, first %d):\n  %s",
					len(h.soft), max, joinLines(h.soft[:max]))
			}
		})
	}
}

// TestLineNumberingRulings pins the explicit line-semantics rulings:
// 0-based lines, the newline is the last character of its line, and
// the position after a trailing newline is line N+1, rune 0.
func TestLineNumberingRulings(t *testing.T) {
	lib, _ := Init(LibraryOptions{})
	g, err := lib.Open(FileOptions{DataString: "ab\ncd\n"})
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	cases := []struct {
		bytePos    int64
		line, rune int64
	}{
		{0, 0, 0}, // 'a' starts line 0
		{2, 0, 2}, // the newline is ON line 0
		{3, 1, 0}, // 'c' starts line 1
		{5, 1, 2}, // second newline is ON line 1
		{6, 2, 0}, // EOF after trailing newline: line N+1, rune 0
	}
	for _, c := range cases {
		line, r, err := g.ByteToLineRune(c.bytePos)
		if err != nil {
			t.Errorf("ByteToLineRune(%d): %v", c.bytePos, err)
			continue
		}
		if line != c.line || r != c.rune {
			t.Errorf("ByteToLineRune(%d) = %d:%d, want %d:%d", c.bytePos, line, r, c.line, c.rune)
		}
		back, err := g.LineRuneToByte(c.line, c.rune)
		if err != nil || back != c.bytePos {
			t.Errorf("LineRuneToByte(%d,%d) = %d,%v want %d", c.line, c.rune, back, err, c.bytePos)
		}
	}
}

// ---------- extended mutation family: overwrite / move / copy / tx ----------

// applyOverwrite: [pos, pos+length) replaced by piece. Marks inside
// consolidate to the START (plain OverwriteBytes = insertBefore
// false) and are REPORTED; marks after shift by the net change.
// Passive-cursor policy is observed (resynced), not asserted.
func (s *refState) applyOverwrite(pos, length int64, piece []byte) []string {
	end := pos + length
	if end > int64(len(s.data)) {
		end = int64(len(s.data))
	}
	net := int64(len(piece)) - (end - pos)
	s.data = append(s.data[:pos:pos], append(append([]byte(nil), piece...), s.data[end:]...)...)
	var inRange []string
	for k, d := range s.decs {
		switch {
		case d >= pos && d < end:
			inRange = append(inRange, k)
			s.decs[k] = pos // consolidate to start
		case d >= end:
			s.decs[k] = d + net
		}
	}
	return inRange
}

// applyMoveCopy models MoveBytes/CopyBytes: all addresses are in the
// ORIGINAL document; src and dst never overlap; the dst window
// [dstStart,dstEnd) is replaced by the src content. Marks in the dst
// window consolidate to the landing block's start (insertBefore false)
// or end (true) and are reported; on a MOVE, src-range marks travel
// with the content and marks elsewhere shift. On COPY the source is
// untouched and the copied content carries no marks.
func (s *refState) applyMoveCopy(srcStart, srcEnd, dstStart, dstEnd int64, isMove, insertBefore bool) []string {
	src := append([]byte(nil), s.data[srcStart:srcEnd]...)
	srcLen := srcEnd - srcStart

	// Build the final content and find where the src block lands.
	var out []byte
	insertAt := int64(-1)
	for p := int64(0); p <= int64(len(s.data)); {
		if p == dstStart {
			insertAt = int64(len(out))
			out = append(out, src...)
			p = dstEnd
			if dstStart == dstEnd && isMove && p == srcStart {
				// fallthrough to src skip below
			}
		}
		if isMove && p == srcStart && srcStart != srcEnd {
			p = srcEnd
			continue
		}
		if p >= int64(len(s.data)) {
			break
		}
		out = append(out, s.data[p])
		p++
	}
	s.data = out

	// posMap maps an original position OUTSIDE src/dst to its final one.
	// At exactly dstEnd: a non-empty window means the mark was after
	// replaced content and always slides past the landing block; a pure
	// insertion point (dstStart == dstEnd) is governed by insertBefore.
	posMap := func(d int64) int64 {
		f := d
		if isMove && d >= srcEnd {
			f -= srcLen
		}
		if d > dstEnd || (d == dstEnd && dstEnd > dstStart) {
			f -= dstEnd - dstStart
			f += srcLen
		} else if d == dstStart && dstStart == dstEnd && insertBefore {
			f += srcLen
		}
		return f
	}

	var displaced []string
	for k, d := range s.decs {
		switch {
		case isMove && d >= srcStart && d < srcEnd:
			s.decs[k] = insertAt + (d - srcStart) // travels with content
		case d >= dstStart && d < dstEnd:
			displaced = append(displaced, k)
			if insertBefore {
				s.decs[k] = insertAt + srcLen
			} else {
				s.decs[k] = insertAt
			}
		default:
			s.decs[k] = posMap(d)
		}
	}
	return displaced
}

// resyncCursors adopts garland's passive-cursor positions (policy for
// the extended ops is observed, not asserted) while hard-verifying
// that each cursor's rune/line coordinates are consistent with its
// byte position under the model's content.
func (h *diffHarness) resyncCursors(tag string) {
	h.t.Helper()
	for i, c := range h.curs {
		got := c.BytePos()
		if got < 0 || got > int64(len(h.model.data)) {
			h.fail("%s: cursor %d bytePos %d out of range 0..%d", tag, i, got, len(h.model.data))
		}
		h.model.cursors[i] = got
		if rp := c.RunePos(); rp != h.model.byteToRune(got) {
			h.fail("%s: cursor %d runePos = %d, inconsistent with bytePos %d (want %d)",
				tag, i, rp, got, h.model.byteToRune(got))
		}
		wl, wlr := h.model.lineOf(got)
		if line, lr := c.LinePos(); line != wl || lr != wlr {
			h.fail("%s: cursor %d linePos = %d:%d, inconsistent with bytePos %d (want %d:%d)",
				tag, i, line, lr, got, wl, wlr)
		}
	}
}

func (h *diffHarness) opOverwrite() {
	if len(h.model.data) == 0 {
		return
	}
	actor := h.rnd.Intn(len(h.curs))
	rc := h.model.runeCount()
	startRune := h.rnd.Int63n(rc)
	nRunes := 1 + h.rnd.Int63n(min64(rc-startRune, 8))
	start := h.model.byteOfRuneIndex(startRune)
	end := h.model.byteOfRuneIndex(startRune + nRunes)
	piece := []byte(h.randPiece())
	if err := h.curs[actor].SeekByte(start); err != nil {
		h.fail("overwrite: seek(%d): %v", start, err)
	}
	h.model.cursors[actor] = start
	h.logf("c%d.OverwriteBytes(%d, %q) at %d", actor, end-start, piece, start)
	got, res, err := h.curs[actor].OverwriteBytes(end-start, piece)
	if err != nil {
		h.fail("OverwriteBytes: %v", err)
	}
	inRange := h.model.applyOverwrite(start, end-start, piece)
	if len(got) != len(inRange) {
		var keys []string
		for _, d := range got {
			keys = append(keys, d.Key)
		}
		h.fail("overwrite reported %d marks (%v), model expected %d (%v)", len(got), keys, len(inRange), inRange)
	}
	h.noteMutation(res)
	h.resyncCursors("after overwrite")
	h.check("after overwrite", false)
}

// pickDisjointRanges returns src [s0,s1) and dst [d0,d1) that do not
// overlap (dst may be empty = pure insertion point). ok=false when the
// document is too small to bother.
func (h *diffHarness) pickDisjointRanges() (s0, s1, d0, d1 int64, ok bool) {
	rc := h.model.runeCount()
	if rc < 4 {
		return 0, 0, 0, 0, false
	}
	for try := 0; try < 8; try++ {
		sr := h.rnd.Int63n(rc - 1)
		sn := 1 + h.rnd.Int63n(min64(rc-sr, 6))
		s0, s1 = h.model.byteOfRuneIndex(sr), h.model.byteOfRuneIndex(sr+sn)
		dr := h.rnd.Int63n(rc + 1)
		dn := int64(0)
		if h.rnd.Intn(3) == 0 {
			dn = h.rnd.Int63n(min64(rc-dr+1, 4))
		}
		d0, d1 = h.model.byteOfRuneIndex(dr), h.model.byteOfRuneIndex(dr+dn)
		if s0 < d1 && d0 < s1 { // the library's overlap rule
			continue
		}
		if d0 == d1 && d0 > s0 && d0 < s1 { // insertion point inside src
			continue
		}
		return s0, s1, d0, d1, true
	}
	return 0, 0, 0, 0, false
}

func (h *diffHarness) opMoveCopy() {
	s0, s1, d0, d1, ok := h.pickDisjointRanges()
	if !ok {
		return
	}
	actor := h.rnd.Intn(len(h.curs))
	before := h.rnd.Intn(2) == 0
	isMove := h.rnd.Intn(2) == 0
	h.logf("  pre-op decs: %v", h.model.decs)
	if tree, err := h.g.GetDecorationsInByteRange(0, int64(len(h.model.data))+1); err == nil {
		var td []string
		for _, e := range tree {
			td = append(td, fmt.Sprintf("%s@%d", e.Key, e.Address.Byte))
		}
		h.logf("  pre-op tree decs: %v", td)
	}
	var displaced []RelativeDecoration
	var res ChangeResult
	var err error
	if isMove {
		h.logf("c%d.MoveBytes(%d,%d -> %d,%d, before=%v)", actor, s0, s1, d0, d1, before)
		var mr MoveResult
		mr, err = h.curs[actor].MoveBytes(s0, s1, d0, d1, before)
		displaced, res = mr.DisplacedDecorations, mr.ChangeResult
	} else {
		h.logf("c%d.CopyBytes(%d,%d -> %d,%d, before=%v)", actor, s0, s1, d0, d1, before)
		var cr CopyResult
		cr, err = h.curs[actor].CopyBytes(s0, s1, d0, d1, nil, before)
		displaced, res = cr.DisplacedDecorations, cr.ChangeResult
	}
	if err != nil {
		h.fail("move/copy: %v", err)
	}
	wantDisplaced := h.model.applyMoveCopy(s0, s1, d0, d1, isMove, before)
	if len(displaced) != len(wantDisplaced) {
		var keys []string
		for _, d := range displaced {
			keys = append(keys, d.Key)
		}
		h.fail("move/copy displaced %d marks (%v), model expected %d (%v)",
			len(displaced), keys, len(wantDisplaced), wantDisplaced)
	}
	h.noteMutation(res)
	h.resyncCursors("after move/copy")
	h.check("after move/copy", false)
}

func (h *diffHarness) opTransaction() {
	preFR := ForkRevision{h.fork, h.rev}
	preModel := h.model.clone()
	if err := h.g.TransactionStart("tx"); err != nil {
		h.fail("TransactionStart: %v", err)
	}
	h.logf("TransactionStart")
	n := 2 + h.rnd.Intn(3)
	for i := 0; i < n; i++ {
		switch h.rnd.Intn(3) {
		case 0:
			h.opInsert()
		case 1:
			h.opDelete()
		case 2:
			h.opDecorate()
		}
	}
	if h.rnd.Intn(4) == 0 {
		h.logf("TransactionRollback")
		if err := h.g.TransactionRollback(); err != nil {
			h.fail("TransactionRollback: %v", err)
		}
		delete(h.snaps, ForkRevision{h.fork, h.rev})
		h.fork, h.rev = preFR.Fork, preFR.Revision
		h.model = preModel
		h.check("after rollback", false)
	} else {
		res, err := h.g.TransactionCommit()
		if err != nil {
			h.fail("TransactionCommit: %v", err)
		}
		h.logf("TransactionCommit -> (fork %d, rev %d)", res.Fork, res.Revision)
		h.noteMutation(res)
		h.check("after commit", true)
	}
}
