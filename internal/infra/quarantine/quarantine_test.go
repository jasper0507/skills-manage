package quarantine_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jasper0507/skills-manage/internal/infra/quarantine"
)

func writeSkill(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: x\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestIsolate_RejectsUnsafeEntryID(t *testing.T) {
	root := t.TempDir()
	live := filepath.Join(root, "sk")
	writeSkill(t, live)
	q := quarantine.New()
	for _, id := range []string{"", "..", ".", "a/b", `a\b`, "../x", "foo/../bar"} {
		if _, err := q.Isolate(live, root, id); err == nil {
			t.Errorf("Isolate with entryID %q should fail", id)
		}
	}
	if _, err := os.Stat(live); err != nil {
		t.Fatalf("live path should remain after rejected isolate: %v", err)
	}
}

func TestPurge_RefusesTrashRootAndOutside(t *testing.T) {
	root := t.TempDir()
	live := filepath.Join(root, "sk")
	writeSkill(t, live)
	q := quarantine.New()
	qPath, err := q.Isolate(live, root, "trash_abc123")
	if err != nil {
		t.Fatal(err)
	}
	trashRoot := filepath.Join(root, quarantine.TrashDirName)

	if err := q.Purge(trashRoot); err == nil {
		t.Fatal("Purge of trash root must be refused")
	}
	// Entry still present.
	if _, err := os.Stat(qPath); err != nil {
		t.Fatalf("entry should survive trash-root purge attempt: %v", err)
	}
	// Outside path refused.
	other := filepath.Join(root, "other")
	writeSkill(t, other)
	if err := q.Purge(other); err == nil {
		t.Fatal("Purge outside trash must be refused")
	}
	if _, err := os.Stat(filepath.Join(other, "SKILL.md")); err != nil {
		t.Fatalf("outside skill deleted: %v", err)
	}
	// Valid entry purge works.
	if err := q.Purge(qPath); err != nil {
		t.Fatalf("Purge entry: %v", err)
	}
	if _, err := os.Stat(qPath); !os.IsNotExist(err) {
		t.Fatalf("entry should be gone, err=%v", err)
	}
}
