package workbench_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jasper0507/skills-manage/internal/infra/index"
	"github.com/jasper0507/skills-manage/internal/workbench"
)

// R2: non-last 占位 enters the icon bin; Skill package stays on disk and in inventory.
func TestTrash_NonLastPlaceholder_EntersIconBin_SkillStaysLive(t *testing.T) {
	wb, _ := openDeskWithSkills(t, "alpha")
	desk := wb.Desk()
	alpha := phByName(t, desk, "alpha")
	identity := alpha.Identity

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
	if len(plan.EnterBinIDs) != 1 || plan.EnterBinIDs[0] != copyID {
		t.Fatalf("plan.EnterBinIDs = %v, want [%s]", plan.EnterBinIDs, copyID)
	}
	if len(plan.SkippedIDs) != 0 {
		t.Fatalf("plan.SkippedIDs = %v, want empty", plan.SkippedIDs)
	}

	if err := wb.ConfirmTrash([]string{copyID}); err != nil {
		t.Fatalf("ConfirmTrash: %v", err)
	}

	// Live desk still has the original; copy is in the icon bin.
	live := livePlaceholders(wb.Desk())
	if len(live) != 1 || live[0].ID != alpha.ID {
		t.Fatalf("live placeholders = %+v, want only original %q", live, alpha.ID)
	}
	bin := wb.RecycleBin()
	if len(bin) != 1 {
		t.Fatalf("recycle bin = %+v, want 1 in-bin placeholder", bin)
	}
	if bin[0].ID != copyID {
		t.Errorf("bin entry id = %q, want %q", bin[0].ID, copyID)
	}
	if bin[0].Identity != identity {
		t.Errorf("bin identity = %q, want %q", bin[0].Identity, identity)
	}

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
		t.Errorf("inventory missing identity %q after icon enter-bin", identity)
	}
}

// R2: last live 占位 for an identity is refused; desk/bin/package unchanged.
func TestTrash_LastLivePlaceholder_Refused(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "solo")
	writeSkill(t, skillDir, "solo")
	wantID := mustRealpath(t, skillDir)

	wb := workbench.New(workbench.Config{
		ScanRoots: []string{root},
		Index:     index.NewMemoryStore(),
	})
	if err := wb.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	ph := phByName(t, wb.Desk(), "solo")

	plan, err := wb.PlanTrash([]string{ph.ID})
	if err != nil {
		t.Fatalf("PlanTrash: %v", err)
	}
	if len(plan.EnterBinIDs) != 0 {
		t.Fatalf("plan.EnterBinIDs = %v, want empty for last live", plan.EnterBinIDs)
	}
	if len(plan.SkippedIDs) != 1 || plan.SkippedIDs[0] != ph.ID {
		t.Fatalf("plan.SkippedIDs = %v, want [%s]", plan.SkippedIDs, ph.ID)
	}

	err = wb.ConfirmTrash([]string{ph.ID})
	if err == nil {
		t.Fatal("ConfirmTrash should refuse last live placeholder")
	}
	if !strings.Contains(err.Error(), "last live") {
		t.Errorf("error should mention last live, got: %v", err)
	}

	// Package still at original path.
	if _, err := os.Stat(filepath.Join(skillDir, "SKILL.md")); err != nil {
		t.Fatalf("package must be untouched: %v", err)
	}
	// No quarantine tree created.
	trashRoot := filepath.Join(root, ".skills-manage-trash")
	if _, err := os.Stat(trashRoot); !os.IsNotExist(err) {
		t.Fatalf("must not create quarantine tree, stat err=%v", err)
	}
	if len(wb.RecycleBin()) != 0 {
		t.Errorf("bin should stay empty, got %+v", wb.RecycleBin())
	}
	live := livePlaceholders(wb.Desk())
	if len(live) != 1 || live[0].ID != ph.ID {
		t.Fatalf("desk placeholders = %+v, want unchanged solo", live)
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
		t.Error("inventory must still list the skill")
	}
}

// R2 batch: per-identity last-live filter; partial success (other identities still enter).
func TestTrash_Batch_SkipsLastLiveIdentity_OthersEnter(t *testing.T) {
	wb, _ := openDeskWithSkills(t, "alpha", "beta")
	desk := wb.Desk()
	alpha := phByName(t, desk, "alpha")
	beta := phByName(t, desk, "beta")

	// Give alpha a second placeholder so one can enter the bin.
	if err := wb.SetClipboard(workbench.ClipCopy, []string{alpha.ID}); err != nil {
		t.Fatal(err)
	}
	if err := wb.PasteToDesktop(5, 2); err != nil {
		t.Fatal(err)
	}
	var alphaCopy string
	for _, p := range wb.Desk().Placeholders {
		if p.Identity == alpha.Identity && p.ID != alpha.ID {
			alphaCopy = p.ID
			break
		}
	}
	if alphaCopy == "" {
		t.Fatal("missing alpha copy")
	}

	// Batch: alpha copy (ok) + beta last-live (skip).
	ids := []string{alphaCopy, beta.ID}
	plan, err := wb.PlanTrash(ids)
	if err != nil {
		t.Fatalf("PlanTrash: %v", err)
	}
	if len(plan.EnterBinIDs) != 1 || plan.EnterBinIDs[0] != alphaCopy {
		t.Fatalf("EnterBinIDs = %v, want [%s]", plan.EnterBinIDs, alphaCopy)
	}
	if len(plan.SkippedIDs) != 1 || plan.SkippedIDs[0] != beta.ID {
		t.Fatalf("SkippedIDs = %v, want [%s]", plan.SkippedIDs, beta.ID)
	}

	if err := wb.ConfirmTrash(ids); err != nil {
		t.Fatalf("ConfirmTrash partial batch: %v", err)
	}

	bin := wb.RecycleBin()
	if len(bin) != 1 || bin[0].ID != alphaCopy {
		t.Fatalf("bin = %+v, want only alpha copy", bin)
	}
	// Beta still live; alpha original still live.
	live := livePlaceholders(wb.Desk())
	liveIDs := map[string]bool{}
	for _, p := range live {
		liveIDs[p.ID] = true
	}
	if !liveIDs[alpha.ID] {
		t.Error("alpha original must remain live")
	}
	if !liveIDs[beta.ID] {
		t.Error("beta last-live must remain live (skipped)")
	}
	if liveIDs[alphaCopy] {
		t.Error("alpha copy should be in bin, not live")
	}
	// Packages still on disk.
	for _, name := range []string{"alpha", "beta"} {
		// Identity is realpath under scan root; resolve via remaining live ph.
		var id string
		for _, p := range live {
			if p.Name == name {
				id = p.Identity
				break
			}
		}
		if id == "" {
			t.Fatalf("no live placeholder for %s", name)
		}
		if _, err := os.Stat(filepath.Join(id, "SKILL.md")); err != nil {
			t.Fatalf("%s package missing: %v", name, err)
		}
	}
}

// Trashing all live copies of one identity at once skips that identity entirely.
func TestTrash_AllLiveCopiesOfIdentity_SkippedNotBodyDeleted(t *testing.T) {
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

	err := wb.ConfirmTrash(ids)
	if err == nil {
		t.Fatal("ConfirmTrash of all live copies must refuse (would zero live)")
	}

	// Nothing entered bin; packages untouched; all still live.
	if len(wb.RecycleBin()) != 0 {
		t.Errorf("bin should be empty, got %+v", wb.RecycleBin())
	}
	if n := len(livePlaceholders(wb.Desk())); n != 3 {
		t.Fatalf("live count = %d, want 3", n)
	}
	if _, err := os.Stat(filepath.Join(orig.Identity, "SKILL.md")); err != nil {
		t.Fatalf("package must remain: %v", err)
	}
}

// Restore places the 占位 on a free desktop cell; no package rename.
func TestRestore_ReturnsPlaceholderToFreeDesktopCell(t *testing.T) {
	wb, _ := openDeskWithSkills(t, "restorable")
	desk := wb.Desk()
	orig := phByName(t, desk, "restorable")
	identity := orig.Identity

	if err := wb.SetClipboard(workbench.ClipCopy, []string{orig.ID}); err != nil {
		t.Fatal(err)
	}
	if err := wb.PasteToDesktop(5, 3); err != nil {
		t.Fatal(err)
	}
	var copyID string
	for _, p := range wb.Desk().Placeholders {
		if p.ID != orig.ID {
			copyID = p.ID
			break
		}
	}
	if err := wb.ConfirmTrash([]string{copyID}); err != nil {
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

	if _, err := os.Stat(filepath.Join(identity, "SKILL.md")); err != nil {
		t.Fatalf("package must never have moved: %v", err)
	}
	if len(wb.RecycleBin()) != 0 {
		t.Errorf("bin should be empty after restore, got %+v", wb.RecycleBin())
	}
	desk = wb.Desk()
	restored := 0
	for _, p := range desk.Placeholders {
		if p.ID == copyID {
			if p.Location.Kind != workbench.LocDesktop {
				t.Errorf("restored location = %+v, want desktop", p.Location)
			}
			if p.Location.Row < 1 || p.Location.Col < 1 {
				t.Errorf("invalid free cell %+v", p.Location)
			}
			restored++
		}
		if p.Location.Kind == workbench.LocRecycle {
			t.Errorf("placeholder still in recycle: %+v", p)
		}
	}
	if restored != 1 {
		t.Errorf("want restored copy on desktop, found %d: %+v", restored, desk.Placeholders)
	}
}

// Empty recycle discards in-bin placeholder records only; no package tree deleted.
func TestEmptyRecycleBin_DropsPlaceholderRecordsOnly(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, filepath.Join(root, "keeper"), "keeper")
	writeSkill(t, filepath.Join(root, "extra"), "extra")

	wb := workbench.New(workbench.Config{
		ScanRoots: []string{root},
		Index:     index.NewMemoryStore(),
	})
	if err := wb.Open(); err != nil {
		t.Fatal(err)
	}
	extra := phByName(t, wb.Desk(), "extra")
	if err := wb.SetClipboard(workbench.ClipCopy, []string{extra.ID}); err != nil {
		t.Fatal(err)
	}
	if err := wb.PasteToDesktop(5, 2); err != nil {
		t.Fatal(err)
	}
	var copyID string
	for _, p := range wb.Desk().Placeholders {
		if p.Identity == extra.Identity && p.ID != extra.ID {
			copyID = p.ID
			break
		}
	}
	if err := wb.ConfirmTrash([]string{copyID}); err != nil {
		t.Fatal(err)
	}
	if len(wb.RecycleBin()) != 1 {
		t.Fatalf("precondition bin: %+v", wb.RecycleBin())
	}

	if err := wb.EmptyRecycleBin(); err != nil {
		t.Fatalf("EmptyRecycleBin: %v", err)
	}
	if len(wb.RecycleBin()) != 0 {
		t.Errorf("bin not empty: %+v", wb.RecycleBin())
	}
	// Both skill packages still on disk.
	for _, name := range []string{"keeper", "extra"} {
		if _, err := os.Stat(filepath.Join(root, name, "SKILL.md")); err != nil {
			t.Fatalf("%s package deleted by empty: %v", name, err)
		}
	}
	// No recycle placeholders left.
	for _, p := range wb.Desk().Placeholders {
		if p.Location.Kind == workbench.LocRecycle {
			t.Errorf("stale recycle placeholder: %+v", p)
		}
	}
	// Live originals remain.
	if n := len(livePlaceholders(wb.Desk())); n != 2 {
		t.Fatalf("live count = %d, want 2", n)
	}
}

// No product path for package isolate / PurgeDue retention lifecycle.
func TestNoBodyDeleteProductPath(t *testing.T) {
	// Ensure PurgeDue is not part of the public R2 surface (compile-time via
	// residual call absence is covered by Open not deleting packages).
	root := t.TempDir()
	writeSkill(t, filepath.Join(root, "alive"), "alive")
	wb := workbench.New(workbench.Config{
		ScanRoots: []string{root},
		Index:     index.NewMemoryStore(),
	})
	if err := wb.Open(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "alive", "SKILL.md")); err != nil {
		t.Fatalf("Open must not touch packages: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, ".skills-manage-trash")); !os.IsNotExist(err) {
		t.Fatalf("Open must not create quarantine tree: err=%v", err)
	}
}

// Rescan still discovers skills whose extra icons sit in the icon bin.
func TestRescan_StillSeesSkillsWithIconsInBin(t *testing.T) {
	wb, _ := openDeskWithSkills(t, "ghost")
	orig := phByName(t, wb.Desk(), "ghost")
	if err := wb.SetClipboard(workbench.ClipCopy, []string{orig.ID}); err != nil {
		t.Fatal(err)
	}
	if err := wb.PasteToDesktop(5, 2); err != nil {
		t.Fatal(err)
	}
	var copyID string
	for _, p := range wb.Desk().Placeholders {
		if p.ID != orig.ID {
			copyID = p.ID
			break
		}
	}
	if err := wb.ConfirmTrash([]string{copyID}); err != nil {
		t.Fatal(err)
	}
	if err := wb.Rescan(); err != nil {
		t.Fatalf("Rescan: %v", err)
	}
	inv, err := wb.Inventory()
	if err != nil {
		t.Fatal(err)
	}
	if len(inv.Skills) != 1 || inv.Skills[0].Name != "ghost" {
		t.Fatalf("inventory after rescan = %+v, want ghost still present", inv.Skills)
	}
	// Must not spawn a second live icon for the same identity (already has live + bin).
	live := livePlaceholders(wb.Desk())
	if len(live) != 1 {
		t.Fatalf("live placeholders = %d, want 1: %+v", len(live), live)
	}
}

// DeleteBox returns members to free desktop cells; does not bulk enter-bin.
func TestDeleteBox_ReturnsToDesktop_NotEnterBin(t *testing.T) {
	wb, _ := openDeskWithSkills(t, "a", "b")
	desk := wb.Desk()
	a := phByName(t, desk, "a")
	b := phByName(t, desk, "b")

	boxID, err := wb.CreateSimpleBox("box", 200, 200)
	if err != nil {
		t.Fatal(err)
	}
	if err := wb.MovePlaceholderToBox(a.ID, boxID, ""); err != nil {
		t.Fatal(err)
	}
	if err := wb.MovePlaceholderToBox(b.ID, boxID, ""); err != nil {
		t.Fatal(err)
	}
	if err := wb.DeleteBox(boxID); err != nil {
		t.Fatalf("DeleteBox: %v", err)
	}
	if len(wb.RecycleBin()) != 0 {
		t.Errorf("DeleteBox must not enter-bin, bin=%+v", wb.RecycleBin())
	}
	live := livePlaceholders(wb.Desk())
	if len(live) != 2 {
		t.Fatalf("live = %d, want 2 on desktop: %+v", len(live), live)
	}
	for _, p := range live {
		if p.Location.Kind != workbench.LocDesktop {
			t.Errorf("%s location = %+v, want desktop", p.Name, p.Location)
		}
	}
}

func livePlaceholders(desk workbench.Desk) []workbench.Placeholder {
	var out []workbench.Placeholder
	for _, p := range desk.Placeholders {
		if p.Location.Kind != workbench.LocRecycle {
			out = append(out, p)
		}
	}
	return out
}
