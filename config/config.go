// Package config holds CLI/runtime configuration for skills-manage.
// Values come from flags (and later optional files); defaults match product rules in CONTEXT.md.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Config is the assembled runtime configuration for a command.
type Config struct {
	// ScanRoots are filesystem roots to walk for skill packages.
	// Empty means use DefaultScanRoots(home, cwd).
	ScanRoots []string

	// IndexPath is the central index JSON path (desk/serve).
	// Empty means DefaultIndexPath(user config dir).
	IndexPath string

	// Addr is the HTTP listen address for serve (e.g. "127.0.0.1:0").
	Addr string

	// NoOpen skips opening the default browser on serve.
	NoOpen bool
}

// Defaults for serve when flags omit values.
const (
	DefaultServeAddr = "127.0.0.1:0"
)

// ResolveScanRoots returns explicit roots or common defaults under home/cwd.
func ResolveScanRoots(roots []string) ([]string, error) {
	if len(roots) > 0 {
		return append([]string(nil), roots...), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("resolve working directory: %w", err)
	}
	return DefaultScanRoots(home, cwd), nil
}

// ResolveIndexPath returns explicit path or $CONFIG/skills-manage/index.json.
func ResolveIndexPath(indexPath string) (string, error) {
	if indexPath != "" {
		return indexPath, nil
	}
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("resolve config directory: %w", err)
	}
	return DefaultIndexPath(cfg), nil
}

// MultiFlag accumulates repeated -root values for flag.Var.
type MultiFlag []string

func (m *MultiFlag) String() string {
	return strings.Join(*m, ",")
}

func (m *MultiFlag) Set(value string) error {
	if value == "" {
		return nil
	}
	// Expand leading ~ for convenience.
	if strings.HasPrefix(value, "~"+string(os.PathSeparator)) || value == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		if value == "~" {
			value = home
		} else {
			value = filepath.Join(home, value[2:])
		}
	}
	*m = append(*m, value)
	return nil
}
