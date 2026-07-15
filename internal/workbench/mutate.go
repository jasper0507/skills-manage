package workbench

import "github.com/jasper0507/skills-manage/internal/infra/index"

// withMutation snapshots the in-memory index document (and session clipboard /
// selection), runs fn, and on any error restores the snapshots. On success,
// normalizes membership/placement (safety-net repair of ghosts/duplicates) then
// persists the document once. Write paths update membership and desktop/recycle
// placement only (E3.2); rehome remains a safety net, not the primary strip of
// dual-write LocBox.
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
	// Safety net: membership truth + free-cell repair before the single persist.
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
