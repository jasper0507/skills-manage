// Package quarantine isolates skill packages out of live scan paths via same-FS rename.
// Invoked only through the Workbench body-delete lifecycle (not a second product seam).
package quarantine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

// TrashDirName is the per-scan-root quarantine directory (excluded from scanners).
const TrashDirName = ".skills-manage-trash"

// Adapter moves skill packages into and out of app-private quarantine trees.
type Adapter interface {
	// Isolate renames livePath into <scanRoot>/.skills-manage-trash/<entryID>.
	// livePath must already be a canonical realpath of a skill package directory.
	// entryID must be a single safe path segment (no separators, no "..").
	Isolate(livePath, scanRoot, entryID string) (quarantinePath string, err error)

	// Restore renames quarantinePath back to originalPath when originalPath is free.
	// Returns a conflict error if originalPath already exists.
	Restore(quarantinePath, originalPath string) error

	// Purge permanently deletes quarantinePath after verifying it is a direct
	// child of a .skills-manage-trash directory (never the trash root itself).
	Purge(quarantinePath string) error
}

// FS is the real-filesystem quarantine adapter.
type FS struct{}

// New returns a filesystem Adapter.
func New() Adapter {
	return FS{}
}

// Isolate performs same-FS rename into the scan-root trash directory.
func (FS) Isolate(livePath, scanRoot, entryID string) (string, error) {
	if err := validateEntryID(entryID); err != nil {
		return "", err
	}
	if err := validateUnderRoot(livePath, scanRoot); err != nil {
		return "", err
	}
	// Refuse paths already inside trash.
	if isQuarantineEntry(livePath) {
		return "", fmt.Errorf("quarantine: path already under trash: %s", livePath)
	}
	info, err := os.Stat(livePath)
	if err != nil {
		return "", fmt.Errorf("quarantine: live path: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("quarantine: live path is not a directory: %s", livePath)
	}
	// Skill package check: SKILL.md must exist.
	skillMD := filepath.Join(livePath, "SKILL.md")
	st, err := os.Stat(skillMD)
	if err != nil || st.IsDir() {
		return "", fmt.Errorf("quarantine: not a skill package (missing SKILL.md): %s", livePath)
	}

	trashRoot := filepath.Join(filepath.Clean(scanRoot), TrashDirName)
	if err := os.MkdirAll(trashRoot, 0o755); err != nil {
		return "", fmt.Errorf("quarantine: create trash root: %w", err)
	}
	// Trash root itself must not be a symlink (FreeDesktop-style).
	trashInfo, err := os.Lstat(trashRoot)
	if err != nil {
		return "", err
	}
	if trashInfo.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("quarantine: trash root must not be a symlink: %s", trashRoot)
	}

	dest := filepath.Join(trashRoot, entryID)
	// Belt-and-suspenders: dest must be a direct child of trashRoot.
	if filepath.Dir(dest) != trashRoot || filepath.Base(dest) != entryID {
		return "", fmt.Errorf("quarantine: refused unsafe destination: %s", dest)
	}
	if _, err := os.Lstat(dest); err == nil {
		return "", fmt.Errorf("quarantine: destination already exists: %s", dest)
	} else if !os.IsNotExist(err) {
		return "", err
	}

	if err := os.Rename(livePath, dest); err != nil {
		return "", fmt.Errorf("quarantine: rename into trash: %w", err)
	}
	return dest, nil
}

// Restore renames back when originalPath is free.
func (FS) Restore(quarantinePath, originalPath string) error {
	if !isQuarantineEntry(quarantinePath) {
		return fmt.Errorf("quarantine: restore source not a trash entry: %s", quarantinePath)
	}
	if _, err := os.Stat(quarantinePath); err != nil {
		return fmt.Errorf("quarantine: restore source missing: %w", err)
	}
	// Lstat: any directory entry (including broken symlink) occupies the name.
	if _, err := os.Lstat(originalPath); err == nil {
		return &PathOccupiedError{Path: originalPath}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("quarantine: check original path: %w", err)
	}
	// Ensure parent of original exists.
	if err := os.MkdirAll(filepath.Dir(originalPath), 0o755); err != nil {
		return fmt.Errorf("quarantine: create original parent: %w", err)
	}
	if err := os.Rename(quarantinePath, originalPath); err != nil {
		if os.IsExist(err) {
			return &PathOccupiedError{Path: originalPath}
		}
		return fmt.Errorf("quarantine: rename restore: %w", err)
	}
	return nil
}

// Purge permanently deletes a single quarantine entry directory only.
func (FS) Purge(quarantinePath string) error {
	if quarantinePath == "" {
		return fmt.Errorf("quarantine: empty purge path")
	}
	if !isQuarantineEntry(quarantinePath) {
		return fmt.Errorf("quarantine: refuse purge outside trash entry: %s", quarantinePath)
	}
	// Resolve realpath and re-check after resolution (symlink escape).
	real, err := filepath.EvalSymlinks(quarantinePath)
	if err != nil {
		if os.IsNotExist(err) {
			// Already gone — treat as success for idempotent purge.
			return nil
		}
		// If the entry itself is a broken symlink under trash, remove the link only.
		if _, lerr := os.Lstat(quarantinePath); lerr == nil && isQuarantineEntry(quarantinePath) {
			if rerr := os.Remove(quarantinePath); rerr != nil {
				return fmt.Errorf("quarantine: purge broken link: %w", rerr)
			}
			return nil
		}
		return fmt.Errorf("quarantine: resolve purge path: %w", err)
	}
	if !isQuarantineEntry(real) {
		return fmt.Errorf("quarantine: refuse purge outside trash entry after resolve: %s", real)
	}
	if err := os.RemoveAll(real); err != nil {
		return fmt.Errorf("quarantine: purge: %w", err)
	}
	return nil
}

// PathOccupiedError is returned when restore cannot overwrite an existing path.
type PathOccupiedError struct {
	Path string
}

func (e *PathOccupiedError) Error() string {
	return fmt.Sprintf("restore path occupied: %s", e.Path)
}

// IsPathOccupied reports whether err is (or wraps) a restore conflict.
func IsPathOccupied(err error) bool {
	var po *PathOccupiedError
	return errors.As(err, &po)
}

// FindScanRoot returns the scan root that contains identity (as a realpath prefix).
// Roots should already be absolute; identity is the skill realpath.
func FindScanRoot(identity string, scanRoots []string) (string, error) {
	identity = filepath.Clean(identity)
	var best string
	for _, root := range scanRoots {
		if root == "" {
			continue
		}
		r, err := realpathOrAbs(root)
		if err != nil {
			continue
		}
		if identity == r || strings.HasPrefix(identity, r+string(os.PathSeparator)) {
			if best == "" || len(r) > len(best) {
				best = r
			}
		}
	}
	if best == "" {
		return "", fmt.Errorf("quarantine: identity not under any scan root: %s", identity)
	}
	return best, nil
}

// IsQuarantineEntry reports whether path is a direct child of a trash directory.
// The trash root itself is not an entry (cannot be purged as a single skill).
func IsQuarantineEntry(path string) bool {
	return isQuarantineEntry(path)
}

func validateEntryID(entryID string) error {
	if entryID == "" {
		return fmt.Errorf("quarantine: empty entry id")
	}
	if entryID == "." || entryID == ".." {
		return fmt.Errorf("quarantine: invalid entry id %q", entryID)
	}
	if strings.Contains(entryID, string(os.PathSeparator)) || strings.Contains(entryID, "/") || strings.Contains(entryID, "\\") {
		return fmt.Errorf("quarantine: entry id must be a single path segment: %q", entryID)
	}
	if filepath.Base(entryID) != entryID {
		return fmt.Errorf("quarantine: entry id must be a single path segment: %q", entryID)
	}
	// Restrict to printable non-space characters; Workbench uses trash_<hex>.
	for _, r := range entryID {
		if r > unicode.MaxASCII || (!unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '-') {
			return fmt.Errorf("quarantine: entry id has invalid character: %q", entryID)
		}
	}
	return nil
}

func validateUnderRoot(livePath, scanRoot string) error {
	live := filepath.Clean(livePath)
	root := filepath.Clean(scanRoot)
	if live == root {
		return fmt.Errorf("quarantine: refuse isolating entire scan root: %s", live)
	}
	if !strings.HasPrefix(live, root+string(os.PathSeparator)) {
		return fmt.Errorf("quarantine: path outside scan root: %s not under %s", live, root)
	}
	return nil
}

// isQuarantineEntry is true only for direct children of .../.skills-manage-trash/<id>.
// Refuses the trash root itself and nested paths deeper than one level under trash.
func isQuarantineEntry(path string) bool {
	clean := filepath.Clean(path)
	if clean == "" || clean == "." {
		return false
	}
	base := filepath.Base(clean)
	if base == TrashDirName || base == "." || base == ".." {
		return false
	}
	parent := filepath.Base(filepath.Dir(clean))
	return parent == TrashDirName
}

func realpathOrAbs(path string) (string, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		// Root may not exist yet; fall back to abs.
		return filepath.Abs(path)
	}
	return filepath.Abs(resolved)
}
