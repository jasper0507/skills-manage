package workbench

import "github.com/jasper0507/skills-manage/internal/infra/index"

// boxMembership is one box/compartment claim for a placeholder (ItemIDs truth).
type boxMembership struct {
	boxID         string
	compartmentID string // empty for simple boxes
}

// rehomeFromMembership makes box ItemIDs the membership source of truth and
// derives placeholder Location from it. Prefers ItemIDs when Location diverges
// ("in ItemIDs but Location says desktop" → LocBox; "Location says box but not
// in ItemIDs" → free desktop cell). Drops unknown/recycle IDs from ItemIDs.
// Does not touch recycle-bin placeholders' kind.
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

	// Derive Location from membership; free desktop for box-location ghosts.
	occupied := w.occupiedDesktopCells()
	for i := range w.doc.Placeholders {
		p := &w.doc.Placeholders[i]
		if p.Location.Kind == LocRecycle {
			// Recycle is not a box member; strip any accidental box coords.
			p.Location = index.Location{Kind: LocRecycle}
			continue
		}
		if m, ok := claimed[p.ID]; ok {
			loc := index.Location{Kind: LocBox, BoxID: m.boxID}
			if m.compartmentID != "" {
				loc.CompartmentID = m.compartmentID
			}
			// Leaving a desktop cell: free it for subsequent ghost placement.
			if p.Location.Kind == LocDesktop {
				delete(occupied, cell{p.Location.Row, p.Location.Col})
			}
			p.Location = loc
			continue
		}
		// Not a box member.
		if p.Location.Kind == LocBox {
			free := nextFreeCellInViewport(occupied)
			occupied[free] = true
			p.Location = index.Location{Kind: LocDesktop, Row: free.row, Col: free.col}
		}
	}
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
		// Recycle placeholders cannot live in a box.
		if idx, ok := w.placeholderIndex(id); ok {
			if w.doc.Placeholders[idx].Location.Kind == LocRecycle {
				continue
			}
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
