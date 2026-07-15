package workbench

type point struct{ x, y float64 }

// findBoxPosWithoutIconOverlap snaps (x,y) and nudges until the box rectangle
// does not cover desktop skill icons or the recycle icon. ok is false when no
// clear slot is found within the search radius (refuse cover rather than silent overlap).
func (w *Workbench) findBoxPosWithoutIconOverlap(x, y, width, height float64, excludeBoxID string) (point, bool) {
	px := snap(x, boxSnapGrid)
	py := snap(y, boxSnapGrid)
	if w.boxPosOK(px, py, width, height, excludeBoxID) {
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
			if w.boxPosOK(tx, ty, width, height, excludeBoxID) {
				return point{tx, ty}, true
			}
		}
	}
	return point{px, py}, false
}

func (w *Workbench) boxPosOK(x, y, width, height float64, excludeBoxID string) bool {
	rect := rect{x, y, width, height}
	for _, p := range w.doc.Placeholders {
		if p.Location.Kind != LocDesktop {
			continue
		}
		if rectsOverlap(rect, iconRectAtCell(p.Location.Row, p.Location.Col), 2) {
			return false
		}
	}
	if w.doc.RecycleIcon.Kind == LocDesktop {
		r := w.doc.RecycleIcon
		if rectsOverlap(rect, iconRectAtCell(r.Row, r.Col), 2) {
			return false
		}
	}
	_ = excludeBoxID // boxes may partially stack; only icon coverage is forbidden
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
	if w.doc.RecycleIcon.Kind == LocDesktop {
		occ[cell{w.doc.RecycleIcon.Row, w.doc.RecycleIcon.Col}] = true
	}
	for _, p := range w.doc.Placeholders {
		if p.Location.Kind != LocDesktop {
			continue
		}
		occ[cell{p.Location.Row, p.Location.Col}] = true
	}
	return occ
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
