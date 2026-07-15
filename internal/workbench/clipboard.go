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
		if _, ok := w.placeholderIndex(id); !ok {
			continue
		}
		if w.placeholderInRecycle(id) {
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
	return w.withMutation(func() error {
		if row < 1 || col < 1 {
			return fmt.Errorf("invalid grid cell (%d,%d)", row, col)
		}
		cb := w.clipboard
		if cb == nil || len(cb.PlaceholderIDs) == 0 {
			return fmt.Errorf("clipboard empty")
		}

		prefer := cell{row, col}

		if cb.Mode == ClipCopy {
			// Collect valid sources first so we allocate the right number of cells.
			type src struct {
				identity string
			}
			sources := make([]src, 0, len(cb.PlaceholderIDs))
			for _, srcID := range cb.PlaceholderIDs {
				srcIdx, ok := w.placeholderIndex(srcID)
				if !ok {
					continue
				}
				sources = append(sources, src{identity: w.doc.Placeholders[srcIdx].Identity})
			}
			occupied := w.occupiedDesktopCells()
			// Copy stacks downward in the preferred column (historical paste behavior).
			cells := allocateDesktopCells(occupied, len(sources), prefer, true, stackDown)
			for i, s := range sources {
				newID := w.newPlaceholderID()
				w.doc.Placeholders = append(w.doc.Placeholders, index.PlaceholderRecord{
					ID:       newID,
					Identity: s.identity,
				})
				w.setDesktopPlacement(len(w.doc.Placeholders)-1, cells[i].row, cells[i].col)
			}
			// Copy mode keeps clipboard.
			return nil
		}

		// Cut: move existing placeholders onto free cells; clear clipboard.
		ids := make([]string, 0, len(cb.PlaceholderIDs))
		for _, srcID := range cb.PlaceholderIDs {
			if _, ok := w.placeholderIndex(srcID); !ok {
				continue
			}
			if w.placeholderInRecycle(srcID) {
				continue
			}
			ids = append(ids, srcID)
		}
		w.placeExistingOnDesktop(ids, prefer, true, stackDown)
		w.clipboard = nil
		return nil
	})
}

// PasteToBox pastes the clipboard into a box's current compartment (or simple box body).
// Empty compartmentID uses the active compartment for composite boxes.
func (w *Workbench) PasteToBox(boxID, compartmentID string) error {
	return w.withMutation(func() error {
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
			// Resolve target once so a bad compartment fails before any admit.
			cid, err := w.resolveBoxTarget(box, compartmentID)
			if err != nil {
				return err
			}
			for _, srcID := range cb.PlaceholderIDs {
				srcIdx, ok := w.placeholderIndex(srcID)
				if !ok {
					continue
				}
				src := w.doc.Placeholders[srcIdx]
				newID := w.newPlaceholderID()
				w.doc.Placeholders = append(w.doc.Placeholders, index.PlaceholderRecord{
					ID:       newID,
					Identity: src.Identity,
				})
				// admitMember: ItemIDs + clear placement (membership-only in-box).
				if err := w.admitMember(box, newID, cid); err != nil {
					return err
				}
			}
			return nil
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
		return nil
	})
}

// resolveBoxTarget returns the compartment id for a paste/move into a box.
// Simple boxes return "". Empty compartmentID uses the active compartment.
// This is the only place that resolves compartment; admitMember requires the result.
func (w *Workbench) resolveBoxTarget(box *index.BoxRecord, compartmentID string) (string, error) {
	if box.Kind == BoxSimple {
		return "", nil
	}
	cid := compartmentID
	if cid == "" {
		cid = box.ActiveCompartmentID
	}
	for _, c := range box.Compartments {
		if c.ID == cid {
			return cid, nil
		}
	}
	return "", fmt.Errorf("unknown compartment %q", cid)
}

// admitMember adds phID to the box/compartment ItemIDs and clears durable
// placement. resolvedCompartmentID must come from resolveBoxTarget ("" for simple).
// In-box truth is membership only — callers must not also write LocBox.
func (w *Workbench) admitMember(box *index.BoxRecord, phID, resolvedCompartmentID string) error {
	if box.Kind == BoxSimple {
		if !containsID(box.ItemIDs, phID) {
			box.ItemIDs = append(box.ItemIDs, phID)
		}
	} else {
		found := false
		for i := range box.Compartments {
			if box.Compartments[i].ID != resolvedCompartmentID {
				continue
			}
			if !containsID(box.Compartments[i].ItemIDs, phID) {
				box.Compartments[i].ItemIDs = append(box.Compartments[i].ItemIDs, phID)
			}
			found = true
			break
		}
		if !found {
			return fmt.Errorf("unknown compartment %q", resolvedCompartmentID)
		}
	}
	if idx, ok := w.placeholderIndex(phID); ok {
		w.clearPlacement(idx)
	}
	return nil
}

func (w *Workbench) movePlaceholderToBoxNoPersist(placeholderID, boxID, compartmentID string) error {
	if _, ok := w.placeholderIndex(placeholderID); !ok {
		return fmt.Errorf("unknown placeholder %q", placeholderID)
	}
	if w.placeholderInRecycle(placeholderID) {
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
	cid, err := w.resolveBoxTarget(box, compartmentID)
	if err != nil {
		return err
	}
	w.removePlaceholderFromContainers(placeholderID)
	// Re-resolve box after membership strip (ItemIDs slices are reassigned on filter).
	bIdx, ok = w.boxIndex(boxID)
	if !ok {
		return fmt.Errorf("unknown box %q", boxID)
	}
	return w.admitMember(&w.doc.Boxes[bIdx], placeholderID, cid)
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
	return w.withMutation(func() error {
		bIdx, ok := w.boxIndex(boxID)
		if !ok {
			return fmt.Errorf("unknown box %q", boxID)
		}
		// Pre-resolve compartment so a bad target fails before any mutation.
		if _, err := w.resolveBoxTarget(&w.doc.Boxes[bIdx], compartmentID); err != nil {
			return err
		}

		ids := make([]string, 0, len(placeholderIDs))
		seen := make(map[string]bool, len(placeholderIDs))
		for _, id := range placeholderIDs {
			if seen[id] {
				continue
			}
			if _, ok := w.placeholderIndex(id); !ok {
				continue
			}
			if w.placeholderInRecycle(id) {
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
		return nil
	})
}
