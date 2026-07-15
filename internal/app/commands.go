// Command implementations (inventory, desk, serve).
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
