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

func containsStr(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

// assertMemberPlacementEmpty checks the index document: phID is listed in exactly
// one ItemIDs claim and has no durable placement (empty kind).
func assertMemberPlacementEmpty(t *testing.T, store index.Store, phID string) {
	t.Helper()
	doc, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	claims := 0
	for _, box := range doc.Boxes {
		ids := box.ItemIDs
		if box.Kind == index.BoxComposite {
			ids = nil
			for _, c := range box.Compartments {
				ids = append(ids, c.ItemIDs...)
			}
		}
		for _, id := range ids {
			if id == phID {
				claims++
			}
		}
	}
	if claims != 1 {
		t.Errorf("membership claims for %s = %d, want 1", phID, claims)
	}
	for _, p := range doc.Placeholders {
		if p.ID != phID {
			continue
		}
		if p.Location.Kind != "" {
			t.Errorf("member %s placement = %+v, want empty (membership only)", phID, p.Location)
		}
		return
	}
	t.Errorf("placeholder %s not found in document", phID)
}

// assertExactlyOneMembership fails if any placeholder id appears in more than
// one ItemIDs list across the document.
func assertExactlyOneMembership(t *testing.T, store index.Store) {
	t.Helper()
	doc, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]string{}
	for _, box := range doc.Boxes {
		ids := box.ItemIDs
		if box.Kind == index.BoxComposite {
			ids = nil
			for _, c := range box.Compartments {
				ids = append(ids, c.ItemIDs...)
			}
		}
		for _, id := range ids {
			if prev, ok := seen[id]; ok {
				t.Errorf("placeholder %s in both %s and %s", id, prev, box.ID)
			}
			seen[id] = box.ID
		}
	}
	for _, p := range doc.Placeholders {
		if _, ok := seen[p.ID]; !ok {
			continue
		}
		if p.Location.Kind != "" {
			t.Errorf("member %s placement = %+v, want empty", p.ID, p.Location)
		}
	}
}
