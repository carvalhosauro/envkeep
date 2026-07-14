package cli

import (
	"io"
	"path/filepath"
	"strings"

	"github.com/carvalhosauro/envkeep/internal/envfile"
	"github.com/carvalhosauro/envkeep/internal/git"
	"github.com/carvalhosauro/envkeep/internal/state"
	"github.com/carvalhosauro/envkeep/internal/vault"
)

// Status prints, for every worktree of the repo, its active environment and how
// its local env file relates to that environment's vault: clean / ahead /
// behind / diverged / conflict / absent / unsynced. Each worktree resolves its
// own active environment (--env > marker > default_env, D25); passing --env
// forces the comparison environment for every worktree.
func Status(w io.Writer, cwd, envFileFlag, envFlag string) error {
	ctx, err := Resolve(cwd, envFileFlag, envFlag)
	if err != nil {
		return err
	}
	p := &printer{w: w}

	envs, err := vault.Environments(ctx.CommonDir)
	if err != nil {
		return err
	}

	// Lazy per-env parsed-vault cache: only slow-path worktrees pay the parse,
	// and each environment's vault is parsed at most once across the listing
	// (#11 — mirrors check's deferred parse, now keyed by environment).
	parsed := map[string]envfile.Env{}
	loadVault := func(env string) (envfile.Env, error) {
		if v, ok := parsed[env]; ok {
			return v, nil
		}
		v, _, err := readVaultFor(ctx, env)
		if err != nil {
			return nil, err
		}
		parsed[env] = v
		return v, nil
	}

	wts, err := git.Worktrees(cwd)
	if err != nil {
		return err
	}

	p.printf("vault: %s\n", ctx.vaultDir())
	switch {
	case len(envs) > 0:
		line := "  environments: " + strings.Join(envs, ", ")
		if ctx.DefaultEnv != "" {
			line += " (default: " + ctx.DefaultEnv + ")"
		}
		p.printf("%s\n", line)
	default:
		if _, ok := mtimeNanos(ctx.vaultPath("")); !ok {
			p.printf("  (no vault yet — run 'envkeep push' from a worktree)\n")
		}
	}
	for _, wt := range wts {
		if wt.Bare {
			continue
		}
		env, label := worktreeStatus(ctx, wt, loadVault)
		p.printf("  %-24s %-8s %s\n", worktreeName(wt), envLabel(env), label)
	}
	return p.err
}

func worktreeName(wt git.Worktree) string {
	if wt.Branch != "" {
		return wt.Branch
	}
	return filepath.Base(wt.Path)
}

// envLabel renders an environment name for the status column; the unnamed
// (legacy) environment shows as a dash.
func envLabel(env string) string {
	if env == "" {
		return "-"
	}
	return env
}

// worktreeStatus resolves a worktree's active environment and computes its
// one-word status label against that environment's vault. loadVault parses an
// environment's vault lazily — called only on the slow path (an mtime miss or a
// re-point), so a quiescent repo never pays for the parse.
func worktreeStatus(ctx *Context, wt git.Worktree, loadVault func(string) (envfile.Env, error)) (string, string) {
	localPath := filepath.Join(wt.Path, ctx.EnvFile)
	localMTime, localExists := mtimeNanos(localPath)

	wtGitDir, err := git.Dir(wt.Path)
	if err != nil {
		return "", "error: " + err.Error()
	}
	// Read the worktree's active env (and cached mtimes) to select its vault.
	markerEnv, markerLocal, markerVault, hasMarker, err := state.LoadStat(wtGitDir)
	if err != nil {
		return "", "error: " + err.Error()
	}
	env := ctx.resolveEnv(markerEnv)

	vaultMTime, vaultExists := mtimeNanos(ctx.vaultPath(env))
	if !vaultExists {
		if localExists {
			return env, "unsynced (no vault)"
		}
		return env, "absent"
	}
	if !localExists {
		return env, "absent"
	}
	if !hasMarker {
		return env, "unsynced"
	}

	// mtime fast path: same env AND neither file moved since the last sync →
	// clean without loading the base or parsing the vault (D5 cache, #11). A
	// differing env means a re-point is pending → slow path.
	if markerEnv == env && localMTime == markerLocal && vaultMTime == markerVault {
		return env, "clean"
	}

	// mtime miss (or re-point): load the full marker (base drives the 3-way
	// compare), parse the env's vault (once, cached), and read local content.
	marker, ok, err := state.Load(wtGitDir)
	if err != nil {
		return env, "error: " + err.Error()
	}
	if !ok {
		return env, "unsynced"
	}
	vaultEnv, err := loadVault(env)
	if err != nil {
		return env, "error: " + err.Error()
	}
	localEnv, _, err := readEnv(localPath)
	if err != nil {
		return env, "error: " + err.Error()
	}
	overrideEnv, err := readEnvOrEmpty(filepath.Join(wt.Path, ctx.EnvFile+overrideSuffix))
	if err != nil {
		return env, "error: " + err.Error()
	}
	st, _ := envfile.Classify(marker.Base, localEnv.Without(overrideEnv), vaultEnv)
	return env, st.String()
}
