package cli

import (
	"errors"
	"fmt"
	"io"

	"github.com/carvalhosauro/envkeep/internal/envfile"
	"github.com/carvalhosauro/envkeep/internal/fsutil"
	"github.com/carvalhosauro/envkeep/internal/state"
)

// ErrRefused marks a Pull guard's refusal — ahead, conflict, or a re-point
// blocked by unpushed edits (E4) — rather than a genuine I/O or resolve error.
// Cascade (UseCascade, D28) uses errors.Is(err, ErrRefused) to downgrade a
// refusal into a per-worktree skip instead of aborting the whole fan-out.
var ErrRefused = errors.New("refused")

// refusal wraps a refusal error so it satisfies errors.Is(err, ErrRefused)
// while its Error() text stays byte-identical to the wrapped message — Pull
// already shipped with these exact strings (Deliverable A), and a direct
// `envkeep pull` caller must see them unchanged.
type refusal struct{ err error }

func (r refusal) Error() string        { return r.err.Error() }
func (r refusal) Unwrap() error        { return r.err }
func (r refusal) Is(target error) bool { return target == ErrRefused }

// refused wraps err as a classifiable refusal (see refusal).
func refused(err error) error { return refusal{err} }

// Pull writes the active environment's vault into the current worktree's local
// env, composed with the per-worktree override (override wins, D9), preserving
// the local file's key order and comments (D11). The environment is resolved by
// --env > marker > default_env (D25); targeting a non-existent env needs
// --create (D26). It refuses when the local env holds changes not yet pushed
// (D5), and — when switching to a different env (a re-point) — refuses to
// discard unpushed edits in the worktree's current env (E4).
func Pull(w io.Writer, cwd, envFileFlag, envFlag string, create, dryRun bool) error {
	ctx, err := Resolve(cwd, envFileFlag, envFlag)
	if err != nil {
		return err
	}
	return pullResolved(w, ctx, create, dryRun)
}

// pullResolved is Pull's body over an already-resolved Context, letting a
// caller that already resolved the repo (Use) skip a second git rev-parse +
// config.Load (D25's resolution is otherwise paid twice per `use`).
func pullResolved(w io.Writer, ctx *Context, create, dryRun bool) error {
	p := &printer{w: w}

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
	if !vaultExists && !create {
		return errors.New("no vault yet; run 'envkeep push' first")
	}
	overrideEnv, err := readEnvOrEmpty(ctx.self.overridePath)
	if err != nil {
		return err
	}
	localFile, localExists, err := readFile(ctx.self.localPath)
	if err != nil {
		return err
	}
	if !localExists {
		localFile = envfile.New()
	}
	localEnv := localFile.Map()
	localShared := localEnv.Without(overrideEnv)

	sameEnv := hasMarker && marker.Env == activeEnv
	switch {
	case sameEnv:
		switch st, conflicts := envfile.Classify(marker.Base, localShared, vaultEnv); st {
		case envfile.Clean:
			// Content agrees with the vault. Retire a stale base so status/check
			// stop reporting a false diverged and the mtime fast path is restored.
			if !marker.Base.Equal(vaultEnv) {
				if err := saveMarker(ctx, activeEnv, vaultEnv); err != nil {
					return err
				}
			}
			p.printf("already in sync; nothing to pull\n")
			return p.err
		case envfile.Ahead:
			return refused(errors.New("local has changes not in the vault; run 'envkeep push' first"))
		case envfile.Conflict:
			printConflicts(p, conflicts)
			if p.err != nil {
				return p.err
			}
			return refused(errors.New("conflict: vault and local changed the same key(s); resolve, then pull"))
		}
		// Behind or Diverged: safe to apply.
	case hasMarker:
		// Re-point to a different environment (marker.Env != activeEnv). Guard the
		// current env's unpushed local edits (E4) — never discard them silently.
		if !marker.Base.Equal(localShared) {
			return refused(fmt.Errorf("local has changes not pushed to environment %q; push or discard before switching to %q", marker.Env, activeEnv))
		}
		// Local matches the current env's base → safe to re-point.
	}

	target := vaultEnv.Union(overrideEnv) // effective local = vault ⊕ override
	d := localEnv.Diff(target)
	if d.Empty() {
		// Nothing to write, but refresh the marker when it is missing
		// (byte-identical yet never synced — #17), stale, or points at a
		// different env (a re-point), so status/check settle to clean and record
		// the active env. Guard on vaultExists so a just-created empty env with
		// no vault yet does not get a marker with nothing behind it.
		if vaultExists && (!sameEnv || !marker.Base.Equal(vaultEnv)) {
			if err := saveMarker(ctx, activeEnv, vaultEnv); err != nil {
				return err
			}
		}
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
	// Always 0600 — the file provably holds secrets, so a pre-existing wider
	// mode (e.g. 0644) is tightened rather than preserved (#23), matching the
	// vault and marker policy.
	if err := fsutil.WriteFileAtomic(ctx.self.localPath, localFile.Render(), fsutil.SecretFilePerm); err != nil {
		return err
	}
	if err := saveMarker(ctx, activeEnv, vaultEnv); err != nil {
		return err
	}
	p.printf("pulled %s -> %s\n", ctx.vaultPath(activeEnv), ctx.EnvFile)
	return p.err
}
