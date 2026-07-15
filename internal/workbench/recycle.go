package workbench

import (
	"fmt"
	"path/filepath"

	"github.com/jasper0507/skills-manage/internal/infra/index"
)

// RecycleIconAction refuses copy/cut/delete of the 回收站 system icon.
// The recycle affordance is movable and may enter a box, but is never copyable,
// cuttable, or deletable as an icon.
func (w *Workbench) RecycleIconAction(action string) error {
	switch action {
	case "copy", "cut", "delete":
		return fmt.Errorf("recycle system icon cannot be copied, cut, or deleted")
	default:
		return fmt.Errorf("unknown recycle action %q", action)
	}
}

// MoveRecycleToDesktop places the 回收站 system icon on a desktop grid cell.
// If the cell is occupied by a skill icon, the recycle icon takes the nearest free cell
// (it never auto-boxes with a skill).
func (w *Workbench) MoveRecycleToDesktop(row, col int) error {
	if err := w.requireOpen(); err != nil {
		return err
	}
	if row < 1 || col < 1 {
		return fmt.Errorf("invalid grid cell (%d,%d)", row, col)
	}
	// Park recycle so it does not block free-cell search for its own move.
	w.doc.RecycleIcon = index.Location{Kind: LocDesktop, Row: -1, Col: -1}
	occupied := w.occupiedDesktopCells()
	free := cell{row, col}
	if occupied[free] {
		free = nextFreeCell(occupied, col, row)
	}
	w.doc.RecycleIcon = index.Location{Kind: LocDesktop, Row: free.row, Col: free.col}
	return w.persist()
}

// MoveRecycleToBox puts the 回收站 system icon into a box (simple or current/compartment).
func (w *Workbench) MoveRecycleToBox(boxID, compartmentID string) error {
	if err := w.requireOpen(); err != nil {
		return err
	}
	bIdx, ok := w.boxIndex(boxID)
	if !ok {
		return fmt.Errorf("unknown box %q", boxID)
	}
	box := &w.doc.Boxes[bIdx]
	loc := index.Location{Kind: LocBox, BoxID: boxID}
	if box.Kind == BoxSimple {
		loc.CompartmentID = ""
	} else {
		cid := compartmentID
		if cid == "" {
			cid = box.ActiveCompartmentID
		}
		found := false
		for _, c := range box.Compartments {
			if c.ID == cid {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("unknown compartment %q in box %q", cid, boxID)
		}
		loc.CompartmentID = cid
	}
	w.doc.RecycleIcon = loc
	return w.persist()
}

// TrashPlan describes the effect of ConfirmTrash for the given placeholder ids (R2).
// EnterBinIDs will move into the icon recycle bin; SkippedIDs are last-live placeholders
// for their Skill 身份 (refused so the workbench keeps at least one live entry).
type TrashPlan struct {
	EnterBinIDs []string `json:"enterBinIds"`
	SkippedIDs  []string `json:"skippedIds"`
}

// RecycleView is one in-bin 占位 (icon-level soft trash; not a package quarantine).
type RecycleView struct {
	ID       string `json:"id"` // placeholder id (also restore entry id)
	Identity string `json:"identity"`
	Name     string `json:"name"`
}

// RecycleBin returns placeholders currently in the icon-level recycle bin.
func (w *Workbench) RecycleBin() []RecycleView {
	nameByID := make(map[string]string, len(w.doc.Skills))
	for _, s := range w.doc.Skills {
		nameByID[s.Identity] = s.Name
	}
	out := make([]RecycleView, 0)
	for _, p := range w.doc.Placeholders {
		if p.Location.Kind != LocRecycle {
			continue
		}
		name := nameByID[p.Identity]
		if name == "" {
			name = w.skillName(p.Identity)
		}
		out = append(out, RecycleView{
			ID:       p.ID,
			Identity: p.Identity,
			Name:     name,
		})
	}
	return out
}

// PlanTrash returns what ConfirmTrash would do without mutating state or disk.
func (w *Workbench) PlanTrash(placeholderIDs []string) (TrashPlan, error) {
	if err := w.requireOpen(); err != nil {
		return TrashPlan{}, err
	}
	return w.planTrash(placeholderIDs)
}

// ConfirmTrash moves eligible placeholders into the icon recycle bin (R2).
// Non-last live placeholders for an identity enter the bin; if selected placeholders
// would leave an identity with zero live 占位, those placeholders are skipped.
// Batch is partial-success: other identities still enter. If nothing can enter,
// returns an error and leaves state unchanged.
//
// Skill packages are never isolated, renamed, or deleted.
func (w *Workbench) ConfirmTrash(placeholderIDs []string) error {
	if err := w.requireOpen(); err != nil {
		return err
	}
	plan, err := w.planTrash(placeholderIDs)
	if err != nil {
		return err
	}
	if len(plan.EnterBinIDs) == 0 {
		if len(plan.SkippedIDs) > 0 {
			return fmt.Errorf("refuse enter-bin: last live placeholder for identity")
		}
		return fmt.Errorf("no valid placeholders to trash")
	}

	for _, id := range plan.EnterBinIDs {
		w.removePlaceholderFromContainers(id)
		if idx, ok := w.placeholderIndex(id); ok {
			w.doc.Placeholders[idx].Location = index.Location{Kind: LocRecycle}
		}
	}

	w.pruneClipboardAfterTrash()
	w.selectedIDs = nil
	return w.persist()
}

func (w *Workbench) planTrash(placeholderIDs []string) (TrashPlan, error) {
	// Always non-nil slices so JSON encodes [] not null (stable HTTP contract).
	plan := TrashPlan{
		EnterBinIDs: []string{},
		SkippedIDs:  []string{},
	}
	// Dedupe requested ids; ignore unknown and already-recycled.
	requested := make([]string, 0, len(placeholderIDs))
	seenReq := map[string]bool{}
	for _, id := range placeholderIDs {
		if seenReq[id] {
			continue
		}
		idx, ok := w.placeholderIndex(id)
		if !ok {
			continue
		}
		if w.doc.Placeholders[idx].Location.Kind == LocRecycle {
			continue
		}
		seenReq[id] = true
		requested = append(requested, id)
	}
	if len(requested) == 0 {
		return TrashPlan{}, fmt.Errorf("no valid placeholders to trash")
	}

	// Group requested by identity.
	type group struct {
		identity string
		ids      []string
	}
	byIdentity := map[string]*group{}
	var order []string
	for _, id := range requested {
		idx, _ := w.placeholderIndex(id)
		ident := w.doc.Placeholders[idx].Identity
		g, ok := byIdentity[ident]
		if !ok {
			g = &group{identity: ident}
			byIdentity[ident] = g
			order = append(order, ident)
		}
		g.ids = append(g.ids, id)
	}

	for _, ident := range order {
		g := byIdentity[ident]
		liveIDs := w.livePlaceholderIDs(ident)
		selected := make(map[string]bool, len(g.ids))
		for _, id := range g.ids {
			selected[id] = true
		}
		remaining := 0
		for _, id := range liveIDs {
			if !selected[id] {
				remaining++
			}
		}
		if remaining > 0 {
			// Safe: at least one live 占位 stays for this identity.
			plan.EnterBinIDs = append(plan.EnterBinIDs, g.ids...)
			continue
		}
		// Would zero live placeholders for this identity → skip (R2 last-live guard).
		plan.SkippedIDs = append(plan.SkippedIDs, g.ids...)
	}
	return plan, nil
}

func (w *Workbench) livePlaceholderIDs(identity string) []string {
	var ids []string
	for _, p := range w.doc.Placeholders {
		if p.Identity == identity && p.Location.Kind != LocRecycle {
			ids = append(ids, p.ID)
		}
	}
	return ids
}

func (w *Workbench) skillName(identity string) string {
	for _, s := range w.doc.Skills {
		if s.Identity == identity {
			return s.Name
		}
	}
	return filepath.Base(identity)
}

func (w *Workbench) removePlaceholderEntirely(id string) {
	w.removePlaceholderFromContainers(id)
	out := make([]index.PlaceholderRecord, 0, len(w.doc.Placeholders))
	for _, p := range w.doc.Placeholders {
		if p.ID != id {
			out = append(out, p)
		}
	}
	w.doc.Placeholders = out
}

func (w *Workbench) pruneClipboardAfterTrash() {
	if w.clipboard == nil {
		return
	}
	kept := make([]string, 0, len(w.clipboard.PlaceholderIDs))
	for _, id := range w.clipboard.PlaceholderIDs {
		idx, ok := w.placeholderIndex(id)
		if !ok {
			continue
		}
		if w.doc.Placeholders[idx].Location.Kind == LocRecycle {
			continue
		}
		kept = append(kept, id)
	}
	if len(kept) == 0 {
		w.clipboard = nil
		return
	}
	w.clipboard.PlaceholderIDs = kept
}

// Restore returns an in-bin 占位 to a free desktop grid cell.
// entryID is the placeholder id (see RecycleView.ID). No package rename.
func (w *Workbench) Restore(entryID string) error {
	if err := w.requireOpen(); err != nil {
		return err
	}
	idx, ok := w.placeholderIndex(entryID)
	if !ok {
		return fmt.Errorf("recycle entry %q not found", entryID)
	}
	if w.doc.Placeholders[idx].Location.Kind != LocRecycle {
		return fmt.Errorf("recycle entry %q not found", entryID)
	}

	occupied := w.occupiedDesktopCells()
	free := nextFreeCellInViewport(occupied)
	w.doc.Placeholders[idx].Location = index.Location{
		Kind: LocDesktop, Row: free.row, Col: free.col,
	}
	return w.persist()
}

// EmptyRecycleBin discards all in-bin placeholder records only.
// Skill packages on disk are never deleted.
func (w *Workbench) EmptyRecycleBin() error {
	if err := w.requireOpen(); err != nil {
		return err
	}
	var toDrop []string
	for _, p := range w.doc.Placeholders {
		if p.Location.Kind == LocRecycle {
			toDrop = append(toDrop, p.ID)
		}
	}
	for _, id := range toDrop {
		w.removePlaceholderEntirely(id)
	}
	// Drop any legacy body-delete recycle entries from the index document.
	w.doc.RecycleBin = nil
	return w.persist()
}
