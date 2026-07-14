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

// Process exit codes: 0 success, 1 runtime error, 2 usage/flag-parse error.
const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		usage()
		return exitError
	}
	switch args[0] {
	case "version", "--version", "-v":
		fmt.Println("envkeep", buildinfo.Version)
		return exitOK
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
		return exitOK
	default:
		fmt.Fprintf(os.Stderr, "envkeep: unknown command %q\n", args[0])
		usage()
		return exitError
	}
}

func runStatus(args []string) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	file := fs.String("file", "", "tracked env filename (overrides repo config)")
	env := fs.String("env", "", "environment to compare against (default: each worktree's active env)")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	return dispatch(func(cwd string) error {
		return cli.Status(os.Stdout, cwd, *file, *env)
	})
}

func runPush(args []string) int {
	fs := flag.NewFlagSet("push", flag.ContinueOnError)
	file := fs.String("file", "", "tracked env filename (overrides repo config)")
	env := fs.String("env", "", "target environment (default: this worktree's active env)")
	create := fs.Bool("create", false, "create the environment if it does not exist")
	fs.BoolVar(create, "c", false, "shorthand for --create")
	dry := fs.Bool("dry-run", false, "show what would change without writing")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	return dispatch(func(cwd string) error {
		return cli.Push(os.Stdout, cwd, *file, *env, *create, *dry)
	})
}

func runPull(args []string) int {
	fs := flag.NewFlagSet("pull", flag.ContinueOnError)
	file := fs.String("file", "", "tracked env filename (overrides repo config)")
	env := fs.String("env", "", "environment to pull (default: this worktree's active env)")
	create := fs.Bool("create", false, "create the environment if it does not exist")
	fs.BoolVar(create, "c", false, "shorthand for --create")
	dry := fs.Bool("dry-run", false, "show what would change without writing")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	return dispatch(func(cwd string) error {
		return cli.Pull(os.Stdout, cwd, *file, *env, *create, *dry)
	})
}

func runCheck(args []string) int {
	fs := flag.NewFlagSet("check", flag.ContinueOnError)
	porcelain := fs.Bool("porcelain", false, "print a bare state token (for scripts/prompts)")
	if err := fs.Parse(args); err != nil {
		return exitUsage
	}
	return dispatch(func(cwd string) error {
		return cli.Check(os.Stdout, cwd, *porcelain)
	})
}

func runHook(args []string) int {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: envkeep hook <zsh|bash>")
		return exitUsage
	}
	snippet, err := hook.Snippet(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "envkeep:", err)
		return exitError
	}
	fmt.Print(snippet)
	return exitOK
}

func dispatch(fn func(cwd string) error) int {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(os.Stderr, "envkeep:", err)
		return exitError
	}
	if err := fn(cwd); err != nil {
		fmt.Fprintln(os.Stderr, "envkeep:", err)
		return exitError
	}
	return exitOK
}

func usage() {
	fmt.Fprint(os.Stderr, `envkeep — keep .env in sync across git worktrees

usage:
  envkeep status [--env E]         show each worktree's active env and sync state
  envkeep push [--env E] [-c]      merge this worktree's .env into the env's vault
  envkeep pull [--env E] [-c]      write the env's vault into this worktree's .env
  envkeep check                    quiet drift check for the current worktree (for hooks)
  envkeep hook <zsh|bash>          print shell integration to source in your rc file
  envkeep version

flags:
  --file NAME   tracked env filename (overrides repo config; default .env)
  --env NAME    environment to target; unset uses the worktree's active env,
                else the repo default_env, else the unnamed (legacy) vault
  -c, --create  create the environment (push/pull) if it does not exist
  --dry-run     show what would change without writing (push/pull)

environments:
  A key can hold different values per environment (e.g. local / homo / prod).
  Environments work like git branches: 'envkeep pull --env prod' switches only
  to one that exists; add '--create' (-c) to make a new one. Each worktree keeps
  its own active environment.

shell integration:
  eval "$(envkeep hook zsh)"   # or: bash — warns on drift when you cd into a worktree
`)
}
