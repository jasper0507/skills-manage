package workbench

import "github.com/jasper0507/skills-manage/internal/infra/index"

// Placement write primitives for skill placeholders on the index document.
//
// Durable placement is only:
//   - desktop (kind + 1-based row/col)
//   - recycle (kind only)
//   - empty  (member-only: membership is ItemIDs; no parallel LocBox)
//
// LocBox is never written for skill placeholders — Desk projects it from membership.

func (w *Workbench) setDesktopPlacement(idx, row, col int) {
	w.doc.Placeholders[idx].Location = index.Location{
		Kind: LocDesktop, Row: row, Col: col,
	}
}

func (w *Workbench) setRecyclePlacement(idx int) {
	w.doc.Placeholders[idx].Location = index.Location{Kind: LocRecycle}
}

// clearPlacement marks a placeholder as member-only (no durable placement).
func (w *Workbench) clearPlacement(idx int) {
	w.doc.Placeholders[idx].Location = index.Location{}
}

// parkOffGrid temporarily removes a placeholder from desktop occupancy
// (empty placement, not recycle). Used while allocating free cells mid-mutation.
func (w *Workbench) parkOffGrid(idx int) {
	w.doc.Placeholders[idx].Location = index.Location{}
}

// placeStack controls how multi-item desktop placement advances the search origin.
type placeStack int

const (
	// stackRight: next prefer is (row, col+1) — multi-drop / icon→desktop.
	stackRight placeStack = iota
	// stackDown: next prefer is (row+1, col) — paste-cut / delete-box return.
	stackDown
)

// placeExistingOnDesktop strips box membership, frees vacated desktop cells,
// and places each id on a free grid cell near prefer.
// When pinExact is true, the first id takes prefer if that cell is free.
func (w *Workbench) placeExistingOnDesktop(ids []string, prefer cell, pinExact bool, stack placeStack) {
	occupied := w.occupiedDesktopCells()
	for _, id := range ids {
		idx, ok := w.placeholderIndex(id)
		if !ok {
			continue
		}
		loc := w.doc.Placeholders[idx].Location
		if validDesktopPlacement(loc) {
			delete(occupied, cell{loc.Row, loc.Col})
		}
		w.removePlaceholderFromContainers(id)
		w.parkOffGrid(idx)
	}

	origin := prefer
	first := true
	for _, id := range ids {
		idx, ok := w.placeholderIndex(id)
		if !ok {
			continue
		}
		free := nextFreeCell(occupied, origin.col, origin.row)
		if pinExact && first && !occupied[prefer] {
			free = prefer
		}
		first = false
		w.setDesktopPlacement(idx, free.row, free.col)
		occupied[free] = true
		switch stack {
		case stackRight:
			origin = cell{free.row, free.col + 1}
			if origin.col < 1 {
				origin.col = 1
			}
		case stackDown:
			origin = cell{free.row + 1, free.col}
		}
	}
}

// allocateDesktopCells picks n free cells near prefer, updating occupied.
// When pinExact is true, the first cell is prefer if free.
// Used when creating new placeholders (copy-paste).
func allocateDesktopCells(occupied map[cell]bool, n int, prefer cell, pinExact bool, stack placeStack) []cell {
	out := make([]cell, 0, n)
	origin := prefer
	for i := 0; i < n; i++ {
		free := nextFreeCell(occupied, origin.col, origin.row)
		if pinExact && i == 0 && !occupied[prefer] {
			free = prefer
		}
		occupied[free] = true
		out = append(out, free)
		switch stack {
		case stackRight:
			origin = cell{free.row, free.col + 1}
			if origin.col < 1 {
				origin.col = 1
			}
		case stackDown:
			origin = cell{free.row + 1, free.col}
		}
	}
	return out
}

// placeOneInViewport puts one placeholder on the next free viewport cell.
func (w *Workbench) placeOneInViewport(idx int) {
	occupied := w.occupiedDesktopCells()
	free := nextFreeCellInViewport(occupied)
	w.setDesktopPlacement(idx, free.row, free.col)
}
