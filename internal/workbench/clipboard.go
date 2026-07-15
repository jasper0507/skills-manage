package workbench

import (
	"fmt"

	"github.com/jasper0507/skills-manage/internal/infra/index"
)

// SetClipboard puts skill placeholders on the session clipboard (copy or cut).
// Unknown ids and placeholders already in recycle are filtered out. The recycle
// system icon is never a placeholder and cannot be clipboard-targeted.
func (w *Workbench) SetClipboard(mode string, placeholderIDs []string) error {
	if err := w.requireOpen(); err != nil {
		return err
	}
	if mode != ClipCopy && mode != ClipCut {
		return fmt.Errorf("clipboard mode must be %q or %q", ClipCopy, ClipCut)
	}
	ids := make([]string, 0, len(placeholderIDs))
	seen := make(map[string]bool, len(placeholderIDs))
	for _, id := range placeholderIDs {
		if seen[id] {
			continue
		}
		idx, ok := w.placeholderIndex(id)
		if !ok {
			continue
		}
		if w.doc.Placeholders[idx].Location.Kind == LocRecycle {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return fmt.Errorf("clipboard empty: no valid placeholders")
	}
	w.clipboard = &Clipboard{Mode: mode, PlaceholderIDs: ids}
	return nil
}

// PasteToDesktop pastes the clipboard onto free desktop grid cells near (row, col).
// Copy mode duplicates icons (same skill identity); cut mode moves and clears the clipboard.
func (w *Workbench) PasteToDesktop(row, col int) error {
	if err := w.requireOpen(); err != nil {
		return err
	}
	if row < 1 || col < 1 {
		return fmt.Errorf("invalid grid cell (%d,%d)", row, col)
	}
	cb := w.clipboard
	if cb == nil || len(cb.PlaceholderIDs) == 0 {
		return fmt.Errorf("clipboard empty")
	}

	occupied := w.occupiedDesktopCells()
	preferRow, preferCol := row, col

	if cb.Mode == ClipCopy {
		for _, srcID := range cb.PlaceholderIDs {
			srcIdx, ok := w.placeholderIndex(srcID)
			if !ok {
				continue
			}
			src := w.doc.Placeholders[srcIdx]
			free := nextFreeCell(occupied, preferCol, preferRow)
			occupied[free] = true
			preferRow = free.row
			// next paste stacks downward in the same preferred column first.
			newID := w.newPlaceholderID()
			w.doc.Placeholders = append(w.doc.Placeholders, index.PlaceholderRecord{
				ID:       newID,
				Identity: src.Identity,
				Location: index.Location{Kind: LocDesktop, Row: free.row, Col: free.col},
			})
		}
		// Copy mode keeps clipboard.
		return w.persist()
	}

	// Cut: move existing placeholders. Free vacated desktop cells so multi-item
	// paste and self-cell paste can land on the requested free slots.
	for _, srcID := range cb.PlaceholderIDs {
		srcIdx, ok := w.placeholderIndex(srcID)
		if !ok {
			continue
		}
		if w.doc.Placeholders[srcIdx].Location.Kind == LocRecycle {
			continue
		}
		loc := w.doc.Placeholders[srcIdx].Location
		if loc.Kind == LocDesktop {
			delete(occupied, cell{loc.Row, loc.Col})
		}
		w.removePlaceholderFromContainers(srcID)
		// Prefer the exact requested cell when free (first mover), else next free.
		free := nextFreeCell(occupied, preferCol, preferRow)
		if !occupied[cell{row, col}] {
			free = cell{row, col}
		}
		occupied[free] = true
		preferRow = free.row + 1
		w.doc.Placeholders[srcIdx].Location = index.Location{
			Kind: LocDesktop, Row: free.row, Col: free.col,
		}
	}
	w.clipboard = nil
	return w.persist()
}

// PasteToBox pastes the clipboard into a box's current compartment (or simple box body).
// Empty compartmentID uses the active compartment for composite boxes.
func (w *Workbench) PasteToBox(boxID, compartmentID string) error {
	if err := w.requireOpen(); err != nil {
		return err
	}
	cb := w.clipboard
	if cb == nil || len(cb.PlaceholderIDs) == 0 {
		return fmt.Errorf("clipboard empty")
	}
	bIdx, ok := w.boxIndex(boxID)
	if !ok {
		return fmt.Errorf("unknown box %q", boxID)
	}
	box := &w.doc.Boxes[bIdx]

	if cb.Mode == ClipCopy {
		for _, srcID := range cb.PlaceholderIDs {
			srcIdx, ok := w.placeholderIndex(srcID)
			if !ok {
				continue
			}
			src := w.doc.Placeholders[srcIdx]
			newID := w.newPlaceholderID()
			loc, err := w.locationForBoxMember(box, boxID, compartmentID)
			if err != nil {
				return err
			}
			w.doc.Placeholders = append(w.doc.Placeholders, index.PlaceholderRecord{
				ID:       newID,
				Identity: src.Identity,
				Location: loc,
			})
			if err := w.appendPlaceholderToBox(box, newID, loc.CompartmentID); err != nil {
				return err
			}
		}
		return w.persist()
	}

	// Cut: move into box (same membership rules as MovePlaceholderToBox).
	for _, srcID := range cb.PlaceholderIDs {
		if _, ok := w.placeholderIndex(srcID); !ok {
			continue
		}
		if err := w.movePlaceholderToBoxNoPersist(srcID, boxID, compartmentID); err != nil {
			return err
		}
	}
	w.clipboard = nil
	return w.persist()
}

func (w *Workbench) locationForBoxMember(box *index.BoxRecord, boxID, compartmentID string) (index.Location, error) {
	if box.Kind == BoxSimple {
		return index.Location{Kind: LocBox, BoxID: boxID}, nil
	}
	cid := compartmentID
	if cid == "" {
		cid = box.ActiveCompartmentID
	}
	for _, c := range box.Compartments {
		if c.ID == cid {
			return index.Location{Kind: LocBox, BoxID: boxID, CompartmentID: cid}, nil
		}
	}
	return index.Location{}, fmt.Errorf("unknown compartment %q", cid)
}

func (w *Workbench) appendPlaceholderToBox(box *index.BoxRecord, phID, compartmentID string) error {
	if box.Kind == BoxSimple {
		if !containsID(box.ItemIDs, phID) {
			box.ItemIDs = append(box.ItemIDs, phID)
		}
		return nil
	}
	cid := compartmentID
	if cid == "" {
		cid = box.ActiveCompartmentID
	}
	for i := range box.Compartments {
		if box.Compartments[i].ID == cid {
			if !containsID(box.Compartments[i].ItemIDs, phID) {
				box.Compartments[i].ItemIDs = append(box.Compartments[i].ItemIDs, phID)
			}
			return nil
		}
	}
	return fmt.Errorf("unknown compartment %q", cid)
}

func (w *Workbench) movePlaceholderToBoxNoPersist(placeholderID, boxID, compartmentID string) error {
	phIdx, ok := w.placeholderIndex(placeholderID)
	if !ok {
		return fmt.Errorf("unknown placeholder %q", placeholderID)
	}
	if w.doc.Placeholders[phIdx].Location.Kind == LocRecycle {
		return fmt.Errorf("cannot move placeholder in recycle")
	}
	bIdx, ok := w.boxIndex(boxID)
	if !ok {
		return fmt.Errorf("unknown box %q", boxID)
	}
	box := &w.doc.Boxes[bIdx]
	if box.Kind != BoxSimple && box.Kind != BoxComposite {
		return fmt.Errorf("unknown box kind %q", box.Kind)
	}
	// Resolve target fully before mutating membership (avoids partial strip on error).
	loc, err := w.locationForBoxMember(box, boxID, compartmentID)
	if err != nil {
		return err
	}
	w.removePlaceholderFromContainers(placeholderID)
	// Re-resolve box after membership strip (ItemIDs slices are reassigned on filter).
	bIdx, ok = w.boxIndex(boxID)
	if !ok {
		return fmt.Errorf("unknown box %q", boxID)
	}
	box = &w.doc.Boxes[bIdx]
	if err := w.appendPlaceholderToBox(box, placeholderID, loc.CompartmentID); err != nil {
		return err
	}
	w.doc.Placeholders[phIdx].Location = loc
	return nil
}

// EnableMultiSelect turns multi-select on with the given placeholder pre-selected.
func (w *Workbench) EnableMultiSelect(placeholderID string) error {
	if err := w.requireOpen(); err != nil {
		return err
	}
	if placeholderID != "" {
		if _, ok := w.placeholderIndex(placeholderID); !ok {
			return fmt.Errorf("unknown placeholder %q", placeholderID)
		}
	}
	w.multiSelect = true
	if placeholderID == "" {
		w.selectedIDs = nil
	} else {
		w.selectedIDs = []string{placeholderID}
	}
	return nil
}

// DisableMultiSelect exits multi-select and clears the selection.
func (w *Workbench) DisableMultiSelect() {
	w.multiSelect = false
	w.selectedIDs = nil
}

// ToggleSelected toggles a placeholder in the multi-select set. No-op if multi-select is off.
func (w *Workbench) ToggleSelected(placeholderID string) error {
	if err := w.requireOpen(); err != nil {
		return err
	}
	if !w.multiSelect {
		return nil
	}
	if _, ok := w.placeholderIndex(placeholderID); !ok {
		return fmt.Errorf("unknown placeholder %q", placeholderID)
	}
	for i, id := range w.selectedIDs {
		if id == placeholderID {
			w.selectedIDs = append(w.selectedIDs[:i], w.selectedIDs[i+1:]...)
			return nil
		}
	}
	w.selectedIDs = append(w.selectedIDs, placeholderID)
	return nil
}

// MovePlaceholdersToBox bulk-files placeholders into a box's current compartment.
// Used for multi-select drag into a box. Unknown and recycle placeholders are skipped.
func (w *Workbench) MovePlaceholdersToBox(placeholderIDs []string, boxID, compartmentID string) error {
	if err := w.requireOpen(); err != nil {
		return err
	}
	bIdx, ok := w.boxIndex(boxID)
	if !ok {
		return fmt.Errorf("unknown box %q", boxID)
	}
	// Pre-resolve compartment so a bad target fails before any mutation.
	if _, err := w.locationForBoxMember(&w.doc.Boxes[bIdx], boxID, compartmentID); err != nil {
		return err
	}

	ids := make([]string, 0, len(placeholderIDs))
	seen := make(map[string]bool, len(placeholderIDs))
	for _, id := range placeholderIDs {
		if seen[id] {
			continue
		}
		idx, ok := w.placeholderIndex(id)
		if !ok {
			continue
		}
		if w.doc.Placeholders[idx].Location.Kind == LocRecycle {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	for _, id := range ids {
		if err := w.movePlaceholderToBoxNoPersist(id, boxID, compartmentID); err != nil {
			return err
		}
	}
	return w.persist()
}

// CreateSimpleBox places an empty 普通盒子 at (x,y), nudged off desktop skill icons.
