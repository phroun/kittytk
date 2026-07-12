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
//   - Decorations deleted with a range reappear on undo.
//   - Cursors are part of undo history (phase 1: observed and logged,
//     not hard-asserted, until the exact restore rule is pinned down).
//   - Invalid UTF-8 only needs to count consistently (phase 1 sticks
//     to valid UTF-8 content).
//
// Where the interface docs and the code visibly disagree, the model
// follows OBSERVED code behavior so this harness hunts real logic
// errors rather than known documentation drift:
//   - insertBytesAt shifts every other cursor with bytePos >= pos,
//     regardless of the insertBefore flag (docs say the flag decides).

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

// insert applies an insertion. Decorations at exactly pos slide right
// only when insertBefore; other cursors at >= pos always slide
// (observed code behavior); the acting cursor lands after the insert.
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
		if s.cursors[i] >= pos {
			s.cursors[i] += n
		}
	}
	s.cursors[actor] = pos + n
}

// del applies a deletion of [pos, pos+n) and returns removed keys.
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
			delete(s.decs, k)
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
	}
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

	for k, want := range m.decs {
		addr, err := g.GetDecorationPosition(k)
		if err != nil {
			h.fail("%s: decoration %q missing (want byte %d): %v", tag, k, want, err)
		}
		if addr.Mode != ByteMode || addr.Byte != want {
			h.fail("%s: decoration %q at mode=%d byte=%d, want byte %d", tag, k, addr.Mode, addr.Byte, want)
		}
	}

	for i, c := range h.curs {
		if got := c.BytePos(); got != m.cursors[i] {
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
			h.fail("%s: cursor %d linePos = %d:%d, want %d:%d", tag, i, line, lr, wantLine, wantLR)
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
		h.fail("delete returned %d decorations, model removed %d (%v)", len(got), len(removed), removed)
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
	if h.rev == 0 {
		return
	}
	target := RevisionID(h.rnd.Int63n(int64(h.rev)))
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

func min64(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// ---------- the test ----------

func TestDifferentialRandomOps(t *testing.T) {
	seeds := []int64{1, 2, 3, 4}
	for _, seed := range seeds {
		seed := seed
		t.Run(fmt.Sprintf("seed%d", seed), func(t *testing.T) {
			h := newDiffHarness(t, seed, "Hello, World!\nSecond line\n中文 αβγ\n")
			h.check("initial", true)
			for i := 0; i < 300; i++ {
				if len(h.model.data) > 8192 {
					h.opDelete()
					continue
				}
				switch h.rnd.Intn(10) {
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
