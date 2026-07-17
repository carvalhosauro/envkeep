package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/carvalhosauro/envkeep/internal/envfile"
	"github.com/carvalhosauro/envkeep/internal/state"
)

// Push merges the current worktree's local env (minus override keys) into the
// active environment's vault. It is a union merge — local wins on shared keys,
// vault-only keys are kept — so a worktree can never delete a key another
// worktree added (D8). The environment is resolved by --env > marker >
// default_env (D25); targeting a non-existent env needs --create (D26). It
// refuses when the vault holds changes the local env would clobber (D5), and —
// when pushing to an env other than the worktree's own (a cross-env push,
// where no 3-way base exists) — refuses to overwrite keys whose value differs
// in the target vault unless force is set (#20).
func Push(w io.Writer, cwd, envFileFlag, envFlag string, create, dryRun, force bool) error {
	ctx, err := Resolve(cwd, envFileFlag, envFlag)
	if err != nil {
		return err
	}
	return pushResolved(w, ctx, create, dryRun, force)
}

// pushResolved is Push's body over an already-resolved Context, letting a
// caller that already resolved the repo (Use) skip a second git rev-parse +
// config.Load (D25's resolution is otherwise paid twice per `use`).
func pushResolved(w io.Writer, ctx *Context, create, dryRun, force bool) error {
	p := &printer{w: w}

	localShared, ok, err := readShared(ctx.self.localPath, ctx.self.overridePath)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("no %s in this worktree to push", ctx.EnvFile)
	}

	marker, hasMarker, err := state.Load(ctx.self.gitDir)
	if err != nil {
		return err
	}
	activeEnv := ctx.resolveEnv(marker.Env)
	if err := ctx.ensureTargetEnv(activeEnv, create, dryRun); err != nil {
		return err
	}

	vaultEnv, vaultExists, err := ctx.readVault(activeEnv)
	if err != nil {
		return err
	}

	// The 3-way base guard only applies when the marker's base belongs to the
	// env being pushed (marker.Env == activeEnv). A cross-env or first-time push
	// has no valid base here, so it falls back to a plain union — which is safe
	// by D8 (union never deletes a key another worktree added).
	sameEnv := hasMarker && marker.Env == activeEnv
	if vaultExists && sameEnv {
		switch st, conflicts := envfile.Classify(marker.Base, localShared, vaultEnv); st {
		case envfile.Clean:
			if !marker.Base.Equal(vaultEnv) {
				if err := saveMarker(ctx, activeEnv, vaultEnv); err != nil {
					return err
				}
			}
			p.printf("already in sync; nothing to push\n")
			return p.err
		case envfile.Behind:
			return errors.New("vault has changes not in your local env; run 'envkeep pull' first")
		case envfile.Conflict:
			printConflicts(p, conflicts)
			if p.err != nil {
				return p.err
			}
			return errors.New("conflict: vault and local changed the same key(s); resolve, then push")
		}
		// Ahead or Diverged: safe to merge.
	}

	newVault := vaultEnv.Union(localShared)
	d := vaultEnv.Diff(newVault)
	if d.Empty() {
		// Vault already holds every local key. Refresh the marker when it is
		// missing (byte-identical yet never synced — #17), stale, or points at a
		// different env, so status/check settle to clean and the fast path is
		// right. Guard on vaultExists so we never write a marker with no vault
		// behind it (#17).
		if vaultExists && (!sameEnv || !marker.Base.Equal(vaultEnv)) {
			if err := saveMarker(ctx, activeEnv, vaultEnv); err != nil {
				return err
			}
		}
		p.printf("already in sync; nothing to push\n")
		return p.err
	}
	// Cross-env push (marker points at a different env): there is no 3-way base
	// against the target vault, so a differing key can't be told apart from a
	// clobber. Never silently overwrite another env's values (#20).
	if hasMarker && !sameEnv && !force && len(d.Changed) > 0 {
		p.printf("keys that differ in environment %q:\n", activeEnv)
		for _, c := range d.Changed {
			p.printf("  %s\n", c.Key)
		}
		if p.err != nil {
			return p.err
		}
		return fmt.Errorf("cross-env push would overwrite key(s) in %q; re-run with --force to overwrite", activeEnv)
	}
	printDelta(p, "vault", d)
	if dryRun {
		p.printf("(dry run — vault not written)\n")
		return p.err
	}
	if err := ctx.vaultStore(activeEnv).Write(newVault); err != nil {
		return err
	}
	if err := saveMarker(ctx, activeEnv, newVault); err != nil {
		return err
	}
	p.printf("pushed %s -> %s\n", ctx.EnvFile, ctx.vaultPath(activeEnv))
	return p.err
}
