// Package scanner discovers local Skill packages under scan roots.
// Used only as an adapter injected into workbench.Config (not a product seam).
package scanner

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// quarantineDirName is excluded from live inventory (per-scan-root isolation).
const quarantineDirName = ".skills-manage-trash"

// Skill is a package discovered on disk.
type Skill struct {
	Identity string // realpath of the skill package directory
	Name     string
}

// Scanner walks scan roots and returns unique skills by realpath identity.
type Scanner interface {
	Scan(roots []string) ([]Skill, error)
}

// FSScanner discovers skills on the real filesystem.
type FSScanner struct{}

// New returns a filesystem Scanner.
func New() Scanner {
	return FSScanner{}
}

// Scan walks each root, finds directories containing SKILL.md, collapses
// identity by realpath, and skips quarantine locations.
func (FSScanner) Scan(roots []string) ([]Skill, error) {
	byID := make(map[string]Skill)
	var order []string

	for _, root := range roots {
		if root == "" {
			continue
		}
		info, err := os.Stat(root)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if !info.IsDir() {
			continue
		}
		// WalkDir does not follow a symlink root; resolve first.
		walkPath, err := realpath(root)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		if err := walkRoot(walkPath, byID, &order); err != nil {
			return nil, err
		}
	}

	out := make([]Skill, 0, len(order))
	for _, id := range order {
		out = append(out, byID[id])
	}
	return out, nil
}

func walkRoot(root string, byID map[string]Skill, order *[]string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			// Permission errors on individual entries: skip.
			if os.IsPermission(err) {
				if d != nil && d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			return err
		}

		name := d.Name()
		if name == quarantineDirName {
			// SkipDir only on real directories; on non-dirs it would skip siblings.
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		// Skip other hidden entries (dirs, package symlinks, files) except the root itself.
		if path != root && strings.HasPrefix(name, ".") {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		// Follow symlinks for "is this a directory?" — skill installs often symlink packages.
		info, statErr := os.Stat(path)
		if statErr != nil || !info.IsDir() {
			return nil
		}

		skillMD := filepath.Join(path, "SKILL.md")
		st, err := os.Stat(skillMD)
		if err != nil || st.IsDir() {
			// Symlink dirs are not descended into by WalkDir; only real dirs continue.
			return nil
		}

		identity, err := realpath(path)
		if err != nil {
			return nil
		}
		if _, exists := byID[identity]; exists {
			return skipTreeIfDir(d)
		}

		displayName := nameFromFrontmatter(skillMD)
		if displayName == "" {
			displayName = filepath.Base(identity)
		}
		byID[identity] = Skill{Identity: identity, Name: displayName}
		*order = append(*order, identity)
		// Do not look for nested skill packages inside a real skill directory.
		// For package symlinks, return nil so WalkDir still visits siblings.
		return skipTreeIfDir(d)
	})
}

// skipTreeIfDir returns SkipDir only when WalkDir treats the entry as a directory.
// SkipDir on a non-directory (e.g. a package symlink) skips remaining siblings.
func skipTreeIfDir(d os.DirEntry) error {
	if d.IsDir() {
		return filepath.SkipDir
	}
	return nil
}

func realpath(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(resolved)
	if err != nil {
		return "", err
	}
	return abs, nil
}

func nameFromFrontmatter(skillMD string) string {
	f, err := os.Open(skillMD)
	if err != nil {
		return ""
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	// Cap line size for safety.
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)

	if !sc.Scan() {
		return ""
	}
	line := strings.TrimSpace(sc.Text())
	if line != "---" {
		return ""
	}
	for sc.Scan() {
		line = strings.TrimSpace(sc.Text())
		if line == "---" {
			break
		}
		// Simple YAML: "name: value" (optional space after colon).
		if strings.HasPrefix(line, "name:") || strings.HasPrefix(line, "name :") {
			_, rest, ok := strings.Cut(line, ":")
			if !ok {
				continue
			}
			v := strings.TrimSpace(rest)
			v = strings.Trim(v, `"'`)
			return v
		}
	}
	return ""
}
