package cli

import (
	"errors"
	"io"
	"path/filepath"

	"github.com/carvalhosauro/envkeep/internal/envfile"
	"github.com/carvalhosauro/envkeep/internal/git"
	"github.com/carvalhosauro/envkeep/internal/state"
	"github.com/carvalhosauro/envkeep/internal/vault"
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

	vaultEnv, vaultErr := ctx.Vault.Read()
	vaultExists := true
	switch {
	case errors.Is(vaultErr, vault.ErrNotFound):
		vaultExists = false
		vaultEnv = envfile.Env{}
	case vaultErr != nil:
		return vaultErr
	}
	vaultMTime, _ := mtimeNanos(ctx.VaultPath)

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
		label := worktreeStatus(ctx, wt, vaultEnv, vaultExists, vaultMTime)
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
func worktreeStatus(ctx *Context, wt git.Worktree, vaultEnv envfile.Env, vaultExists bool, vaultMTime int64) string {
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
	marker, hasMarker, err := state.Load(wtGitDir)
	if err != nil {
		return "error: " + err.Error()
	}
	if !hasMarker {
		return "unsynced"
	}

	// mtime fast path: if neither file moved since the last sync, it is clean
	// without parsing anything (D5 cache).
	if localMTime == marker.LocalMTime && vaultMTime == marker.VaultMTime {
		return "clean"
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
