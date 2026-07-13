package cmd

import (
	"errors"
	"io"
	"os"

	"github.com/carvalhosauro/envkeep/internal/envfile"
	"github.com/carvalhosauro/envkeep/internal/fsutil"
	"github.com/carvalhosauro/envkeep/internal/state"
)

// Pull writes the shared vault into the current worktree's local env, composed
// with the per-worktree override (override wins, D9), preserving the local
// file's key order and comments (D11). It refuses when the local env holds
// changes not yet pushed (D5).
func Pull(w io.Writer, cwd, envFileFlag string, dryRun bool) error {
	ctx, err := Resolve(cwd, envFileFlag)
	if err != nil {
		return err
	}
	p := &printer{w: w}

	vaultEnv, vaultExists, err := readVault(ctx)
	if err != nil {
		return err
	}
	if !vaultExists {
		return errors.New("no vault yet; run 'envkeep push' first")
	}
	overrideEnv, err := readEnvOrEmpty(ctx.OverridePath)
	if err != nil {
		return err
	}
	localFile, localExists, err := readFile(ctx.LocalPath)
	if err != nil {
		return err
	}
	if !localExists {
		localFile = envfile.New()
	}
	localEnv := localFile.Map()
	localShared := localEnv.Without(overrideEnv)

	marker, hasMarker, err := state.Load(ctx.GitDir)
	if err != nil {
		return err
	}
	if hasMarker {
		switch st, conflicts := envfile.Classify(marker.Base, localShared, vaultEnv); st {
		case envfile.Clean:
			p.printf("already in sync; nothing to pull\n")
			return p.err
		case envfile.Ahead:
			return errors.New("local has changes not in the vault; run 'envkeep push' first")
		case envfile.Conflict:
			printConflicts(p, conflicts)
			if p.err != nil {
				return p.err
			}
			return errors.New("conflict: vault and local changed the same key(s); resolve, then pull")
		}
		// Behind or Diverged: safe to apply.
	}

	target := vaultEnv.Union(overrideEnv) // effective local = vault ⊕ override
	d := localEnv.Diff(target)
	if d.Empty() {
		p.printf("already in sync; nothing to pull\n")
		return p.err
	}
	printDelta(p, ctx.EnvFile, d)
	if dryRun {
		p.printf("(dry run — %s not written)\n", ctx.EnvFile)
		return p.err
	}

	for k, v := range target {
		localFile.Set(k, v)
	}
	for _, k := range localFile.Keys() {
		if _, keep := target[k]; !keep {
			localFile.Delete(k)
		}
	}
	if err := fsutil.WriteFileAtomic(ctx.LocalPath, localFile.Render(), localPerm(ctx.LocalPath, localExists)); err != nil {
		return err
	}
	if err := saveMarker(ctx, vaultEnv); err != nil {
		return err
	}
	p.printf("pulled %s -> %s\n", ctx.VaultPath, ctx.EnvFile)
	return p.err
}

// localPerm preserves an existing local env file's permission bits, defaulting
// to owner-only 0600 for a new file (it holds secrets).
func localPerm(path string, exists bool) os.FileMode {
	if exists {
		if fi, err := os.Stat(path); err == nil {
			return fi.Mode().Perm()
		}
	}
	return 0o600
}
