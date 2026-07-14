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
// refuses when the vault holds changes the local env would clobber (D5).
func Push(w io.Writer, cwd, envFileFlag, envFlag string, create, dryRun bool) error {
	ctx, err := Resolve(cwd, envFileFlag, envFlag)
	if err != nil {
		return err
	}
	p := &printer{w: w}

	localEnv, ok, err := readEnv(ctx.LocalPath)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("no %s in this worktree to push", ctx.EnvFile)
	}
	overrideEnv, err := readEnvOrEmpty(ctx.OverridePath)
	if err != nil {
		return err
	}
	localShared := localEnv.Without(overrideEnv)

	marker, hasMarker, err := state.Load(ctx.GitDir)
	if err != nil {
		return err
	}
	activeEnv := ctx.resolveEnv(marker.Env)
	if err := ensureTargetEnv(ctx, activeEnv, create, dryRun); err != nil {
		return err
	}

	vaultEnv, vaultExists, err := readVaultFor(ctx, activeEnv)
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
