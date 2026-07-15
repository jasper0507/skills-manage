// Package app assembles configuration, Workbench, and transports, then runs CLI commands.
// cmd/skills-manage only calls Run; domain rules stay in internal/workbench.
package app

import (
	"fmt"
	"os"
)

// Run is the process entry used by cmd/skills-manage.
// Returns a process exit code (0 success, non-zero failure).
func Run(args []string) int {
	if len(args) < 2 {
		printUsage()
		return 2
	}
	var err error
	switch args[1] {
	case "inventory", "list":
		err = runInventory(args[2:])
	case "desk":
		err = runDesk(args[2:])
	case "serve", "workbench":
		err = runServe(args[2:])
	case "help", "-h", "--help":
		printUsage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "skills-manage: unknown command %q\n", args[1])
		printUsage()
		return 2
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "skills-manage: %v\n", err)
		return 1
	}
	return 0
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `skills-manage — local agent skill taxonomy workbench

Usage:
  skills-manage inventory [flags]
  skills-manage list [flags]
  skills-manage desk [flags]
  skills-manage serve [flags]
  skills-manage workbench [flags]   (alias of serve)

Commands:
  inventory, list   Discover local Skills under scan roots (realpath identity)
  desk              Open/rescan desk: 占位 grid + 回收站; persist 中央索引
  serve, workbench  Start localhost HTTP + embedded UI; open browser (no daemon)

Flags for inventory / desk / serve:
  -root path    Scan root to include (repeatable). When omitted, uses common
                user and project skill paths (bundled/system trees excluded).
  -index path   Central index JSON (desk/serve). Default: $CONFIG/skills-manage/index.json

Flags for serve:
  -addr host:port   Listen address (default 127.0.0.1:0 = ephemeral port)
  -no-open          Do not open the default browser

Examples:
  skills-manage inventory
  skills-manage inventory -root ~/.agents/skills -root ./.claude/skills
  skills-manage desk
  skills-manage serve
  skills-manage serve -root /tmp/skills -index /tmp/sm-index.json -addr 127.0.0.1:8765
`)
}
