package workbench

import (
	"fmt"

	"github.com/jasper0507/skills-manage/internal/infra/index"
)

// MovePlaceholderToDesktop drops a skill 占位 onto a desktop grid cell.
// Icon → icon on the same cell creates a 普通盒子 named 「普通盒子N」 containing both.
// Empty cells (or recycle-only cells) place the mover on a free cell without stacking.
func (w *Workbench) MovePlaceholderToDesktop(placeholderID string, row, col int) error {
	return w.MovePlaceholdersToDesktop([]string{placeholderID}, row, col)
}

// MovePlaceholdersToDesktop drops one or more 占位 onto the desktop grid (multi-select).
// If the target cell holds a non-mover skill, all movers merge with it into a 普通盒子.
// Otherwise each mover is parked then placed into free cells starting at (row, col).
func (w *Workbench) MovePlaceholdersToDesktop(placeholderIDs []string, row, col int) error {
	return w.withMutation(func() error {
		return w.movePlaceholdersToDesktopNoPersist(placeholderIDs, row, col)
	})
}

func (w *Workbench) movePlaceholdersToDesktopNoPersist(placeholderIDs []string, row, col int) error {
	if row < 1 || col < 1 {
		return fmt.Errorf("invalid grid cell (%d,%d)", row, col)
	}
	ids := make([]string, 0, len(placeholderIDs))
	seen := make(map[string]bool, len(placeholderIDs))
	for _, id := range placeholderIDs {
		if id == "" || seen[id] {
			continue
		}
		idx, ok := w.placeholderIndex(id)
		if !ok {
			return fmt.Errorf("unknown placeholder %q", id)
		}
		if w.doc.Placeholders[idx].Location.Kind == LocRecycle {
			return fmt.Errorf("cannot move placeholder in recycle")
		}
		seen[id] = true
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return fmt.Errorf("no placeholders to move")
	}

	// Icon → icon: another skill occupies this cell (excluding the movers).
	// Match prototype: only non-selected occupants trigger auto-box with all movers.
	if occID, found := w.skillAtDesktopCellExcluding(row, col, ids); found {
		all := append([]string{occID}, ids...)
		return w.mergeIconsIntoAutoBoxNoPersist(all, row, col)
	}

	// Park movers so they do not block free-cell search. Membership strip + location
	// update together (single write path for leave-box → desktop).
	for _, id := range ids {
		idx, _ := w.placeholderIndex(id)
		w.removePlaceholderFromContainers(id)
		w.doc.Placeholders[idx].Location = index.Location{Kind: LocDesktop, Row: -1, Col: -1}
	}

	occupied := w.occupiedDesktopCells()
	// Prefer requested cell first, then walk free cells.
	startRow, startCol := row, col
	for _, id := range ids {
		idx, _ := w.placeholderIndex(id)
		free := nextFreeCell(occupied, startCol, startRow)
		if !occupied[cell{row, col}] && startRow == row && startCol == col {
			// First placement: exact cell when free.
			free = cell{row, col}
		}
		w.doc.Placeholders[idx].Location = index.Location{Kind: LocDesktop, Row: free.row, Col: free.col}
		occupied[free] = true
		startRow = free.row
		startCol = free.col + 1
		if startCol < 1 {
			startCol = 1
		}
	}
	return nil
}

func (w *Workbench) skillAtDesktopCell(row, col int, excludePhID string) (string, bool) {
	return w.skillAtDesktopCellExcluding(row, col, []string{excludePhID})
}

func (w *Workbench) skillAtDesktopCellExcluding(row, col int, excludePhIDs []string) (string, bool) {
	ex := make(map[string]bool, len(excludePhIDs))
	for _, id := range excludePhIDs {
		ex[id] = true
	}
	return w.skillAtDesktopCellExcludeMap(row, col, ex)
}

func (w *Workbench) skillAtDesktopCellExcludeMap(row, col int, exclude map[string]bool) (string, bool) {
	for _, p := range w.doc.Placeholders {
		if exclude[p.ID] {
			continue
		}
		if p.Location.Kind != LocDesktop {
			continue
		}
		if p.Location.Row == row && p.Location.Col == col {
			return p.ID, true
		}
	}
	return "", false
}

func (w *Workbench) removePlaceholderFromContainers(phID string) {
	for i := range w.doc.Boxes {
		b := &w.doc.Boxes[i]
		if b.Kind == BoxSimple {
			b.ItemIDs = filterID(b.ItemIDs, phID)
			continue
		}
		for j := range b.Compartments {
			b.Compartments[j].ItemIDs = filterID(b.Compartments[j].ItemIDs, phID)
		}
	}
}

func filterID(ids []string, remove string) []string {
	if len(ids) == 0 {
		return nil
	}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != remove {
			out = append(out, id)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// mergeIconsIntoAutoBoxNoPersist creates a sequenced 普通盒子 containing the given placeholders.
func (w *Workbench) mergeIconsIntoAutoBoxNoPersist(phIDs []string, nearRow, nearCol int) error {
	// Deduplicate while preserving order.
	seen := make(map[string]bool, len(phIDs))
	ids := make([]string, 0, len(phIDs))
	for _, id := range phIDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}

	// Resolve placement before mutating membership.
	x := float64(iconGridOriginX + (nearCol-1)*iconGridCellW)
	y := float64(iconGridOriginY + (nearRow-1)*iconGridCellH)
	// Exclude movers from collision: temporarily park them off-desktop for the search.
	type savedLoc struct {
		idx int
		loc index.Location
	}
	saved := make([]savedLoc, 0, len(ids))
	for _, id := range ids {
		idx, ok := w.placeholderIndex(id)
		if !ok {
			continue
		}
		saved = append(saved, savedLoc{idx, w.doc.Placeholders[idx].Location})
		w.doc.Placeholders[idx].Location = index.Location{Kind: LocDesktop, Row: -1, Col: -1}
	}
	pos, ok := w.findBoxPosWithoutIconOverlap(x, y, defaultSimpleBoxW, defaultSimpleBoxH, "")
	// Restore so failure leaves state unchanged; success path rewrites locations below.
	// (Caller withMutation also snapshots the full document.)
	for _, s := range saved {
		w.doc.Placeholders[s.idx].Location = s.loc
	}
	if !ok {
		return fmt.Errorf("no free box position that avoids covering desktop skill icons")
	}

	for _, id := range ids {
		w.removePlaceholderFromContainers(id)
	}

	if w.doc.BoxNameSeq < 1 {
		w.doc.BoxNameSeq = 1
	}
	tag := fmt.Sprintf("普通盒子%d", w.doc.BoxNameSeq)
	w.doc.BoxNameSeq++

	boxID := w.newBoxID()
	box := index.BoxRecord{
		ID:      boxID,
		Kind:    BoxSimple,
		Tag:     tag,
		X:       pos.x,
		Y:       pos.y,
		W:       defaultSimpleBoxW,
		H:       defaultSimpleBoxH,
		ItemIDs: make([]string, 0, len(ids)),
	}
	for _, id := range ids {
		idx, ok := w.placeholderIndex(id)
		if !ok {
			continue
		}
		// Membership + Location together (ItemIDs is membership truth).
		box.ItemIDs = append(box.ItemIDs, id)
		w.doc.Placeholders[idx].Location = index.Location{
			Kind:  LocBox,
			BoxID: boxID,
		}
	}
	w.doc.Boxes = append(w.doc.Boxes, box)
	return nil
}

func (w *Workbench) ensureRecycleDefault() {
	if w.doc.RecycleIcon.Kind == "" {
		w.doc.RecycleIcon = index.Location{Kind: LocDesktop, Row: 1, Col: 1}
		return
	}
	if w.doc.RecycleIcon.Kind == LocDesktop && (w.doc.RecycleIcon.Row == 0 || w.doc.RecycleIcon.Col == 0) {
		// Zero means unset for 1-based grid; restore product default.
		w.doc.RecycleIcon.Row = 1
		w.doc.RecycleIcon.Col = 1
	}
}

func (w *Workbench) reconcileFromScan() error {
	found, err := w.scan.Scan(w.scanRoots)
	if err != nil {
		return err
	}

	// Refresh skill name cache for known identities; keep full inventory snapshot.
	skills := make([]index.SkillRecord, 0, len(found))
	for _, s := range found {
		skills = append(skills, index.SkillRecord{Identity: s.Identity, Name: s.Name})
	}
	w.doc.Skills = skills

	// Identities that already have at least one placeholder (any location).
	have := make(map[string]bool, len(w.doc.Placeholders))
	for _, p := range w.doc.Placeholders {
		have[p.Identity] = true
	}

	occupied := w.occupiedDesktopCells()

	for _, s := range found {
		if have[s.Identity] {
			continue
		}
		// Row-major within one-screen viewport (not a tall first-column stack).
		c := nextFreeCellInViewport(occupied)
		occupied[c] = true
		id := w.newPlaceholderID()
		w.doc.Placeholders = append(w.doc.Placeholders, index.PlaceholderRecord{
			ID:       id,
			Identity: s.Identity,
			Location: index.Location{Kind: LocDesktop, Row: c.row, Col: c.col},
		})
		have[s.Identity] = true
	}
	if w.doc.SchemaVersion == 0 {
		w.doc.SchemaVersion = index.SchemaVersion
	}
	return nil
}
