package workbench

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"path/filepath"

	"github.com/jasper0507/skills-manage/internal/infra/index"
	"github.com/jasper0507/skills-manage/internal/infra/scanner"
)

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

// Open loads the central index, normalizes legacy body-delete recycle metadata,
// repairs membership/placement (rehome once on load), reconciles with a fresh scan
// (places only brand-new skills), and persists the desk.
// Does not run package purge, isolate, or any Skill-package lifecycle.
// Safe to call once at session start; subsequent sessions call Open again.
func (w *Workbench) Open() error {
	doc, err := w.store.Load()
	if err != nil {
		return fmt.Errorf("load index: %w", err)
	}
	w.doc = doc
	w.normalizeLegacyIndex()
	// Recycle icon default before free-cell repair so ghosts do not land on (1,1).
	w.ensureRecycleDefault()
	// Load-only repair: membership truth + free-cell for ghosts (not a write-path net).
	w.rehomeFromMembership()
	if err := w.reconcileFromScan(); err != nil {
		return err
	}
	w.opened = true
	if err := w.persist(); err != nil {
		w.opened = false
		return err
	}
	return nil
}

// normalizeLegacyIndex makes an on-disk document from tickets #2–#7 safe for R2:
// drop body-delete RecycleEntry rows (quarantine path, purge-after, states) and
// keep kind=recycle placeholders as the only icon-bin members. No filesystem ops.
func (w *Workbench) normalizeLegacyIndex() {
	// Product recycle is placeholders with LocRecycle — never the body lifecycle table.
	w.doc.RecycleBin = nil

	// Normalize recycle locations: bin members are kind-only (no desktop cell / box coords).
	for i := range w.doc.Placeholders {
		if w.doc.Placeholders[i].Location.Kind != LocRecycle {
			continue
		}
		w.setRecyclePlacement(i)
	}
}

// Rescan re-discovers inventory and places only skills that have no placeholder yet.
// Existing coordinates and stored box metadata are left unchanged.
func (w *Workbench) Rescan() error {
	if !w.opened {
		return w.Open()
	}
	return w.withMutation(func() error {
		return w.reconcileFromScan()
	})
}

// Desk returns the current desktop view (placeholders + recycle icon + boxes).
// Names come from the last Open/Rescan inventory snapshot in the index.
// Box contents are always exposed as icon placeholders.
// In-box Location is projected from membership (ItemIDs); the index document
// does not store parallel LocBox for members (E3.1 / ADR-0002).
func (w *Workbench) Desk() Desk {
	nameByID := make(map[string]string, len(w.doc.Skills))
	for _, s := range w.doc.Skills {
		nameByID[s.Identity] = s.Name
	}

	membership := w.membershipByPlaceholder()
	phByID := make(map[string]Placeholder, len(w.doc.Placeholders))
	phs := make([]Placeholder, 0, len(w.doc.Placeholders))
	for _, p := range w.doc.Placeholders {
		name := nameByID[p.Identity]
		if name == "" {
			name = filepath.Base(p.Identity)
		}
		m, isMember := membership[p.ID]
		ph := Placeholder{
			ID:       p.ID,
			Identity: p.Identity,
			Name:     name,
			Location: projectLocation(p.Location, m, isMember),
		}
		phs = append(phs, ph)
		phByID[p.ID] = ph
	}

	boxes := make([]Box, 0, len(w.doc.Boxes))
	for _, b := range w.doc.Boxes {
		boxes = append(boxes, w.viewBox(b, phByID, membership))
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

func (w *Workbench) viewBox(b index.BoxRecord, phByID map[string]Placeholder, membership map[string]boxMembership) Box {
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
		out.Items = placeholdersForContainer(b.ID, "", b.ItemIDs, phByID, membership)
		return out
	}
	out.Compartments = make([]Compartment, 0, len(b.Compartments))
	for _, c := range b.Compartments {
		out.Compartments = append(out.Compartments, Compartment{
			ID:    c.ID,
			Tag:   c.Tag,
			Items: placeholdersForContainer(b.ID, c.ID, c.ItemIDs, phByID, membership),
		})
	}
	return out
}

// placeholdersForContainer lists items that have a valid membership claim for this
// box/compartment (same filters as buildMembershipClaims — skips recycle/dup/unknown).
func placeholdersForContainer(
	boxID, compartmentID string,
	ids []string,
	phByID map[string]Placeholder,
	membership map[string]boxMembership,
) []Placeholder {
	out := make([]Placeholder, 0, len(ids))
	for _, id := range ids {
		m, ok := membership[id]
		if !ok || m.boxID != boxID || m.compartmentID != compartmentID {
			continue
		}
		if p, ok := phByID[id]; ok {
			out = append(out, p)
		}
	}
	return out
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


