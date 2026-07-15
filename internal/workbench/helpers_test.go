package workbench_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jasper0507/skills-manage/internal/infra/index"
	"github.com/jasper0507/skills-manage/internal/workbench"
)

func writeSkill(t *testing.T, dir, frontmatterName string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	content := "---\nname: " + frontmatterName + "\ndescription: test skill\n---\n\n# Body\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustRealpath(t *testing.T, path string) string {
	t.Helper()
	rp, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	abs, err := filepath.Abs(rp)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func newWB(t *testing.T, roots []string, store index.Store) *workbench.Workbench {
	t.Helper()
	return workbench.New(workbench.Config{
		ScanRoots: roots,
		Index:     store,
	})
}

func openDeskWithSkills(t *testing.T, names ...string) (*workbench.Workbench, index.Store) {
	t.Helper()
	root := t.TempDir()
	for _, n := range names {
		writeSkill(t, filepath.Join(root, n), n)
	}
	store := index.NewMemoryStore()
	wb := newWB(t, []string{root}, store)
	if err := wb.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	return wb, store
}

func phByName(t *testing.T, desk workbench.Desk, name string) workbench.Placeholder {
	t.Helper()
	for _, p := range desk.Placeholders {
		if p.Name == name {
			return p
		}
	}
	t.Fatalf("placeholder %q not found", name)
	return workbench.Placeholder{}
}

func fmtCell(row, col int) string {
	return fmt.Sprintf("%d,%d", row, col)
}
