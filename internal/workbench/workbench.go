// Package workbench is the primary application facade for skills-manage.
// Callers (CLI, HTTP, tests) speak only to Workbench for product behavior.
package workbench

import (
	"path/filepath"

	"github.com/jasper0507/skills-manage/internal/scanner"
)

// Skill is a discovered local skill package. Identity is the canonical realpath.
type Skill struct {
	Identity string // Skill 身份: normalized realpath
	Name     string // display name (frontmatter name, else directory name)
}

// Inventory is the live 清单 of Skills discovered from 扫描根.
type Inventory struct {
	Skills []Skill
}

// Config configures a Workbench.
type Config struct {
	// ScanRoots are filesystem roots to walk for skill packages.
	// Empty means scan nothing (CLI fills defaults before constructing Workbench).
	ScanRoots []string

	// Scanner discovers packages under scan roots. Nil uses the default filesystem scanner.
	Scanner scanner.Scanner
}

// Workbench is the sole primary product seam.
type Workbench struct {
	scanRoots []string
	scan      scanner.Scanner
}

// New constructs a Workbench. Scanner defaults to the filesystem implementation when nil.
func New(cfg Config) *Workbench {
	sc := cfg.Scanner
	if sc == nil {
		sc = scanner.New()
	}
	return &Workbench{
		scanRoots: append([]string(nil), cfg.ScanRoots...),
		scan:      sc,
	}
}

// Inventory returns the live Skill 清单 for configured scan roots.
func (w *Workbench) Inventory() (Inventory, error) {
	found, err := w.scan.Scan(w.scanRoots)
	if err != nil {
		return Inventory{}, err
	}
	skills := make([]Skill, 0, len(found))
	for _, s := range found {
		skills = append(skills, Skill{
			Identity: s.Identity,
			Name:     s.Name,
		})
	}
	return Inventory{Skills: skills}, nil
}

// DefaultScanRoots returns sensible user-level and project-level skill paths.
// Bundled/system trees are not included.
func DefaultScanRoots(home, projectRoot string) []string {
	var roots []string
	userDirs := []string{
		".agents/skills",
		".claude/skills",
		".codex/skills",
		".grok/skills",
	}
	for _, rel := range userDirs {
		if home != "" {
			roots = append(roots, filepath.Join(home, rel))
		}
	}
	projectDirs := []string{
		".agents/skills",
		".claude/skills",
		".codex/skills",
		".grok/skills",
	}
	for _, rel := range projectDirs {
		if projectRoot != "" {
			roots = append(roots, filepath.Join(projectRoot, rel))
		}
	}
	return roots
}
