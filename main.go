// Command envkeep keeps .env files in sync across the git worktrees of one
// repository. See docs/ for design and roadmap.
package main

import (
	"fmt"
	"os"

	"github.com/carvalhosauro/envkeep/internal/buildinfo"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) > 0 {
		switch args[0] {
		case "version", "--version", "-v":
			fmt.Println("envkeep", buildinfo.Version)
			return 0
		}
	}
	// Subcommands (status/push/pull/hook) arrive in Phase 1 — see docs/ROADMAP.md.
	fmt.Fprintln(os.Stderr, "envkeep: no command implemented yet (v1 in progress)")
	return 1
}
