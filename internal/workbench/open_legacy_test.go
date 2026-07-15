package workbench_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jasper0507/skills-manage/internal/infra/index"
	"github.com/jasper0507/skills-manage/internal/workbench"
)

// Legacy body-delete index: RecycleEntry with quarantine path / purge-after / states
// plus kind=recycle placeholders. Open must not touch packages or run PurgeDue.
func TestOpen_LegacyBodyRecycleEntry_NoPackageOps_StripsBodyTable(t *testing.T) {
	root := t.TempDir()
	// Live skill that must survive Open even if a "due" purge entry points at it maliciously.
	liveDir := filepath.Join(root, "precious")
	writeSkill(t, liveDir, "precious")
	liveID := mustRealpath(t, liveDir)

	// Orphan tree under legacy trash dir (former quarantine payload). Must not be purged on Open.
	qPath := filepath.Join(root, ".skills-manage-trash", "trash_deadbeef")
	writeSkill(t, filepath.Join(qPath, "old-name"), "old-name")
	qSkillMD := filepath.Join(qPath, "old-name", "SKILL.md")

	// Soft-trashed icon for a different skill that is still on disk (multi-copy leftover under R2,
	// or incomplete body migration). Body RecycleEntry is incomplete / body-centric.
	extraDir := filepath.Join(root, "extra")
	writeSkill(t, extraDir, "extra")
	extraID := mustRealpath(t, extraDir)

	store := index.NewMemoryStore()
	past := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	doc := index.Document{
		SchemaVersion: index.SchemaVersion,
		Skills: []index.SkillRecord{
			{Identity: liveID, Name: "precious"},
			{Identity: extraID, Name: "extra"},
		},
		Placeholders: []index.PlaceholderRecord{
			{
				ID: "ph_live", Identity: liveID,
				Location: index.Location{Kind: index.LocDesktop, Row: 1, Col: 2},
			},
			{
				ID: "ph_extra_live", Identity: extraID,
				Location: index.Location{Kind: index.LocDesktop, Row: 1, Col: 3},
			},
			{
				// Icon-bin member without needing body metadata completeness.
				ID: "ph_extra_bin", Identity: extraID,
				Location: index.Location{Kind: index.LocRecycle},
			},
			{
				// Recycle icon for a body-deleted identity whose package is only under trash.
				// Product must keep it as R2 bin member (restorable as icon, no package rename).
				ID: "ph_ghost", Identity: filepath.Join(qPath, "old-name"),
				Location: index.Location{Kind: index.LocRecycle},
			},
		},
		RecycleIcon: index.Location{Kind: index.LocDesktop, Row: 1, Col: 1},
		Boxes: []index.BoxRecord{
			{
				ID: "box_1", Kind: index.BoxSimple, Tag: "kept",
				X: 200, Y: 200, W: 240, H: 220,
				ItemIDs: []string{"ph_live"},
			},
		},
		RecycleBin: []index.RecycleEntry{
			{
				ID:             "trash_deadbeef",
				Identity:       liveID, // malicious / stale: points at live skill
				Name:           "precious",
				OriginalPath:   liveID,
				QuarantinePath: liveID, // attack shape from old purge tests
				DeletedAt:      past,
				PurgeAfter:     past, // long past due
				PlaceholderIDs: []string{"ph_live"},
				State:          index.RecycleStateQuarantined,
			},
			{
				ID:             "trash_extra",
				Identity:       extraID,
				Name:           "extra",
				OriginalPath:   extraID,
				QuarantinePath: qPath,
				DeletedAt:      past,
				PurgeAfter:     past,
				PlaceholderIDs: []string{"ph_extra_bin"},
				State:          index.RecycleStateQuarantined,
			},
		},
		BoxNameSeq: 2,
	}
	if err := store.Save(doc); err != nil {
		t.Fatal(err)
	}

	wb := workbench.New(workbench.Config{
		ScanRoots: []string{root},
		Index:     store,
	})
	if err := wb.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}

	// Packages untouched.
	if _, err := os.Stat(filepath.Join(liveDir, "SKILL.md")); err != nil {
		t.Fatalf("live package deleted/moved on Open: %v", err)
	}
	if _, err := os.Stat(qSkillMD); err != nil {
		t.Fatalf("legacy quarantine tree purged on Open: %v", err)
	}
	if _, err := os.Stat(filepath.Join(extraDir, "SKILL.md")); err != nil {
		t.Fatalf("extra package touched: %v", err)
	}

	// Icon bin is placeholders, not body lifecycle table.
	bin := wb.RecycleBin()
	if len(bin) != 2 {
		t.Fatalf("RecycleBin() = %+v, want 2 icon members (ph_extra_bin, ph_ghost)", bin)
	}
	ids := map[string]bool{}
	for _, e := range bin {
		ids[e.ID] = true
		// No body fields on product view.
	}
	if !ids["ph_extra_bin"] || !ids["ph_ghost"] {
		t.Fatalf("bin ids = %v, want ph_extra_bin and ph_ghost", ids)
	}

	// Desk layout preserved: box still present with member; live icons remain.
	desk := wb.Desk()
	if len(desk.Boxes) != 1 || desk.Boxes[0].ID != "box_1" {
		t.Fatalf("boxes = %+v, want box_1 preserved", desk.Boxes)
	}
	if desk.Boxes[0].Tag != "kept" {
		t.Errorf("box tag = %q, want kept", desk.Boxes[0].Tag)
	}
	liveCount := 0
	for _, p := range desk.Placeholders {
		if p.Location.Kind != workbench.LocRecycle {
			liveCount++
		}
	}
	if liveCount < 2 {
		t.Fatalf("live placeholders = %d, want at least precious+extra: %+v", liveCount, desk.Placeholders)
	}

	// Subsequent load must not rehydrate body RecycleEntry as product state.
	saved, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.RecycleBin) != 0 {
		t.Errorf("persisted RecycleBin = %+v, want stripped/empty after Open", saved.RecycleBin)
	}
	// Icon-bin placeholders and box membership survive persist.
	recyclePH := 0
	for _, p := range saved.Placeholders {
		if p.Location.Kind == index.LocRecycle {
			recyclePH++
		}
	}
	if recyclePH != 2 {
		t.Errorf("persisted recycle placeholders = %d, want 2", recyclePH)
	}
	if len(saved.Boxes) != 1 || len(saved.Boxes[0].ItemIDs) != 1 || saved.Boxes[0].ItemIDs[0] != "ph_live" {
		t.Errorf("persisted boxes = %+v, want box with ph_live", saved.Boxes)
	}
}

// kind=recycle placeholders without any RecycleEntry body row remain restorable under R2.
func TestOpen_RecyclePlaceholdersWithoutBodyEntry_RestorableAndEmptyable(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, filepath.Join(root, "alpha"), "alpha")
	alphaID := mustRealpath(t, filepath.Join(root, "alpha"))

	store := index.NewMemoryStore()
	doc := index.Document{
		SchemaVersion: index.SchemaVersion,
		Skills:        []index.SkillRecord{{Identity: alphaID, Name: "alpha"}},
		Placeholders: []index.PlaceholderRecord{
			{
				ID: "ph_live", Identity: alphaID,
				Location: index.Location{Kind: index.LocDesktop, Row: 1, Col: 2},
			},
			{
				ID: "ph_bin", Identity: alphaID,
				Location: index.Location{Kind: index.LocRecycle},
			},
		},
		RecycleIcon: index.Location{Kind: index.LocDesktop, Row: 1, Col: 1},
		// No RecycleBin body table at all (R2-only or already stripped).
	}
	if err := store.Save(doc); err != nil {
		t.Fatal(err)
	}

	wb := workbench.New(workbench.Config{ScanRoots: []string{root}, Index: store})
	if err := wb.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	bin := wb.RecycleBin()
	if len(bin) != 1 || bin[0].ID != "ph_bin" {
		t.Fatalf("bin = %+v, want ph_bin", bin)
	}

	if err := wb.Restore("ph_bin"); err != nil {
		t.Fatalf("Restore without body entry: %v", err)
	}
	if len(wb.RecycleBin()) != 0 {
		t.Fatalf("bin after restore = %+v", wb.RecycleBin())
	}
	// Package never moved.
	if _, err := os.Stat(filepath.Join(root, "alpha", "SKILL.md")); err != nil {
		t.Fatalf("package missing: %v", err)
	}
	// Restored to desktop.
	found := false
	for _, p := range wb.Desk().Placeholders {
		if p.ID == "ph_bin" && p.Location.Kind == workbench.LocDesktop {
			found = true
		}
	}
	if !found {
		t.Fatalf("ph_bin not on desktop after restore: %+v", wb.Desk().Placeholders)
	}

	// Re-enter and empty.
	if err := wb.ConfirmTrash([]string{"ph_bin"}); err != nil {
		t.Fatalf("ConfirmTrash: %v", err)
	}
	if err := wb.EmptyRecycleBin(); err != nil {
		t.Fatalf("EmptyRecycleBin: %v", err)
	}
	if len(wb.RecycleBin()) != 0 {
		t.Fatalf("bin after empty = %+v", wb.RecycleBin())
	}
	if _, err := os.Stat(filepath.Join(root, "alpha", "SKILL.md")); err != nil {
		t.Fatalf("package deleted by empty: %v", err)
	}
	// Live original remains.
	live := 0
	for _, p := range wb.Desk().Placeholders {
		if p.Location.Kind != workbench.LocRecycle {
			live++
		}
	}
	if live != 1 {
		t.Fatalf("live count = %d, want 1", live)
	}
}

// Incomplete body row (missing quarantine path / state) must not block R2 bin membership.
func TestOpen_IncompleteBodyEntry_Ignored_IconBinFromPlaceholders(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, filepath.Join(root, "beta"), "beta")
	betaID := mustRealpath(t, filepath.Join(root, "beta"))

	store := index.NewMemoryStore()
	doc := index.Document{
		SchemaVersion: index.SchemaVersion,
		Placeholders: []index.PlaceholderRecord{
			{
				ID: "ph_live", Identity: betaID,
				Location: index.Location{Kind: index.LocDesktop, Row: 2, Col: 2},
			},
			{
				ID: "ph_bin", Identity: betaID,
				Location: index.Location{Kind: index.LocRecycle},
			},
		},
		RecycleIcon: index.Location{Kind: index.LocDesktop, Row: 1, Col: 1},
		RecycleBin: []index.RecycleEntry{
			{
				// Incomplete: no quarantine path, no purge-after, odd state.
				ID:             "trash_incomplete",
				Identity:       betaID,
				PlaceholderIDs: []string{"ph_bin"},
				State:          "unknown-legacy",
			},
		},
	}
	if err := store.Save(doc); err != nil {
		t.Fatal(err)
	}

	wb := workbench.New(workbench.Config{ScanRoots: []string{root}, Index: store})
	if err := wb.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if len(wb.RecycleBin()) != 1 || wb.RecycleBin()[0].ID != "ph_bin" {
		t.Fatalf("bin = %+v", wb.RecycleBin())
	}
	// Product path does not surface body table after Open.
	saved, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(saved.RecycleBin) != 0 {
		t.Errorf("body RecycleBin should be cleared on Open persist, got %+v", saved.RecycleBin)
	}
}
