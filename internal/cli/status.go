package cli

import (
	"io"
	"path/filepath"

	"github.com/carvalhosauro/envkeep/internal/envfile"
	"github.com/carvalhosauro/envkeep/internal/git"
	"github.com/carvalhosauro/envkeep/internal/state"
)

// Status prints, for every worktree of the repo, how its local env file relates
// to the shared vault: clean / ahead / behind / diverged / conflict / absent /
// unsynced.
func Status(w io.Writer, cwd, envFileFlag string) error {
	ctx, err := Resolve(cwd, envFileFlag)
	if err != nil {
		return err
	}
	p := &printer{w: w}

	// Stat the vault mtime up front (cheap) for the mtime fast path, but defer
	// parsing it: only slow-path worktrees (an mtime miss) need the parsed vault,
	// and the parse dominates cost, so parse at most once and only on first need.
	// When every worktree is clean, the vault is never read (#11, mirrors check).
	vaultMTime, vaultExists := mtimeNanos(ctx.VaultPath)
	var vaultEnv envfile.Env
	var vaultErr error
	vaultLoaded := false
	loadVault := func() (envfile.Env, error) {
		if !vaultLoaded {
			vaultEnv, _, vaultErr = readVault(ctx)
			vaultLoaded = true
		}
		return vaultEnv, vaultErr
	}

	wts, err := git.Worktrees(cwd)
	if err != nil {
		return err
	}

	p.printf("vault: %s\n", ctx.VaultPath)
	if !vaultExists {
		p.printf("  (no vault yet — run 'envkeep push' from a worktree)\n")
	}
	for _, wt := range wts {
		if wt.Bare {
			continue
		}
		label := worktreeStatus(ctx, wt, vaultExists, vaultMTime, loadVault)
		p.printf("  %-24s %s\n", worktreeName(wt), label)
	}
	return p.err
}

func worktreeName(wt git.Worktree) string {
	if wt.Branch != "" {
		return wt.Branch
	}
	return filepath.Base(wt.Path)
}

// worktreeStatus computes the one-word status label for a single worktree.
// loadVault parses the shared vault lazily — it is called only on the slow path
// (an mtime miss), so a quiescent repo never pays for the parse.
func worktreeStatus(ctx *Context, wt git.Worktree, vaultExists bool, vaultMTime int64, loadVault func() (envfile.Env, error)) string {
	localPath := filepath.Join(wt.Path, ctx.EnvFile)
	localMTime, localExists := mtimeNanos(localPath)

	if !vaultExists {
		if localExists {
			return "unsynced (no vault)"
		}
		return "absent"
	}
	if !localExists {
		return "absent"
	}

	wtGitDir, err := git.Dir(wt.Path)
	if err != nil {
		return "error: " + err.Error()
	}

	// mtime fast path: read only the marker's cached mtimes (LoadStat, no base)
	// and, if neither file moved since the last sync, it is clean without loading
	// the base or parsing the vault (D5 cache, #11 — mirrors check.go).
	markerLocal, markerVault, hasMarker, err := state.LoadStat(wtGitDir)
	if err != nil {
		return "error: " + err.Error()
	}
	if !hasMarker {
		return "unsynced"
	}
	if localMTime == markerLocal && vaultMTime == markerVault {
		return "clean"
	}

	// mtime miss: load the full marker (its base drives the 3-way compare), parse
	// the vault (once, shared across slow-path worktrees), and read local content.
	marker, ok, err := state.Load(wtGitDir)
	if err != nil {
		return "error: " + err.Error()
	}
	if !ok {
		return "unsynced"
	}
	vaultEnv, err := loadVault()
	if err != nil {
		return "error: " + err.Error()
	}
	localEnv, _, err := readEnv(localPath)
	if err != nil {
		return "error: " + err.Error()
	}
	overrideEnv, err := readEnvOrEmpty(filepath.Join(wt.Path, ctx.EnvFile+overrideSuffix))
	if err != nil {
		return "error: " + err.Error()
	}
	st, _ := envfile.Classify(marker.Base, localEnv.Without(overrideEnv), vaultEnv)
	return st.String()
}
