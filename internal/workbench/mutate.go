package workbench

import "github.com/jasper0507/skills-manage/internal/infra/index"

// withMutation snapshots the in-memory index document (and session clipboard /
// selection), runs fn, and on any error restores the snapshots. On success,
// normalizes membership/placement (strip dual-write LocBox, repair ghosts) then
// persists the document once. Covers multi-step place/move, recycle, and box
// structure ops so failures never leave a half-applied desk and disk never
// re-grows dual-write shape (E3.1).
func (w *Workbench) withMutation(fn func() error) error {
	if err := w.requireOpen(); err != nil {
		return err
	}
	snap := index.CloneDocument(w.doc)
	clipSnap := cloneClipboard(w.clipboard)
	selSnap := append([]string(nil), w.selectedIDs...)
	multiSnap := w.multiSelect
	if err := fn(); err != nil {
		w.doc = snap
		w.clipboard = clipSnap
		w.selectedIDs = selSnap
		w.multiSelect = multiSnap
		return err
	}
	// Mutations may still dual-write LocBox in memory (E3.2 cleans write path);
	// normalize before the single persist so the index stays membership-truth.
	w.rehomeFromMembership()
	if err := w.persist(); err != nil {
		w.doc = snap
		w.clipboard = clipSnap
		w.selectedIDs = selSnap
		w.multiSelect = multiSnap
		return err
	}
	return nil
}

func cloneClipboard(c *Clipboard) *Clipboard {
	if c == nil {
		return nil
	}
	return &Clipboard{
		Mode:           c.Mode,
		PlaceholderIDs: append([]string(nil), c.PlaceholderIDs...),
	}
}
