// Package workbench is the primary application facade for skills-manage.
// Callers (CLI, HTTP, tests) speak only to Workbench for product behavior.
package workbench

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/jasper0507/skills-manage/internal/index"
	"github.com/jasper0507/skills-manage/internal/scanner"
)

// LocationKind is where a 占位 or system icon lives on the workbench.
type LocationKind = index.LocationKind

// Location kinds (re-exported for callers at the Workbench seam).
const (
	LocDesktop = index.LocDesktop
	LocBox     = index.LocBox
	LocRecycle = index.LocRecycle
)

// Location is a desk position (desktop grid cell, box, or recycle).
type Location = index.Location

// Skill is a discovered local skill package. Identity is the canonical realpath.
type Skill struct {
	Identity string // Skill 身份: normalized realpath
	Name     string // display name (frontmatter name, else directory name)
}

// Inventory is the live 清单 of Skills discovered from 扫描根.
type Inventory struct {
	Skills []Skill
}

// Placeholder is a 占位 shortcut icon on the desk (not a file copy).
type Placeholder struct {
	ID       string
	Identity string // Skill 身份 (realpath)
	Name     string // display name from latest inventory when known
	Location Location
}

// SystemIcon is a non-skill desk icon (e.g. 回收站).
type SystemIcon struct {
	Location Location
}

// Box kinds (re-exported for callers at the Workbench seam).
const (
	BoxSimple    = index.BoxSimple
	BoxComposite = index.BoxComposite
)

// Default simple / composite box geometry (matches accepted prototype).
const (
	defaultSimpleBoxW    = 240
	defaultSimpleBoxH    = 220
	defaultCompositeBoxW = 280
	defaultCompositeBoxH = 260
)

// Icon grid pixel layout used for box↔icon collision (matches prototypes/workbench-desktop).
const (
	iconGridOriginX = 16
	iconGridOriginY = 16
	iconGridCellW   = 90
	iconGridCellH   = 96
	iconW           = 86
	iconH           = 90
	boxSnapGrid     = 16
)

// Compartment is one 隔间 of a composite box, with contained placeholders as icons.
type Compartment struct {
	ID    string
	Tag   string
	Items []Placeholder
}

// Box is a 普通盒子 or 组合盒子 on the desk.
type Box struct {
	ID                  string
	Kind                string // simple | composite
	Tag                 string // simple: single tag / display name
	Title               string // composite: 盒标题
	X, Y, W, H          float64
	Items               []Placeholder // simple box contents as icons
	Compartments        []Compartment
	ActiveCompartmentID string
}

// Desk is the external desktop view: placeholders + recycle system icon + boxes.
type Desk struct {
	Placeholders []Placeholder
	RecycleIcon  SystemIcon
	Boxes        []Box
}

// Config configures a Workbench.
type Config struct {
	// ScanRoots are filesystem roots to walk for skill packages.
	// Empty means scan nothing (CLI fills defaults before constructing Workbench).
	ScanRoots []string

	// Scanner discovers packages under scan roots. Nil uses the default filesystem scanner.
	Scanner scanner.Scanner

	// Index is the 中央索引 store. Nil uses an in-memory store (ephemeral).
	Index index.Store
}

// Workbench is the sole primary product seam.
type Workbench struct {
	scanRoots []string
	scan      scanner.Scanner
	store     index.Store

	doc    index.Document
	opened bool
}

// New constructs a Workbench. Scanner defaults to the filesystem implementation when nil.
// Index defaults to an in-memory store when nil.
func New(cfg Config) *Workbench {
	sc := cfg.Scanner
	if sc == nil {
		sc = scanner.New()
	}
	st := cfg.Index
	if st == nil {
		st = index.NewMemoryStore()
	}
	return &Workbench{
		scanRoots: append([]string(nil), cfg.ScanRoots...),
		scan:      sc,
		store:     st,
	}
}

// Inventory returns the live Skill 清单 for configured scan roots (no desk mutation).
func (w *Workbench) Inventory() (Inventory, error) {
	found, err := w.scan.Scan(w.scanRoots)
	if err != nil {
		return Inventory{}, err
	}
	skills := make([]Skill, 0, len(found))
	for _, s := range found {
		skills = append(skills, Skill{
			Identity: s.Identity,
			Name:     s.Name,
		})
	}
	return Inventory{Skills: skills}, nil
}

// Open loads the central index, reconciles with a fresh scan (places only brand-new skills),
// and persists the desk. Safe to call once at session start; subsequent sessions call Open again.
func (w *Workbench) Open() error {
	doc, err := w.store.Load()
	if err != nil {
		return fmt.Errorf("load index: %w", err)
	}
	w.doc = doc
	w.ensureRecycleDefault()
	if err := w.reconcileFromScan(); err != nil {
		return err
	}
	if err := w.persist(); err != nil {
		return err
	}
	w.opened = true
	return nil
}

// Rescan re-discovers inventory and places only skills that have no placeholder yet.
// Existing coordinates and stored box metadata are left unchanged.
func (w *Workbench) Rescan() error {
	if !w.opened {
		return w.Open()
	}
	if err := w.reconcileFromScan(); err != nil {
		return err
	}
	return w.persist()
}

// Desk returns the current desktop view (placeholders + recycle icon + boxes).
// Names come from the last Open/Rescan inventory snapshot in the index.
// Box contents are always exposed as icon placeholders.
func (w *Workbench) Desk() Desk {
	nameByID := make(map[string]string, len(w.doc.Skills))
	for _, s := range w.doc.Skills {
		nameByID[s.Identity] = s.Name
	}

	phByID := make(map[string]Placeholder, len(w.doc.Placeholders))
	phs := make([]Placeholder, 0, len(w.doc.Placeholders))
	for _, p := range w.doc.Placeholders {
		name := nameByID[p.Identity]
		if name == "" {
			name = filepath.Base(p.Identity)
		}
		ph := Placeholder{
			ID:       p.ID,
			Identity: p.Identity,
			Name:     name,
			Location: p.Location,
		}
		phs = append(phs, ph)
		phByID[p.ID] = ph
	}

	boxes := make([]Box, 0, len(w.doc.Boxes))
	for _, b := range w.doc.Boxes {
		boxes = append(boxes, w.viewBox(b, phByID))
	}

	return Desk{
		Placeholders: phs,
		RecycleIcon:  SystemIcon{Location: w.doc.RecycleIcon},
		Boxes:        boxes,
	}
}

func (w *Workbench) viewBox(b index.BoxRecord, phByID map[string]Placeholder) Box {
	out := Box{
		ID:                  b.ID,
		Kind:                b.Kind,
		Tag:                 b.Tag,
		Title:               b.Title,
		X:                   b.X,
		Y:                   b.Y,
		W:                   b.W,
		H:                   b.H,
		ActiveCompartmentID: b.ActiveCompartmentID,
	}
	if b.Kind == BoxSimple {
		out.Items = placeholdersForIDs(b.ItemIDs, phByID)
		return out
	}
	out.Compartments = make([]Compartment, 0, len(b.Compartments))
	for _, c := range b.Compartments {
		out.Compartments = append(out.Compartments, Compartment{
			ID:    c.ID,
			Tag:   c.Tag,
			Items: placeholdersForIDs(c.ItemIDs, phByID),
		})
	}
	return out
}

func placeholdersForIDs(ids []string, phByID map[string]Placeholder) []Placeholder {
	out := make([]Placeholder, 0, len(ids))
	for _, id := range ids {
		if p, ok := phByID[id]; ok {
			out = append(out, p)
		}
	}
	return out
}

// MovePlaceholderToDesktop drops a skill 占位 onto a desktop grid cell.
// Icon → icon on the same cell creates a 普通盒子 named 「普通盒子N」 containing both.
// Empty cells (or recycle-only cells) place the mover on a free cell without stacking.
func (w *Workbench) MovePlaceholderToDesktop(placeholderID string, row, col int) error {
	if err := w.requireOpen(); err != nil {
		return err
	}
	if row < 1 || col < 1 {
		return fmt.Errorf("invalid grid cell (%d,%d)", row, col)
	}
	phIdx, ok := w.placeholderIndex(placeholderID)
	if !ok {
		return fmt.Errorf("unknown placeholder %q", placeholderID)
	}
	if w.doc.Placeholders[phIdx].Location.Kind == LocRecycle {
		return fmt.Errorf("cannot move placeholder in recycle")
	}

	// Icon → icon: another skill occupies this cell (excluding the mover).
	if occID, found := w.skillAtDesktopCell(row, col, placeholderID); found {
		return w.mergeIconsIntoAutoBox([]string{occID, placeholderID}, row, col)
	}

	// Park mover off desk lists while searching free cells.
	w.removePlaceholderFromContainers(placeholderID)
	w.doc.Placeholders[phIdx].Location = index.Location{Kind: LocDesktop, Row: -1, Col: -1}

	free := nextFreeCell(w.occupiedDesktopCells(), col, row)
	// Prefer the requested cell if free.
	if !w.occupiedDesktopCells()[cell{row, col}] {
		free = cell{row, col}
	}
	w.doc.Placeholders[phIdx].Location = index.Location{Kind: LocDesktop, Row: free.row, Col: free.col}
	return w.persist()
}

func (w *Workbench) requireOpen() error {
	if !w.opened {
		return fmt.Errorf("workbench not open; call Open first")
	}
	return nil
}

func (w *Workbench) placeholderIndex(id string) (int, bool) {
	for i, p := range w.doc.Placeholders {
		if p.ID == id {
			return i, true
		}
	}
	return -1, false
}

func (w *Workbench) boxIndex(id string) (int, bool) {
	for i, b := range w.doc.Boxes {
		if b.ID == id {
			return i, true
		}
	}
	return -1, false
}

// skillAtDesktopCell returns the placeholder id of a skill icon at (row,col), if any.
func (w *Workbench) skillAtDesktopCell(row, col int, excludePhID string) (string, bool) {
	for _, p := range w.doc.Placeholders {
		if p.ID == excludePhID {
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

// mergeIconsIntoAutoBox creates a sequenced 普通盒子 containing the given placeholders.
func (w *Workbench) mergeIconsIntoAutoBox(phIDs []string, nearRow, nearCol int) error {
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
		w.doc.Placeholders[idx].Location = index.Location{
			Kind:  LocBox,
			BoxID: boxID,
		}
		box.ItemIDs = append(box.ItemIDs, id)
	}
	w.doc.Boxes = append(w.doc.Boxes, box)
	return w.persist()
}

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

func (w *Workbench) newBoxID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("box_fallback_%d", len(w.doc.Boxes)+1)
	}
	return "box_" + hex.EncodeToString(b[:])
}

func (w *Workbench) newCompartmentID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("cmp_fallback_%d", len(w.doc.Boxes)+1)
	}
	return "cmp_" + hex.EncodeToString(b[:])
}

// MovePlaceholderToBox files a 占位 into a box's current 隔间/标签.
// For simple boxes, compartmentID is ignored. For composite boxes, empty
// compartmentID uses the active compartment.
func (w *Workbench) MovePlaceholderToBox(placeholderID, boxID, compartmentID string) error {
	if err := w.requireOpen(); err != nil {
		return err
	}
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
	var loc index.Location
	var targetSimple *[]string
	var targetCompItemIDs *[]string
	if box.Kind == BoxSimple {
		loc = index.Location{Kind: LocBox, BoxID: boxID}
		targetSimple = &box.ItemIDs
	} else {
		cid := compartmentID
		if cid == "" {
			cid = box.ActiveCompartmentID
		}
		cIdx := -1
		for i, c := range box.Compartments {
			if c.ID == cid {
				cIdx = i
				break
			}
		}
		if cIdx < 0 {
			return fmt.Errorf("unknown compartment %q", cid)
		}
		loc = index.Location{Kind: LocBox, BoxID: boxID, CompartmentID: cid}
		targetCompItemIDs = &box.Compartments[cIdx].ItemIDs
	}

	w.removePlaceholderFromContainers(placeholderID)
	if targetSimple != nil {
		if !containsID(*targetSimple, placeholderID) {
			*targetSimple = append(*targetSimple, placeholderID)
		}
	} else if !containsID(*targetCompItemIDs, placeholderID) {
		*targetCompItemIDs = append(*targetCompItemIDs, placeholderID)
	}
	w.doc.Placeholders[phIdx].Location = loc
	return w.persist()
}

func containsID(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// ComposeBoxes merges source into target:
//   - simple → simple: target becomes composite; both tags become 隔间; title from target tag
//   - simple → composite: append compartment from source
//   - composite → composite: refused
//   - composite → simple: refused (only simple may be the source)
func (w *Workbench) ComposeBoxes(sourceBoxID, targetBoxID string) error {
	if err := w.requireOpen(); err != nil {
		return err
	}
	if sourceBoxID == targetBoxID {
		return fmt.Errorf("cannot compose a box with itself")
	}
	sIdx, ok := w.boxIndex(sourceBoxID)
	if !ok {
		return fmt.Errorf("unknown source box %q", sourceBoxID)
	}
	tIdx, ok := w.boxIndex(targetBoxID)
	if !ok {
		return fmt.Errorf("unknown target box %q", targetBoxID)
	}
	src := w.doc.Boxes[sIdx]
	tgt := &w.doc.Boxes[tIdx]

	if src.Kind == BoxComposite && tgt.Kind == BoxComposite {
		return fmt.Errorf("composite → composite merge is not allowed")
	}
	if src.Kind != BoxSimple {
		return fmt.Errorf("compose requires simple source box")
	}

	if tgt.Kind == BoxSimple {
		return w.composeSimpleIntoSimple(sIdx, tIdx)
	}
	return w.addSimpleToComposite(sIdx, tIdx)
}

func (w *Workbench) composeSimpleIntoSimple(sIdx, tIdx int) error {
	src := w.doc.Boxes[sIdx]
	tgt := &w.doc.Boxes[tIdx]

	// Expanded composite geometry must not cover desktop icons — check before mutate.
	pos, ok := w.findBoxPosWithoutIconOverlap(tgt.X, tgt.Y, defaultCompositeBoxW, defaultCompositeBoxH, tgt.ID)
	if !ok {
		return fmt.Errorf("no free box position that avoids covering desktop skill icons")
	}

	c1 := index.CompartmentRecord{
		ID:      w.newCompartmentID(),
		Tag:     tgt.Tag,
		ItemIDs: append([]string(nil), tgt.ItemIDs...),
	}
	c2Tag := ensureUniqueTag([]string{c1.Tag}, src.Tag)
	c2 := index.CompartmentRecord{
		ID:      w.newCompartmentID(),
		Tag:     c2Tag,
		ItemIDs: append([]string(nil), src.ItemIDs...),
	}

	for _, phID := range c1.ItemIDs {
		if i, ok := w.placeholderIndex(phID); ok {
			w.doc.Placeholders[i].Location = index.Location{
				Kind: LocBox, BoxID: tgt.ID, CompartmentID: c1.ID,
			}
		}
	}
	for _, phID := range c2.ItemIDs {
		if i, ok := w.placeholderIndex(phID); ok {
			w.doc.Placeholders[i].Location = index.Location{
				Kind: LocBox, BoxID: tgt.ID, CompartmentID: c2.ID,
			}
		}
	}

	// Recycle icon in either box moves into first compartment of the composite.
	r := &w.doc.RecycleIcon
	if r.Kind == LocBox && (r.BoxID == src.ID || r.BoxID == tgt.ID) {
		r.BoxID = tgt.ID
		r.CompartmentID = c1.ID
		r.Row, r.Col = 0, 0
	}

	tgt.Kind = BoxComposite
	tgt.Title = tgt.Tag
	tgt.Tag = ""
	tgt.ItemIDs = nil
	tgt.Compartments = []index.CompartmentRecord{c1, c2}
	tgt.ActiveCompartmentID = c1.ID
	tgt.W = defaultCompositeBoxW
	tgt.H = defaultCompositeBoxH
	tgt.X, tgt.Y = pos.x, pos.y

	w.doc.Boxes = append(w.doc.Boxes[:sIdx], w.doc.Boxes[sIdx+1:]...)
	// Mutations on tgt are applied before delete; when sIdx < tIdx the element
	// shifts left but keeps those field values. Do not use tgt after this line.
	return w.persist()
}

func (w *Workbench) addSimpleToComposite(sIdx, tIdx int) error {
	src := w.doc.Boxes[sIdx]
	tgt := &w.doc.Boxes[tIdx]

	existing := make([]string, 0, len(tgt.Compartments))
	for _, c := range tgt.Compartments {
		existing = append(existing, c.Tag)
	}
	tag := ensureUniqueTag(existing, src.Tag)
	c := index.CompartmentRecord{
		ID:      w.newCompartmentID(),
		Tag:     tag,
		ItemIDs: append([]string(nil), src.ItemIDs...),
	}
	for _, phID := range c.ItemIDs {
		if i, ok := w.placeholderIndex(phID); ok {
			w.doc.Placeholders[i].Location = index.Location{
				Kind: LocBox, BoxID: tgt.ID, CompartmentID: c.ID,
			}
		}
	}
	r := &w.doc.RecycleIcon
	if r.Kind == LocBox && r.BoxID == src.ID {
		r.BoxID = tgt.ID
		r.CompartmentID = c.ID
		r.Row, r.Col = 0, 0
	}
	tgt.Compartments = append(tgt.Compartments, c)
	tgt.ActiveCompartmentID = c.ID

	w.doc.Boxes = append(w.doc.Boxes[:sIdx], w.doc.Boxes[sIdx+1:]...)
	return w.persist()
}

func ensureUniqueTag(used []string, tag string) string {
	set := make(map[string]bool, len(used))
	for _, t := range used {
		set[t] = true
	}
	if !set[tag] {
		return tag
	}
	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s-%d", tag, n)
		if !set[candidate] {
			return candidate
		}
	}
}

// MoveBox places a box at (x,y). The rectangle must not cover desktop Skill icons
// (or the recycle icon); the position is nudged to the nearest free soft-grid slot.
func (w *Workbench) MoveBox(boxID string, x, y float64) error {
	if err := w.requireOpen(); err != nil {
		return err
	}
	bIdx, ok := w.boxIndex(boxID)
	if !ok {
		return fmt.Errorf("unknown box %q", boxID)
	}
	box := &w.doc.Boxes[bIdx]
	pos, ok := w.findBoxPosWithoutIconOverlap(x, y, box.W, box.H, boxID)
	if !ok {
		return fmt.Errorf("box placement would cover desktop skill icons")
	}
	box.X, box.Y = pos.x, pos.y
	return w.persist()
}

// SetActiveCompartment switches the current 隔间 of a composite box.
func (w *Workbench) SetActiveCompartment(boxID, compartmentID string) error {
	if err := w.requireOpen(); err != nil {
		return err
	}
	bIdx, ok := w.boxIndex(boxID)
	if !ok {
		return fmt.Errorf("unknown box %q", boxID)
	}
	box := &w.doc.Boxes[bIdx]
	if box.Kind != BoxComposite {
		return fmt.Errorf("box %q is not composite", boxID)
	}
	for _, c := range box.Compartments {
		if c.ID == compartmentID {
			box.ActiveCompartmentID = compartmentID
			return w.persist()
		}
	}
	return fmt.Errorf("unknown compartment %q", compartmentID)
}

// EjectCompartment pulls a 隔间 out as a new 普通盒子. If the composite is left
// with one compartment, it demotes to simple immediately.
func (w *Workbench) EjectCompartment(compositeBoxID, compartmentID string, x, y float64) error {
	if err := w.requireOpen(); err != nil {
		return err
	}
	bIdx, ok := w.boxIndex(compositeBoxID)
	if !ok {
		return fmt.Errorf("unknown box %q", compositeBoxID)
	}
	box := &w.doc.Boxes[bIdx]
	if box.Kind != BoxComposite {
		return fmt.Errorf("box %q is not composite", compositeBoxID)
	}
	cIdx := -1
	for i, c := range box.Compartments {
		if c.ID == compartmentID {
			cIdx = i
			break
		}
	}
	if cIdx < 0 {
		return fmt.Errorf("unknown compartment %q", compartmentID)
	}

	// Placement first so a refuse leaves the composite unchanged.
	pos, ok := w.findBoxPosWithoutIconOverlap(x, y, defaultSimpleBoxW, defaultSimpleBoxH, "")
	if !ok {
		return fmt.Errorf("no free box position that avoids covering desktop skill icons")
	}

	comp := box.Compartments[cIdx]
	box.Compartments = append(box.Compartments[:cIdx], box.Compartments[cIdx+1:]...)

	newID := w.newBoxID()
	newBox := index.BoxRecord{
		ID:      newID,
		Kind:    BoxSimple,
		Tag:     comp.Tag,
		X:       pos.x,
		Y:       pos.y,
		W:       defaultSimpleBoxW,
		H:       defaultSimpleBoxH,
		ItemIDs: append([]string(nil), comp.ItemIDs...),
	}
	for _, phID := range newBox.ItemIDs {
		if i, ok := w.placeholderIndex(phID); ok {
			w.doc.Placeholders[i].Location = index.Location{Kind: LocBox, BoxID: newID}
		}
	}
	r := &w.doc.RecycleIcon
	if r.Kind == LocBox && r.BoxID == compositeBoxID && r.CompartmentID == compartmentID {
		r.BoxID = newID
		r.CompartmentID = ""
	}

	// Demote or remove residual composite before appending (avoids slice realloc
	// invalidating the residual box element while we still index it).
	if len(box.Compartments) == 1 {
		w.demoteCompositeIfSingle(bIdx)
	} else if len(box.Compartments) == 0 {
		w.doc.Boxes = append(w.doc.Boxes[:bIdx], w.doc.Boxes[bIdx+1:]...)
	} else if box.ActiveCompartmentID == compartmentID {
		box.ActiveCompartmentID = box.Compartments[0].ID
	}

	w.doc.Boxes = append(w.doc.Boxes, newBox)
	return w.persist()
}

func (w *Workbench) demoteCompositeIfSingle(bIdx int) {
	box := &w.doc.Boxes[bIdx]
	if box.Kind != BoxComposite || len(box.Compartments) != 1 {
		return
	}
	last := box.Compartments[0]
	box.Kind = BoxSimple
	box.Tag = last.Tag
	box.Title = ""
	box.ItemIDs = append([]string(nil), last.ItemIDs...)
	box.Compartments = nil
	box.ActiveCompartmentID = ""
	box.W = defaultSimpleBoxW
	box.H = defaultSimpleBoxH
	for _, phID := range box.ItemIDs {
		if i, ok := w.placeholderIndex(phID); ok {
			w.doc.Placeholders[i].Location = index.Location{Kind: LocBox, BoxID: box.ID}
		}
	}
	r := &w.doc.RecycleIcon
	if r.Kind == LocBox && r.BoxID == box.ID {
		r.CompartmentID = ""
	}
}

// RenameBoxTag renames a simple box tag, or a composite compartment tag when
// compartmentID is set.
func (w *Workbench) RenameBoxTag(boxID, newTag, compartmentID string) error {
	if err := w.requireOpen(); err != nil {
		return err
	}
	tag := strings.TrimSpace(newTag)
	if tag == "" {
		return fmt.Errorf("tag must not be empty")
	}
	bIdx, ok := w.boxIndex(boxID)
	if !ok {
		return fmt.Errorf("unknown box %q", boxID)
	}
	box := &w.doc.Boxes[bIdx]
	if box.Kind == BoxSimple {
		box.Tag = tag
		return w.persist()
	}
	if compartmentID == "" {
		return fmt.Errorf("composite rename requires compartment id")
	}
	cIdx := -1
	others := make([]string, 0, len(box.Compartments))
	for i, c := range box.Compartments {
		if c.ID == compartmentID {
			cIdx = i
			continue
		}
		others = append(others, c.Tag)
	}
	if cIdx < 0 {
		return fmt.Errorf("unknown compartment %q", compartmentID)
	}
	box.Compartments[cIdx].Tag = ensureUniqueTag(others, tag)
	return w.persist()
}

// RenameBoxTitle renames a composite box's 盒标题.
func (w *Workbench) RenameBoxTitle(boxID, title string) error {
	if err := w.requireOpen(); err != nil {
		return err
	}
	bIdx, ok := w.boxIndex(boxID)
	if !ok {
		return fmt.Errorf("unknown box %q", boxID)
	}
	box := &w.doc.Boxes[bIdx]
	if box.Kind != BoxComposite {
		return fmt.Errorf("box %q is not composite", boxID)
	}
	box.Title = strings.TrimSpace(title)
	return w.persist()
}

// DeleteBox removes a box and returns all contained placeholders (and recycle icon
// if inside) to free desktop grid cells. Skill bodies are never deleted.
func (w *Workbench) DeleteBox(boxID string) error {
	if err := w.requireOpen(); err != nil {
		return err
	}
	bIdx, ok := w.boxIndex(boxID)
	if !ok {
		return fmt.Errorf("unknown box %q", boxID)
	}
	box := w.doc.Boxes[bIdx]

	ids := make([]string, 0)
	if box.Kind == BoxSimple {
		ids = append(ids, box.ItemIDs...)
	} else {
		for _, c := range box.Compartments {
			ids = append(ids, c.ItemIDs...)
		}
	}

	// Remove box first so free-cell search ignores it (geometry only; cells are icon-only).
	w.doc.Boxes = append(w.doc.Boxes[:bIdx], w.doc.Boxes[bIdx+1:]...)

	occupied := w.occupiedDesktopCells()
	// Prefer cells near the former box position.
	startRow := int(box.Y-iconGridOriginY)/iconGridCellH + 1
	startCol := int(box.X-iconGridOriginX)/iconGridCellW + 1
	if startRow < 1 {
		startRow = 1
	}
	if startCol < 1 {
		startCol = 1
	}
	for _, phID := range ids {
		idx, ok := w.placeholderIndex(phID)
		if !ok {
			continue
		}
		free := nextFreeCell(occupied, startCol, startRow)
		occupied[free] = true
		w.doc.Placeholders[idx].Location = index.Location{
			Kind: LocDesktop, Row: free.row, Col: free.col,
		}
		// Spread subsequent icons along the column.
		startRow = free.row + 1
	}

	r := &w.doc.RecycleIcon
	if r.Kind == LocBox && r.BoxID == boxID {
		free := nextFreeCell(occupied, startCol, startRow)
		r.Kind = LocDesktop
		r.Row, r.Col = free.row, free.col
		r.BoxID, r.CompartmentID = "", ""
	}

	return w.persist()
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
		cell := nextFreeCell(occupied, 1, 2)
		occupied[cell] = true
		id := w.newPlaceholderID()
		w.doc.Placeholders = append(w.doc.Placeholders, index.PlaceholderRecord{
			ID:       id,
			Identity: s.Identity,
			Location: index.Location{Kind: LocDesktop, Row: cell.row, Col: cell.col},
		})
		have[s.Identity] = true
	}
	if w.doc.SchemaVersion == 0 {
		w.doc.SchemaVersion = index.SchemaVersion
	}
	return nil
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
// Recycle and skill placeholders both occupy cells; no two skill icons share a cell.
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

func (w *Workbench) newPlaceholderID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// Extremely unlikely; fall back to a process-local counter-like value.
		return fmt.Sprintf("ph_fallback_%d", len(w.doc.Placeholders)+1)
	}
	return "ph_" + hex.EncodeToString(b[:])
}

func (w *Workbench) persist() error {
	if err := w.store.Save(w.doc); err != nil {
		return fmt.Errorf("save index: %w", err)
	}
	return nil
}

// DefaultScanRoots returns sensible user-level and project-level skill paths.
// Bundled/system trees are not included.
func DefaultScanRoots(home, projectRoot string) []string {
	var roots []string
	userDirs := []string{
		".agents/skills",
		".claude/skills",
		".codex/skills",
		".grok/skills",
	}
	for _, rel := range userDirs {
		if home != "" {
			roots = append(roots, filepath.Join(home, rel))
		}
	}
	projectDirs := []string{
		".agents/skills",
		".claude/skills",
		".codex/skills",
		".grok/skills",
	}
	for _, rel := range projectDirs {
		if projectRoot != "" {
			roots = append(roots, filepath.Join(projectRoot, rel))
		}
	}
	return roots
}

// DefaultIndexPath returns the user-level 中央索引 path (e.g. under XDG config).
func DefaultIndexPath(configHome string) string {
	return index.DefaultPath(configHome)
}
