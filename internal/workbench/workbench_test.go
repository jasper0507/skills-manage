package workbench_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jasper0507/skills-manage/internal/workbench"
)

func TestInventory_DiscoversSkillPackageUnderScanRoot(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "hello-skill")
	writeSkill(t, skillDir, "hello-skill")

	wb := workbench.New(workbench.Config{ScanRoots: []string{root}})
	inv, err := wb.Inventory()
	if err != nil {
		t.Fatalf("Inventory: %v", err)
	}
	if len(inv.Skills) != 1 {
		t.Fatalf("got %d skills, want 1: %+v", len(inv.Skills), inv.Skills)
	}
	got := inv.Skills[0]
	wantID := mustRealpath(t, skillDir)
	if got.Identity != wantID {
		t.Errorf("Identity = %q, want %q", got.Identity, wantID)
	}
	if got.Name != "hello-skill" {
		t.Errorf("Name = %q, want %q", got.Name, "hello-skill")
	}
}

func TestInventory_CollapsesSymlinksToSameRealpath(t *testing.T) {
	base := t.TempDir()
	realRoot := filepath.Join(base, "real-root")
	linkRoot := filepath.Join(base, "link-root")
	if err := os.MkdirAll(realRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(linkRoot, 0o755); err != nil {
		t.Fatal(err)
	}

	skillDir := filepath.Join(realRoot, "shared-skill")
	writeSkill(t, skillDir, "shared-skill")
	linkPath := filepath.Join(linkRoot, "shared-skill-link")
	if err := os.Symlink(skillDir, linkPath); err != nil {
		t.Fatal(err)
	}

	wb := workbench.New(workbench.Config{ScanRoots: []string{realRoot, linkRoot}})
	inv, err := wb.Inventory()
	if err != nil {
		t.Fatalf("Inventory: %v", err)
	}
	if len(inv.Skills) != 1 {
		t.Fatalf("got %d skills, want 1 (symlink collapse): %+v", len(inv.Skills), inv.Skills)
	}
	wantID := mustRealpath(t, skillDir)
	if inv.Skills[0].Identity != wantID {
		t.Errorf("Identity = %q, want %q", inv.Skills[0].Identity, wantID)
	}
}

func TestInventory_DiscoversSiblingAfterSymlinkPackage(t *testing.T) {
	// Regression: SkipDir on a non-directory (package symlink) must not drop siblings.
	root := t.TempDir()
	realPkg := filepath.Join(root, "real-target")
	writeSkill(t, realPkg, "linked-skill")

	linkPkg := filepath.Join(root, "a-link") // lexically before b-real
	if err := os.Symlink(realPkg, linkPkg); err != nil {
		t.Fatal(err)
	}
	writeSkill(t, filepath.Join(root, "b-real"), "b-real")

	wb := workbench.New(workbench.Config{ScanRoots: []string{root}})
	inv, err := wb.Inventory()
	if err != nil {
		t.Fatalf("Inventory: %v", err)
	}
	if len(inv.Skills) != 2 {
		t.Fatalf("got %d skills, want 2 (symlink package + sibling): %+v", len(inv.Skills), inv.Skills)
	}
	names := map[string]bool{}
	for _, s := range inv.Skills {
		names[s.Name] = true
	}
	if !names["linked-skill"] || !names["b-real"] {
		t.Errorf("skills = %v, want linked-skill and b-real", names)
	}
}

func TestInventory_ScanRootMayBeSymlink(t *testing.T) {
	base := t.TempDir()
	realRoot := filepath.Join(base, "real-skills")
	writeSkill(t, filepath.Join(realRoot, "via-root-link"), "via-root-link")
	linkRoot := filepath.Join(base, "skills-link")
	if err := os.Symlink(realRoot, linkRoot); err != nil {
		t.Fatal(err)
	}

	wb := workbench.New(workbench.Config{ScanRoots: []string{linkRoot}})
	inv, err := wb.Inventory()
	if err != nil {
		t.Fatalf("Inventory: %v", err)
	}
	if len(inv.Skills) != 1 {
		t.Fatalf("got %d skills, want 1 under symlink scan root: %+v", len(inv.Skills), inv.Skills)
	}
	if inv.Skills[0].Name != "via-root-link" {
		t.Errorf("Name = %q, want via-root-link", inv.Skills[0].Name)
	}
}

func TestInventory_ExcludesQuarantineLocations(t *testing.T) {
	root := t.TempDir()
	live := filepath.Join(root, "live-skill")
	writeSkill(t, live, "live-skill")

	trash := filepath.Join(root, ".skills-manage-trash", "dead-id", "trashed-skill")
	writeSkill(t, trash, "trashed-skill")

	wb := workbench.New(workbench.Config{ScanRoots: []string{root}})
	inv, err := wb.Inventory()
	if err != nil {
		t.Fatalf("Inventory: %v", err)
	}
	if len(inv.Skills) != 1 {
		t.Fatalf("got %d skills, want 1 (quarantine excluded): %+v", len(inv.Skills), inv.Skills)
	}
	if inv.Skills[0].Name != "live-skill" {
		t.Errorf("Name = %q, want live-skill", inv.Skills[0].Name)
	}
}

func TestInventory_IgnoresDirectoriesWithoutSkillMD(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "not-a-skill"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "not-a-skill", "README.md"), []byte("# hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeSkill(t, filepath.Join(root, "real-skill"), "real-skill")

	wb := workbench.New(workbench.Config{ScanRoots: []string{root}})
	inv, err := wb.Inventory()
	if err != nil {
		t.Fatalf("Inventory: %v", err)
	}
	if len(inv.Skills) != 1 {
		t.Fatalf("got %d skills, want 1: %+v", len(inv.Skills), inv.Skills)
	}
	if inv.Skills[0].Name != "real-skill" {
		t.Errorf("Name = %q, want real-skill", inv.Skills[0].Name)
	}
}

func TestInventory_UsesDirectoryNameWhenFrontmatterNameMissing(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "dir-named-skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Valid package marker, no name in frontmatter.
	content := "---\ndescription: no name field\n---\n\n# Body\n"
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	wb := workbench.New(workbench.Config{ScanRoots: []string{root}})
	inv, err := wb.Inventory()
	if err != nil {
		t.Fatalf("Inventory: %v", err)
	}
	if len(inv.Skills) != 1 {
		t.Fatalf("got %d skills, want 1: %+v", len(inv.Skills), inv.Skills)
	}
	if inv.Skills[0].Name != "dir-named-skill" {
		t.Errorf("Name = %q, want dir-named-skill", inv.Skills[0].Name)
	}
}

func TestInventory_DiscoversNestedSkillUnderCategoryDir(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "devtools", "fmt-skill")
	writeSkill(t, nested, "fmt-skill")

	wb := workbench.New(workbench.Config{ScanRoots: []string{root}})
	inv, err := wb.Inventory()
	if err != nil {
		t.Fatalf("Inventory: %v", err)
	}
	if len(inv.Skills) != 1 {
		t.Fatalf("got %d skills, want 1: %+v", len(inv.Skills), inv.Skills)
	}
	if inv.Skills[0].Identity != mustRealpath(t, nested) {
		t.Errorf("Identity = %q, want nested package realpath", inv.Skills[0].Identity)
	}
}

func TestInventory_MissingScanRootIsSkipped(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, filepath.Join(root, "ok"), "ok")
	missing := filepath.Join(root, "does-not-exist")

	wb := workbench.New(workbench.Config{ScanRoots: []string{missing, root}})
	inv, err := wb.Inventory()
	if err != nil {
		t.Fatalf("Inventory: %v", err)
	}
	if len(inv.Skills) != 1 {
		t.Fatalf("got %d skills, want 1: %+v", len(inv.Skills), inv.Skills)
	}
}

func TestDefaultScanRoots_IncludesCommonUserAndProjectPaths(t *testing.T) {
	roots := workbench.DefaultScanRoots("/home/alice", "/home/alice/proj")
	want := []string{
		"/home/alice/.agents/skills",
		"/home/alice/.claude/skills",
		"/home/alice/.codex/skills",
		"/home/alice/.grok/skills",
		"/home/alice/proj/.agents/skills",
		"/home/alice/proj/.claude/skills",
		"/home/alice/proj/.codex/skills",
		"/home/alice/proj/.grok/skills",
	}
	got := map[string]bool{}
	for _, r := range roots {
		got[r] = true
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("DefaultScanRoots missing %q; got %v", w, roots)
		}
	}
	// Bundled/system trees must not appear in defaults.
	for _, r := range roots {
		for _, bad := range []string{"/usr/", "/opt/", "bundled", "node_modules"} {
			if strings.Contains(r, bad) {
				t.Errorf("default scan root looks bundled/system: %q", r)
			}
		}
	}
}
