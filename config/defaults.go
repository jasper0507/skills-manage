package config

import (
	"path/filepath"

	"github.com/jasper0507/skills-manage/internal/infra/index"
)

// DefaultScanRoots returns sensible user-level and project-level skill paths.
// Bundled/system trees are not included.
func DefaultScanRoots(home, projectRoot string) []string {
	var roots []string
	dirs := []string{
		".agents/skills",
		".claude/skills",
		".codex/skills",
		".grok/skills",
	}
	for _, rel := range dirs {
		if home != "" {
			roots = append(roots, filepath.Join(home, rel))
		}
	}
	for _, rel := range dirs {
		if projectRoot != "" {
			roots = append(roots, filepath.Join(projectRoot, rel))
		}
	}
	return roots
}

// DefaultIndexPath returns the user-level 中央索引 path (e.g. under XDG config).
func DefaultIndexPath(configHome string) string {
	return index.DefaultPath(configHome)
}
