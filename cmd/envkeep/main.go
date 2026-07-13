// Command envkeep keeps .env files in sync across the git worktrees of one
// repository. See docs/ for design and roadmap.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/carvalhosauro/envkeep/internal/buildinfo"
	"github.com/carvalhosauro/envkeep/internal/cli"
	"github.com/carvalhosauro/envkeep/internal/hook"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		usage()
		return 1
	}
	switch args[0] {
	case "version", "--version", "-v":
		fmt.Println("envkeep", buildinfo.Version)
		return 0
	case "status":
		return runStatus(args[1:])
	case "push":
		return runPush(args[1:])
	case "pull":
		return runPull(args[1:])
	case "check":
		return runCheck(args[1:])
	case "hook":
		return runHook(args[1:])
	case "help", "-h", "--help":
		usage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "envkeep: unknown command %q\n", args[0])
		usage()
		return 1
	}
}

func runStatus(args []string) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	file := fs.String("file", "", "tracked env filename (overrides repo config)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	return dispatch(func(cwd string) error {
		return cli.Status(os.Stdout, cwd, *file)
	})
}

func runPush(args []string) int {
	fs := flag.NewFlagSet("push", flag.ContinueOnError)
	file := fs.String("file", "", "tracked env filename (overrides repo config)")
	dry := fs.Bool("dry-run", false, "show what would change without writing")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	return dispatch(func(cwd string) error {
		return cli.Push(os.Stdout, cwd, *file, *dry)
	})
}

func runPull(args []string) int {
	fs := flag.NewFlagSet("pull", flag.ContinueOnError)
	file := fs.String("file", "", "tracked env filename (overrides repo config)")
	dry := fs.Bool("dry-run", false, "show what would change without writing")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	return dispatch(func(cwd string) error {
		return cli.Pull(os.Stdout, cwd, *file, *dry)
	})
}

func runCheck(args []string) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	porcelain := fs.Bool("porcelain", false, "print a bare state token (for scripts/prompts)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	return dispatch(func(cwd string) error {
		return cli.Check(os.Stdout, cwd, *porcelain)
	})
}

func runHook(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: envkeep hook <zsh|bash>")
		return 2
	}
	snippet, err := hook.Snippet(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "envkeep:", err)
		return 1
	}
	fmt.Print(snippet)
	return 0
}

func dispatch(fn func(cwd string) error) int {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "envkeep:", err)
		return 1
	}
	if err := fn(cwd); err != nil {
		fmt.Fprintln(os.Stderr, "envkeep:", err)
		return 1
	}
	return 0
}

func usage() {
	fmt.Fprint(os.Stderr, `envkeep — keep .env in sync across git worktrees

usage:
  envkeep status            show each worktree's sync state vs the vault
  envkeep push [--dry-run]  merge this worktree's .env into the vault
  envkeep pull [--dry-run]  write the vault into this worktree's .env
  envkeep check             quiet drift check for the current worktree (for hooks)
  envkeep hook <zsh|bash>   print shell integration to source in your rc file
  envkeep version

flags:
  --file NAME   tracked env filename (overrides repo config; default .env)

shell integration:
  eval "$(envkeep hook zsh)"   # or: bash — warns on drift when you cd into a worktree
`)
}
