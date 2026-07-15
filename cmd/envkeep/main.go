// Command envkeep keeps .env files in sync across the git worktrees of one
// repository. See docs/ for design and roadmap.
package main

import (
	"os"

	"github.com/carvalhosauro/envkeep/internal/cli"
)

func main() {
	os.Exit(cli.Execute())
}
