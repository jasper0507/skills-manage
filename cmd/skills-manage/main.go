// Command skills-manage is the CLI for the local skill taxonomy workbench.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/jasper0507/skills-manage/internal/index"
	"github.com/jasper0507/skills-manage/internal/server"
	"github.com/jasper0507/skills-manage/internal/ui"
	"github.com/jasper0507/skills-manage/internal/workbench"
)

func main() {
	if len(os.Args) < 2 {
		usage(2)
	}
	switch os.Args[1] {
	case "inventory", "list":
		if err := runInventory(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "skills-manage: %v\n", err)
			os.Exit(1)
		}
	case "desk":
		if err := runDesk(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "skills-manage: %v\n", err)
			os.Exit(1)
		}
	case "serve", "workbench":
		if err := runServe(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "skills-manage: %v\n", err)
			os.Exit(1)
		}
	case "help", "-h", "--help":
		usage(0)
	default:
		fmt.Fprintf(os.Stderr, "skills-manage: unknown command %q\n", os.Args[1])
		usage(2)
	}
}

func runInventory(args []string) error {
	fs := flag.NewFlagSet("inventory", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var roots multiFlag
	fs.Var(&roots, "root", "scan root to include (repeatable); default: common user and project skill paths")
	if err := fs.Parse(args); err != nil {
		return err
	}

	scanRoots, err := resolveScanRoots(roots)
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

	// Stable, demo-friendly table: name and identity (realpath).
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
	var roots multiFlag
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
	var roots multiFlag
	fs.Var(&roots, "root", "scan root to include (repeatable); default: common user and project skill paths")
	indexPath := fs.String("index", "", "central index JSON path (default: user config dir)")
	addr := fs.String("addr", "127.0.0.1:0", "listen address (default ephemeral port on localhost)")
	noOpen := fs.Bool("no-open", false, "do not open the default browser")
	if err := fs.Parse(args); err != nil {
		return err
	}

	wb, path, err := openWorkbench(roots, *indexPath)
	if err != nil {
		return err
	}

	srv := server.New(wb).WithStatic(ui.FS)
	url, serve, closeFn, err := server.ListenAndServe(*addr, srv.Handler())
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	defer closeFn()

	fmt.Printf("分类工作台  %s\n", url)
	fmt.Printf("INDEX       %s\n", path)
	fmt.Fprintf(os.Stderr, "Press Ctrl+C to stop (process exits with the command; no daemon).\n")

	if !*noOpen {
		if err := openBrowser(url); err != nil {
			fmt.Fprintf(os.Stderr, "skills-manage: open browser: %v (open %s manually)\n", err, url)
		}
	}

	// Stop cleanly on SIGINT/SIGTERM so the short-lived server is not left hanging.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	errCh := make(chan error, 1)
	go func() { errCh <- serve() }()

	select {
	case sig := <-sigCh:
		fmt.Fprintf(os.Stderr, "\nstopping (%v)…\n", sig)
		_ = closeFn()
		<-errCh
		return nil
	case err := <-errCh:
		return err
	}
}

func openWorkbench(roots multiFlag, indexPath string) (*workbench.Workbench, string, error) {
	scanRoots, err := resolveScanRoots(roots)
	if err != nil {
		return nil, "", err
	}
	path := indexPath
	if path == "" {
		cfg, err := os.UserConfigDir()
		if err != nil {
			return nil, "", fmt.Errorf("resolve config directory: %w", err)
		}
		path = workbench.DefaultIndexPath(cfg)
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

func resolveScanRoots(roots multiFlag) ([]string, error) {
	scanRoots := []string(roots)
	if len(scanRoots) > 0 {
		return scanRoots, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("resolve home directory: %w", err)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("resolve working directory: %w", err)
	}
	return workbench.DefaultScanRoots(home, cwd), nil
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported OS %q", runtime.GOOS)
	}
	return cmd.Start()
}

func usage(code int) {
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
	os.Exit(code)
}

// multiFlag accumulates repeated -root values.
type multiFlag []string

func (m *multiFlag) String() string {
	return strings.Join(*m, ",")
}

func (m *multiFlag) Set(value string) error {
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
