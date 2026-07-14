package index_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/jasper0507/skills-manage/internal/index"
)

func TestFileStore_AtomicSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "index.json")
	store := index.NewFileStore(path)

	doc := index.Document{
		SchemaVersion: index.SchemaVersion,
		Placeholders: []index.PlaceholderRecord{
			{
				ID:       "ph_1",
				Identity: "/home/u/.agents/skills/hello",
				Location: index.Location{Kind: index.LocDesktop, Row: 2, Col: 1},
			},
		},
		RecycleIcon: index.Location{Kind: index.LocDesktop, Row: 1, Col: 1},
		Boxes: []index.BoxRecord{
			{ID: "box_1", Kind: "simple", Tag: "design"},
		},
		BoxNameSeq: 2,
	}
	if err := store.Save(doc); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// No leftover temp files.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.Name() != "index.json" {
			t.Errorf("unexpected file left after atomic save: %s", e.Name())
		}
	}

	// Readable JSON object (not partial).
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var probe map[string]json.RawMessage
	if err := json.Unmarshal(raw, &probe); err != nil {
		t.Fatalf("saved file is not valid JSON object: %v\n%s", err, raw)
	}
	// Assert external fields by decoding through Load — not key order.
	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.Placeholders) != 1 || got.Placeholders[0].Identity != "/home/u/.agents/skills/hello" {
		t.Errorf("placeholders = %+v", got.Placeholders)
	}
	if got.Placeholders[0].Location.Row != 2 || got.Placeholders[0].Location.Col != 1 {
		t.Errorf("location = %+v", got.Placeholders[0].Location)
	}
	if got.RecycleIcon.Row != 1 || got.RecycleIcon.Col != 1 {
		t.Errorf("recycle = %+v", got.RecycleIcon)
	}
	if len(got.Boxes) != 1 || got.Boxes[0].Tag != "design" {
		t.Errorf("boxes = %+v", got.Boxes)
	}
}

func TestFileStore_MissingFileIsEmptyDesk(t *testing.T) {
	store := index.NewFileStore(filepath.Join(t.TempDir(), "missing", "index.json"))
	doc, err := store.Load()
	if err != nil {
		t.Fatalf("Load missing: %v", err)
	}
	if doc.RecycleIcon.Row != 1 || doc.RecycleIcon.Col != 1 {
		t.Errorf("default recycle = %+v, want (1,1)", doc.RecycleIcon)
	}
	if len(doc.Placeholders) != 0 {
		t.Errorf("placeholders = %v, want empty", doc.Placeholders)
	}
}

func TestDefaultPath(t *testing.T) {
	got := index.DefaultPath("/home/alice/.config")
	want := "/home/alice/.config/skills-manage/index.json"
	if got != want {
		t.Errorf("DefaultPath = %q, want %q", got, want)
	}
}
