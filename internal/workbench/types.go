package workbench

import (
	"github.com/jasper0507/skills-manage/internal/infra/index"
	"github.com/jasper0507/skills-manage/internal/infra/scanner"
)

// LocationKind is where a 占位 or system icon lives on the workbench.
type LocationKind = index.LocationKind

// Location kinds (re-exported for callers at the Workbench seam).
const (
	LocDesktop = index.LocDesktop
	LocBox     = index.LocBox
	LocRecycle = index.LocRecycle
)

// Location is a desk position (desktop grid cell, box, or recycle).
type Location = index.Location

// Skill is a discovered local skill package. Identity is the canonical realpath.
type Skill struct {
	Identity string `json:"identity"` // Skill 身份: normalized realpath
	Name     string `json:"name"`     // display name (frontmatter name, else directory name)
}

// Inventory is the live 清单 of Skills discovered from 扫描根.
type Inventory struct {
	Skills []Skill `json:"skills"`
}

// Placeholder is a 占位 shortcut icon on the desk (not a file copy).
type Placeholder struct {
	ID       string   `json:"id"`
	Identity string   `json:"identity"` // Skill 身份 (realpath)
	Name     string   `json:"name"`     // display name from latest inventory when known
	Location Location `json:"location"`
}

// SystemIcon is a non-skill desk icon (e.g. 回收站).
type SystemIcon struct {
	Location Location `json:"location"`
}

// Box kinds (re-exported for callers at the Workbench seam).
const (
	BoxSimple    = index.BoxSimple
	BoxComposite = index.BoxComposite
)

// Default simple / composite box geometry (matches accepted prototype).
const (
	defaultSimpleBoxW    = 240
	defaultSimpleBoxH    = 220
	defaultCompositeBoxW = 280
	defaultCompositeBoxH = 260
)

// Icon grid pixel layout used for box↔icon collision (matches prototypes/workbench-desktop).
const (
	iconGridOriginX = 16
	iconGridOriginY = 16
	iconGridCellW   = 90
	iconGridCellH   = 96
	iconW           = 86
	iconH           = 90
	boxSnapGrid     = 16
)

// Default one-screen viewport for auto-placement of new unfiled 占位.
// Approx desktop area: origin+(cols*cellW) ≈ 16+12*90 = 1096px wide,
// origin+(rows*cellH) ≈ 16+8*96 = 784px tall — fits a single typical workbench view
// without stacking an endless first column.
const (
	DefaultViewportCols = 12
	DefaultViewportRows = 8
)

// Compartment is one 隔间 of a composite box, with contained placeholders as icons.
type Compartment struct {
	ID    string        `json:"id"`
	Tag   string        `json:"tag"`
	Items []Placeholder `json:"items"`
}

// Box is a 普通盒子 or 组合盒子 on the desk.
type Box struct {
	ID                  string        `json:"id"`
	Kind                string        `json:"kind"`  // simple | composite
	Tag                 string        `json:"tag"`   // simple: single tag / display name
	Title               string        `json:"title"` // composite: 盒标题
	X                   float64       `json:"x"`
	Y                   float64       `json:"y"`
	W                   float64       `json:"w"`
	H                   float64       `json:"h"`
	Items               []Placeholder `json:"items,omitempty"` // simple box contents as icons
	Compartments        []Compartment `json:"compartments,omitempty"`
	ActiveCompartmentID string        `json:"activeCompartmentId,omitempty"`
}

// Clipboard modes (Windows-style copy vs cut).
const (
	ClipCopy = "copy"
	ClipCut  = "cut"
)

// Clipboard holds session copy/cut targets (placeholder ids). Not persisted in the index.
type Clipboard struct {
	Mode           string   `json:"mode"` // ClipCopy | ClipCut
	PlaceholderIDs []string `json:"placeholderIds"`
}

// Desk is the external desktop view: placeholders + recycle system icon + boxes.
type Desk struct {
	Placeholders []Placeholder `json:"placeholders"`
	RecycleIcon  SystemIcon    `json:"recycleIcon"`
	Boxes        []Box         `json:"boxes"`
	Clipboard    *Clipboard    `json:"clipboard"` // nil when empty
	MultiSelect  bool          `json:"multiSelect"`
	SelectedIDs  []string      `json:"selectedIds"`
}

// Config configures a Workbench.
type Config struct {
	// ScanRoots are filesystem roots to walk for skill packages.
	// Empty means scan nothing (CLI fills defaults before constructing Workbench).
	ScanRoots []string

	// Scanner discovers packages under scan roots. Nil uses the default filesystem scanner.
	Scanner scanner.Scanner

	// Index is the 中央索引 store. Nil uses an in-memory store (ephemeral).
	Index index.Store
}

// Workbench is the sole primary product seam.
type Workbench struct {
	scanRoots []string
	scan      scanner.Scanner
	store     index.Store

	doc    index.Document
	opened bool

	// Session-only UI state (not written to the 中央索引).
	clipboard   *Clipboard
	multiSelect bool
	selectedIDs []string
}
