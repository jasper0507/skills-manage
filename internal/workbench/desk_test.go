package workbench_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/jasper0507/skills-manage/internal/infra/index"
	"github.com/jasper0507/skills-manage/internal/workbench"
)

func TestDesk_DefaultLayout_RecycleAt11_RowMajorWithinViewport(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, filepath.Join(root, "alpha"), "alpha")
	writeSkill(t, filepath.Join(root, "beta"), "beta")

	store := index.NewMemoryStore()
	wb := newWB(t, []string{root}, store)
	if err := wb.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}

	desk := wb.Desk()

	if desk.RecycleIcon.Location.Kind != workbench.LocDesktop {
		t.Fatalf("recycle kind = %q, want desktop", desk.RecycleIcon.Location.Kind)
	}
	if desk.RecycleIcon.Location.Row != 1 || desk.RecycleIcon.Location.Col != 1 {
		t.Errorf("recycle at (%d,%d), want (1,1)", desk.RecycleIcon.Location.Row, desk.RecycleIcon.Location.Col)
	}

	if len(desk.Placeholders) != 2 {
		t.Fatalf("got %d placeholders, want 2: %+v", len(desk.Placeholders), desk.Placeholders)
	}

	// Row-major one-screen fill: recycle occupies (1,1); skills take (1,2), (1,3), …
	// Not a tall first-column stack.
	byName := map[string]workbench.Placeholder{}
	for _, p := range desk.Placeholders {
		byName[p.Name] = p
		if p.Location.Kind != workbench.LocDesktop {
			t.Errorf("%s location kind = %q, want desktop", p.Name, p.Location.Kind)
		}
		if p.Location.Row < 1 || p.Location.Col < 1 {
			t.Errorf("%s invalid cell %+v", p.Name, p.Location)
		}
		if p.Location.Row > workbench.DefaultViewportRows || p.Location.Col > workbench.DefaultViewportCols {
			t.Errorf("%s at (%d,%d) outside default viewport %dx%d",
				p.Name, p.Location.Row, p.Location.Col,
				workbench.DefaultViewportRows, workbench.DefaultViewportCols)
		}
	}
	// Two free cells after recycle: (1,2) and (1,3) in scan order.
	wantCells := map[string]struct{ row, col int }{
		"alpha": {1, 2},
		"beta":  {1, 3},
	}
	for name, want := range wantCells {
		got := byName[name].Location
		if got.Row != want.row || got.Col != want.col {
			t.Errorf("%s at (%d,%d), want (%d,%d) row-major after recycle",
				name, got.Row, got.Col, want.row, want.col)
		}
	}
	// Must not stack both in column 1.
	if byName["alpha"].Location.Col == 1 && byName["beta"].Location.Col == 1 {
		t.Error("both skills in col 1; default layout must fill across the row")
	}
}

func TestDesk_DefaultLayout_FillsAcrossNotOnlyColumn1(t *testing.T) {
	root := t.TempDir()
	// More skills than one row of leftover cells after recycle: cols 2..12 = 11 cells on row 1.
	names := make([]string, 0, 14)
	for i := 0; i < 14; i++ {
		n := fmt.Sprintf("s%02d", i)
		names = append(names, n)
		writeSkill(t, filepath.Join(root, n), n)
	}
	wb := newWB(t, []string{root}, index.NewMemoryStore())
	if err := wb.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	desk := wb.Desk()
	if len(desk.Placeholders) != 14 {
		t.Fatalf("got %d placeholders, want 14", len(desk.Placeholders))
	}
	cols := map[int]int{}
	rows := map[int]int{}
	for _, p := range desk.Placeholders {
		cols[p.Location.Col]++
		rows[p.Location.Row]++
		if p.Location.Col > workbench.DefaultViewportCols {
			t.Errorf("%s col %d > viewport", p.Name, p.Location.Col)
		}
	}
	if len(cols) < 2 {
		t.Fatalf("used only cols %v; want multi-column fill", cols)
	}
	if len(rows) < 2 {
		t.Fatalf("used only rows %v; 14 skills should wrap to row 2+", rows)
	}
}

func TestDesk_PlaceholdersReferenceSkillIdentity(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "named-skill")
	writeSkill(t, skillDir, "named-skill")
	wantID := mustRealpath(t, skillDir)

	wb := newWB(t, []string{root}, index.NewMemoryStore())
	if err := wb.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	desk := wb.Desk()
	if len(desk.Placeholders) != 1 {
		t.Fatalf("got %d placeholders, want 1", len(desk.Placeholders))
	}
	got := desk.Placeholders[0]
	if got.Identity != wantID {
		t.Errorf("Identity = %q, want realpath %q", got.Identity, wantID)
	}
	// Placeholder is a shortcut ref, not a copied path under the index store.
	if got.Identity == "" {
		t.Error("Identity must not be empty")
	}
}

func TestDesk_RestartRestoresSameLayout(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, filepath.Join(root, "keep-me"), "keep-me")
	writeSkill(t, filepath.Join(root, "also"), "also")

	store := index.NewMemoryStore()
	wb1 := newWB(t, []string{root}, store)
	if err := wb1.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	before := wb1.Desk()

	// Simulate restart: new Workbench, same central index store.
	wb2 := newWB(t, []string{root}, store)
	if err := wb2.Open(); err != nil {
		t.Fatalf("re-Open: %v", err)
	}
	after := wb2.Desk()

	if after.RecycleIcon.Location != before.RecycleIcon.Location {
		t.Errorf("recycle moved: before %+v after %+v", before.RecycleIcon.Location, after.RecycleIcon.Location)
	}
	if len(after.Placeholders) != len(before.Placeholders) {
		t.Fatalf("placeholder count %d → %d", len(before.Placeholders), len(after.Placeholders))
	}
	beforeByID := map[string]workbench.Placeholder{}
	for _, p := range before.Placeholders {
		beforeByID[p.Identity] = p
	}
	for _, p := range after.Placeholders {
		prev, ok := beforeByID[p.Identity]
		if !ok {
			t.Errorf("unexpected placeholder identity %q", p.Identity)
			continue
		}
		if p.Location != prev.Location {
			t.Errorf("%s location changed: %+v → %+v", p.Identity, prev.Location, p.Location)
		}
		if p.ID != prev.ID {
			t.Errorf("%s id changed: %q → %q (ids should be stable across restart)", p.Identity, prev.ID, p.ID)
		}
	}
}

func TestDesk_FileIndex_RestartRestoresDesk(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, filepath.Join(root, "persist-skill"), "persist-skill")
	indexPath := filepath.Join(t.TempDir(), "skills-manage", "index.json")
	store := index.NewFileStore(indexPath)

	wb1 := newWB(t, []string{root}, store)
	if err := wb1.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	before := wb1.Desk()
	if len(before.Placeholders) != 1 {
		t.Fatalf("want 1 placeholder, got %d", len(before.Placeholders))
	}

	wb2 := newWB(t, []string{root}, index.NewFileStore(indexPath))
	if err := wb2.Open(); err != nil {
		t.Fatalf("re-Open: %v", err)
	}
	after := wb2.Desk()
	if after.Placeholders[0].Location != before.Placeholders[0].Location {
		t.Errorf("location not restored: %+v → %+v", before.Placeholders[0].Location, after.Placeholders[0].Location)
	}
	if after.Placeholders[0].Identity != before.Placeholders[0].Identity {
		t.Errorf("identity not restored")
	}
	if after.RecycleIcon.Location.Row != 1 || after.RecycleIcon.Location.Col != 1 {
		t.Errorf("recycle not restored at (1,1): %+v", after.RecycleIcon.Location)
	}
}

func TestDesk_Rescan_PreservesPlacement_OnlyNewSkillsGetFreeCells(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, filepath.Join(root, "old-a"), "old-a")
	writeSkill(t, filepath.Join(root, "old-b"), "old-b")

	store := index.NewMemoryStore()
	wb := newWB(t, []string{root}, store)
	if err := wb.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	// User-like: pretend old-a was moved to a non-default cell by rewriting index.
	// We only mutate via store to avoid needing Move API (ticket #4 territory).
	doc, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for i := range doc.Placeholders {
		if filepath.Base(doc.Placeholders[i].Identity) == "old-a" {
			doc.Placeholders[i].Location = index.Location{Kind: workbench.LocDesktop, Row: 5, Col: 3}
		}
	}
	if err := store.Save(doc); err != nil {
		t.Fatal(err)
	}
	// Reload desk from mutated index.
	wb = newWB(t, []string{root}, store)
	if err := wb.Open(); err != nil {
		t.Fatalf("Open after manual move: %v", err)
	}
	mid := wb.Desk()
	var oldARow, oldACol int
	for _, p := range mid.Placeholders {
		if p.Name == "old-a" {
			oldARow, oldACol = p.Location.Row, p.Location.Col
		}
	}
	if oldARow != 5 || oldACol != 3 {
		t.Fatalf("setup: old-a at (%d,%d), want (5,3)", oldARow, oldACol)
	}

	// Brand-new skill appears on disk.
	writeSkill(t, filepath.Join(root, "brand-new"), "brand-new")
	if err := wb.Rescan(); err != nil {
		t.Fatalf("Rescan: %v", err)
	}
	after := wb.Desk()

	byName := map[string]workbench.Placeholder{}
	for _, p := range after.Placeholders {
		byName[p.Name] = p
	}
	if len(byName) != 3 {
		t.Fatalf("got %d placeholders, want 3: %v", len(byName), byName)
	}
	if byName["old-a"].Location.Row != 5 || byName["old-a"].Location.Col != 3 {
		t.Errorf("old-a moved on rescan: %+v", byName["old-a"].Location)
	}
	// old-b must keep its pre-rescan cell.
	for _, p := range mid.Placeholders {
		if p.Name == "old-b" {
			if byName["old-b"].Location != p.Location {
				t.Errorf("old-b moved: %+v → %+v", p.Location, byName["old-b"].Location)
			}
		}
	}
	neu := byName["brand-new"]
	if neu.Location.Kind != workbench.LocDesktop {
		t.Errorf("brand-new kind = %q", neu.Location.Kind)
	}
	// New skill must not land on occupied cells.
	occupied := map[string]bool{
		"5,3": true, // old-a
	}
	for _, p := range mid.Placeholders {
		if p.Name == "old-b" {
			occupied[fmtCell(p.Location.Row, p.Location.Col)] = true
		}
	}
	if mid.RecycleIcon.Location.Kind == workbench.LocDesktop {
		occupied[fmtCell(mid.RecycleIcon.Location.Row, mid.RecycleIcon.Location.Col)] = true
	}
	key := fmtCell(neu.Location.Row, neu.Location.Col)
	if occupied[key] {
		t.Errorf("brand-new placed on occupied cell %s", key)
	}
}

func TestDesk_NoTwoSkillPlaceholdersShareCell(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"s1", "s2", "s3", "s4", "s5"} {
		writeSkill(t, filepath.Join(root, name), name)
	}
	wb := newWB(t, []string{root}, index.NewMemoryStore())
	if err := wb.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	desk := wb.Desk()
	seen := map[string]string{} // cell -> name
	for _, p := range desk.Placeholders {
		if p.Location.Kind != workbench.LocDesktop {
			continue
		}
		key := fmtCell(p.Location.Row, p.Location.Col)
		if other, ok := seen[key]; ok {
			t.Errorf("cell %s shared by %q and %q", key, other, p.Name)
		}
		seen[key] = p.Name
	}
	// Recycle occupies (1,1); no skill may sit there.
	rkey := fmtCell(desk.RecycleIcon.Location.Row, desk.RecycleIcon.Location.Col)
	if name, ok := seen[rkey]; ok {
		t.Errorf("skill %q shares recycle cell %s", name, rkey)
	}
}

func TestDesk_Rescan_PreservesBoxesMetadata(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, filepath.Join(root, "only"), "only")
	store := index.NewMemoryStore()

	// Seed index with a box record (future ticket data) and open.
	seed := index.Document{
		SchemaVersion: index.SchemaVersion,
		Placeholders:  nil,
		RecycleIcon:   index.Location{Kind: workbench.LocDesktop, Row: 1, Col: 1},
		Boxes: []index.BoxRecord{
			{ID: "box_seed", Kind: "simple", Tag: "design", X: 320, Y: 32, W: 240, H: 220},
		},
		BoxNameSeq: 3,
	}
	if err := store.Save(seed); err != nil {
		t.Fatal(err)
	}

	wb := newWB(t, []string{root}, store)
	if err := wb.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	writeSkill(t, filepath.Join(root, "extra"), "extra")
	if err := wb.Rescan(); err != nil {
		t.Fatalf("Rescan: %v", err)
	}

	doc, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Boxes) != 1 || doc.Boxes[0].ID != "box_seed" || doc.Boxes[0].Tag != "design" {
		t.Errorf("boxes metadata not preserved: %+v", doc.Boxes)
	}
	if doc.BoxNameSeq != 3 {
		t.Errorf("boxNameSeq = %d, want 3", doc.BoxNameSeq)
	}
}
