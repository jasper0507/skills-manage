package workbench

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/jasper0507/skills-manage/internal/infra/index"
	"github.com/jasper0507/skills-manage/internal/infra/quarantine"
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

// BodyTrashPlan is one skill identity that will enter body quarantine on ConfirmTrash.
// Path is the realpath shown in the confirmation UI.
type BodyTrashPlan struct {
	Identity       string   `json:"identity"`
	Path           string   `json:"path"`
	Name           string   `json:"name"`
	PlaceholderIDs []string `json:"placeholderIds"` // all live placeholders for this identity
}

// TrashPlan describes the effect of ConfirmTrash for the given placeholder ids.
type TrashPlan struct {
	IconOnlyIDs []string        `json:"iconOnlyIds"` // non-last placeholders: remove icons only
	BodyItems   []BodyTrashPlan `json:"bodyItems"`   // last placeholder(s) for an identity: quarantine
}

// RecycleView is one entry in the product recycle bin (external view).
type RecycleView struct {
	ID             string    `json:"id"`
	Identity       string    `json:"identity"`
	Name           string    `json:"name"`
	OriginalPath   string    `json:"originalPath"`
	QuarantinePath string    `json:"quarantinePath"`
	DeletedAt      time.Time `json:"deletedAt"`
	PurgeAfter     time.Time `json:"purgeAfter"`
	PlaceholderIDs []string  `json:"placeholderIds"`
	State          string    `json:"state"`
}

// RecycleBin returns quarantined skill entries (restorable until purge).
func (w *Workbench) RecycleBin() []RecycleView {
	out := make([]RecycleView, 0, len(w.doc.RecycleBin))
	for _, e := range w.doc.RecycleBin {
		if e.State == index.RecycleStatePurged {
			continue
		}
		out = append(out, RecycleView{
			ID:             e.ID,
			Identity:       e.Identity,
			Name:           e.Name,
			OriginalPath:   e.OriginalPath,
			QuarantinePath: e.QuarantinePath,
			DeletedAt:      e.DeletedAt,
			PurgeAfter:     e.PurgeAfter,
			PlaceholderIDs: append([]string(nil), e.PlaceholderIDs...),
			State:          e.State,
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

// ConfirmTrash applies trash for the given placeholders.
// Non-last placeholders are removed as icons only; last placeholders for an
// identity quarantine the skill package via same-FS rename and move all of
// that identity's placeholders into the recycle lifecycle.
//
// After each successful body quarantine rename, the index is persisted so a
// crash leaves a recoverable RecycleEntry rather than an orphan trash tree.
func (w *Workbench) ConfirmTrash(placeholderIDs []string) error {
	if err := w.requireOpen(); err != nil {
		return err
	}
	plan, err := w.planTrash(placeholderIDs)
	if err != nil {
		return err
	}

	// Icon-only first (does not touch disk packages).
	for _, id := range plan.IconOnlyIDs {
		w.removePlaceholderEntirely(id)
	}
	if len(plan.IconOnlyIDs) > 0 {
		if err := w.persist(); err != nil {
			return err
		}
	}

	// Body quarantine: one entry per identity; persist after each rename.
	now := w.clock()
	for _, body := range plan.BodyItems {
		if err := w.quarantineIdentity(body, now); err != nil {
			return err
		}
		if err := w.persist(); err != nil {
			return err
		}
	}

	w.pruneClipboardAfterTrash()
	w.selectedIDs = nil
	return w.persist()
}

func (w *Workbench) planTrash(placeholderIDs []string) (TrashPlan, error) {
	// Always non-nil slices so JSON encodes [] not null (stable HTTP contract).
	plan := TrashPlan{
		IconOnlyIDs: []string{},
		BodyItems:   []BodyTrashPlan{},
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
			// Non-last: only the selected icons go away.
			plan.IconOnlyIDs = append(plan.IconOnlyIDs, g.ids...)
			continue
		}
		// Last remaining placeholder(s) for this identity → body quarantine.
		// All live placeholders for the identity enter recycle once.
		name := w.skillName(ident)
		plan.BodyItems = append(plan.BodyItems, BodyTrashPlan{
			Identity:       ident,
			Path:           ident,
			Name:           name,
			PlaceholderIDs: append([]string(nil), liveIDs...),
		})
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

func (w *Workbench) quarantineIdentity(body BodyTrashPlan, now time.Time) error {
	// Refuse if already quarantined for this identity.
	for _, e := range w.doc.RecycleBin {
		if e.Identity == body.Identity && e.State != index.RecycleStatePurged {
			return fmt.Errorf("skill already in recycle lifecycle: %s", body.Identity)
		}
	}

	scanRoot, err := quarantine.FindScanRoot(body.Identity, w.scanRoots)
	if err != nil {
		return fmt.Errorf("body delete refused: %w", err)
	}

	entryID := w.newRecycleEntryID()
	qPath, err := w.q.Isolate(body.Identity, scanRoot, entryID)
	if err != nil {
		return fmt.Errorf("quarantine: %w", err)
	}

	// Move all live placeholders for this identity into recycle location.
	phIDs := append([]string(nil), body.PlaceholderIDs...)
	for _, id := range phIDs {
		w.removePlaceholderFromContainers(id)
		if idx, ok := w.placeholderIndex(id); ok {
			w.doc.Placeholders[idx].Location = index.Location{Kind: LocRecycle}
		}
	}

	// Drop from live skill cache; inventory rescans will not see quarantined tree.
	w.removeSkillRecord(body.Identity)

	w.doc.RecycleBin = append(w.doc.RecycleBin, index.RecycleEntry{
		ID:             entryID,
		Identity:       body.Identity,
		Name:           body.Name,
		OriginalPath:   body.Identity,
		QuarantinePath: qPath,
		DeletedAt:      now.UTC(),
		PurgeAfter:     now.UTC().Add(RecycleRetention),
		PlaceholderIDs: phIDs,
		State:          index.RecycleStateQuarantined,
	})
	return nil
}

func (w *Workbench) removeSkillRecord(identity string) {
	out := make([]index.SkillRecord, 0, len(w.doc.Skills))
	for _, s := range w.doc.Skills {
		if s.Identity != identity {
			out = append(out, s)
		}
	}
	w.doc.Skills = out
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

func (w *Workbench) newRecycleEntryID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("trash_%d", len(w.doc.RecycleBin)+1)
	}
	return "trash_" + hex.EncodeToString(b[:])
}

// Restore renames a quarantined skill back to its original path when free and
// places one placeholder on a free desktop cell.
func (w *Workbench) Restore(entryID string) error {
	if err := w.requireOpen(); err != nil {
		return err
	}
	eIdx := -1
	for i, e := range w.doc.RecycleBin {
		if e.ID == entryID && e.State != index.RecycleStatePurged {
			eIdx = i
			break
		}
	}
	if eIdx < 0 {
		return fmt.Errorf("recycle entry %q not found", entryID)
	}
	e := w.doc.RecycleBin[eIdx]

	if err := w.q.Restore(e.QuarantinePath, e.OriginalPath); err != nil {
		if quarantine.IsPathOccupied(err) {
			return fmt.Errorf("restore refused: original path occupied (will not overwrite): %s", e.OriginalPath)
		}
		return fmt.Errorf("restore: %w", err)
	}

	// Prefer restoring the first recorded placeholder; drop extras (icon copies).
	var keepID string
	if len(e.PlaceholderIDs) > 0 {
		keepID = e.PlaceholderIDs[0]
	}
	for _, id := range e.PlaceholderIDs {
		if id == keepID {
			continue
		}
		w.removePlaceholderEntirely(id)
	}

	occupied := w.occupiedDesktopCells()
	free := nextFreeCellInViewport(occupied)
	if keepID != "" {
		if idx, ok := w.placeholderIndex(keepID); ok {
			w.doc.Placeholders[idx].Location = index.Location{
				Kind: LocDesktop, Row: free.row, Col: free.col,
			}
		} else {
			// Placeholder missing; recreate.
			keepID = w.newPlaceholderID()
			w.doc.Placeholders = append(w.doc.Placeholders, index.PlaceholderRecord{
				ID:       keepID,
				Identity: e.OriginalPath,
				Location: index.Location{Kind: LocDesktop, Row: free.row, Col: free.col},
			})
		}
	} else {
		keepID = w.newPlaceholderID()
		w.doc.Placeholders = append(w.doc.Placeholders, index.PlaceholderRecord{
			ID:       keepID,
			Identity: e.OriginalPath,
			Location: index.Location{Kind: LocDesktop, Row: free.row, Col: free.col},
		})
	}

	// Refresh skill name from restored package via rescan of known name, or basename.
	name := e.Name
	if name == "" {
		name = filepath.Base(e.OriginalPath)
	}
	w.doc.Skills = append(w.doc.Skills, index.SkillRecord{
		Identity: e.OriginalPath,
		Name:     name,
	})

	// Remove recycle entry.
	w.doc.RecycleBin = append(w.doc.RecycleBin[:eIdx], w.doc.RecycleBin[eIdx+1:]...)
	return w.persist()
}

// EmptyRecycleBin permanently deletes all quarantined packages (allowlisted trash paths only).
func (w *Workbench) EmptyRecycleBin() error {
	if err := w.requireOpen(); err != nil {
		return err
	}
	if len(w.doc.RecycleBin) == 0 {
		return nil
	}
	// Copy ids so we can mutate the slice.
	ids := make([]string, 0, len(w.doc.RecycleBin))
	for _, e := range w.doc.RecycleBin {
		if e.State != index.RecycleStatePurged {
			ids = append(ids, e.ID)
		}
	}
	for _, id := range ids {
		if err := w.purgeEntry(id, true); err != nil {
			_ = w.persist()
			return err
		}
	}
	return w.persist()
}

// PurgeDue permanently deletes quarantine entries whose PurgeAfter has passed.
func (w *Workbench) PurgeDue() error {
	if err := w.requireOpen(); err != nil {
		return err
	}
	if err := w.purgeDueNoPersist(); err != nil {
		_ = w.persist()
		return err
	}
	return w.persist()
}

func (w *Workbench) purgeDueNoPersist() error {
	now := w.clock().UTC()
	var due []string
	for _, e := range w.doc.RecycleBin {
		if e.State == index.RecycleStatePurged {
			continue
		}
		if !e.PurgeAfter.After(now) {
			due = append(due, e.ID)
		}
	}
	for _, id := range due {
		if err := w.purgeEntry(id, true); err != nil {
			return err
		}
	}
	return nil
}

// purgeEntry removes quarantine bytes and index bookkeeping for one entry.
// force ignores retention (used by Empty and PurgeDue which already filtered).
func (w *Workbench) purgeEntry(entryID string, force bool) error {
	eIdx := -1
	for i, e := range w.doc.RecycleBin {
		if e.ID == entryID {
			eIdx = i
			break
		}
	}
	if eIdx < 0 {
		return fmt.Errorf("recycle entry %q not found", entryID)
	}
	e := &w.doc.RecycleBin[eIdx]
	if e.State == index.RecycleStatePurged {
		// Already gone; drop record.
		w.doc.RecycleBin = append(w.doc.RecycleBin[:eIdx], w.doc.RecycleBin[eIdx+1:]...)
		return nil
	}
	if !force && e.PurgeAfter.After(w.clock().UTC()) {
		return fmt.Errorf("entry %q not yet due for purge", entryID)
	}

	if err := w.validatePurgePath(*e); err != nil {
		return err
	}

	// Durable purging state before destructive FS work.
	e.State = index.RecycleStatePurging
	if err := w.persist(); err != nil {
		return err
	}

	if err := w.q.Purge(e.QuarantinePath); err != nil {
		return fmt.Errorf("purge %s: %w", entryID, err)
	}

	// Re-find index after persist (slice identity is fine; re-locate for safety).
	eIdx = -1
	for i, ent := range w.doc.RecycleBin {
		if ent.ID == entryID {
			eIdx = i
			break
		}
	}
	if eIdx < 0 {
		return nil
	}
	e = &w.doc.RecycleBin[eIdx]

	// Remove recycle placeholders.
	for _, id := range e.PlaceholderIDs {
		w.removePlaceholderEntirely(id)
	}
	// Drop entry from bin.
	w.doc.RecycleBin = append(w.doc.RecycleBin[:eIdx], w.doc.RecycleBin[eIdx+1:]...)
	return nil
}

// validatePurgePath ensures QuarantinePath is a direct child of a scan-root trash
// dir and (when possible) matches the entry id leaf name.
func (w *Workbench) validatePurgePath(e index.RecycleEntry) error {
	q := filepath.Clean(e.QuarantinePath)
	if !quarantine.IsQuarantineEntry(q) {
		return fmt.Errorf("purge refused: path is not a quarantine entry: %s", q)
	}
	// Must sit under some configured scan root's trash.
	underRoot := false
	for _, root := range w.scanRoots {
		if root == "" {
			continue
		}
		r, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		if rp, err := filepath.EvalSymlinks(r); err == nil {
			r = rp
		}
		trashRoot := filepath.Join(filepath.Clean(r), quarantine.TrashDirName)
		if filepath.Dir(q) == trashRoot || strings.HasPrefix(q, trashRoot+string(filepath.Separator)) {
			// Direct child only.
			if filepath.Dir(q) == trashRoot {
				underRoot = true
				break
			}
		}
	}
	if !underRoot {
		return fmt.Errorf("purge refused: quarantine path not under a scan-root trash: %s", q)
	}
	if e.ID != "" && filepath.Base(q) != e.ID {
		return fmt.Errorf("purge refused: quarantine path leaf %q != entry id %q", filepath.Base(q), e.ID)
	}
	return nil
}
