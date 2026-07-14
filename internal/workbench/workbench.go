// Package workbench is the primary application facade for skills-manage.
// Callers (CLI, HTTP, tests) speak only to Workbench for product behavior.
package workbench

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/jasper0507/skills-manage/internal/index"
	"github.com/jasper0507/skills-manage/internal/quarantine"
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
	Identity string `json:"identity"` // Skill 身份: normalized realpath
	Name     string `json:"name"`     // display name (frontmatter name, else directory name)
}

// Inventory is the live 清单 of Skills discovered from 扫描根.
type Inventory struct {
	Skills []Skill `json:"skills"`
}

// Placeholder is a 占位 shortcut icon on the desk (not a file copy).
type Placeholder struct {
	ID       string   `json:"id"`
	Identity string   `json:"identity"` // Skill 身份 (realpath)
	Name     string   `json:"name"`     // display name from latest inventory when known
	Location Location `json:"location"`
}

// SystemIcon is a non-skill desk icon (e.g. 回收站).
type SystemIcon struct {
	Location Location `json:"location"`
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
	ID    string        `json:"id"`
	Tag   string        `json:"tag"`
	Items []Placeholder `json:"items"`
}

// Box is a 普通盒子 or 组合盒子 on the desk.
type Box struct {
	ID                  string        `json:"id"`
	Kind                string        `json:"kind"`  // simple | composite
	Tag                 string        `json:"tag"`   // simple: single tag / display name
	Title               string        `json:"title"` // composite: 盒标题
	X                   float64       `json:"x"`
	Y                   float64       `json:"y"`
	W                   float64       `json:"w"`
	H                   float64       `json:"h"`
	Items               []Placeholder `json:"items,omitempty"` // simple box contents as icons
	Compartments        []Compartment `json:"compartments,omitempty"`
	ActiveCompartmentID string        `json:"activeCompartmentId,omitempty"`
}

// Clipboard modes (Windows-style copy vs cut).
const (
	ClipCopy = "copy"
	ClipCut  = "cut"
)

// Clipboard holds session copy/cut targets (placeholder ids). Not persisted in the index.
type Clipboard struct {
	Mode           string   `json:"mode"` // ClipCopy | ClipCut
	PlaceholderIDs []string `json:"placeholderIds"`
}

// Desk is the external desktop view: placeholders + recycle system icon + boxes.
type Desk struct {
	Placeholders []Placeholder `json:"placeholders"`
	RecycleIcon  SystemIcon    `json:"recycleIcon"`
	Boxes        []Box         `json:"boxes"`
	Clipboard    *Clipboard    `json:"clipboard"` // nil when empty
	MultiSelect  bool          `json:"multiSelect"`
	SelectedIDs  []string      `json:"selectedIds"`
}

// Retention is how long quarantined skills remain restorable before purge-due.
const RecycleRetention = 30 * 24 * time.Hour

// Config configures a Workbench.
type Config struct {
	// ScanRoots are filesystem roots to walk for skill packages.
	// Empty means scan nothing (CLI fills defaults before constructing Workbench).
	ScanRoots []string

	// Scanner discovers packages under scan roots. Nil uses the default filesystem scanner.
	Scanner scanner.Scanner

	// Index is the 中央索引 store. Nil uses an in-memory store (ephemeral).
	Index index.Store

	// Quarantine isolates skill packages on body-delete. Nil uses the real FS adapter.
	Quarantine quarantine.Adapter

	// Clock returns "now" for 30-day retention. Nil uses time.Now.
	Clock func() time.Time
}

// Workbench is the sole primary product seam.
type Workbench struct {
	scanRoots []string
	scan      scanner.Scanner
	store     index.Store
	q         quarantine.Adapter
	clock     func() time.Time

	doc    index.Document
	opened bool

	// Session-only UI state (not written to the 中央索引).
	clipboard   *Clipboard
	multiSelect bool
	selectedIDs []string
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
	q := cfg.Quarantine
	if q == nil {
		q = quarantine.New()
	}
	clock := cfg.Clock
	if clock == nil {
		clock = time.Now
	}
	return &Workbench{
		scanRoots: append([]string(nil), cfg.ScanRoots...),
		scan:      sc,
		store:     st,
		q:         q,
		clock:     clock,
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
// runs due purge for expired quarantine entries, and persists the desk.
// Safe to call once at session start; subsequent sessions call Open again.
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
	// Mark open before PurgeDue (which requires an open workbench).
	w.opened = true
	if err := w.purgeDueNoPersist(); err != nil {
		w.opened = false
		return err
	}
	if err := w.persist(); err != nil {
		w.opened = false
		return err
	}
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

	var clip *Clipboard
	if w.clipboard != nil && len(w.clipboard.PlaceholderIDs) > 0 {
		clip = &Clipboard{
			Mode:           w.clipboard.Mode,
			PlaceholderIDs: append([]string(nil), w.clipboard.PlaceholderIDs...),
		}
	}
	sel := append([]string(nil), w.selectedIDs...)
	if sel == nil {
		sel = []string{}
	}

	return Desk{
		Placeholders: phs,
		RecycleIcon:  SystemIcon{Location: w.doc.RecycleIcon},
		Boxes:        boxes,
		Clipboard:    clip,
		MultiSelect:  w.multiSelect,
		SelectedIDs:  sel,
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
	return w.MovePlaceholdersToDesktop([]string{placeholderID}, row, col)
}

// MovePlaceholdersToDesktop drops one or more 占位 onto the desktop grid (multi-select).
// If the target cell holds a non-mover skill, all movers merge with it into a 普通盒子.
// Otherwise each mover is parked then placed into free cells starting at (row, col).
func (w *Workbench) MovePlaceholdersToDesktop(placeholderIDs []string, row, col int) error {
	if err := w.requireOpen(); err != nil {
		return err
	}
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
		return w.mergeIconsIntoAutoBox(all, row, col)
	}

	// Park movers so they do not block free-cell search.
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
	if err := w.movePlaceholderToBoxNoPersist(placeholderID, boxID, compartmentID); err != nil {
		return err
	}
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
// Empty tag defaults to 「新建」.
func (w *Workbench) CreateSimpleBox(tag string, x, y float64) (string, error) {
	if err := w.requireOpen(); err != nil {
		return "", err
	}
	tag = strings.TrimSpace(tag)
	if tag == "" {
		tag = "新建"
	}
	pos, ok := w.findBoxPosWithoutIconOverlap(x, y, defaultSimpleBoxW, defaultSimpleBoxH, "")
	if !ok {
		return "", fmt.Errorf("no free box position that avoids covering desktop skill icons")
	}
	id := w.newBoxID()
	w.doc.Boxes = append(w.doc.Boxes, index.BoxRecord{
		ID:   id,
		Kind: BoxSimple,
		Tag:  tag,
		X:    pos.x,
		Y:    pos.y,
		W:    defaultSimpleBoxW,
		H:    defaultSimpleBoxH,
	})
	if err := w.persist(); err != nil {
		return "", err
	}
	return id, nil
}

// CreateCompositeBox places an empty 组合盒子 with the given title and compartment tags.
// A single compartment demotes immediately to a 普通盒子 (product rule).
// Empty title defaults to 「组合盒」; empty tags default to [「默认」].
func (w *Workbench) CreateCompositeBox(title string, tags []string, x, y float64) (string, error) {
	if err := w.requireOpen(); err != nil {
		return "", err
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = "组合盒"
	}
	clean := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t != "" {
			clean = append(clean, t)
		}
	}
	if len(clean) == 0 {
		clean = []string{"默认"}
	}

	// Single compartment → demote to simple immediately.
	if len(clean) == 1 {
		return w.CreateSimpleBox(clean[0], x, y)
	}

	pos, ok := w.findBoxPosWithoutIconOverlap(x, y, defaultCompositeBoxW, defaultCompositeBoxH, "")
	if !ok {
		return "", fmt.Errorf("no free box position that avoids covering desktop skill icons")
	}

	comps := make([]index.CompartmentRecord, 0, len(clean))
	used := make([]string, 0, len(clean))
	for _, t := range clean {
		tag := ensureUniqueTag(used, t)
		used = append(used, tag)
		comps = append(comps, index.CompartmentRecord{
			ID:  w.newCompartmentID(),
			Tag: tag,
		})
	}
	id := w.newBoxID()
	w.doc.Boxes = append(w.doc.Boxes, index.BoxRecord{
		ID:                  id,
		Kind:                BoxComposite,
		Title:               title,
		X:                   pos.x,
		Y:                   pos.y,
		W:                   defaultCompositeBoxW,
		H:                   defaultCompositeBoxH,
		Compartments:        comps,
		ActiveCompartmentID: comps[0].ID,
	})
	if err := w.persist(); err != nil {
		return "", err
	}
	return id, nil
}

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

	var plan TrashPlan
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
			PlaceholderIDs: liveIDs,
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
	free := nextFreeCell(occupied, 1, 2)
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
