package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"

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

// Use switches the current worktree to ctx.EnvFlag. Switching to an existing env
// pulls its content. With create, a NON-EXISTENT env is created from the current
// worktree's env — like `git checkout -b`: the new vault is snapshotted from local
// content and the worktree re-points to it, leaving local intact (push-create path,
// D26). An existing env with -c just switches. ctx must come from Resolve with the
// target environment in envFlag (as `use <env>` does).
func Use(ctx *Context, w io.Writer, create, dryRun bool) error {
	if create && !vault.EnvExists(ctx.CommonDir, ctx.EnvFlag) {
		return push(ctx, w, PushOpts{SyncOpts: SyncOpts{Create: true, DryRun: dryRun}}) // checkout -b: create from current
	}
	return pull(ctx, w, SyncOpts{Create: create, DryRun: dryRun}) // switch to existing
}

// UseCascade switches every worktree in the repo to ctx.EnvFlag — the opt-in
// cascade fan-out (D28) — reusing Pull's guards per worktree instead of
// re-implementing conflict/ahead detection. A worktree whose current
// environment is ahead, conflicted, or holds edits the re-point guard refuses
// to discard (E4) is skipped and reported rather than clobbered; every other
// worktree is switched. dryRun previews without writing anything. An error
// from pull that is not a refusal (errors.Is(err, ErrRefused)) — a genuine
// I/O or resolve failure — aborts the whole cascade immediately: the summary
// of worktrees already switched or skipped before the failure is still
// flushed to w, and the returned error is wrapped with the failing worktree's
// path (via %w, so errors.Is/As still see the underlying error) for context.
// ctx must come from Resolve with the target environment in envFlag.
func UseCascade(ctx *Context, w io.Writer, dryRun bool) error {
	e := ctx.EnvFlag
	if !vault.EnvExists(ctx.CommonDir, e) {
		return fmt.Errorf("unknown environment %q", e.String())
	}
	wts, err := git.Worktrees(filepath.Dir(ctx.self.localPath))
	if err != nil {
		return err
	}

	type skipped struct{ path, reason string }
	var applied []string
	var skips []skipped
	verb := "switched"
	if dryRun {
		verb = "would switch"
	}
	envName := e.String()
	// printSummary writes the "applied"/"skipped" lines collected so far. It is
	// shared by the normal end-of-loop report and by an abort mid-loop: a
	// genuine (non-refusal) error must not discard the report of worktrees
	// already switched or skipped before the one that failed (D28's contract is
	// that switched worktrees are reported, abort or not).
	printSummary := func() error {
		p := &printer{w: w}
		for _, path := range applied {
			p.printf("%s: %s to %q\n", path, verb, envName)
		}
		for _, s := range skips {
			p.printf("%s: skipped (%s)\n", s.path, s.reason)
		}
		return p.err
	}

	for _, wt := range wts {
		if wt.Bare {
			continue
		}
		wtCtx, err := ctx.ForWorktree(wt.Path)
		if err != nil {
			flushErr := printSummary()
			aborted := fmt.Errorf("%s: %w", wt.Path, err)
			if flushErr != nil {
				return errors.Join(flushErr, aborted)
			}
			return aborted
		}
		var buf bytes.Buffer
		switch err := pull(wtCtx, &buf, SyncOpts{DryRun: dryRun}); {
		case err == nil:
			applied = append(applied, wt.Path)
		case errors.Is(err, ErrRefused):
			skips = append(skips, skipped{path: wt.Path, reason: err.Error()})
		default:
			// Flush what was already switched/skipped before this worktree, then
			// abort with the failing worktree named so the caller can tell which
			// one broke the cascade.
			flushErr := printSummary()
			aborted := fmt.Errorf("%s: %w", wt.Path, err)
			if flushErr != nil {
				return errors.Join(flushErr, aborted)
			}
			return aborted
		}
	}

	if err := printSummary(); err != nil {
		return err
	}
	p := &printer{w: w}
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
