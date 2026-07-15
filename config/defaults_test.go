package config_test

import (
	"strings"
	"testing"

	"github.com/jasper0507/skills-manage/config"
)

func TestDefaultScanRoots_IncludesCommonUserAndProjectPaths(t *testing.T) {
	roots := config.DefaultScanRoots("/home/alice", "/home/alice/proj")
	want := []string{
		"/home/alice/.agents/skills",
		"/home/alice/.claude/skills",
		"/home/alice/.codex/skills",
		"/home/alice/.grok/skills",
		"/home/alice/proj/.agents/skills",
		"/home/alice/proj/.claude/skills",
		"/home/alice/proj/.codex/skills",
		"/home/alice/proj/.grok/skills",
	}
	got := map[string]bool{}
	for _, r := range roots {
		got[r] = true
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("DefaultScanRoots missing %q; got %v", w, roots)
		}
	}
	// Bundled/system trees must not appear in defaults.
	for _, r := range roots {
		for _, bad := range []string{"/usr/", "/opt/", "bundled", "node_modules"} {
			if strings.Contains(r, bad) {
				t.Errorf("default scan root looks bundled/system: %q", r)
			}
		}
	}
}
