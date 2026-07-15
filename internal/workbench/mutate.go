package workbench

import "github.com/jasper0507/skills-manage/internal/infra/index"

// withMutation snapshots the in-memory index document, runs fn, and on any error
// restores the snapshot. On success, persists once. Covers multi-step place/move,
// recycle, and box structure ops so failures never leave a half-applied desk.
func (w *Workbench) withMutation(fn func() error) error {
	if err := w.requireOpen(); err != nil {
		return err
	}
	snap := index.CloneDocument(w.doc)
	if err := fn(); err != nil {
		w.doc = snap
		return err
	}
	if err := w.persist(); err != nil {
		w.doc = snap
		return err
	}
	return nil
}
