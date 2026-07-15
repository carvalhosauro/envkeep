package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/carvalhosauro/envkeep/internal/envfile"
	"github.com/carvalhosauro/envkeep/internal/state"
	"github.com/carvalhosauro/envkeep/internal/vault"
)

// Push merges the current worktree's local env (minus override keys) into the
// shared vault. It is a union merge — local wins on shared keys, vault-only keys
// are kept — so a worktree can never delete a key another worktree added (D8).
// It refuses when the vault holds changes the local env would clobber (D5).
func Push(w io.Writer, cwd, envFileFlag string, dryRun bool) error {
	ctx, err := Resolve(cwd, envFileFlag)
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

	vaultEnv, vaultExists, err := readVault(ctx)
	if err != nil {
		return err
	}
	marker, hasMarker, err := state.Load(ctx.GitDir)
	if err != nil {
		return err
	}

	if vaultExists && hasMarker {
		switch st, conflicts := envfile.Classify(marker.Base, localShared, vaultEnv); st {
		case envfile.Clean:
			// Content agrees with the vault. Retire a stale base so status/check
			// stop reporting a false diverged and the mtime fast path is restored.
			if !marker.Base.Equal(vaultEnv) {
				if err := saveMarker(ctx, vaultEnv); err != nil {
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
		// Byte-identical yet never synced: no marker means status/check keep
		// reporting unsynced. Establish the marker so it settles to clean (#17).
		if vaultExists && !hasMarker {
			if err := saveMarker(ctx, newVault); err != nil {
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
	if err := ctx.Vault.Write(newVault); err != nil {
		return err
	}
	if err := saveMarker(ctx, newVault); err != nil {
		return err
	}
	p.printf("pushed %s -> %s\n", ctx.EnvFile, ctx.VaultPath)
	return p.err
}

// readVault reads the vault, mapping the fresh (not-yet-created) state to an
// empty set with exists=false.
func readVault(ctx *Context) (envfile.Env, bool, error) {
	env, err := ctx.Vault.Read()
	if errors.Is(err, vault.ErrNotFound) {
		return envfile.Env{}, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return env, true, nil
}
