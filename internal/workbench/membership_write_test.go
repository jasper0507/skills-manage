package workbench_test

import (
	"testing"

	"github.com/jasper0507/skills-manage/internal/infra/index"
	"github.com/jasper0507/skills-manage/internal/workbench"
)

// E3.2: write path updates membership / desktop / recycle placement only.
// LocBox is a Desk projection, never durable dual-write on the index document.

func TestE32_MoveIntoBox_ItemIDsOnly_NoDurableLocBox(t *testing.T) {
	wb, store := openDeskWithSkills(t, "a", "b")
	desk := wb.Desk()
	a := phByName(t, desk, "a")
	homeRow, homeCol := a.Location.Row, a.Location.Col

	boxID, err := wb.CreateSimpleBox("box", 200, 200)
	if err != nil {
		t.Fatal(err)
	}
	if err := wb.MovePlaceholderToBox(a.ID, boxID, ""); err != nil {
		t.Fatal(err)
	}

	// Desk projects in-box Location from membership.
	desk = wb.Desk()
	if len(desk.Boxes) != 1 || len(desk.Boxes[0].Items) != 1 {
		t.Fatalf("box items = %+v", desk.Boxes)
	}
	it := desk.Boxes[0].Items[0]
	if it.ID != a.ID || it.Location.Kind != workbench.LocBox || it.Location.BoxID != boxID {
		t.Errorf("desk projection = %+v, want LocBox %s", it, boxID)
	}
	assertMemberPlacementEmpty(t, store, a.ID)

	// Vacated desktop cell is free for another icon (collision ignores members).
	b := phByName(t, desk, "b")
	if err := wb.MovePlaceholderToDesktop(b.ID, homeRow, homeCol); err != nil {
		t.Fatalf("place onto vacated cell: %v", err)
	}
	desk = wb.Desk()
	b = phByName(t, desk, "b")
	if b.Location.Kind != workbench.LocDesktop || b.Location.Row != homeRow || b.Location.Col != homeCol {
		t.Errorf("b after place = %+v, want desktop (%d,%d)", b.Location, homeRow, homeCol)
	}
}

func TestE32_LeaveBox_DesktopPlace_ClearsMembershipSetsDesktopOnly(t *testing.T) {
	wb, store := openDeskWithSkills(t, "a", "b")
	desk := wb.Desk()
	a := phByName(t, desk, "a")
	boxID, err := wb.CreateSimpleBox("box", 200, 200)
	if err != nil {
		t.Fatal(err)
	}
	if err := wb.MovePlaceholderToBox(a.ID, boxID, ""); err != nil {
		t.Fatal(err)
	}

	if err := wb.MovePlaceholderToDesktop(a.ID, 4, 5); err != nil {
		t.Fatal(err)
	}

	desk = wb.Desk()
	a = phByName(t, desk, "a")
	if a.Location.Kind != workbench.LocDesktop || a.Location.Row != 4 || a.Location.Col != 5 {
		t.Errorf("desk a = %+v, want desktop 4,5", a.Location)
	}
	for _, box := range desk.Boxes {
		if box.ID == boxID && len(box.Items) != 0 {
			t.Errorf("box still has members: %+v", box.Items)
		}
	}

	doc, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, box := range doc.Boxes {
		if box.ID == boxID && len(box.ItemIDs) != 0 {
			t.Errorf("persisted ItemIDs after leave = %v", box.ItemIDs)
		}
	}
	for _, p := range doc.Placeholders {
		if p.ID != a.ID {
			continue
		}
		if p.Location.Kind != index.LocDesktop || p.Location.Row != 4 || p.Location.Col != 5 {
			t.Errorf("document placement = %+v, want desktop 4,5", p.Location)
		}
	}
}

func TestE32_DeleteBox_MembersReturnDesktopOnly(t *testing.T) {
	wb, store := openDeskWithSkills(t, "a", "b")
	desk := wb.Desk()
	a, b := phByName(t, desk, "a"), phByName(t, desk, "b")
	if err := wb.MovePlaceholderToDesktop(b.ID, a.Location.Row, a.Location.Col); err != nil {
		t.Fatal(err)
	}
	desk = wb.Desk()
	boxID := desk.Boxes[0].ID

	if err := wb.DeleteBox(boxID); err != nil {
		t.Fatal(err)
	}

	desk = wb.Desk()
	if len(desk.Boxes) != 0 {
		t.Fatalf("boxes after delete = %+v", desk.Boxes)
	}
	for _, name := range []string{"a", "b"} {
		p := phByName(t, desk, name)
		if p.Location.Kind != workbench.LocDesktop {
			t.Errorf("%s kind = %q, want desktop", name, p.Location.Kind)
		}
	}

	doc, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Boxes) != 0 {
		t.Errorf("persisted boxes = %+v", doc.Boxes)
	}
	for _, p := range doc.Placeholders {
		if p.Location.Kind != index.LocDesktop {
			t.Errorf("doc %s location = %+v, want desktop only", p.ID, p.Location)
		}
	}
}

func TestE32_CutPasteDesktopFromBox_ClearsMembership(t *testing.T) {
	wb, store := openDeskWithSkills(t, "a", "b")
	desk := wb.Desk()
	a := phByName(t, desk, "a")
	boxID, err := wb.CreateSimpleBox("box", 200, 200)
	if err != nil {
		t.Fatal(err)
	}
	if err := wb.MovePlaceholderToBox(a.ID, boxID, ""); err != nil {
		t.Fatal(err)
	}
	if err := wb.SetClipboard(workbench.ClipCut, []string{a.ID}); err != nil {
		t.Fatal(err)
	}
	if err := wb.PasteToDesktop(6, 7); err != nil {
		t.Fatal(err)
	}

	desk = wb.Desk()
	a = phByName(t, desk, "a")
	if a.Location.Kind != workbench.LocDesktop {
		t.Errorf("a = %+v, want desktop", a.Location)
	}
	doc, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, box := range doc.Boxes {
		if containsStr(box.ItemIDs, a.ID) {
			t.Errorf("a still in ItemIDs after cut-paste desktop: %v", box.ItemIDs)
		}
	}
}

func TestE32_RecycleEnter_StripsMembership_RecycleOnly(t *testing.T) {
	// Two live placeholders for same identity so enter-bin is allowed (last-live guard).
	root := t.TempDir()
	writeSkill(t, root+"/alpha", "alpha")
	alphaID := mustRealpath(t, root+"/alpha")
	store := index.NewMemoryStore()
	doc := index.Document{
		SchemaVersion: index.SchemaVersion,
		Skills:        []index.SkillRecord{{Identity: alphaID, Name: "alpha"}},
		Placeholders: []index.PlaceholderRecord{
			{ID: "ph_live", Identity: alphaID, Location: index.Location{Kind: index.LocDesktop, Row: 1, Col: 2}},
			{ID: "ph_box", Identity: alphaID, Location: index.Location{}},
		},
		RecycleIcon: index.Location{Kind: index.LocDesktop, Row: 1, Col: 1},
		Boxes: []index.BoxRecord{{
			ID: "box_1", Kind: index.BoxSimple, Tag: "t",
			X: 200, Y: 200, W: 240, H: 220,
			ItemIDs: []string{"ph_box"},
		}},
	}
	if err := store.Save(doc); err != nil {
		t.Fatal(err)
	}
	wb := workbench.New(workbench.Config{ScanRoots: []string{root}, Index: store})
	if err := wb.Open(); err != nil {
		t.Fatal(err)
	}

	if err := wb.ConfirmTrash([]string{"ph_box"}); err != nil {
		t.Fatalf("ConfirmTrash: %v", err)
	}

	desk := wb.Desk()
	for _, p := range desk.Placeholders {
		if p.ID == "ph_box" && p.Location.Kind != workbench.LocRecycle {
			t.Errorf("desk ph_box = %+v, want recycle", p.Location)
		}
	}
	for _, box := range desk.Boxes {
		if len(box.Items) != 0 {
			t.Errorf("box still has items after trash: %+v", box.Items)
		}
	}

	saved, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, box := range saved.Boxes {
		if containsStr(box.ItemIDs, "ph_box") {
			t.Errorf("membership still lists recycled ph: %v", box.ItemIDs)
		}
	}
	for _, p := range saved.Placeholders {
		if p.ID != "ph_box" {
			continue
		}
		if p.Location.Kind != index.LocRecycle {
			t.Errorf("doc recycle placement = %+v", p.Location)
		}
		if p.Location.Row != 0 || p.Location.Col != 0 || p.Location.BoxID != "" {
			t.Errorf("recycle placement must be kind-only: %+v", p.Location)
		}
	}

	// Restore → desktop only, still not a box member.
	if err := wb.Restore("ph_box"); err != nil {
		t.Fatal(err)
	}
	saved, err = store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range saved.Placeholders {
		if p.ID != "ph_box" {
			continue
		}
		if p.Location.Kind != index.LocDesktop || p.Location.Row < 1 || p.Location.Col < 1 {
			t.Errorf("restored placement = %+v, want free desktop", p.Location)
		}
	}
	for _, box := range saved.Boxes {
		if containsStr(box.ItemIDs, "ph_box") {
			t.Errorf("restore must not re-join box: %v", box.ItemIDs)
		}
	}
}

func TestE32_MoveBetweenBoxes_OneMembershipOnly(t *testing.T) {
	wb, store := openDeskWithSkills(t, "a", "b", "c")
	desk := wb.Desk()
	a, b, c := phByName(t, desk, "a"), phByName(t, desk, "b"), phByName(t, desk, "c")

	box1, err := wb.CreateSimpleBox("one", 200, 200)
	if err != nil {
		t.Fatal(err)
	}
	box2, err := wb.CreateSimpleBox("two", 500, 200)
	if err != nil {
		t.Fatal(err)
	}
	if err := wb.MovePlaceholderToBox(a.ID, box1, ""); err != nil {
		t.Fatal(err)
	}
	// Move a from box1 → box2; b stays free, c into box2 for company.
	if err := wb.MovePlaceholderToBox(a.ID, box2, ""); err != nil {
		t.Fatal(err)
	}
	if err := wb.MovePlaceholderToBox(c.ID, box2, ""); err != nil {
		t.Fatal(err)
	}
	_ = b

	assertMemberPlacementEmpty(t, store, a.ID)
	assertMemberPlacementEmpty(t, store, c.ID)
	// Source box empty of a; target holds a.
	doc, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, box := range doc.Boxes {
		if box.ID == box1 && containsStr(box.ItemIDs, a.ID) {
			t.Errorf("a still in source box ItemIDs: %v", box.ItemIDs)
		}
		if box.ID == box2 && !containsStr(box.ItemIDs, a.ID) {
			t.Errorf("a missing from target ItemIDs: %v", box.ItemIDs)
		}
	}
}

func TestE32_AutoBoxAndPasteCopy_NoDurableLocBox(t *testing.T) {
	wb, store := openDeskWithSkills(t, "alpha", "beta", "gamma")
	desk := wb.Desk()
	alpha, beta, gamma := phByName(t, desk, "alpha"), phByName(t, desk, "beta"), phByName(t, desk, "gamma")

	// Icon-on-icon auto-box.
	if err := wb.MovePlaceholderToDesktop(beta.ID, alpha.Location.Row, alpha.Location.Col); err != nil {
		t.Fatal(err)
	}
	desk = wb.Desk()
	boxID := desk.Boxes[0].ID

	// Copy gamma into box.
	if err := wb.SetClipboard(workbench.ClipCopy, []string{gamma.ID}); err != nil {
		t.Fatal(err)
	}
	if err := wb.PasteToBox(boxID, ""); err != nil {
		t.Fatal(err)
	}

	// Desk projects box; document members have empty placement only.
	desk = wb.Desk()
	if n := len(desk.Boxes[0].Items); n != 3 {
		t.Fatalf("desk box items = %d, want 3", n)
	}
	for _, it := range desk.Boxes[0].Items {
		if it.Location.Kind != workbench.LocBox || it.Location.BoxID != boxID {
			t.Errorf("projected %s = %+v", it.Name, it.Location)
		}
		assertMemberPlacementEmpty(t, store, it.ID)
	}
}

func TestE32_ComposeEject_MembershipOnly(t *testing.T) {
	wb, store := openDeskWithSkills(t, "a", "b", "c", "d")
	desk := wb.Desk()
	a, b := phByName(t, desk, "a"), phByName(t, desk, "b")
	c, d := phByName(t, desk, "c"), phByName(t, desk, "d")
	if err := wb.MovePlaceholderToDesktop(b.ID, a.Location.Row, a.Location.Col); err != nil {
		t.Fatal(err)
	}
	if err := wb.MovePlaceholderToDesktop(d.ID, c.Location.Row, c.Location.Col); err != nil {
		t.Fatal(err)
	}
	desk = wb.Desk()
	src, tgt := desk.Boxes[0].ID, desk.Boxes[1].ID
	if err := wb.ComposeBoxes(src, tgt); err != nil {
		t.Fatal(err)
	}
	desk = wb.Desk()
	if desk.Boxes[0].Kind != workbench.BoxComposite {
		t.Fatalf("want composite, got %+v", desk.Boxes)
	}
	compID := desk.Boxes[0].ID
	// Eject first compartment.
	cmp0 := desk.Boxes[0].Compartments[0].ID
	if err := wb.EjectCompartment(compID, cmp0, 600, 400); err != nil {
		t.Fatal(err)
	}
	// Structure ops only reshuffle ItemIDs; still one claim per id, no LocBox.
	assertExactlyOneMembership(t, store)
}
