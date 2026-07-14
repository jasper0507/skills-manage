package workbench_test

import (
	"path/filepath"
	"testing"

	"github.com/jasper0507/skills-manage/internal/workbench"
)

func TestClipboard_CopyPasteToDesktop_CreatesPlaceholderSameIdentity(t *testing.T) {
	wb, store := openDeskWithSkills(t, "alpha")
	desk := wb.Desk()
	alpha := phByName(t, desk, "alpha")
	wantIdentity := alpha.Identity

	if err := wb.SetClipboard(workbench.ClipCopy, []string{alpha.ID}); err != nil {
		t.Fatalf("SetClipboard: %v", err)
	}
	desk = wb.Desk()
	if desk.Clipboard == nil || desk.Clipboard.Mode != workbench.ClipCopy {
		t.Fatalf("clipboard = %+v, want copy mode", desk.Clipboard)
	}

	// Paste onto a free cell (row 5, col 2).
	if err := wb.PasteToDesktop(5, 2); err != nil {
		t.Fatalf("PasteToDesktop: %v", err)
	}

	desk = wb.Desk()
	if len(desk.Placeholders) != 2 {
		t.Fatalf("got %d placeholders, want 2 (original + copy): %+v", len(desk.Placeholders), desk.Placeholders)
	}
	// Both share skill identity; copy is icon-only (no second skill package).
	ids := map[string]bool{}
	var copyPh workbench.Placeholder
	for _, p := range desk.Placeholders {
		if p.Identity != wantIdentity {
			t.Errorf("placeholder %s identity = %q, want %q", p.ID, p.Identity, wantIdentity)
		}
		ids[p.ID] = true
		if p.ID != alpha.ID {
			copyPh = p
		}
	}
	if len(ids) != 2 {
		t.Fatalf("placeholder ids not unique: %v", ids)
	}
	if copyPh.Location.Kind != workbench.LocDesktop {
		t.Errorf("copy location kind = %q, want desktop", copyPh.Location.Kind)
	}
	// Copy mode keeps clipboard after paste (Windows-like).
	if desk.Clipboard == nil {
		t.Error("copy paste should not clear clipboard")
	}

	// Effects persist via index.
	doc, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Placeholders) != 2 {
		t.Errorf("index has %d placeholders, want 2", len(doc.Placeholders))
	}
	for _, p := range doc.Placeholders {
		if p.Identity != wantIdentity {
			t.Errorf("index placeholder identity = %q, want %q", p.Identity, wantIdentity)
		}
	}
}

func TestClipboard_CutPasteToDesktop_MovesAndClearsClipboard(t *testing.T) {
	wb, _ := openDeskWithSkills(t, "alpha", "beta")
	desk := wb.Desk()
	alpha := phByName(t, desk, "alpha")

	if err := wb.SetClipboard(workbench.ClipCut, []string{alpha.ID}); err != nil {
		t.Fatalf("SetClipboard: %v", err)
	}
	if err := wb.PasteToDesktop(8, 3); err != nil {
		t.Fatalf("PasteToDesktop: %v", err)
	}

	desk = wb.Desk()
	if desk.Clipboard != nil {
		t.Errorf("clipboard after cut-paste = %+v, want cleared", desk.Clipboard)
	}
	if len(desk.Placeholders) != 2 {
		t.Fatalf("cut must move, not duplicate: got %d placeholders", len(desk.Placeholders))
	}
	moved := phByName(t, desk, "alpha")
	if moved.ID != alpha.ID {
		t.Errorf("cut should keep same placeholder id, got %q want %q", moved.ID, alpha.ID)
	}
	if moved.Location.Kind != workbench.LocDesktop {
		t.Errorf("kind = %q, want desktop", moved.Location.Kind)
	}
	if moved.Location.Row != 8 || moved.Location.Col != 3 {
		t.Errorf("alpha at (%d,%d), want exact free cell (8,3)", moved.Location.Row, moved.Location.Col)
	}
}

func TestClipboard_CutPasteToDesktop_SelfCellAndMultiItem(t *testing.T) {
	wb, _ := openDeskWithSkills(t, "alpha", "beta")
	desk := wb.Desk()
	alpha := phByName(t, desk, "alpha")
	beta := phByName(t, desk, "beta")
	// Self-cell: cut alpha and paste onto its own cell.
	fromRow, fromCol := alpha.Location.Row, alpha.Location.Col
	if err := wb.SetClipboard(workbench.ClipCut, []string{alpha.ID}); err != nil {
		t.Fatal(err)
	}
	if err := wb.PasteToDesktop(fromRow, fromCol); err != nil {
		t.Fatal(err)
	}
	desk = wb.Desk()
	alpha = phByName(t, desk, "alpha")
	if alpha.Location.Row != fromRow || alpha.Location.Col != fromCol {
		t.Errorf("self-cell cut-paste: alpha at (%d,%d), want (%d,%d)",
			alpha.Location.Row, alpha.Location.Col, fromRow, fromCol)
	}

	// Multi-item: cut alpha+beta, paste at free (10,4); stack downward in col 4.
	alpha, beta = phByName(t, desk, "alpha"), phByName(t, desk, "beta")
	if err := wb.SetClipboard(workbench.ClipCut, []string{alpha.ID, beta.ID}); err != nil {
		t.Fatal(err)
	}
	if err := wb.PasteToDesktop(10, 4); err != nil {
		t.Fatal(err)
	}
	desk = wb.Desk()
	a, b := phByName(t, desk, "alpha"), phByName(t, desk, "beta")
	if a.Location.Row != 10 || a.Location.Col != 4 {
		t.Errorf("alpha at (%d,%d), want (10,4)", a.Location.Row, a.Location.Col)
	}
	if b.Location.Row != 11 || b.Location.Col != 4 {
		t.Errorf("beta at (%d,%d), want (11,4) stacked under alpha", b.Location.Row, b.Location.Col)
	}
}

func TestClipboard_PasteToBox_CurrentCompartment(t *testing.T) {
	wb, _ := openDeskWithSkills(t, "alpha", "beta", "gamma")
	desk := wb.Desk()
	alpha, beta, gamma := phByName(t, desk, "alpha"), phByName(t, desk, "beta"), phByName(t, desk, "gamma")

	// Box from alpha+beta.
	if err := wb.MovePlaceholderToDesktop(beta.ID, alpha.Location.Row, alpha.Location.Col); err != nil {
		t.Fatal(err)
	}
	desk = wb.Desk()
	boxID := desk.Boxes[0].ID

	// Copy gamma → paste into box.
	if err := wb.SetClipboard(workbench.ClipCopy, []string{gamma.ID}); err != nil {
		t.Fatal(err)
	}
	if err := wb.PasteToBox(boxID, ""); err != nil {
		t.Fatalf("PasteToBox: %v", err)
	}

	desk = wb.Desk()
	box := desk.Boxes[0]
	// Original gamma on desktop + copy inside box → 2 gamma-named icons; box has 3 items.
	if len(box.Items) != 3 {
		t.Fatalf("box items = %d, want 3: %+v", len(box.Items), box.Items)
	}
	gammaCount := 0
	for _, it := range box.Items {
		if it.Name == "gamma" {
			gammaCount++
			if it.Identity != gamma.Identity {
				t.Errorf("pasted gamma identity = %q, want %q", it.Identity, gamma.Identity)
			}
			if it.ID == gamma.ID {
				t.Error("copy paste into box must create a new placeholder id")
			}
		}
	}
	if gammaCount != 1 {
		t.Errorf("gamma copies in box = %d, want 1", gammaCount)
	}
	// Original still on desktop.
	g := phByName(t, desk, "gamma")
	// phByName returns first match; verify at least one desktop gamma remains.
	desktopGamma := 0
	for _, p := range desk.Placeholders {
		if p.Name == "gamma" && p.Location.Kind == workbench.LocDesktop {
			desktopGamma++
		}
	}
	if desktopGamma != 1 {
		t.Errorf("desktop gamma count = %d, want 1 (original)", desktopGamma)
	}
	_ = g
}

func TestClipboard_CutPasteToBox_MovesIntoBox(t *testing.T) {
	wb, _ := openDeskWithSkills(t, "alpha", "beta", "gamma")
	desk := wb.Desk()
	alpha, beta, gamma := phByName(t, desk, "alpha"), phByName(t, desk, "beta"), phByName(t, desk, "gamma")
	if err := wb.MovePlaceholderToDesktop(beta.ID, alpha.Location.Row, alpha.Location.Col); err != nil {
		t.Fatal(err)
	}
	desk = wb.Desk()
	boxID := desk.Boxes[0].ID

	if err := wb.SetClipboard(workbench.ClipCut, []string{gamma.ID}); err != nil {
		t.Fatal(err)
	}
	if err := wb.PasteToBox(boxID, ""); err != nil {
		t.Fatalf("PasteToBox: %v", err)
	}

	desk = wb.Desk()
	if desk.Clipboard != nil {
		t.Error("cut-paste should clear clipboard")
	}
	box := desk.Boxes[0]
	if len(box.Items) != 3 {
		t.Fatalf("box items = %d, want 3", len(box.Items))
	}
	// No desktop gamma.
	for _, p := range desk.Placeholders {
		if p.Name == "gamma" && p.Location.Kind == workbench.LocDesktop {
			t.Error("gamma still on desktop after cut-paste into box")
		}
	}
}

func TestMultiSelect_EnablePreSelectsCurrent_BulkMoveIntoBox(t *testing.T) {
	wb, _ := openDeskWithSkills(t, "a", "b", "c", "d")
	desk := wb.Desk()
	a, b := phByName(t, desk, "a"), phByName(t, desk, "b")
	c, d := phByName(t, desk, "c"), phByName(t, desk, "d")

	// Create a box for the bulk target.
	if err := wb.MovePlaceholderToDesktop(b.ID, a.Location.Row, a.Location.Col); err != nil {
		t.Fatal(err)
	}
	desk = wb.Desk()
	boxID := desk.Boxes[0].ID

	// Enter multi-select with current (c) pre-selected.
	if err := wb.EnableMultiSelect(c.ID); err != nil {
		t.Fatalf("EnableMultiSelect: %v", err)
	}
	desk = wb.Desk()
	if !desk.MultiSelect {
		t.Fatal("MultiSelect not enabled")
	}
	if len(desk.SelectedIDs) != 1 || desk.SelectedIDs[0] != c.ID {
		t.Fatalf("SelectedIDs = %v, want [%s]", desk.SelectedIDs, c.ID)
	}

	// Select d as well.
	if err := wb.ToggleSelected(d.ID); err != nil {
		t.Fatalf("ToggleSelected: %v", err)
	}
	desk = wb.Desk()
	if len(desk.SelectedIDs) != 2 {
		t.Fatalf("SelectedIDs = %v, want 2", desk.SelectedIDs)
	}

	// Bulk move selected into box.
	if err := wb.MovePlaceholdersToBox(desk.SelectedIDs, boxID, ""); err != nil {
		t.Fatalf("MovePlaceholdersToBox: %v", err)
	}

	desk = wb.Desk()
	box := desk.Boxes[0]
	if len(box.Items) != 4 {
		t.Fatalf("box items = %d, want 4 (a,b + c,d): %+v", len(box.Items), box.Items)
	}
	names := map[string]bool{}
	for _, it := range box.Items {
		names[it.Name] = true
	}
	if !names["c"] || !names["d"] {
		t.Errorf("bulk move missing c/d: %v", names)
	}
}

func TestMultiSelect_DisableClearsSelection(t *testing.T) {
	wb, _ := openDeskWithSkills(t, "alpha")
	desk := wb.Desk()
	alpha := phByName(t, desk, "alpha")
	if err := wb.EnableMultiSelect(alpha.ID); err != nil {
		t.Fatal(err)
	}
	wb.DisableMultiSelect()
	desk = wb.Desk()
	if desk.MultiSelect {
		t.Error("MultiSelect still on")
	}
	if len(desk.SelectedIDs) != 0 {
		t.Errorf("SelectedIDs = %v, want empty", desk.SelectedIDs)
	}
}

func TestCreateBox_DesktopEmptySpace_SimpleAndComposite(t *testing.T) {
	wb, store := openDeskWithSkills(t, "alpha")
	desk := wb.Desk()
	// Place well clear of icons (far right).
	simpleID, err := wb.CreateSimpleBox("工具", 400, 100)
	if err != nil {
		t.Fatalf("CreateSimpleBox: %v", err)
	}
	if simpleID == "" {
		t.Fatal("empty simple box id")
	}

	compID, err := wb.CreateCompositeBox("Go 开发", []string{"fmt", "test"}, 400, 400)
	if err != nil {
		t.Fatalf("CreateCompositeBox: %v", err)
	}

	desk = wb.Desk()
	if len(desk.Boxes) != 2 {
		t.Fatalf("got %d boxes, want 2: %+v", len(desk.Boxes), desk.Boxes)
	}
	var simple, comp *workbench.Box
	for i := range desk.Boxes {
		b := &desk.Boxes[i]
		switch b.ID {
		case simpleID:
			simple = b
		case compID:
			comp = b
		}
	}
	if simple == nil || simple.Kind != workbench.BoxSimple || simple.Tag != "工具" {
		t.Errorf("simple = %+v, want simple tag 工具", simple)
	}
	if simple != nil && len(simple.Items) != 0 {
		t.Errorf("new simple box should be empty, got %d items", len(simple.Items))
	}
	if comp == nil || comp.Kind != workbench.BoxComposite || comp.Title != "Go 开发" {
		t.Errorf("composite = %+v, want title Go 开发", comp)
	}
	if comp != nil {
		if len(comp.Compartments) != 2 {
			t.Fatalf("compartments = %d, want 2", len(comp.Compartments))
		}
		tags := map[string]bool{}
		for _, c := range comp.Compartments {
			tags[c.Tag] = true
		}
		if !tags["fmt"] || !tags["test"] {
			t.Errorf("compartment tags = %v, want fmt and test", tags)
		}
	}

	// Persist via index across restart.
	alpha := phByName(t, desk, "alpha")
	root := filepath.Dir(alpha.Identity)
	wb2 := newWB(t, []string{root}, store)
	if err := wb2.Open(); err != nil {
		t.Fatalf("reopen: %v", err)
	}
	desk2 := wb2.Desk()
	if len(desk2.Boxes) != 2 {
		t.Errorf("after restart got %d boxes, want 2", len(desk2.Boxes))
	}
}

func TestCreateCompositeBox_SingleTagDemotesToSimple(t *testing.T) {
	wb, _ := openDeskWithSkills(t, "alpha")
	id, err := wb.CreateCompositeBox("主题", []string{"唯一"}, 500, 100)
	if err != nil {
		t.Fatal(err)
	}
	desk := wb.Desk()
	var box *workbench.Box
	for i := range desk.Boxes {
		if desk.Boxes[i].ID == id {
			box = &desk.Boxes[i]
			break
		}
	}
	if box == nil {
		t.Fatal("box not found")
	}
	if box.Kind != workbench.BoxSimple {
		t.Errorf("kind = %q, want simple (single compartment demotes)", box.Kind)
	}
	if box.Tag != "唯一" {
		t.Errorf("tag = %q, want 唯一", box.Tag)
	}
}

func TestRecycle_SystemIconCannotBeCopiedCutOrDeleted(t *testing.T) {
	wb, _ := openDeskWithSkills(t, "alpha")
	desk := wb.Desk()

	// Recycle is a system icon, never a skill placeholder.
	for _, p := range desk.Placeholders {
		if p.Name == "回收站" {
			t.Fatal("recycle must not appear as a skill placeholder")
		}
	}
	if desk.RecycleIcon.Location.Kind != workbench.LocDesktop {
		t.Fatalf("recycle kind = %q", desk.RecycleIcon.Location.Kind)
	}
	before := desk.RecycleIcon.Location

	// Explicit product rule: copy/cut/delete of the recycle system icon is refused.
	for _, action := range []string{"copy", "cut", "delete"} {
		if err := wb.RecycleIconAction(action); err == nil {
			t.Errorf("RecycleIconAction(%q) succeeded; want refuse", action)
		}
	}

	// Clipboard only accepts placeholder IDs; cut+paste of skills leaves recycle untouched.
	alpha := phByName(t, desk, "alpha")
	if err := wb.SetClipboard(workbench.ClipCut, []string{alpha.ID}); err != nil {
		t.Fatal(err)
	}
	if err := wb.PasteToDesktop(6, 2); err != nil {
		t.Fatal(err)
	}
	desk = wb.Desk()
	if desk.RecycleIcon.Location != before {
		t.Errorf("recycle moved/changed: before %+v after %+v", before, desk.RecycleIcon.Location)
	}
}

func TestClipboard_PasteEmptyFails(t *testing.T) {
	wb, _ := openDeskWithSkills(t, "alpha")
	if err := wb.PasteToDesktop(3, 3); err == nil {
		t.Fatal("PasteToDesktop with empty clipboard should fail")
	}
}

func TestClipboard_IgnoresUnknownIDs(t *testing.T) {
	wb, _ := openDeskWithSkills(t, "alpha")
	if err := wb.SetClipboard(workbench.ClipCopy, []string{"no-such-id"}); err == nil {
		t.Fatal("SetClipboard with only unknown ids should fail")
	}
}
