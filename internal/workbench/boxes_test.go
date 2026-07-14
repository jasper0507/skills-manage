package workbench_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/jasper0507/skills-manage/internal/index"
	"github.com/jasper0507/skills-manage/internal/workbench"
)

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

func TestBox_IconOnIcon_CreatesSequencedSimpleBox(t *testing.T) {
	wb, _ := openDeskWithSkills(t, "alpha", "beta")
	desk := wb.Desk()
	alpha := phByName(t, desk, "alpha")
	beta := phByName(t, desk, "beta")

	// Drop beta onto alpha's cell → auto simple box containing both.
	if err := wb.MovePlaceholderToDesktop(beta.ID, alpha.Location.Row, alpha.Location.Col); err != nil {
		t.Fatalf("MovePlaceholderToDesktop: %v", err)
	}

	desk = wb.Desk()
	if len(desk.Boxes) != 1 {
		t.Fatalf("got %d boxes, want 1: %+v", len(desk.Boxes), desk.Boxes)
	}
	box := desk.Boxes[0]
	if box.Kind != workbench.BoxSimple {
		t.Errorf("kind = %q, want simple", box.Kind)
	}
	if !strings.HasPrefix(box.Tag, "普通盒子") {
		t.Errorf("tag = %q, want sequenced 普通盒子N", box.Tag)
	}
	if box.Tag != "普通盒子1" {
		t.Errorf("first auto box tag = %q, want 普通盒子1", box.Tag)
	}

	// Both placeholders inside, visible as icons on the box.
	if len(box.Items) != 2 {
		t.Fatalf("box items = %d, want 2: %+v", len(box.Items), box.Items)
	}
	names := map[string]bool{}
	for _, it := range box.Items {
		names[it.Name] = true
		if it.Location.Kind != workbench.LocBox {
			t.Errorf("%s location kind = %q, want box", it.Name, it.Location.Kind)
		}
		if it.Location.BoxID != box.ID {
			t.Errorf("%s boxId = %q, want %q", it.Name, it.Location.BoxID, box.ID)
		}
	}
	if !names["alpha"] || !names["beta"] {
		t.Errorf("box items = %v, want alpha and beta", names)
	}

	// Neither remains as a free desktop skill icon.
	for _, p := range desk.Placeholders {
		if p.Name == "alpha" || p.Name == "beta" {
			if p.Location.Kind == workbench.LocDesktop {
				t.Errorf("%s still on desktop after auto-box", p.Name)
			}
		}
	}
}

func TestBox_IconOnIcon_SequencesDefaultNames(t *testing.T) {
	wb, _ := openDeskWithSkills(t, "a", "b", "c", "d")
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
	if len(desk.Boxes) != 2 {
		t.Fatalf("got %d boxes, want 2", len(desk.Boxes))
	}
	tags := map[string]bool{}
	for _, box := range desk.Boxes {
		tags[box.Tag] = true
	}
	if !tags["普通盒子1"] || !tags["普通盒子2"] {
		t.Errorf("tags = %v, want 普通盒子1 and 普通盒子2", tags)
	}
}

func TestBox_IconOnBox_JoinsCurrentCompartment(t *testing.T) {
	wb, _ := openDeskWithSkills(t, "alpha", "beta", "gamma")
	desk := wb.Desk()
	alpha, beta, gamma := phByName(t, desk, "alpha"), phByName(t, desk, "beta"), phByName(t, desk, "gamma")

	// Create simple box from alpha+beta.
	if err := wb.MovePlaceholderToDesktop(beta.ID, alpha.Location.Row, alpha.Location.Col); err != nil {
		t.Fatal(err)
	}
	desk = wb.Desk()
	if len(desk.Boxes) != 1 {
		t.Fatalf("setup: want 1 box, got %d", len(desk.Boxes))
	}
	boxID := desk.Boxes[0].ID

	// Drop gamma into that box (simple → single current compartment).
	if err := wb.MovePlaceholderToBox(gamma.ID, boxID, ""); err != nil {
		t.Fatalf("MovePlaceholderToBox: %v", err)
	}
	desk = wb.Desk()
	box := desk.Boxes[0]
	if len(box.Items) != 3 {
		t.Fatalf("items = %d, want 3: %+v", len(box.Items), box.Items)
	}
	names := map[string]bool{}
	for _, it := range box.Items {
		names[it.Name] = true
	}
	if !names["gamma"] {
		t.Errorf("gamma not in box: %v", names)
	}
	g := phByName(t, desk, "gamma")
	if g.Location.Kind != workbench.LocBox || g.Location.BoxID != boxID {
		t.Errorf("gamma location = %+v", g.Location)
	}
}

func TestBox_IconOnComposite_JoinsActiveCompartment(t *testing.T) {
	wb, _ := openDeskWithSkills(t, "a", "b", "c", "d", "e")
	desk := wb.Desk()
	// Two simple boxes → compose → composite with two compartments.
	a, b := phByName(t, desk, "a"), phByName(t, desk, "b")
	c, d := phByName(t, desk, "c"), phByName(t, desk, "d")
	e := phByName(t, desk, "e")
	if err := wb.MovePlaceholderToDesktop(b.ID, a.Location.Row, a.Location.Col); err != nil {
		t.Fatal(err)
	}
	if err := wb.MovePlaceholderToDesktop(d.ID, c.Location.Row, c.Location.Col); err != nil {
		t.Fatal(err)
	}
	desk = wb.Desk()
	src, tgt := desk.Boxes[0].ID, desk.Boxes[1].ID
	// Compose source into target (order: simple→simple).
	if err := wb.ComposeBoxes(src, tgt); err != nil {
		t.Fatalf("ComposeBoxes: %v", err)
	}
	desk = wb.Desk()
	if len(desk.Boxes) != 1 || desk.Boxes[0].Kind != workbench.BoxComposite {
		t.Fatalf("want one composite, got %+v", desk.Boxes)
	}
	comp := desk.Boxes[0]
	if len(comp.Compartments) != 2 {
		t.Fatalf("want 2 compartments, got %+v", comp.Compartments)
	}
	// Active should be first (target's original tag compartment).
	active := comp.ActiveCompartmentID
	if active == "" {
		t.Fatal("active compartment empty")
	}
	if err := wb.MovePlaceholderToBox(e.ID, comp.ID, ""); err != nil {
		t.Fatalf("MovePlaceholderToBox: %v", err)
	}
	desk = wb.Desk()
	comp = desk.Boxes[0]
	var found bool
	for _, c := range comp.Compartments {
		if c.ID != active {
			continue
		}
		for _, it := range c.Items {
			if it.Name == "e" {
				found = true
			}
		}
	}
	if !found {
		t.Errorf("e not in active compartment %q: %+v", active, comp.Compartments)
	}
}

func TestBox_Move_CannotCoverDesktopSkillIcons(t *testing.T) {
	wb, _ := openDeskWithSkills(t, "a", "b", "stay")
	desk := wb.Desk()
	a, b := phByName(t, desk, "a"), phByName(t, desk, "b")
	stay := phByName(t, desk, "stay")
	if err := wb.MovePlaceholderToDesktop(b.ID, a.Location.Row, a.Location.Col); err != nil {
		t.Fatal(err)
	}
	desk = wb.Desk()
	box := desk.Boxes[0]

	// Pixel rect covering stay's icon cell (prototype grid).
	// stay is at some (row,col); convert to icon origin.
	coverX := float64(16 + (stay.Location.Col-1)*90)
	coverY := float64(16 + (stay.Location.Row-1)*96)

	if err := wb.MoveBox(box.ID, coverX, coverY); err != nil {
		t.Fatalf("MoveBox: %v", err)
	}
	desk = wb.Desk()
	moved := desk.Boxes[0]
	// Must not sit exactly on the covering coords if that would overlap stay.
	// Either nudged away or refused by leaving a non-overlapping position.
	if boxOverlapsIcon(moved.X, moved.Y, moved.W, moved.H, stay.Location.Row, stay.Location.Col) {
		t.Errorf("box at (%.0f,%.0f) still covers skill icon at cell (%d,%d)",
			moved.X, moved.Y, stay.Location.Row, stay.Location.Col)
	}
}

func boxOverlapsIcon(bx, by, bw, bh float64, row, col int) bool {
	ix := float64(16 + (col-1)*90)
	iy := float64(16 + (row-1)*96)
	const pad = 2.0
	const iw, ih = 86.0, 90.0
	return bx+pad < ix+iw-pad &&
		bx+bw-pad > ix+pad &&
		by+pad < iy+ih-pad &&
		by+bh-pad > iy+pad
}

func TestBox_Compose_SimpleIntoSimple(t *testing.T) {
	wb, _ := openDeskWithSkills(t, "a", "b", "c", "d")
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
	// Rename so tags are predictable for compose.
	boxA, boxB := desk.Boxes[0], desk.Boxes[1]
	if err := wb.RenameBoxTag(boxA.ID, "design", ""); err != nil {
		t.Fatal(err)
	}
	if err := wb.RenameBoxTag(boxB.ID, "eng", ""); err != nil {
		t.Fatal(err)
	}
	// simple design → simple eng: title from target eng
	if err := wb.ComposeBoxes(boxA.ID, boxB.ID); err != nil {
		t.Fatalf("ComposeBoxes: %v", err)
	}
	desk = wb.Desk()
	if len(desk.Boxes) != 1 {
		t.Fatalf("got %d boxes, want 1", len(desk.Boxes))
	}
	comp := desk.Boxes[0]
	if comp.Kind != workbench.BoxComposite {
		t.Fatalf("kind = %q, want composite", comp.Kind)
	}
	if comp.Title != "eng" {
		t.Errorf("title = %q, want eng (from target)", comp.Title)
	}
	if len(comp.Compartments) != 2 {
		t.Fatalf("compartments = %d, want 2", len(comp.Compartments))
	}
	tags := map[string]bool{}
	for _, c := range comp.Compartments {
		tags[c.Tag] = true
		if len(c.Items) == 0 {
			t.Errorf("compartment %q has no icons", c.Tag)
		}
	}
	if !tags["eng"] || !tags["design"] {
		t.Errorf("compartment tags = %v, want eng and design", tags)
	}
}

func TestBox_Compose_SimpleIntoComposite_AppendsCompartment(t *testing.T) {
	wb, _ := openDeskWithSkills(t, "a", "b", "c", "d", "e", "f")
	desk := wb.Desk()
	// Three simple boxes.
	pairs := [][2]string{{"a", "b"}, {"c", "d"}, {"e", "f"}}
	for _, pair := range pairs {
		p0 := phByName(t, desk, pair[0])
		p1 := phByName(t, desk, pair[1])
		if err := wb.MovePlaceholderToDesktop(p1.ID, p0.Location.Row, p0.Location.Col); err != nil {
			t.Fatal(err)
		}
		desk = wb.Desk()
	}
	if len(desk.Boxes) != 3 {
		t.Fatalf("setup: want 3 boxes, got %d", len(desk.Boxes))
	}
	// Rename for clarity.
	_ = wb.RenameBoxTag(desk.Boxes[0].ID, "t1", "")
	_ = wb.RenameBoxTag(desk.Boxes[1].ID, "t2", "")
	_ = wb.RenameBoxTag(desk.Boxes[2].ID, "t3", "")
	// Compose first two into composite, then append third.
	if err := wb.ComposeBoxes(desk.Boxes[0].ID, desk.Boxes[1].ID); err != nil {
		t.Fatal(err)
	}
	desk = wb.Desk()
	var compositeID, simpleID string
	for _, b := range desk.Boxes {
		if b.Kind == workbench.BoxComposite {
			compositeID = b.ID
		} else {
			simpleID = b.ID
		}
	}
	if compositeID == "" || simpleID == "" {
		t.Fatalf("want composite + simple, got %+v", desk.Boxes)
	}
	if err := wb.ComposeBoxes(simpleID, compositeID); err != nil {
		t.Fatalf("simple→composite: %v", err)
	}
	desk = wb.Desk()
	if len(desk.Boxes) != 1 {
		t.Fatalf("got %d boxes, want 1", len(desk.Boxes))
	}
	comp := desk.Boxes[0]
	if comp.Kind != workbench.BoxComposite {
		t.Fatalf("kind = %q", comp.Kind)
	}
	if len(comp.Compartments) != 3 {
		t.Fatalf("compartments = %d, want 3: %+v", len(comp.Compartments), comp.Compartments)
	}
}

func TestBox_Compose_CompositeIntoComposite_Refused(t *testing.T) {
	wb, _ := openDeskWithSkills(t, "a", "b", "c", "d", "e", "f", "g", "h")
	desk := wb.Desk()
	// Four pairs → four simple boxes → two composites.
	for _, pair := range [][2]string{{"a", "b"}, {"c", "d"}, {"e", "f"}, {"g", "h"}} {
		p0 := phByName(t, desk, pair[0])
		p1 := phByName(t, desk, pair[1])
		if err := wb.MovePlaceholderToDesktop(p1.ID, p0.Location.Row, p0.Location.Col); err != nil {
			t.Fatal(err)
		}
		desk = wb.Desk()
	}
	ids := []string{desk.Boxes[0].ID, desk.Boxes[1].ID, desk.Boxes[2].ID, desk.Boxes[3].ID}
	if err := wb.ComposeBoxes(ids[0], ids[1]); err != nil {
		t.Fatal(err)
	}
	if err := wb.ComposeBoxes(ids[2], ids[3]); err != nil {
		t.Fatal(err)
	}
	desk = wb.Desk()
	if len(desk.Boxes) != 2 {
		t.Fatalf("setup: want 2 composites, got %+v", desk.Boxes)
	}
	err := wb.ComposeBoxes(desk.Boxes[0].ID, desk.Boxes[1].ID)
	if err == nil {
		t.Fatal("expected composite→composite to be refused")
	}
	if !strings.Contains(err.Error(), "composite") {
		t.Errorf("error = %v, want mention of composite", err)
	}
	desk = wb.Desk()
	if len(desk.Boxes) != 2 {
		t.Errorf("boxes mutated after refused merge: %d", len(desk.Boxes))
	}
}

func TestBox_EjectCompartment_BecomesSimple_AndDemotes(t *testing.T) {
	wb, _ := openDeskWithSkills(t, "a", "b", "c", "d")
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
	_ = wb.RenameBoxTag(desk.Boxes[0].ID, "left", "")
	_ = wb.RenameBoxTag(desk.Boxes[1].ID, "right", "")
	if err := wb.ComposeBoxes(desk.Boxes[0].ID, desk.Boxes[1].ID); err != nil {
		t.Fatal(err)
	}
	desk = wb.Desk()
	comp := desk.Boxes[0]
	// Eject one compartment → new simple; remaining one demotes to simple.
	ejectID := comp.Compartments[1].ID
	ejectTag := comp.Compartments[1].Tag
	if err := wb.EjectCompartment(comp.ID, ejectID, 400, 100); err != nil {
		t.Fatalf("EjectCompartment: %v", err)
	}
	desk = wb.Desk()
	if len(desk.Boxes) != 2 {
		t.Fatalf("got %d boxes, want 2 (ejected simple + demoted simple)", len(desk.Boxes))
	}
	for _, box := range desk.Boxes {
		if box.Kind != workbench.BoxSimple {
			t.Errorf("box %q kind = %q, want simple (demote/eject)", box.ID, box.Kind)
		}
		if len(box.Compartments) != 0 {
			t.Errorf("simple box still has compartments: %+v", box.Compartments)
		}
		if len(box.Items) == 0 {
			t.Errorf("box %q (#%s) has no icons", box.ID, box.Tag)
		}
	}
	tags := map[string]bool{}
	for _, box := range desk.Boxes {
		tags[box.Tag] = true
	}
	if !tags[ejectTag] {
		t.Errorf("ejected tag %q missing; tags=%v", ejectTag, tags)
	}
}

func TestBox_Rename_SimpleTag_CompartmentTag_Title(t *testing.T) {
	wb, _ := openDeskWithSkills(t, "a", "b", "c", "d")
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
	if err := wb.RenameBoxTag(desk.Boxes[0].ID, "renamed-simple", ""); err != nil {
		t.Fatal(err)
	}
	desk = wb.Desk()
	if desk.Boxes[0].Tag != "renamed-simple" {
		t.Errorf("simple tag = %q", desk.Boxes[0].Tag)
	}

	if err := wb.ComposeBoxes(desk.Boxes[0].ID, desk.Boxes[1].ID); err != nil {
		t.Fatal(err)
	}
	desk = wb.Desk()
	comp := desk.Boxes[0]
	if err := wb.RenameBoxTitle(comp.ID, "Go 开发"); err != nil {
		t.Fatal(err)
	}
	cid := comp.Compartments[0].ID
	if err := wb.RenameBoxTag(comp.ID, "单元测试", cid); err != nil {
		t.Fatal(err)
	}
	desk = wb.Desk()
	comp = desk.Boxes[0]
	if comp.Title != "Go 开发" {
		t.Errorf("title = %q", comp.Title)
	}
	found := false
	for _, c := range comp.Compartments {
		if c.ID == cid && c.Tag == "单元测试" {
			found = true
		}
	}
	if !found {
		t.Errorf("compartment tag not renamed: %+v", comp.Compartments)
	}
}

func TestBox_Delete_ReturnsPlaceholdersToFreeCells(t *testing.T) {
	wb, store := openDeskWithSkills(t, "a", "b", "c")
	desk := wb.Desk()
	a, b := phByName(t, desk, "a"), phByName(t, desk, "b")
	if err := wb.MovePlaceholderToDesktop(b.ID, a.Location.Row, a.Location.Col); err != nil {
		t.Fatal(err)
	}
	desk = wb.Desk()
	boxID := desk.Boxes[0].ID
	// Put c into the box too.
	c := phByName(t, desk, "c")
	if err := wb.MovePlaceholderToBox(c.ID, boxID, ""); err != nil {
		t.Fatal(err)
	}

	if err := wb.DeleteBox(boxID); err != nil {
		t.Fatalf("DeleteBox: %v", err)
	}
	desk = wb.Desk()
	if len(desk.Boxes) != 0 {
		t.Fatalf("boxes remain: %+v", desk.Boxes)
	}
	// All three placeholders back on desktop free cells; no body delete.
	if len(desk.Placeholders) != 3 {
		t.Fatalf("placeholders = %d, want 3", len(desk.Placeholders))
	}
	seen := map[string]string{}
	for _, p := range desk.Placeholders {
		if p.Location.Kind != workbench.LocDesktop {
			t.Errorf("%s kind = %q, want desktop", p.Name, p.Location.Kind)
		}
		key := fmtCell(p.Location.Row, p.Location.Col)
		if other, ok := seen[key]; ok {
			t.Errorf("cell %s shared by %s and %s", key, other, p.Name)
		}
		seen[key] = p.Name
	}
	// Recycle still present; no skill body deleted (still on desk as placeholders).
	if desk.RecycleIcon.Location.Kind != workbench.LocDesktop {
		t.Errorf("recycle location = %+v", desk.RecycleIcon.Location)
	}

	// Index persists the resulting structure (empty boxes, desktop placeholders).
	doc, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Boxes) != 0 {
		t.Errorf("index boxes = %+v", doc.Boxes)
	}
	if len(doc.Placeholders) != 3 {
		t.Errorf("index placeholders = %d", len(doc.Placeholders))
	}
}

func TestBox_MovePlaceholderToBox_BadCompartmentLeavesMembershipIntact(t *testing.T) {
	wb, _ := openDeskWithSkills(t, "a", "b", "c", "d", "e")
	desk := wb.Desk()
	a, b := phByName(t, desk, "a"), phByName(t, desk, "b")
	c, d := phByName(t, desk, "c"), phByName(t, desk, "d")
	e := phByName(t, desk, "e")
	if err := wb.MovePlaceholderToDesktop(b.ID, a.Location.Row, a.Location.Col); err != nil {
		t.Fatal(err)
	}
	if err := wb.MovePlaceholderToDesktop(d.ID, c.Location.Row, c.Location.Col); err != nil {
		t.Fatal(err)
	}
	desk = wb.Desk()
	if err := wb.ComposeBoxes(desk.Boxes[0].ID, desk.Boxes[1].ID); err != nil {
		t.Fatal(err)
	}
	desk = wb.Desk()
	comp := desk.Boxes[0]
	// Park e in the composite active compartment first.
	if err := wb.MovePlaceholderToBox(e.ID, comp.ID, ""); err != nil {
		t.Fatal(err)
	}
	desk = wb.Desk()
	beforeItems := 0
	for _, c := range desk.Boxes[0].Compartments {
		beforeItems += len(c.Items)
	}

	err := wb.MovePlaceholderToBox(e.ID, comp.ID, "cmp_does_not_exist")
	if err == nil {
		t.Fatal("expected error for unknown compartment")
	}
	desk = wb.Desk()
	afterItems := 0
	var stillHasE bool
	for _, c := range desk.Boxes[0].Compartments {
		afterItems += len(c.Items)
		for _, it := range c.Items {
			if it.ID == e.ID {
				stillHasE = true
			}
		}
	}
	if afterItems != beforeItems {
		t.Errorf("item count changed on failed move: %d → %d", beforeItems, afterItems)
	}
	if !stillHasE {
		t.Error("e dropped from compartments after failed MovePlaceholderToBox")
	}
}

func TestBox_StructurePersistsAcrossRestart(t *testing.T) {
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
	_ = wb.RenameBoxTag(desk.Boxes[0].ID, "one", "")
	_ = wb.RenameBoxTag(desk.Boxes[1].ID, "two", "")
	if err := wb.ComposeBoxes(desk.Boxes[0].ID, desk.Boxes[1].ID); err != nil {
		t.Fatal(err)
	}
	before := wb.Desk()

	// Restart with same store; scan root is parent of skill packages.
	scanRoot := filepath.Dir(before.Placeholders[0].Identity)
	wb2 := newWB(t, []string{scanRoot}, store)
	if err := wb2.Open(); err != nil {
		t.Fatalf("re-Open: %v", err)
	}
	after := wb2.Desk()
	if len(after.Boxes) != 1 {
		t.Fatalf("boxes after restart = %d, want 1", len(after.Boxes))
	}
	if after.Boxes[0].Kind != workbench.BoxComposite {
		t.Errorf("kind = %q", after.Boxes[0].Kind)
	}
	if after.Boxes[0].Title != before.Boxes[0].Title {
		t.Errorf("title %q → %q", before.Boxes[0].Title, after.Boxes[0].Title)
	}
	if len(after.Boxes[0].Compartments) != 2 {
		t.Errorf("compartments = %+v", after.Boxes[0].Compartments)
	}
	// Items still visible as icons in compartments.
	totalItems := 0
	for _, c := range after.Boxes[0].Compartments {
		totalItems += len(c.Items)
	}
	if totalItems != 4 {
		t.Errorf("total compartment icons = %d, want 4", totalItems)
	}
}
