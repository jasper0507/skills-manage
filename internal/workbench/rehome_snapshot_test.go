package workbench_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/jasper0507/skills-manage/internal/infra/index"
	"github.com/jasper0507/skills-manage/internal/workbench"
)

// countingStore records Save calls and can inject failures for snapshot tests.
type countingStore struct {
	mu       sync.Mutex
	inner    *index.MemoryStore
	saves    int
	failNext bool
}

func newCountingStore() *countingStore {
	return &countingStore{inner: index.NewMemoryStore()}
}

func (c *countingStore) Load() (index.Document, error) {
	return c.inner.Load()
}

func (c *countingStore) Save(doc index.Document) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.failNext {
		c.failNext = false
		return errors.New("injected save failure")
	}
	c.saves++
	return c.inner.Save(doc)
}

func (c *countingStore) saveCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.saves
}

// Open repairs "in ItemIDs but Location says desktop" preferring membership truth.
func TestOpen_Rehome_ItemIDsWinsOverDesktopLocation(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root+"/alpha", "alpha")
	alphaID := mustRealpath(t, root+"/alpha")

	store := index.NewMemoryStore()
	doc := index.Document{
		SchemaVersion: index.SchemaVersion,
		Skills:        []index.SkillRecord{{Identity: alphaID, Name: "alpha"}},
		Placeholders: []index.PlaceholderRecord{{
			ID: "ph_a", Identity: alphaID,
			// Divergent: claims desktop while listed in box ItemIDs.
			Location: index.Location{Kind: index.LocDesktop, Row: 2, Col: 3},
		}},
		RecycleIcon: index.Location{Kind: index.LocDesktop, Row: 1, Col: 1},
		Boxes: []index.BoxRecord{{
			ID: "box_1", Kind: index.BoxSimple, Tag: "design",
			X: 200, Y: 200, W: 240, H: 220,
			ItemIDs: []string{"ph_a"},
		}},
	}
	if err := store.Save(doc); err != nil {
		t.Fatal(err)
	}

	wb := workbench.New(workbench.Config{ScanRoots: []string{root}, Index: store})
	if err := wb.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}

	desk := wb.Desk()
	if len(desk.Boxes) != 1 {
		t.Fatalf("boxes = %d, want 1", len(desk.Boxes))
	}
	if len(desk.Boxes[0].Items) != 1 || desk.Boxes[0].Items[0].ID != "ph_a" {
		t.Fatalf("box items = %+v, want ph_a (ItemIDs membership)", desk.Boxes[0].Items)
	}
	ph := desk.Boxes[0].Items[0]
	if ph.Location.Kind != workbench.LocBox || ph.Location.BoxID != "box_1" {
		t.Errorf("rehomed location = %+v, want box box_1", ph.Location)
	}
	// Placeholder list agrees with membership (no desktop ghost with stale coords).
	for _, p := range desk.Placeholders {
		if p.ID == "ph_a" && p.Location.Kind != workbench.LocBox {
			t.Errorf("desk ph location still diverged: %+v", p.Location)
		}
	}
}

// Ghost rehome must not land on default recycle cell (1,1).
func TestOpen_RehomeGhost_DoesNotStackOnRecycleDefault(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root+"/solo", "solo")
	soloID := mustRealpath(t, root+"/solo")

	store := index.NewMemoryStore()
	doc := index.Document{
		SchemaVersion: index.SchemaVersion,
		Skills:        []index.SkillRecord{{Identity: soloID, Name: "solo"}},
		Placeholders: []index.PlaceholderRecord{{
			ID: "ph_ghost", Identity: soloID,
			Location: index.Location{Kind: index.LocBox, BoxID: "missing"},
		}},
		// Unset recycle → Open defaults to (1,1); ghost free-cell must avoid it.
		RecycleIcon: index.Location{},
		Boxes:       nil,
	}
	if err := store.Save(doc); err != nil {
		t.Fatal(err)
	}
	wb := workbench.New(workbench.Config{ScanRoots: []string{root}, Index: store})
	if err := wb.Open(); err != nil {
		t.Fatal(err)
	}
	desk := wb.Desk()
	if desk.RecycleIcon.Location.Row != 1 || desk.RecycleIcon.Location.Col != 1 {
		t.Fatalf("recycle default = %+v, want (1,1)", desk.RecycleIcon.Location)
	}
	for _, p := range desk.Placeholders {
		if p.Location.Kind != workbench.LocDesktop {
			continue
		}
		if p.Location.Row == 1 && p.Location.Col == 1 {
			t.Fatalf("placeholder stacked on recycle: %+v", p.Location)
		}
	}
}

// Location claims box but ItemIDs does not include the id → free desktop (ItemIDs wins).
func TestOpen_Rehome_LocationBoxWithoutMembership_GoesDesktop(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root+"/beta", "beta")
	betaID := mustRealpath(t, root+"/beta")

	store := index.NewMemoryStore()
	doc := index.Document{
		SchemaVersion: index.SchemaVersion,
		Skills:        []index.SkillRecord{{Identity: betaID, Name: "beta"}},
		Placeholders: []index.PlaceholderRecord{{
			ID: "ph_b", Identity: betaID,
			Location: index.Location{Kind: index.LocBox, BoxID: "box_empty"},
		}},
		RecycleIcon: index.Location{Kind: index.LocDesktop, Row: 1, Col: 1},
		Boxes: []index.BoxRecord{{
			ID: "box_empty", Kind: index.BoxSimple, Tag: "empty",
			X: 200, Y: 200, W: 240, H: 220,
			// ItemIDs empty — membership truth says not in box.
			ItemIDs: nil,
		}},
	}
	if err := store.Save(doc); err != nil {
		t.Fatal(err)
	}

	wb := workbench.New(workbench.Config{ScanRoots: []string{root}, Index: store})
	if err := wb.Open(); err != nil {
		t.Fatalf("Open: %v", err)
	}
	desk := wb.Desk()
	if len(desk.Boxes) != 1 || len(desk.Boxes[0].Items) != 0 {
		t.Fatalf("box must stay empty by ItemIDs: %+v", desk.Boxes)
	}
	found := false
	for _, p := range desk.Placeholders {
		if p.ID != "ph_b" {
			continue
		}
		found = true
		if p.Location.Kind != workbench.LocDesktop {
			t.Errorf("ghost location = %+v, want desktop free cell", p.Location)
		}
		if p.Location.Row < 1 || p.Location.Col < 1 {
			t.Errorf("invalid free cell %+v", p.Location)
		}
	}
	if !found {
		t.Fatal("ph_b missing from desk")
	}
}

// Happy path place/move: ItemIDs and Location agree after MovePlaceholderToBox.
func TestMoveToBox_MembershipAndLocationAgree(t *testing.T) {
	wb, store := openDeskWithSkills(t, "a", "b")
	desk := wb.Desk()
	a, b := phByName(t, desk, "a"), phByName(t, desk, "b")

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

	desk = wb.Desk()
	if len(desk.Boxes) != 1 || len(desk.Boxes[0].Items) != 2 {
		t.Fatalf("box items = %+v", desk.Boxes)
	}
	for _, it := range desk.Boxes[0].Items {
		if it.Location.Kind != workbench.LocBox || it.Location.BoxID != boxID {
			t.Errorf("item %s location = %+v, want box %s", it.Name, it.Location, boxID)
		}
	}

	// Persist agrees: ItemIDs is truth and Location matches.
	doc, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Boxes) != 1 || len(doc.Boxes[0].ItemIDs) != 2 {
		t.Fatalf("persisted ItemIDs = %+v", doc.Boxes)
	}
	idSet := map[string]bool{}
	for _, id := range doc.Boxes[0].ItemIDs {
		idSet[id] = true
	}
	for _, p := range doc.Placeholders {
		if !idSet[p.ID] {
			continue
		}
		if p.Location.Kind != index.LocBox || p.Location.BoxID != boxID {
			t.Errorf("persisted %s location = %+v", p.ID, p.Location)
		}
	}
}

// Failed mutation (bad compartment) leaves document equal to pre-op snapshot.
func TestMutation_BadCompartment_RollsBackDocument(t *testing.T) {
	wb, store := openDeskWithSkills(t, "a", "b")
	desk := wb.Desk()
	a := phByName(t, desk, "a")

	boxID, err := wb.CreateCompositeBox("combo", []string{"t1", "t2"}, 200, 200)
	if err != nil {
		t.Fatal(err)
	}
	// Snapshot desk before failed op.
	before := wb.Desk()
	beforeDoc, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}

	err = wb.MovePlaceholderToBox(a.ID, boxID, "no-such-compartment")
	if err == nil {
		t.Fatal("expected bad compartment error")
	}

	after := wb.Desk()
	if len(after.Boxes) != len(before.Boxes) {
		t.Fatalf("boxes changed on failed op: before %d after %d", len(before.Boxes), len(after.Boxes))
	}
	// a still live on desktop, not half-filed.
	for _, p := range after.Placeholders {
		if p.ID == a.ID && p.Location.Kind != workbench.LocDesktop {
			t.Errorf("a location after fail = %+v, want desktop", p.Location)
		}
	}
	// Box items empty (composite created empty).
	for _, b := range after.Boxes {
		if b.ID != boxID {
			continue
		}
		for _, c := range b.Compartments {
			if len(c.Items) != 0 {
				t.Errorf("compartment gained items on failed move: %+v", c.Items)
			}
		}
	}

	afterDoc, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(afterDoc.Placeholders) != len(beforeDoc.Placeholders) {
		t.Errorf("store placeholders changed on failed op")
	}
	// No partial ItemIDs.
	for _, b := range afterDoc.Boxes {
		if b.ID != boxID {
			continue
		}
		for _, c := range b.Compartments {
			if len(c.ItemIDs) != 0 {
				t.Errorf("persisted ItemIDs after fail = %v", c.ItemIDs)
			}
		}
	}
}

// Persist failure after in-memory mutation rolls document back; no half-applied desk.
func TestMutation_PersistFailure_RollsBackInMemory(t *testing.T) {
	root := t.TempDir()
	writeSkill(t, root+"/a", "a")
	writeSkill(t, root+"/b", "b")
	cs := newCountingStore()
	wb := workbench.New(workbench.Config{ScanRoots: []string{root}, Index: cs})
	if err := wb.Open(); err != nil {
		t.Fatal(err)
	}
	desk := wb.Desk()
	a, b := phByName(t, desk, "a"), phByName(t, desk, "b")
	boxID, err := wb.CreateSimpleBox("box", 200, 200)
	if err != nil {
		t.Fatal(err)
	}
	beforeSaves := cs.saveCount()

	// Next successful path mutates then Save fails → full doc rollback.
	cs.failNext = true
	err = wb.MovePlaceholderToBox(a.ID, boxID, "")
	if err == nil {
		t.Fatal("expected injected save failure")
	}
	if cs.saveCount() != beforeSaves {
		t.Errorf("failed Save should not count as success save; saves=%d before=%d", cs.saveCount(), beforeSaves)
	}

	desk = wb.Desk()
	// a must still be on desktop (not in box in memory).
	for _, p := range desk.Placeholders {
		if p.ID == a.ID && p.Location.Kind != workbench.LocDesktop {
			t.Errorf("in-memory a after failed persist = %+v, want desktop", p.Location)
		}
	}
	for _, box := range desk.Boxes {
		if box.ID == boxID && len(box.Items) != 0 {
			t.Errorf("box items after failed persist = %+v, want empty", box.Items)
		}
	}

	// Store still has pre-move state (empty box).
	doc, err := cs.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, box := range doc.Boxes {
		if box.ID == boxID && len(box.ItemIDs) != 0 {
			t.Errorf("store ItemIDs after fail = %v", box.ItemIDs)
		}
	}

	// Subsequent success works and persists once.
	beforeSaves = cs.saveCount()
	if err := wb.MovePlaceholderToBox(b.ID, boxID, ""); err != nil {
		t.Fatalf("retry: %v", err)
	}
	if cs.saveCount() != beforeSaves+1 {
		t.Errorf("successful op should persist once; saves delta = %d", cs.saveCount()-beforeSaves)
	}
	if n := len(wb.Desk().Boxes[0].Items); n != 1 {
		t.Fatalf("box items after success = %d, want 1", n)
	}
}

// Multi-step box op failure (compose self) leaves prior desk intact.
func TestCompose_Self_NoMutation(t *testing.T) {
	wb, _ := openDeskWithSkills(t, "a", "b")
	desk := wb.Desk()
	a, b := phByName(t, desk, "a"), phByName(t, desk, "b")
	if err := wb.MovePlaceholderToDesktop(b.ID, a.Location.Row, a.Location.Col); err != nil {
		t.Fatal(err)
	}
	boxID := wb.Desk().Boxes[0].ID
	before := len(wb.Desk().Boxes)

	err := wb.ComposeBoxes(boxID, boxID)
	if err == nil {
		t.Fatal("expected refuse compose self")
	}
	if len(wb.Desk().Boxes) != before {
		t.Errorf("box count changed on refuse")
	}
}

// Recycle enter-bin failure (last live) does not change membership/location.
func TestConfirmTrash_LastLive_RollsBackUnchanged(t *testing.T) {
	wb, store := openDeskWithSkills(t, "solo")
	ph := phByName(t, wb.Desk(), "solo")
	before, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := wb.ConfirmTrash([]string{ph.ID}); err == nil {
		t.Fatal("expected last-live refuse")
	}
	after, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Placeholders) != len(before.Placeholders) {
		t.Fatalf("placeholders changed: %d → %d", len(before.Placeholders), len(after.Placeholders))
	}
	if after.Placeholders[0].Location != before.Placeholders[0].Location {
		t.Errorf("location changed on refuse: %+v → %+v", before.Placeholders[0].Location, after.Placeholders[0].Location)
	}
}
