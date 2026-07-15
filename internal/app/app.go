// Package app assembles configuration, Workbench, and transports, then runs CLI commands.
// cmd/skills-manage only calls Run; domain rules stay in internal/workbench.
package app

import (
	"flag"
	"fmt"
	"os"

	"github.com/jasper0507/skills-manage/config"
	"github.com/jasper0507/skills-manage/internal/infra/index"
	"github.com/jasper0507/skills-manage/internal/server"
	"github.com/jasper0507/skills-manage/internal/ui"
	"github.com/jasper0507/skills-manage/internal/workbench"
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

func runInventory(args []string) error {
	fs := flag.NewFlagSet("inventory", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var roots config.MultiFlag
	fs.Var(&roots, "root", "scan root to include (repeatable); default: common user and project skill paths")
	if err := fs.Parse(args); err != nil {
		return err
	}

	scanRoots, err := config.ResolveScanRoots(roots)
	if err != nil {
		return err
	}

	wb := workbench.New(workbench.Config{ScanRoots: scanRoots})
	inv, err := wb.Inventory()
	if err != nil {
		return err
	}

	if len(inv.Skills) == 0 {
		fmt.Println("No skills found.")
		fmt.Fprintf(os.Stderr, "Scanned %d root(s).\n", len(scanRoots))
		return nil
	}

	nameWidth := len("NAME")
	for _, s := range inv.Skills {
		if len(s.Name) > nameWidth {
			nameWidth = len(s.Name)
		}
	}
	fmt.Printf("%-*s  %s\n", nameWidth, "NAME", "IDENTITY")
	for _, s := range inv.Skills {
		fmt.Printf("%-*s  %s\n", nameWidth, s.Name, s.Identity)
	}
	fmt.Fprintf(os.Stderr, "\n%d skill(s) from %d scan root(s).\n", len(inv.Skills), len(scanRoots))
	return nil
}

func runDesk(args []string) error {
	fs := flag.NewFlagSet("desk", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var roots config.MultiFlag
	fs.Var(&roots, "root", "scan root to include (repeatable); default: common user and project skill paths")
	indexPath := fs.String("index", "", "central index JSON path (default: user config dir)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	wb, path, err := openWorkbench(roots, *indexPath)
	if err != nil {
		return err
	}
	desk := wb.Desk()

	fmt.Printf("INDEX  %s\n", path)
	fmt.Printf("RECYCLE  row=%d col=%d\n", desk.RecycleIcon.Location.Row, desk.RecycleIcon.Location.Col)
	if len(desk.Placeholders) == 0 {
		fmt.Println("No skill placeholders.")
		return nil
	}
	nameWidth := len("NAME")
	for _, p := range desk.Placeholders {
		if len(p.Name) > nameWidth {
			nameWidth = len(p.Name)
		}
	}
	fmt.Printf("%-*s  %4s  %4s  %s\n", nameWidth, "NAME", "ROW", "COL", "IDENTITY")
	for _, p := range desk.Placeholders {
		row, col := p.Location.Row, p.Location.Col
		if p.Location.Kind != workbench.LocDesktop {
			fmt.Printf("%-*s  %4s  %4s  %s  (%s)\n", nameWidth, p.Name, "-", "-", p.Identity, p.Location.Kind)
			continue
		}
		fmt.Printf("%-*s  %4d  %4d  %s\n", nameWidth, p.Name, row, col, p.Identity)
	}
	fmt.Fprintf(os.Stderr, "\n%d placeholder(s); layout persisted to central index.\n", len(desk.Placeholders))
	return nil
}

func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var roots config.MultiFlag
	fs.Var(&roots, "root", "scan root to include (repeatable); default: common user and project skill paths")
	indexPath := fs.String("index", "", "central index JSON path (default: user config dir)")
	addr := fs.String("addr", config.DefaultServeAddr, "listen address (default ephemeral port on localhost)")
	noOpen := fs.Bool("no-open", false, "do not open the default browser")
	if err := fs.Parse(args); err != nil {
		return err
	}

	wb, path, err := openWorkbench(roots, *indexPath)
	if err != nil {
		return err
	}

	srv := server.New(wb).WithStatic(ui.FS)
	return server.Run(server.RunOptions{
		Addr:        *addr,
		Handler:     srv.Handler(),
		IndexPath:   path,
		OpenBrowser: !*noOpen,
	})
}

func openWorkbench(roots config.MultiFlag, indexPath string) (*workbench.Workbench, string, error) {
	scanRoots, err := config.ResolveScanRoots(roots)
	if err != nil {
		return nil, "", err
	}
	path, err := config.ResolveIndexPath(indexPath)
	if err != nil {
		return nil, "", err
	}
	wb := workbench.New(workbench.Config{
		ScanRoots: scanRoots,
		Index:     index.NewFileStore(path),
	})
	if err := wb.Open(); err != nil {
		return nil, "", err
	}
	return wb, path, nil
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
