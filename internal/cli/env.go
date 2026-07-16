package cli

import (
	"bytes"
	"errors"
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

// UseCascade switches every worktree in the repo to envName — the opt-in
// cascade fan-out (D28) — reusing Pull's guards per worktree instead of
// re-implementing conflict/ahead detection. A worktree whose current
// environment is ahead, conflicted, or holds edits the re-point guard refuses
// to discard (E4) is skipped and reported rather than clobbered; every other
// worktree is switched. dryRun previews without writing anything. An error
// from Pull that is not a refusal (errors.Is(err, ErrRefused)) — a genuine
// I/O or resolve failure — aborts the whole cascade immediately.
func UseCascade(w io.Writer, cwd, envName string, dryRun bool) error {
	ctx, err := Resolve(cwd, "", "")
	if err != nil {
		return err
	}
	e := env.Name(envName)
	if !vault.EnvExists(ctx.CommonDir, e) {
		return fmt.Errorf("unknown environment %q", envName)
	}
	wts, err := git.Worktrees(cwd)
	if err != nil {
		return err
	}

	type skipped struct{ path, reason string }
	var applied []string
	var skips []skipped
	for _, wt := range wts {
		if wt.Bare {
			continue
		}
		var buf bytes.Buffer
		switch err := Pull(&buf, wt.Path, "", envName, false, dryRun); {
		case err == nil:
			applied = append(applied, wt.Path)
		case errors.Is(err, ErrRefused):
			skips = append(skips, skipped{path: wt.Path, reason: err.Error()})
		default:
			return err
		}
	}

	p := &printer{w: w}
	verb := "switched"
	if dryRun {
		verb = "would switch"
	}
	for _, path := range applied {
		p.printf("%s: %s to %q\n", path, verb, envName)
	}
	for _, s := range skips {
		p.printf("%s: skipped (%s)\n", s.path, s.reason)
	}
	if len(applied) == 0 && len(skips) == 0 {
		p.printf("no worktrees found\n")
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
