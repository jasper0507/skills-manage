package workbench

import "github.com/jasper0507/skills-manage/internal/infra/index"

// boxMembership is one box/compartment claim for a placeholder (ItemIDs truth).
type boxMembership struct {
	boxID         string
	compartmentID string // empty for simple boxes
}

// rehomeFromMembership normalizes the document to membership-truth shape:
//
//   - Box ItemIDs are the sole record of who is in a box/compartment (first claim
//     wins; unknown / recycle / later-duplicate IDs are stripped).
//   - Members keep no parallel in-box Location on the document (placement for
//     them is membership alone).
//   - Ghost LocBox without membership → free desktop cell in the viewport.
//   - Non-recycle, non-member without a valid desktop cell → free viewport cell.
//   - Recycle placeholders stay kind-only LocRecycle.
//
// Desk projects LocBox from membership for the external view; this function
// only shapes the index document.
func (w *Workbench) rehomeFromMembership() {
	// phID → first membership claim (ItemIDs order wins; later duplicates stripped).
	claimed := make(map[string]boxMembership, len(w.doc.Placeholders))
	phOK := make(map[string]bool, len(w.doc.Placeholders))
	for _, p := range w.doc.Placeholders {
		phOK[p.ID] = true
	}

	for i := range w.doc.Boxes {
		b := &w.doc.Boxes[i]
		if b.Kind == BoxSimple {
			b.ItemIDs = w.cleanItemIDs(b.ItemIDs, phOK, claimed, b.ID, "")
			continue
		}
		for j := range b.Compartments {
			c := &b.Compartments[j]
			c.ItemIDs = w.cleanItemIDs(c.ItemIDs, phOK, claimed, b.ID, c.ID)
		}
	}

	// Placement: members drop LocBox; ghosts and invalid desktop get free cells.
	// Free members' former desktop cells so subsequent ghost placement can reuse them.
	occupied := w.occupiedDesktopCells()
	for i := range w.doc.Placeholders {
		p := &w.doc.Placeholders[i]
		if p.Location.Kind == LocRecycle {
			// Recycle is not a box member; strip any accidental box/desktop coords.
			p.Location = index.Location{Kind: LocRecycle}
			continue
		}
		if _, ok := claimed[p.ID]; ok {
			// Membership is the only box truth — no parallel LocBox (or desktop) on disk.
			if p.Location.Kind == LocDesktop {
				delete(occupied, cell{p.Location.Row, p.Location.Col})
			}
			p.Location = index.Location{}
			continue
		}
		// Not a box member: need a valid desktop placement.
		if p.Location.Kind == LocBox || !validDesktopPlacement(p.Location) {
			if p.Location.Kind == LocDesktop {
				delete(occupied, cell{p.Location.Row, p.Location.Col})
			}
			free := nextFreeCellInViewport(occupied)
			occupied[free] = true
			p.Location = index.Location{Kind: LocDesktop, Row: free.row, Col: free.col}
		}
	}
}

// validDesktopPlacement is true for kind=desktop with 1-based row/col.
func validDesktopPlacement(loc index.Location) bool {
	return loc.Kind == LocDesktop && loc.Row >= 1 && loc.Col >= 1
}

func (w *Workbench) cleanItemIDs(
	ids []string,
	phOK map[string]bool,
	claimed map[string]boxMembership,
	boxID, compartmentID string,
) []string {
	if len(ids) == 0 {
		return nil
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if !phOK[id] {
			continue
		}
		// Recycle placement forbids box membership.
		if w.placeholderInRecycle(id) {
			continue
		}
		if _, already := claimed[id]; already {
			// Prefer earlier ItemIDs claim; drop duplicate membership.
			continue
		}
		claimed[id] = boxMembership{boxID: boxID, compartmentID: compartmentID}
		out = append(out, id)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// placeholderInRecycle is true when the placeholder's durable placement is the
// icon recycle bin (LocRecycle). Recycle is placement truth, not membership.
func (w *Workbench) placeholderInRecycle(phID string) bool {
	idx, ok := w.placeholderIndex(phID)
	if !ok {
		return false
	}
	return w.doc.Placeholders[idx].Location.Kind == LocRecycle
}

// membershipByPlaceholder builds the first-claim map from current ItemIDs
// (no mutation). Used by Desk to project LocBox for the external view and by
// grid occupancy to ignore in-box members. "In a box?" is always this map,
// never Location.Kind == LocBox.
func (w *Workbench) membershipByPlaceholder() map[string]boxMembership {
	claimed := make(map[string]boxMembership, len(w.doc.Placeholders))
	for _, b := range w.doc.Boxes {
		if b.Kind == BoxSimple {
			for _, id := range b.ItemIDs {
				if _, ok := claimed[id]; ok {
					continue
				}
				claimed[id] = boxMembership{boxID: b.ID}
			}
			continue
		}
		for _, c := range b.Compartments {
			for _, id := range c.ItemIDs {
				if _, ok := claimed[id]; ok {
					continue
				}
				claimed[id] = boxMembership{boxID: b.ID, compartmentID: c.ID}
			}
		}
	}
	return claimed
}

// projectLocation overlays membership as LocBox for Desk/HTTP views.
// Recycle and true desktop placement pass through; members always project box.
func projectLocation(loc index.Location, m boxMembership, isMember bool) index.Location {
	if loc.Kind == LocRecycle {
		return index.Location{Kind: LocRecycle}
	}
	if isMember {
		out := index.Location{Kind: LocBox, BoxID: m.boxID}
		if m.compartmentID != "" {
			out.CompartmentID = m.compartmentID
		}
		return out
	}
	return loc
}
