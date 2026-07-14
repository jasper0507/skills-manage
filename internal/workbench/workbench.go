// Package workbench is the primary application facade for skills-manage.
// Callers (CLI, HTTP, tests) speak only to Workbench for product behavior.
package workbench

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"

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

// Desk is the external desktop view: placeholders + recycle system icon.
type Desk struct {
	Placeholders []Placeholder
	RecycleIcon  SystemIcon
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

// Desk returns the current desktop view (placeholders + recycle icon).
// Names come from the last Open/Rescan inventory snapshot in the index.
func (w *Workbench) Desk() Desk {
	nameByID := make(map[string]string, len(w.doc.Skills))
	for _, s := range w.doc.Skills {
		nameByID[s.Identity] = s.Name
	}

	phs := make([]Placeholder, 0, len(w.doc.Placeholders))
	for _, p := range w.doc.Placeholders {
		name := nameByID[p.Identity]
		if name == "" {
			name = filepath.Base(p.Identity)
		}
		phs = append(phs, Placeholder{
			ID:       p.ID,
			Identity: p.Identity,
			Name:     name,
			Location: p.Location,
		})
	}
	return Desk{
		Placeholders: phs,
		RecycleIcon:  SystemIcon{Location: w.doc.RecycleIcon},
	}
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
