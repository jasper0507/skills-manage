// Command skills-manage is the CLI entry for the local skill taxonomy workbench.
// All assembly and command logic lives in internal/app.
package main

import (
	"os"

	"github.com/jasper0507/skills-manage/internal/app"
)

func main() {
	os.Exit(app.Run(os.Args))
}
