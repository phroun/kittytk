package garland

// save.go - the in-place save engine.
//
// DESIGN CONSTRAINT: saving must never require a second copy of the
// document on disk. A temp-file-plus-rename save doubles peak disk
// usage, which defeats the point of a library built to edit a 1.2MB
// file on a 1.44MB floppy. The file is therefore rewritten IN PLACE:
//
//   - The file is opened read-write and is NEVER truncated up front;
//     it only shrinks (if at all) as the very last step.
//   - Warm storage (unmodified spans whose only copy of the bytes IS
//     the original file) keeps working: spans that did not move are
//     not even rewritten, and spans that moved are re-homed to their
//     new offsets afterwards, staying warm.
//   - Content is written in an order that never overwrites warm bytes
//     before they have been read (see below), so no migration is
//     needed for the CURRENT revision at all.
//   - Warm spans that only undo HISTORY references are surgically
//     migrated to cold storage first (SaveOptions.PreserveHistory),
//     because the rewrite may overwrite their backing bytes.
//
// WRITE ORDERING: the new file is a sequence of spans in tree order.
// Each span has a new offset (prefix sum of the new layout) and an old
// source range in the original file (empty for freshly typed content).
// Because the rope preserves relative order, both the old ranges and
// the new ranges are monotonically increasing across spans. Given
// that, a two-phase schedule is provably clobber-free:
//
//   Phase B first, BACK TO FRONT: warm spans that move RIGHT
//     (newOff > oldOff, i.e. net insertions before them). Processing
//     descending by new offset, each span's source is read while still
//     intact: later spans' writes land at or above this span's source
//     end, and phase A has not run yet.
//   Phase A second, FRONT TO BACK: everything else - fresh content,
//     cold/memory-sourced spans, and warm spans moving left or not at
//     all (unmoved ones are simply SKIPPED - their bytes are already
//     correct in the file). Ascending order cannot clobber a remaining
//     warm source: for any still-unwritten left-mover Z after span X,
//     newX+lenX <= newZ <= oldZ.
//
// Move/Copy operations can reorder leaves so that old offsets are no
// longer monotone in tree order; the offending spans are rescued into
// memory before any write (rare, bounded by the moved region).

// SaveOptions configures Save behavior.
type SaveOptions struct {
	// PreserveHistory protects warm-backed data that only OLDER
	// revisions reference: before the rewrite can overwrite its
	// backing bytes it is migrated to cold storage (or held in memory
	// when no cold backend is configured), so undo history survives
	// the save intact.
	//
	// When false, such history keeps pointing at its old file offsets;
	// regions the rewrite left untouched remain valid, and regions
	// that were overwritten fail their hash check on access and become
	// placeholders - undo history may be amputated, but never silently
	// corrupted.
	PreserveHistory bool
}

// saveSpan describes one leaf of the current revision in the new file
// layout.
type saveSpan struct {
	node   *Node
	snap   *NodeSnapshot
	key    ForkRevision
	newOff int64
	length int64
	oldOff int64 // source position in the OLD file layout
	oldLen int64 // length of the old-file source (0 = fresh content)
	warm   bool  // source bytes must be READ from the old file
	skip   bool  // bytes already correct at this offset - do not write
}

// SaveWith overwrites the original file in place with the current
// content. See the file header for the full design.
func (g *Garland) SaveWith(opts SaveOptions) error {
	if g.sourcePath == "" {
		return ErrNoDataSource
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	fs := g.sourceFS
	if fs == nil {
		fs = g.lib.defaultFS
	}
	return g.saveInPlace(fs, opts)
}

func (g *Garland) saveInPlace(fs FileSystemInterface, opts SaveOptions) error {
	// A read handle on the old file is needed for warm sources and
	// history migration. Reuse the warm-storage handle when present.
	readHandle := g.sourceHandle
	ownReadHandle := false
	if readHandle == nil {
		h, err := fs.Open(g.sourcePath, OpenModeRead)
		if err == nil {
			readHandle = h
			ownReadHandle = true
		}
		// A missing file is fine when nothing needs warm reads; the
		// span walk below will fail loudly if it does.
	}
	if ownReadHandle {
		defer func() {
			if readHandle != nil {
				fs.Close(readHandle)
			}
		}()
	}

	// ---- Collect the span layout ----
	spans := make([]saveSpan, 0, 64)
	var walkErr error
	var newCursor, oldCursor int64
	var collect func(nodeID NodeID)
	collect = func(nodeID NodeID) {
		if walkErr != nil {
			return
		}
		node := g.nodeRegistry[nodeID]
		if node == nil {
			return
		}
		snap, key := node.snapshotAtWithKey(g.currentFork, g.currentRevision)
		if snap == nil {
			return
		}
		if !snap.isLeaf {
			collect(snap.leftID)
			collect(snap.rightID)
			return
		}
		if snap.byteCount == 0 {
			return
		}

		sp := saveSpan{
			node:   node,
			snap:   snap,
			key:    key,
			newOff: newCursor,
			length: snap.byteCount,
		}
		switch {
		case snap.storageState == StoragePlaceholder:
			walkErr = ErrColdStorageFailure
			return
		case snap.originalFileOffset >= 0 && snap.originalFileOffset >= oldCursor:
			// Backed by (or identical to) old file content, in order.
			sp.oldOff = snap.originalFileOffset
			sp.oldLen = snap.byteCount
			sp.warm = snap.storageState == StorageWarm
			sp.skip = sp.oldOff == sp.newOff
			oldCursor = sp.oldOff + sp.oldLen
		case snap.originalFileOffset >= 0 && snap.storageState == StorageWarm:
			// Out of order (a Move/Copy rearranged leaves): the
			// two-phase schedule cannot protect this source, so rescue
			// it into memory before any write happens.
			if err := g.readFromWarmStorageWithTrust(node.id, snap); err != nil {
				walkErr = err
				return
			}
			sp.oldOff = oldCursor
		default:
			// Fresh content (or out-of-order but memory/cold-sourced):
			// no old-file source to protect.
			sp.oldOff = oldCursor
		}
		newCursor += sp.length
		spans = append(spans, sp)
	}
	if g.root != nil {
		collect(g.root.id)
	}
	if walkErr != nil {
		return walkErr
	}
	newTotal := newCursor

	// The lowest offset the rewrite will disturb: everything before it
	// is untouched, so warm history pointing there stays valid.
	protectFrom := newTotal
	for _, sp := range spans {
		if !sp.skip && sp.newOff < protectFrom {
			protectFrom = sp.newOff
		}
	}

	// ---- Protect history's warm spans (surgical) ----
	currentSnaps := make(map[*NodeSnapshot]bool, len(spans))
	for _, sp := range spans {
		currentSnaps[sp.snap] = true
	}
	var oldSize int64 = -1
	if readHandle != nil {
		if sz, err := fs.FileSize(readHandle); err == nil {
			oldSize = sz
		}
	}
	if newTotal < oldSize {
		// Truncation destroys the tail too.
		if newTotal < protectFrom {
			protectFrom = newTotal
		}
	}
	if opts.PreserveHistory {
		for _, node := range g.nodeRegistry {
			if node == nil {
				continue
			}
			for key, snap := range node.history {
				if snap == nil || !snap.isLeaf || snap.storageState != StorageWarm {
					continue
				}
				if currentSnaps[snap] {
					continue // protected by the write ordering
				}
				if snap.originalFileOffset+snap.byteCount <= protectFrom {
					continue // rewrite never touches its bytes
				}
				// Read it back while the old file is intact...
				if err := g.readFromWarmStorageWithTrust(node.id, snap); err != nil {
					return err
				}
				// ...and push it to cold if a backend exists (else it
				// simply stays in memory).
				if g.lib.coldStorageBackend != nil && g.loadingStyle != MemoryOnly {
					if err := g.chillSnapshot(node.id, key, snap); err != nil {
						return err
					}
				}
			}
		}
	}

	// ---- Open the write handle: read-write, NO truncation ----
	writeHandle, err := fs.Open(g.sourcePath, OpenModeReadWrite)
	if err != nil {
		return err
	}
	defer fs.Close(writeHandle)

	readWarm := func(sp saveSpan) ([]byte, error) {
		if readHandle == nil {
			return nil, ErrWarmStorageMismatch
		}
		if err := fs.SeekByte(readHandle, sp.oldOff); err != nil {
			return nil, err
		}
		return fs.ReadBytes(readHandle, int(sp.oldLen))
	}
	writeSpan := func(sp *saveSpan) error {
		var data []byte
		switch {
		case sp.warm:
			d, err := readWarm(*sp)
			if err != nil {
				return err
			}
			if int64(len(d)) != sp.length {
				return ErrWarmStorageMismatch
			}
			data = d
		case sp.snap.storageState == StorageCold:
			if err := g.thawSnapshot(sp.node.id, sp.key, sp.snap); err != nil {
				return err
			}
			data = sp.snap.data
		default:
			data = sp.snap.data
		}
		if err := fs.SeekByte(writeHandle, sp.newOff); err != nil {
			return err
		}
		return fs.WriteBytes(writeHandle, data)
	}

	// ---- Phase B: warm right-movers, back to front ----
	for i := len(spans) - 1; i >= 0; i-- {
		sp := &spans[i]
		if sp.skip || !sp.warm || sp.newOff <= sp.oldOff {
			continue
		}
		if err := writeSpan(sp); err != nil {
			return err
		}
	}

	// ---- Phase A: everything else, front to back ----
	for i := range spans {
		sp := &spans[i]
		if sp.skip || (sp.warm && sp.newOff > sp.oldOff) {
			continue
		}
		if err := writeSpan(sp); err != nil {
			return err
		}
	}

	// ---- Shrink at the very end, and only then ----
	if oldSize >= 0 && newTotal < oldSize {
		if err := fs.Truncate(writeHandle, newTotal); err != nil {
			// A stale tail is silent corruption - refuse rather than
			// pretend the save succeeded.
			return err
		}
	}

	// ---- Re-home: the file now matches the buffer at NEW offsets ----
	for i := range spans {
		sp := &spans[i]
		sp.snap.originalFileOffset = sp.newOff
		if sp.warm {
			// Still warm, now against the rewritten file.
			g.updateWarmVerification(sp.node.id)
		}
	}

	// Re-baseline change detection so our own write is not reported as
	// an external modification.
	if g.sourceState != nil {
		g.sourceState.status = SourceStatusNormal
		_ = g.captureSourceInfo()
	}

	return nil
}
