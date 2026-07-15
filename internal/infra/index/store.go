// Package index is the central index (中央索引) adapter: load/save one user-level document.
// Injected into workbench.Config; product code should not invent a second write path.
package index

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SchemaVersion is the current on-disk document version.
const SchemaVersion = 1

// LocationKind matches workbench location kinds for persistence.
type LocationKind string

const (
	LocDesktop LocationKind = "desktop"
	LocBox     LocationKind = "box"
	LocRecycle LocationKind = "recycle"
)

// Location is where a placeholder or system icon lives.
type Location struct {
	Kind          LocationKind `json:"kind"`
	Row           int          `json:"row,omitempty"`
	Col           int          `json:"col,omitempty"`
	BoxID         string       `json:"boxId,omitempty"`
	CompartmentID string       `json:"compartmentId,omitempty"`
}

// PlaceholderRecord is one 占位 shortcut in the index.
type PlaceholderRecord struct {
	ID       string   `json:"id"`
	Identity string   `json:"identity"` // Skill 身份 (realpath)
	Location Location `json:"location"`
}

// SkillRecord caches display metadata for an identity (optional; desk can refresh from scan).
type SkillRecord struct {
	Identity string `json:"identity"`
	Name     string `json:"name"`
}

// Box kinds stored in the index.
const (
	BoxSimple    = "simple"
	BoxComposite = "composite"
)

// CompartmentRecord is one 隔间 inside a composite box.
type CompartmentRecord struct {
	ID      string   `json:"id"`
	Tag     string   `json:"tag"`
	ItemIDs []string `json:"itemIds"`
}

// BoxRecord is a 普通盒子 or 组合盒子 on the desk.
type BoxRecord struct {
	ID                  string              `json:"id"`
	Kind                string              `json:"kind"` // simple | composite
	Tag                 string              `json:"tag,omitempty"`
	Title               string              `json:"title,omitempty"`
	X                   float64             `json:"x,omitempty"`
	Y                   float64             `json:"y,omitempty"`
	W                   float64             `json:"w,omitempty"`
	H                   float64             `json:"h,omitempty"`
	ItemIDs             []string            `json:"itemIds,omitempty"`
	Compartments        []CompartmentRecord `json:"compartments,omitempty"`
	ActiveCompartmentID string              `json:"activeCompartmentId,omitempty"`
}

// Legacy body-delete recycle lifecycle states (pre-R2). Still unmarshaled so old
// index files load; Workbench.Open strips RecycleBin rows and uses placeholders
// with Location.Kind=recycle as the only product icon-bin members.
const (
	RecycleStateQuarantined = "quarantined"
	RecycleStatePurging     = "purging"
	RecycleStatePurged      = "purged"
)

// RecycleEntry is legacy body-delete metadata retained only for index compatibility.
// R2 icon-bin members are PlaceholderRecords with Kind=recycle, not this table.
type RecycleEntry struct {
	ID             string    `json:"id"`
	Identity       string    `json:"identity"`
	Name           string    `json:"name,omitempty"`
	OriginalPath   string    `json:"originalPath,omitempty"`
	QuarantinePath string    `json:"quarantinePath,omitempty"`
	DeletedAt      time.Time `json:"deletedAt,omitempty"`
	PurgeAfter     time.Time `json:"purgeAfter,omitempty"`
	PlaceholderIDs []string  `json:"placeholderIds,omitempty"`
	State          string    `json:"state,omitempty"`
}

// Document is the full central index payload.
type Document struct {
	SchemaVersion int                 `json:"schemaVersion"`
	Skills        []SkillRecord       `json:"skills,omitempty"`
	Placeholders  []PlaceholderRecord `json:"placeholders"`
	RecycleIcon   Location            `json:"recycleIcon"`
	Boxes         []BoxRecord         `json:"boxes,omitempty"`
	RecycleBin    []RecycleEntry      `json:"recycleBin,omitempty"`
	BoxNameSeq    int                 `json:"boxNameSeq,omitempty"`
}

// Store loads and atomically saves the central index document.
type Store interface {
	Load() (Document, error)
	Save(Document) error
}

// MemoryStore is an in-process index for tests.
type MemoryStore struct {
	mu  sync.Mutex
	doc *Document
}

// NewMemoryStore returns an empty in-memory store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

// Load returns a copy of the stored document, or an empty document if never saved.
func (m *MemoryStore) Load() (Document, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.doc == nil {
		return emptyDocument(), nil
	}
	return cloneDocument(*m.doc), nil
}

// Save replaces the in-memory document.
func (m *MemoryStore) Save(doc Document) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := cloneDocument(doc)
	m.doc = &c
	return nil
}

// FileStore persists the index as a single JSON file with atomic replace.
type FileStore struct {
	Path string
}

// NewFileStore returns a file-backed store at path (e.g. ~/.config/skills-manage/index.json).
func NewFileStore(path string) *FileStore {
	return &FileStore{Path: path}
}

// Load reads the JSON document. Missing file yields an empty document.
func (f *FileStore) Load() (Document, error) {
	data, err := os.ReadFile(f.Path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return emptyDocument(), nil
		}
		return Document{}, err
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return emptyDocument(), nil
	}
	var doc Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return Document{}, err
	}
	if doc.SchemaVersion == 0 {
		doc.SchemaVersion = SchemaVersion
	}
	if doc.Placeholders == nil {
		doc.Placeholders = []PlaceholderRecord{}
	}
	return doc, nil
}

// Save writes the document atomically: temp file in the same directory, then rename.
func (f *FileStore) Save(doc Document) error {
	if f.Path == "" {
		return errors.New("index: empty path")
	}
	if doc.SchemaVersion == 0 {
		doc.SchemaVersion = SchemaVersion
	}
	dir := filepath.Dir(f.Path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, ".index-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, f.Path); err != nil {
		return err
	}
	cleanup = false
	return nil
}

// DefaultPath returns the user-level central index path under configHome
// (typically ~/.config/skills-manage/index.json).
func DefaultPath(configHome string) string {
	if configHome == "" {
		return ""
	}
	return filepath.Join(configHome, "skills-manage", "index.json")
}

func emptyDocument() Document {
	return Document{
		SchemaVersion: SchemaVersion,
		Placeholders:  []PlaceholderRecord{},
		RecycleIcon: Location{
			Kind: LocDesktop,
			Row:  1,
			Col:  1,
		},
		BoxNameSeq: 1,
	}
}

// CloneDocument returns a deep copy of the central index document.
// Used by Workbench for mutation snapshots (rollback on failure).
func CloneDocument(doc Document) Document {
	return cloneDocument(doc)
}

func cloneDocument(doc Document) Document {
	out := doc
	if doc.Placeholders != nil {
		out.Placeholders = append([]PlaceholderRecord(nil), doc.Placeholders...)
	} else {
		out.Placeholders = []PlaceholderRecord{}
	}
	if doc.Skills != nil {
		out.Skills = append([]SkillRecord(nil), doc.Skills...)
	}
	if doc.Boxes != nil {
		out.Boxes = make([]BoxRecord, len(doc.Boxes))
		for i, b := range doc.Boxes {
			out.Boxes[i] = cloneBoxRecord(b)
		}
	}
	if doc.RecycleBin != nil {
		out.RecycleBin = make([]RecycleEntry, len(doc.RecycleBin))
		for i, e := range doc.RecycleBin {
			out.RecycleBin[i] = e
			if e.PlaceholderIDs != nil {
				out.RecycleBin[i].PlaceholderIDs = append([]string(nil), e.PlaceholderIDs...)
			}
		}
	} else {
		out.RecycleBin = nil
	}
	return out
}

func cloneBoxRecord(b BoxRecord) BoxRecord {
	out := b
	if b.ItemIDs != nil {
		out.ItemIDs = append([]string(nil), b.ItemIDs...)
	}
	if b.Compartments != nil {
		out.Compartments = make([]CompartmentRecord, len(b.Compartments))
		for i, c := range b.Compartments {
			out.Compartments[i] = c
			if c.ItemIDs != nil {
				out.Compartments[i].ItemIDs = append([]string(nil), c.ItemIDs...)
			}
		}
	}
	return out
}
