package workbench

import "github.com/jasper0507/skills-manage/internal/infra/index"

type point struct{ x, y float64 }

// findBoxPosWithoutIconOverlap snaps (x,y) and nudges until the box rectangle
// does not cover desktop skill icons or the recycle icon. ok is false when no
// clear slot is found within the search radius (refuse cover rather than silent overlap).
// Boxes may stack; only icon coverage is forbidden.
func (w *Workbench) findBoxPosWithoutIconOverlap(x, y, width, height float64) (point, bool) {
	// Snapshot desktop icon rects once; membership does not change during nudge search.
	icons := w.desktopIconRects()
	px := snap(x, boxSnapGrid)
	py := snap(y, boxSnapGrid)
	if boxPosOK(px, py, width, height, icons) {
		return point{px, py}, true
	}
	g := float64(boxSnapGrid)
	for step := g; step < 800; step += g {
		for _, d := range [][2]float64{
			{step, 0}, {-step, 0}, {0, step}, {0, -step},
			{step, step}, {-step, step}, {step, -step}, {-step, -step},
		} {
			tx := maxF(0, px+d[0])
			ty := maxF(0, py+d[1])
			if boxPosOK(tx, ty, width, height, icons) {
				return point{tx, ty}, true
			}
		}
	}
	return point{px, py}, false
}

// desktopIconRects is the set of skill + recycle icon rectangles that occupy
// true desktop placement (membership-only members are excluded).
func (w *Workbench) desktopIconRects() []rect {
	membership := w.membershipByPlaceholder()
	out := make([]rect, 0, len(w.doc.Placeholders)+1)
	for _, p := range w.doc.Placeholders {
		if !isDesktopGridPlacement(p, membership) {
			continue
		}
		out = append(out, iconRectAtCell(p.Location.Row, p.Location.Col))
	}
	r := w.doc.RecycleIcon
	if r.Kind == LocDesktop && r.Row >= 1 && r.Col >= 1 {
		out = append(out, iconRectAtCell(r.Row, r.Col))
	}
	return out
}

func boxPosOK(x, y, width, height float64, icons []rect) bool {
	box := rect{x, y, width, height}
	for _, ir := range icons {
		if rectsOverlap(box, ir, 2) {
			return false
		}
	}
	return true
}

type rect struct{ x, y, w, h float64 }

func iconRectAtCell(row, col int) rect {
	if row < 1 {
		row = 1
	}
	if col < 1 {
		col = 1
	}
	return rect{
		x: float64(iconGridOriginX + (col-1)*iconGridCellW),
		y: float64(iconGridOriginY + (row-1)*iconGridCellH),
		w: iconW,
		h: iconH,
	}
}

func rectsOverlap(a, b rect, pad float64) bool {
	return a.x+pad < b.x+b.w-pad &&
		a.x+a.w-pad > b.x+pad &&
		a.y+pad < b.y+b.h-pad &&
		a.y+a.h-pad > b.y+pad
}

func snap(n, g float64) float64 {
	if g <= 0 {
		return n
	}
	return float64(int(n/g+0.5)) * g
}

func maxF(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

type cell struct {
	row, col int
}

func (w *Workbench) occupiedDesktopCells() map[cell]bool {
	occ := make(map[cell]bool)
	if w.doc.RecycleIcon.Kind == LocDesktop && w.doc.RecycleIcon.Row >= 1 && w.doc.RecycleIcon.Col >= 1 {
		occ[cell{w.doc.RecycleIcon.Row, w.doc.RecycleIcon.Col}] = true
	}
	// Grid occupancy is desktop placement only. In-box members are membership-
	// truth (not LocBox); recycle is placement-only. Stale desktop coords on a
	// member must not block free-cell search (E3.2 / ADR-0002).
	membership := w.membershipByPlaceholder()
	for _, p := range w.doc.Placeholders {
		if !isDesktopGridPlacement(p, membership) {
			continue
		}
		occ[cell{p.Location.Row, p.Location.Col}] = true
	}
	return occ
}

// isDesktopGridPlacement is true when a placeholder occupies a desktop grid cell:
// not a box member, and has a valid 1-based desktop cell (recycle fails the latter).
func isDesktopGridPlacement(p index.PlaceholderRecord, membership map[string]boxMembership) bool {
	if _, member := membership[p.ID]; member {
		return false
	}
	return validDesktopPlacement(p.Location)
}

// nextFreeCell prefers preferCol from startRow downward, then row-major free cells.
// Used near a drop target (user intent). New unfiled skills use nextFreeCellInViewport.
func nextFreeCell(occupied map[cell]bool, preferCol, startRow int) cell {
	if preferCol < 1 {
		preferCol = 1
	}
	if startRow < 1 {
		startRow = 1
	}
	for row := startRow; row < startRow+10_000; row++ {
		c := cell{row, preferCol}
		if !occupied[c] {
			return c
		}
	}
	for row := 1; row < 10_000; row++ {
		for col := 1; col <= 64; col++ {
			c := cell{row, col}
			if !occupied[c] {
				return c
			}
		}
	}
	// Pathological full grid; still return something deterministic.
	return cell{startRow, preferCol}
}

// nextFreeCellInViewport picks the next free cell in row-major order within the
// default one-screen grid (cols left→right, then next row). Overflow continues
// row-major beyond DefaultViewportRows so placement never blocks.
func nextFreeCellInViewport(occupied map[cell]bool) cell {
	for row := 1; row <= DefaultViewportRows; row++ {
		for col := 1; col <= DefaultViewportCols; col++ {
			c := cell{row, col}
			if !occupied[c] {
				return c
			}
		}
	}
	// Viewport full: extend downward row-major at full viewport width.
	for row := DefaultViewportRows + 1; row < DefaultViewportRows+10_000; row++ {
		for col := 1; col <= DefaultViewportCols; col++ {
			c := cell{row, col}
			if !occupied[c] {
				return c
			}
		}
	}
	return cell{1, 1}
}
