// Command skills-manage is the CLI for the local skill taxonomy workbench.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

	scanRoots := []string(roots)
	if len(scanRoots) == 0 {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("resolve home directory: %w", err)
		}
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("resolve working directory: %w", err)
		}
		scanRoots = workbench.DefaultScanRoots(home, cwd)
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

func usage(code int) {
	fmt.Fprintf(os.Stderr, `skills-manage — local agent skill taxonomy workbench

Usage:
  skills-manage inventory [flags]
  skills-manage list [flags]

Commands:
  inventory, list   Discover local Skills under scan roots (realpath identity)

Flags for inventory:
  -root path   Scan root to include (repeatable). When omitted, uses common
               user and project skill paths (bundled/system trees excluded).

Examples:
  skills-manage inventory
  skills-manage inventory -root ~/.agents/skills -root ./.claude/skills
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
