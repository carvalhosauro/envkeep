package cli

import (
	"fmt"
	"io"

	"github.com/carvalhosauro/envkeep/internal/env"
	"github.com/carvalhosauro/envkeep/internal/git"
	"github.com/carvalhosauro/envkeep/internal/state"
	"github.com/carvalhosauro/envkeep/internal/vault"
)

// Envs lists the repo's environments, marking the default with "*".
func Envs(w io.Writer, cwd string) error {
	ctx, err := Resolve(cwd, "", "")
	if err != nil {
		return err
	}
	p := &printer{w: w}
	envs, err := vault.Environments(ctx.CommonDir)
	if err != nil {
		return err
	}
	if len(envs) == 0 {
		p.printf("no environments yet (run 'envkeep use -c <env>' or 'envkeep push --env <env> --create')\n")
		return p.err
	}
	for _, e := range envs {
		marker := " "
		if e == ctx.DefaultEnv {
			marker = "*"
		}
		p.printf("%s %s\n", marker, e.String())
	}
	return p.err
}

// RmEnv deletes a named environment. Unless force is set, it refuses when any
// worktree's active env (marker) still points at it (E5).
func RmEnv(w io.Writer, cwd, envName string, force bool) error {
	ctx, err := Resolve(cwd, "", "")
	if err != nil {
		return err
	}
	e := env.Name(envName)
	if !vault.EnvExists(ctx.CommonDir, e) {
		return fmt.Errorf("unknown environment %q", envName)
	}
	if !force {
		wts, err := git.Worktrees(cwd)
		if err != nil {
			return err
		}
		for _, wt := range wts {
			if wt.Bare {
				continue
			}
			gitDir, err := git.Dir(wt.Path)
			if err != nil {
				continue
			}
			if stat, ok, _ := state.LoadStat(gitDir); ok && stat.Env == e {
				return fmt.Errorf("worktree %q is on environment %q; switch it or use --force", wt.Path, envName)
			}
		}
	}
	if err := vault.RemoveEnv(ctx.CommonDir, e); err != nil {
		return err
	}
	p := &printer{w: w}
	p.printf("removed environment %q\n", envName)
	return p.err
}
