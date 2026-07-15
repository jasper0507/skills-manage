package workbench

import (
	"fmt"
	"strings"

	"github.com/jasper0507/skills-manage/internal/infra/index"
)

// MovePlaceholderToBox files a 占位 into a box's current 隔间/标签.
// For simple boxes, compartmentID is ignored. For composite boxes, empty
// compartmentID uses the active compartment.
func (w *Workbench) MovePlaceholderToBox(placeholderID, boxID, compartmentID string) error {
	if err := w.requireOpen(); err != nil {
		return err
	}
	if err := w.movePlaceholderToBoxNoPersist(placeholderID, boxID, compartmentID); err != nil {
		return err
	}
	return w.persist()
}

func containsID(ids []string, id string) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// ComposeBoxes merges source into target:
//   - simple → simple: target becomes composite; both tags become 隔间; title from target tag
//   - simple → composite: append compartment from source
//   - composite → composite: refused
//   - composite → simple: refused (only simple may be the source)
func (w *Workbench) ComposeBoxes(sourceBoxID, targetBoxID string) error {
	if err := w.requireOpen(); err != nil {
		return err
	}
	if sourceBoxID == targetBoxID {
		return fmt.Errorf("cannot compose a box with itself")
	}
	sIdx, ok := w.boxIndex(sourceBoxID)
	if !ok {
		return fmt.Errorf("unknown source box %q", sourceBoxID)
	}
	tIdx, ok := w.boxIndex(targetBoxID)
	if !ok {
		return fmt.Errorf("unknown target box %q", targetBoxID)
	}
	src := w.doc.Boxes[sIdx]
	tgt := &w.doc.Boxes[tIdx]

	if src.Kind == BoxComposite && tgt.Kind == BoxComposite {
		return fmt.Errorf("composite → composite merge is not allowed")
	}
	if src.Kind != BoxSimple {
		return fmt.Errorf("compose requires simple source box")
	}

	if tgt.Kind == BoxSimple {
		return w.composeSimpleIntoSimple(sIdx, tIdx)
	}
	return w.addSimpleToComposite(sIdx, tIdx)
}

func (w *Workbench) composeSimpleIntoSimple(sIdx, tIdx int) error {
	src := w.doc.Boxes[sIdx]
	tgt := &w.doc.Boxes[tIdx]

	// Expanded composite geometry must not cover desktop icons — check before mutate.
	pos, ok := w.findBoxPosWithoutIconOverlap(tgt.X, tgt.Y, defaultCompositeBoxW, defaultCompositeBoxH, tgt.ID)
	if !ok {
		return fmt.Errorf("no free box position that avoids covering desktop skill icons")
	}

	c1 := index.CompartmentRecord{
		ID:      w.newCompartmentID(),
		Tag:     tgt.Tag,
		ItemIDs: append([]string(nil), tgt.ItemIDs...),
	}
	c2Tag := ensureUniqueTag([]string{c1.Tag}, src.Tag)
	c2 := index.CompartmentRecord{
		ID:      w.newCompartmentID(),
		Tag:     c2Tag,
		ItemIDs: append([]string(nil), src.ItemIDs...),
	}

	for _, phID := range c1.ItemIDs {
		if i, ok := w.placeholderIndex(phID); ok {
			w.doc.Placeholders[i].Location = index.Location{
				Kind: LocBox, BoxID: tgt.ID, CompartmentID: c1.ID,
			}
		}
	}
	for _, phID := range c2.ItemIDs {
		if i, ok := w.placeholderIndex(phID); ok {
			w.doc.Placeholders[i].Location = index.Location{
				Kind: LocBox, BoxID: tgt.ID, CompartmentID: c2.ID,
			}
		}
	}

	// Recycle icon in either box moves into first compartment of the composite.
	r := &w.doc.RecycleIcon
	if r.Kind == LocBox && (r.BoxID == src.ID || r.BoxID == tgt.ID) {
		r.BoxID = tgt.ID
		r.CompartmentID = c1.ID
		r.Row, r.Col = 0, 0
	}

	tgt.Kind = BoxComposite
	tgt.Title = tgt.Tag
	tgt.Tag = ""
	tgt.ItemIDs = nil
	tgt.Compartments = []index.CompartmentRecord{c1, c2}
	tgt.ActiveCompartmentID = c1.ID
	tgt.W = defaultCompositeBoxW
	tgt.H = defaultCompositeBoxH
	tgt.X, tgt.Y = pos.x, pos.y

	w.doc.Boxes = append(w.doc.Boxes[:sIdx], w.doc.Boxes[sIdx+1:]...)
	// Mutations on tgt are applied before delete; when sIdx < tIdx the element
	// shifts left but keeps those field values. Do not use tgt after this line.
	return w.persist()
}

func (w *Workbench) addSimpleToComposite(sIdx, tIdx int) error {
	src := w.doc.Boxes[sIdx]
	tgt := &w.doc.Boxes[tIdx]

	existing := make([]string, 0, len(tgt.Compartments))
	for _, c := range tgt.Compartments {
		existing = append(existing, c.Tag)
	}
	tag := ensureUniqueTag(existing, src.Tag)
	c := index.CompartmentRecord{
		ID:      w.newCompartmentID(),
		Tag:     tag,
		ItemIDs: append([]string(nil), src.ItemIDs...),
	}
	for _, phID := range c.ItemIDs {
		if i, ok := w.placeholderIndex(phID); ok {
			w.doc.Placeholders[i].Location = index.Location{
				Kind: LocBox, BoxID: tgt.ID, CompartmentID: c.ID,
			}
		}
	}
	r := &w.doc.RecycleIcon
	if r.Kind == LocBox && r.BoxID == src.ID {
		r.BoxID = tgt.ID
		r.CompartmentID = c.ID
		r.Row, r.Col = 0, 0
	}
	tgt.Compartments = append(tgt.Compartments, c)
	tgt.ActiveCompartmentID = c.ID

	w.doc.Boxes = append(w.doc.Boxes[:sIdx], w.doc.Boxes[sIdx+1:]...)
	return w.persist()
}

func ensureUniqueTag(used []string, tag string) string {
	set := make(map[string]bool, len(used))
	for _, t := range used {
		set[t] = true
	}
	if !set[tag] {
		return tag
	}
	for n := 2; ; n++ {
		candidate := fmt.Sprintf("%s-%d", tag, n)
		if !set[candidate] {
			return candidate
		}
	}
}

// MoveBox places a box at (x,y). The rectangle must not cover desktop Skill icons
// (or the recycle icon); the position is nudged to the nearest free soft-grid slot.
func (w *Workbench) MoveBox(boxID string, x, y float64) error {
	if err := w.requireOpen(); err != nil {
		return err
	}
	bIdx, ok := w.boxIndex(boxID)
	if !ok {
		return fmt.Errorf("unknown box %q", boxID)
	}
	box := &w.doc.Boxes[bIdx]
	pos, ok := w.findBoxPosWithoutIconOverlap(x, y, box.W, box.H, boxID)
	if !ok {
		return fmt.Errorf("box placement would cover desktop skill icons")
	}
	box.X, box.Y = pos.x, pos.y
	return w.persist()
}

// SetActiveCompartment switches the current 隔间 of a composite box.
func (w *Workbench) SetActiveCompartment(boxID, compartmentID string) error {
	if err := w.requireOpen(); err != nil {
		return err
	}
	bIdx, ok := w.boxIndex(boxID)
	if !ok {
		return fmt.Errorf("unknown box %q", boxID)
	}
	box := &w.doc.Boxes[bIdx]
	if box.Kind != BoxComposite {
		return fmt.Errorf("box %q is not composite", boxID)
	}
	for _, c := range box.Compartments {
		if c.ID == compartmentID {
			box.ActiveCompartmentID = compartmentID
			return w.persist()
		}
	}
	return fmt.Errorf("unknown compartment %q", compartmentID)
}

// EjectCompartment pulls a 隔间 out as a new 普通盒子. If the composite is left
// with one compartment, it demotes to simple immediately.
func (w *Workbench) EjectCompartment(compositeBoxID, compartmentID string, x, y float64) error {
	if err := w.requireOpen(); err != nil {
		return err
	}
	bIdx, ok := w.boxIndex(compositeBoxID)
	if !ok {
		return fmt.Errorf("unknown box %q", compositeBoxID)
	}
	box := &w.doc.Boxes[bIdx]
	if box.Kind != BoxComposite {
		return fmt.Errorf("box %q is not composite", compositeBoxID)
	}
	cIdx := -1
	for i, c := range box.Compartments {
		if c.ID == compartmentID {
			cIdx = i
			break
		}
	}
	if cIdx < 0 {
		return fmt.Errorf("unknown compartment %q", compartmentID)
	}

	// Placement first so a refuse leaves the composite unchanged.
	pos, ok := w.findBoxPosWithoutIconOverlap(x, y, defaultSimpleBoxW, defaultSimpleBoxH, "")
	if !ok {
		return fmt.Errorf("no free box position that avoids covering desktop skill icons")
	}

	comp := box.Compartments[cIdx]
	box.Compartments = append(box.Compartments[:cIdx], box.Compartments[cIdx+1:]...)

	newID := w.newBoxID()
	newBox := index.BoxRecord{
		ID:      newID,
		Kind:    BoxSimple,
		Tag:     comp.Tag,
		X:       pos.x,
		Y:       pos.y,
		W:       defaultSimpleBoxW,
		H:       defaultSimpleBoxH,
		ItemIDs: append([]string(nil), comp.ItemIDs...),
	}
	for _, phID := range newBox.ItemIDs {
		if i, ok := w.placeholderIndex(phID); ok {
			w.doc.Placeholders[i].Location = index.Location{Kind: LocBox, BoxID: newID}
		}
	}
	r := &w.doc.RecycleIcon
	if r.Kind == LocBox && r.BoxID == compositeBoxID && r.CompartmentID == compartmentID {
		r.BoxID = newID
		r.CompartmentID = ""
	}

	// Demote or remove residual composite before appending (avoids slice realloc
	// invalidating the residual box element while we still index it).
	if len(box.Compartments) == 1 {
		w.demoteCompositeIfSingle(bIdx)
	} else if len(box.Compartments) == 0 {
		w.doc.Boxes = append(w.doc.Boxes[:bIdx], w.doc.Boxes[bIdx+1:]...)
	} else if box.ActiveCompartmentID == compartmentID {
		box.ActiveCompartmentID = box.Compartments[0].ID
	}

	w.doc.Boxes = append(w.doc.Boxes, newBox)
	return w.persist()
}

func (w *Workbench) demoteCompositeIfSingle(bIdx int) {
	box := &w.doc.Boxes[bIdx]
	if box.Kind != BoxComposite || len(box.Compartments) != 1 {
		return
	}
	last := box.Compartments[0]
	box.Kind = BoxSimple
	box.Tag = last.Tag
	box.Title = ""
	box.ItemIDs = append([]string(nil), last.ItemIDs...)
	box.Compartments = nil
	box.ActiveCompartmentID = ""
	box.W = defaultSimpleBoxW
	box.H = defaultSimpleBoxH
	for _, phID := range box.ItemIDs {
		if i, ok := w.placeholderIndex(phID); ok {
			w.doc.Placeholders[i].Location = index.Location{Kind: LocBox, BoxID: box.ID}
		}
	}
	r := &w.doc.RecycleIcon
	if r.Kind == LocBox && r.BoxID == box.ID {
		r.CompartmentID = ""
	}
}

// RenameBoxTag renames a simple box tag, or a composite compartment tag when
// compartmentID is set.
func (w *Workbench) RenameBoxTag(boxID, newTag, compartmentID string) error {
	if err := w.requireOpen(); err != nil {
		return err
	}
	tag := strings.TrimSpace(newTag)
	if tag == "" {
		return fmt.Errorf("tag must not be empty")
	}
	bIdx, ok := w.boxIndex(boxID)
	if !ok {
		return fmt.Errorf("unknown box %q", boxID)
	}
	box := &w.doc.Boxes[bIdx]
	if box.Kind == BoxSimple {
		box.Tag = tag
		return w.persist()
	}
	if compartmentID == "" {
		return fmt.Errorf("composite rename requires compartment id")
	}
	cIdx := -1
	others := make([]string, 0, len(box.Compartments))
	for i, c := range box.Compartments {
		if c.ID == compartmentID {
			cIdx = i
			continue
		}
		others = append(others, c.Tag)
	}
	if cIdx < 0 {
		return fmt.Errorf("unknown compartment %q", compartmentID)
	}
	box.Compartments[cIdx].Tag = ensureUniqueTag(others, tag)
	return w.persist()
}

// RenameBoxTitle renames a composite box's 盒标题.
func (w *Workbench) RenameBoxTitle(boxID, title string) error {
	if err := w.requireOpen(); err != nil {
		return err
	}
	bIdx, ok := w.boxIndex(boxID)
	if !ok {
		return fmt.Errorf("unknown box %q", boxID)
	}
	box := &w.doc.Boxes[bIdx]
	if box.Kind != BoxComposite {
		return fmt.Errorf("box %q is not composite", boxID)
	}
	box.Title = strings.TrimSpace(title)
	return w.persist()
}

// DeleteBox removes a box and returns all contained placeholders (and recycle icon
// if inside) to free desktop grid cells. Skill bodies are never deleted.
func (w *Workbench) DeleteBox(boxID string) error {
	if err := w.requireOpen(); err != nil {
		return err
	}
	bIdx, ok := w.boxIndex(boxID)
	if !ok {
		return fmt.Errorf("unknown box %q", boxID)
	}
	box := w.doc.Boxes[bIdx]

	ids := make([]string, 0)
	if box.Kind == BoxSimple {
		ids = append(ids, box.ItemIDs...)
	} else {
		for _, c := range box.Compartments {
			ids = append(ids, c.ItemIDs...)
		}
	}

	// Remove box first so free-cell search ignores it (geometry only; cells are icon-only).
	w.doc.Boxes = append(w.doc.Boxes[:bIdx], w.doc.Boxes[bIdx+1:]...)

	occupied := w.occupiedDesktopCells()
	// Prefer cells near the former box position.
	startRow := int(box.Y-iconGridOriginY)/iconGridCellH + 1
	startCol := int(box.X-iconGridOriginX)/iconGridCellW + 1
	if startRow < 1 {
		startRow = 1
	}
	if startCol < 1 {
		startCol = 1
	}
	for _, phID := range ids {
		idx, ok := w.placeholderIndex(phID)
		if !ok {
			continue
		}
		free := nextFreeCell(occupied, startCol, startRow)
		occupied[free] = true
		w.doc.Placeholders[idx].Location = index.Location{
			Kind: LocDesktop, Row: free.row, Col: free.col,
		}
		// Spread subsequent icons along the column.
		startRow = free.row + 1
	}

	r := &w.doc.RecycleIcon
	if r.Kind == LocBox && r.BoxID == boxID {
		free := nextFreeCell(occupied, startCol, startRow)
		r.Kind = LocDesktop
		r.Row, r.Col = free.row, free.col
		r.BoxID, r.CompartmentID = "", ""
	}

	return w.persist()
}

func (w *Workbench) CreateSimpleBox(tag string, x, y float64) (string, error) {
	if err := w.requireOpen(); err != nil {
		return "", err
	}
	tag = strings.TrimSpace(tag)
	if tag == "" {
		tag = "新建"
	}
	pos, ok := w.findBoxPosWithoutIconOverlap(x, y, defaultSimpleBoxW, defaultSimpleBoxH, "")
	if !ok {
		return "", fmt.Errorf("no free box position that avoids covering desktop skill icons")
	}
	id := w.newBoxID()
	w.doc.Boxes = append(w.doc.Boxes, index.BoxRecord{
		ID:   id,
		Kind: BoxSimple,
		Tag:  tag,
		X:    pos.x,
		Y:    pos.y,
		W:    defaultSimpleBoxW,
		H:    defaultSimpleBoxH,
	})
	if err := w.persist(); err != nil {
		return "", err
	}
	return id, nil
}

// CreateCompositeBox places an empty 组合盒子 with the given title and compartment tags.
// A single compartment demotes immediately to a 普通盒子 (product rule).
// Empty title defaults to 「组合盒」; empty tags default to [「默认」].
func (w *Workbench) CreateCompositeBox(title string, tags []string, x, y float64) (string, error) {
	if err := w.requireOpen(); err != nil {
		return "", err
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = "组合盒"
	}
	clean := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t != "" {
			clean = append(clean, t)
		}
	}
	if len(clean) == 0 {
		clean = []string{"默认"}
	}

	// Single compartment → demote to simple immediately.
	if len(clean) == 1 {
		return w.CreateSimpleBox(clean[0], x, y)
	}

	pos, ok := w.findBoxPosWithoutIconOverlap(x, y, defaultCompositeBoxW, defaultCompositeBoxH, "")
	if !ok {
		return "", fmt.Errorf("no free box position that avoids covering desktop skill icons")
	}

	comps := make([]index.CompartmentRecord, 0, len(clean))
	used := make([]string, 0, len(clean))
	for _, t := range clean {
		tag := ensureUniqueTag(used, t)
		used = append(used, tag)
		comps = append(comps, index.CompartmentRecord{
			ID:  w.newCompartmentID(),
			Tag: tag,
		})
	}
	id := w.newBoxID()
	w.doc.Boxes = append(w.doc.Boxes, index.BoxRecord{
		ID:                  id,
		Kind:                BoxComposite,
		Title:               title,
		X:                   pos.x,
		Y:                   pos.y,
		W:                   defaultCompositeBoxW,
		H:                   defaultCompositeBoxH,
		Compartments:        comps,
		ActiveCompartmentID: comps[0].ID,
	})
	if err := w.persist(); err != nil {
		return "", err
	}
	return id, nil
}
