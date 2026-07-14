package workbench_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jasper0507/skills-manage/internal/index"
	"github.com/jasper0507/skills-manage/internal/workbench"
)

func TestTrash_NonLastPlaceholder_RemovesIconOnly_SkillStaysLive(t *testing.T) {
	wb, _ := openDeskWithSkills(t, "alpha")
	desk := wb.Desk()
	alpha := phByName(t, desk, "alpha")
	identity := alpha.Identity

	// Second placeholder for the same skill (copy).
	if err := wb.SetClipboard(workbench.ClipCopy, []string{alpha.ID}); err != nil {
		t.Fatalf("SetClipboard: %v", err)
	}
	if err := wb.PasteToDesktop(5, 2); err != nil {
		t.Fatalf("PasteToDesktop: %v", err)
	}
	desk = wb.Desk()
	if len(desk.Placeholders) != 2 {
		t.Fatalf("precondition: want 2 placeholders, got %d", len(desk.Placeholders))
	}
	var copyID string
	for _, p := range desk.Placeholders {
		if p.ID != alpha.ID {
			copyID = p.ID
			break
		}
	}

	plan, err := wb.PlanTrash([]string{copyID})
	if err != nil {
		t.Fatalf("PlanTrash: %v", err)
	}
	if len(plan.IconOnlyIDs) != 1 || plan.IconOnlyIDs[0] != copyID {
		t.Fatalf("plan.IconOnlyIDs = %v, want [%s]", plan.IconOnlyIDs, copyID)
	}
	if len(plan.BodyItems) != 0 {
		t.Fatalf("plan.BodyItems = %+v, want empty (non-last)", plan.BodyItems)
	}

	if err := wb.ConfirmTrash([]string{copyID}); err != nil {
		t.Fatalf("ConfirmTrash: %v", err)
	}

	desk = wb.Desk()
	if len(desk.Placeholders) != 1 {
		t.Fatalf("got %d placeholders, want 1 (icon only): %+v", len(desk.Placeholders), desk.Placeholders)
	}
	if desk.Placeholders[0].ID != alpha.ID {
		t.Errorf("remaining placeholder id = %q, want original %q", desk.Placeholders[0].ID, alpha.ID)
	}
	if desk.Placeholders[0].Location.Kind == workbench.LocRecycle {
		t.Error("remaining placeholder must stay live, not enter recycle")
	}
	if bin := wb.RecycleBin(); len(bin) != 0 {
		t.Errorf("recycle bin = %+v, want empty after icon-only trash", bin)
	}

	// Skill package still on disk and in live inventory.
	if _, err := os.Stat(filepath.Join(identity, "SKILL.md")); err != nil {
		t.Fatalf("skill package must remain at live path: %v", err)
	}
	inv, err := wb.Inventory()
	if err != nil {
		t.Fatalf("Inventory: %v", err)
	}
	found := false
	for _, s := range inv.Skills {
		if s.Identity == identity {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("inventory missing identity %q after icon-only trash", identity)
	}
}

func TestTrash_LastPlaceholder_QuarantinesViaRename(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "doomed")
	writeSkill(t, skillDir, "doomed")
	wantID := mustRealpath(t, skillDir)

	store := index.NewMemoryStore()
	wb := workbench.New(workbench.Config{
		ScanRoots: []string{root},
		Index:     store,
	})
	if err := wb.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	desk := wb.Desk()
	ph := phByName(t, desk, "doomed")

	plan, err := wb.PlanTrash([]string{ph.ID})
	if err != nil {
		t.Fatalf("PlanTrash: %v", err)
	}
	if len(plan.IconOnlyIDs) != 0 {
		t.Fatalf("plan.IconOnlyIDs = %v, want empty for last placeholder", plan.IconOnlyIDs)
	}
	if len(plan.BodyItems) != 1 {
		t.Fatalf("plan.BodyItems len = %d, want 1: %+v", len(plan.BodyItems), plan.BodyItems)
	}
	if plan.BodyItems[0].Path != wantID {
		t.Errorf("confirm path = %q, want realpath %q", plan.BodyItems[0].Path, wantID)
	}
	if plan.BodyItems[0].Identity != wantID {
		t.Errorf("body identity = %q, want %q", plan.BodyItems[0].Identity, wantID)
	}

	if err := wb.ConfirmTrash([]string{ph.ID}); err != nil {
		t.Fatalf("ConfirmTrash: %v", err)
	}

	// Live path vacated.
	if _, err := os.Stat(skillDir); !os.IsNotExist(err) {
		t.Fatalf("live path should be vacated, stat err = %v", err)
	}
	// Quarantine under scan-root trash.
	trashRoot := filepath.Join(root, ".skills-manage-trash")
	entries, err := os.ReadDir(trashRoot)
	if err != nil {
		t.Fatalf("read trash root: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("trash root has %d entries, want 1", len(entries))
	}
	qPath := filepath.Join(trashRoot, entries[0].Name())
	if _, err := os.Stat(filepath.Join(qPath, "SKILL.md")); err != nil {
		t.Fatalf("quarantined package missing SKILL.md: %v", err)
	}

	// Not listed as live inventory.
	inv, err := wb.Inventory()
	if err != nil {
		t.Fatalf("Inventory: %v", err)
	}
	for _, s := range inv.Skills {
		if s.Identity == wantID {
			t.Fatalf("quarantined skill still in live inventory: %+v", s)
		}
	}

	desk = wb.Desk()
	if len(desk.Placeholders) != 1 {
		t.Fatalf("got %d placeholders, want 1 in recycle: %+v", len(desk.Placeholders), desk.Placeholders)
	}
	if desk.Placeholders[0].Location.Kind != workbench.LocRecycle {
		t.Errorf("placeholder location = %+v, want recycle", desk.Placeholders[0].Location)
	}

	bin := wb.RecycleBin()
	if len(bin) != 1 {
		t.Fatalf("recycle bin len = %d, want 1: %+v", len(bin), bin)
	}
	if bin[0].OriginalPath != wantID {
		t.Errorf("OriginalPath = %q, want %q", bin[0].OriginalPath, wantID)
	}
	if bin[0].QuarantinePath != qPath {
		// Accept realpath form of quarantine path.
		qReal, _ := filepath.EvalSymlinks(qPath)
		qAbs, _ := filepath.Abs(qReal)
		if bin[0].QuarantinePath != qAbs && bin[0].QuarantinePath != qPath {
			t.Errorf("QuarantinePath = %q, want %q", bin[0].QuarantinePath, qPath)
		}
	}
}

func TestTrash_AllPlaceholdersForIdentityEnterRecycleOnce(t *testing.T) {
	wb, _ := openDeskWithSkills(t, "multi")
	desk := wb.Desk()
	orig := phByName(t, desk, "multi")
	if err := wb.SetClipboard(workbench.ClipCopy, []string{orig.ID}); err != nil {
		t.Fatal(err)
	}
	if err := wb.PasteToDesktop(6, 2); err != nil {
		t.Fatal(err)
	}
	if err := wb.PasteToDesktop(7, 2); err != nil {
		t.Fatal(err)
	}
	desk = wb.Desk()
	if len(desk.Placeholders) != 3 {
		t.Fatalf("want 3 placeholders, got %d", len(desk.Placeholders))
	}
	ids := make([]string, 0, 3)
	for _, p := range desk.Placeholders {
		ids = append(ids, p.ID)
	}

	// Trash all copies at once → one body lifecycle, all enter recycle.
	if err := wb.ConfirmTrash(ids); err != nil {
		t.Fatalf("ConfirmTrash: %v", err)
	}
	desk = wb.Desk()
	if len(desk.Placeholders) != 3 {
		t.Fatalf("want 3 recycle placeholders, got %d", len(desk.Placeholders))
	}
	for _, p := range desk.Placeholders {
		if p.Location.Kind != workbench.LocRecycle {
			t.Errorf("placeholder %s location = %+v, want recycle", p.ID, p.Location)
		}
	}
	bin := wb.RecycleBin()
	if len(bin) != 1 {
		t.Fatalf("want 1 recycle entry for identity, got %d: %+v", len(bin), bin)
	}
	if len(bin[0].PlaceholderIDs) != 3 {
		t.Errorf("entry PlaceholderIDs = %v, want 3", bin[0].PlaceholderIDs)
	}
}

func TestRestore_WhenOriginalFree_RenamesBackAndPlacesDesktopIcon(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "restorable")
	writeSkill(t, skillDir, "restorable")
	wantID := mustRealpath(t, skillDir)

	wb := workbench.New(workbench.Config{
		ScanRoots: []string{root},
		Index:     index.NewMemoryStore(),
	})
	if err := wb.Open(); err != nil {
		t.Fatal(err)
	}
	ph := phByName(t, wb.Desk(), "restorable")
	if err := wb.ConfirmTrash([]string{ph.ID}); err != nil {
		t.Fatalf("ConfirmTrash: %v", err)
	}
	bin := wb.RecycleBin()
	if len(bin) != 1 {
		t.Fatalf("bin = %+v", bin)
	}
	entryID := bin[0].ID

	if err := wb.Restore(entryID); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	if _, err := os.Stat(filepath.Join(skillDir, "SKILL.md")); err != nil {
		t.Fatalf("skill not restored to original path: %v", err)
	}
	if len(wb.RecycleBin()) != 0 {
		t.Errorf("bin should be empty after restore, got %+v", wb.RecycleBin())
	}
	desk := wb.Desk()
	live := 0
	for _, p := range desk.Placeholders {
		if p.Identity == wantID && p.Location.Kind == workbench.LocDesktop {
			live++
		}
		if p.Location.Kind == workbench.LocRecycle {
			t.Errorf("placeholder still in recycle: %+v", p)
		}
	}
	if live != 1 {
		t.Errorf("want 1 desktop placeholder for restored skill, got %d: %+v", live, desk.Placeholders)
	}
	inv, err := wb.Inventory()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, s := range inv.Skills {
		if s.Identity == wantID {
			found = true
		}
	}
	if !found {
		t.Error("restored skill missing from inventory")
	}
}

func TestRestore_WhenOriginalOccupied_FailsWithoutOverwrite(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "conflict")
	writeSkill(t, skillDir, "conflict")

	wb := workbench.New(workbench.Config{
		ScanRoots: []string{root},
		Index:     index.NewMemoryStore(),
	})
	if err := wb.Open(); err != nil {
		t.Fatal(err)
	}
	ph := phByName(t, wb.Desk(), "conflict")
	if err := wb.ConfirmTrash([]string{ph.ID}); err != nil {
		t.Fatalf("ConfirmTrash: %v", err)
	}
	entryID := wb.RecycleBin()[0].ID
	qPath := wb.RecycleBin()[0].QuarantinePath

	// Occupy original path with a different package.
	writeSkill(t, skillDir, "intruder")

	err := wb.Restore(entryID)
	if err == nil {
		t.Fatal("Restore should fail when original path occupied")
	}
	if !strings.Contains(err.Error(), "occupied") {
		t.Errorf("error should mention occupied, got: %v", err)
	}

	// Quarantine payload still present; live path is still the intruder.
	if _, err := os.Stat(filepath.Join(qPath, "SKILL.md")); err != nil {
		t.Fatalf("quarantine should be untouched: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "intruder") {
		t.Errorf("live path must not be overwritten, SKILL.md = %s", data)
	}
	if len(wb.RecycleBin()) != 1 {
		t.Errorf("entry should remain in bin after failed restore")
	}
}

func TestEmptyRecycleBin_PermanentlyDeletesQuarantineOnly(t *testing.T) {
	root := t.TempDir()
	// Keep a live skill that must survive empty.
	writeSkill(t, filepath.Join(root, "keeper"), "keeper")
	writeSkill(t, filepath.Join(root, "gone"), "gone")

	wb := workbench.New(workbench.Config{
		ScanRoots: []string{root},
		Index:     index.NewMemoryStore(),
	})
	if err := wb.Open(); err != nil {
		t.Fatal(err)
	}
	gone := phByName(t, wb.Desk(), "gone")
	if err := wb.ConfirmTrash([]string{gone.ID}); err != nil {
		t.Fatal(err)
	}
	qPath := wb.RecycleBin()[0].QuarantinePath

	if err := wb.EmptyRecycleBin(); err != nil {
		t.Fatalf("EmptyRecycleBin: %v", err)
	}
	if len(wb.RecycleBin()) != 0 {
		t.Errorf("bin not empty: %+v", wb.RecycleBin())
	}
	if _, err := os.Stat(qPath); !os.IsNotExist(err) {
		t.Fatalf("quarantine path should be gone, err=%v", err)
	}
	// Live skill untouched.
	if _, err := os.Stat(filepath.Join(root, "keeper", "SKILL.md")); err != nil {
		t.Fatalf("keeper skill deleted by empty: %v", err)
	}
	// No recycle placeholders left.
	for _, p := range wb.Desk().Placeholders {
		if p.Location.Kind == workbench.LocRecycle {
			t.Errorf("stale recycle placeholder: %+v", p)
		}
	}
}

func TestPurgeDue_UsesInjectableClock(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, filepath.Join(root, "stale"), "stale")

	start := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	now := start
	clock := func() time.Time { return now }

	wb := workbench.New(workbench.Config{
		ScanRoots: []string{root},
		Index:     index.NewMemoryStore(),
		Clock:     clock,
	})
	if err := wb.Open(); err != nil {
		t.Fatal(err)
	}
	ph := phByName(t, wb.Desk(), "stale")
	if err := wb.ConfirmTrash([]string{ph.ID}); err != nil {
		t.Fatal(err)
	}
	qPath := wb.RecycleBin()[0].QuarantinePath
	purgeAfter := wb.RecycleBin()[0].PurgeAfter
	wantAfter := start.Add(workbench.RecycleRetention)
	if !purgeAfter.Equal(wantAfter) {
		t.Errorf("PurgeAfter = %v, want %v", purgeAfter, wantAfter)
	}

	// Not due yet.
	now = start.Add(15 * 24 * time.Hour)
	if err := wb.PurgeDue(); err != nil {
		t.Fatalf("PurgeDue early: %v", err)
	}
	if len(wb.RecycleBin()) != 1 {
		t.Fatalf("should not purge early, bin=%+v", wb.RecycleBin())
	}
	if _, err := os.Stat(qPath); err != nil {
		t.Fatalf("quarantine should still exist: %v", err)
	}

	// Due.
	now = start.Add(30*24*time.Hour + time.Second)
	if err := wb.PurgeDue(); err != nil {
		t.Fatalf("PurgeDue: %v", err)
	}
	if len(wb.RecycleBin()) != 0 {
		t.Errorf("bin should be empty after due purge: %+v", wb.RecycleBin())
	}
	if _, err := os.Stat(qPath); !os.IsNotExist(err) {
		t.Fatalf("quarantine should be deleted, err=%v", err)
	}
}

func TestTrash_RefuseOutsideScanRoot(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, filepath.Join(root, "inside"), "inside")

	// Open workbench with only root; then inject a placeholder whose identity is outside.
	store := index.NewMemoryStore()
	outside := t.TempDir()
	writeSkill(t, filepath.Join(outside, "evil"), "evil")
	outsideID := mustRealpath(t, filepath.Join(outside, "evil"))

	// Seed index with an outside identity placeholder (simulates corrupt/manual index).
	doc := index.Document{
		SchemaVersion: index.SchemaVersion,
		Skills:        []index.SkillRecord{{Identity: outsideID, Name: "evil"}},
		Placeholders: []index.PlaceholderRecord{{
			ID: "ph_evil", Identity: outsideID,
			Location: index.Location{Kind: index.LocDesktop, Row: 2, Col: 1},
		}},
		RecycleIcon: index.Location{Kind: index.LocDesktop, Row: 1, Col: 1},
	}
	if err := store.Save(doc); err != nil {
		t.Fatal(err)
	}

	wb := workbench.New(workbench.Config{
		ScanRoots: []string{root},
		Index:     store,
	})
	if err := wb.Open(); err != nil {
		t.Fatal(err)
	}

	err := wb.ConfirmTrash([]string{"ph_evil"})
	if err == nil {
		t.Fatal("expected refuse body delete outside scan root")
	}
	// Outside package must remain loadable (not deleted).
	if _, err := os.Stat(filepath.Join(outside, "evil", "SKILL.md")); err != nil {
		t.Fatalf("outside skill was modified/deleted: %v", err)
	}
}

func TestTrash_RefuseNonSkillDirectory(t *testing.T) {
	root := t.TempDir()
	// Real skill for desk open, plus a non-skill dir we force into index.
	writeSkill(t, filepath.Join(root, "ok"), "ok")
	nonskill := filepath.Join(root, "not-a-skill")
	if err := os.MkdirAll(nonskill, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nonskill, "readme.txt"), []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	nsID := mustRealpath(t, nonskill)

	store := index.NewMemoryStore()
	wb := workbench.New(workbench.Config{
		ScanRoots: []string{root},
		Index:     store,
	})
	if err := wb.Open(); err != nil {
		t.Fatal(err)
	}
	// Inject non-skill placeholder after open.
	doc, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	doc.Placeholders = append(doc.Placeholders, index.PlaceholderRecord{
		ID: "ph_ns", Identity: nsID,
		Location: index.Location{Kind: index.LocDesktop, Row: 9, Col: 9},
	})
	doc.Skills = append(doc.Skills, index.SkillRecord{Identity: nsID, Name: "not-a-skill"})
	if err := store.Save(doc); err != nil {
		t.Fatal(err)
	}
	// Re-open to pick up injected placeholder.
	wb2 := workbench.New(workbench.Config{
		ScanRoots: []string{root},
		Index:     store,
	})
	if err := wb2.Open(); err != nil {
		t.Fatal(err)
	}

	err = wb2.ConfirmTrash([]string{"ph_ns"})
	if err == nil {
		t.Fatal("expected refuse non-skill body delete")
	}
	if _, err := os.Stat(nonskill); err != nil {
		t.Fatalf("non-skill dir should remain: %v", err)
	}
}

func TestRescan_DoesNotRediscoverQuarantined(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, filepath.Join(root, "ghost"), "ghost")

	wb := workbench.New(workbench.Config{
		ScanRoots: []string{root},
		Index:     index.NewMemoryStore(),
	})
	if err := wb.Open(); err != nil {
		t.Fatal(err)
	}
	ph := phByName(t, wb.Desk(), "ghost")
	if err := wb.ConfirmTrash([]string{ph.ID}); err != nil {
		t.Fatal(err)
	}
	if err := wb.Rescan(); err != nil {
		t.Fatalf("Rescan: %v", err)
	}
	inv, err := wb.Inventory()
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.Skills) != 0 {
		t.Fatalf("quarantined skill rediscovered: %+v", inv.Skills)
	}
	// Still one recycle placeholder, not a new live desk icon.
	live := 0
	for _, p := range wb.Desk().Placeholders {
		if p.Location.Kind == workbench.LocDesktop && p.Name == "ghost" {
			live++
		}
	}
	if live != 0 {
		t.Errorf("rescan recreated live placeholder for quarantined skill")
	}
}

func TestPurge_RefusesPathOutsideTrash(t *testing.T) {
	root := t.TempDir()
	live := filepath.Join(root, "precious")
	writeSkill(t, live, "precious")
	liveID := mustRealpath(t, live)

	store := index.NewMemoryStore()
	// Plant a malicious recycle entry pointing at a live skill (not under trash).
	doc := index.Document{
		SchemaVersion: index.SchemaVersion,
		Placeholders: []index.PlaceholderRecord{{
			ID: "ph_x", Identity: liveID,
			Location: index.Location{Kind: index.LocRecycle},
		}},
		RecycleIcon: index.Location{Kind: index.LocDesktop, Row: 1, Col: 1},
		RecycleBin: []index.RecycleEntry{{
			ID:             "trash_deadbeef",
			Identity:       liveID,
			OriginalPath:   liveID,
			QuarantinePath: liveID, // attack: live path, not trash entry
			DeletedAt:      time.Now().UTC(),
			PurgeAfter:     time.Now().UTC().Add(-time.Hour),
			PlaceholderIDs: []string{"ph_x"},
			State:          index.RecycleStateQuarantined,
		}},
	}
	if err := store.Save(doc); err != nil {
		t.Fatal(err)
	}

	wb := workbench.New(workbench.Config{
		ScanRoots: []string{root},
		Index:     store,
	})
	// Open will try PurgeDue on the due entry — must refuse without deleting live skill.
	err := wb.Open()
	if err == nil {
		// If Open succeeded without purging (refused inside purgeDue), empty explicitly.
		if err := wb.EmptyRecycleBin(); err == nil {
			t.Fatal("EmptyRecycleBin must refuse live-path purge")
		}
	}
	if _, err := os.Stat(filepath.Join(live, "SKILL.md")); err != nil {
		t.Fatalf("live skill must not be deleted by malicious purge path: %v", err)
	}
}

func TestRestore_MultiPlaceholderKeepsOneDesktopIcon(t *testing.T) {
	wb, _ := openDeskWithSkills(t, "copies")
	desk := wb.Desk()
	orig := phByName(t, desk, "copies")
	if err := wb.SetClipboard(workbench.ClipCopy, []string{orig.ID}); err != nil {
		t.Fatal(err)
	}
	if err := wb.PasteToDesktop(5, 3); err != nil {
		t.Fatal(err)
	}
	ids := make([]string, 0, 2)
	for _, p := range wb.Desk().Placeholders {
		ids = append(ids, p.ID)
	}
	if err := wb.ConfirmTrash(ids); err != nil {
		t.Fatal(err)
	}
	entryID := wb.RecycleBin()[0].ID
	if err := wb.Restore(entryID); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	desk = wb.Desk()
	if len(desk.Placeholders) != 1 {
		t.Fatalf("want 1 placeholder after restore, got %d: %+v", len(desk.Placeholders), desk.Placeholders)
	}
	if desk.Placeholders[0].Location.Kind != workbench.LocDesktop {
		t.Errorf("want desktop, got %+v", desk.Placeholders[0].Location)
	}
}
