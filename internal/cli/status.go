package cli

import (
	"io"
	"path/filepath"
	"strings"

	"github.com/carvalhosauro/envkeep/internal/env"
	"github.com/carvalhosauro/envkeep/internal/envfile"
	"github.com/carvalhosauro/envkeep/internal/git"
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
	// and each environment's vault is parsed at most once across the listing.
	parsed := map[env.Name]envfile.Env{}
	loadVault := func(e env.Name) (envfile.Env, error) {
		if v, ok := parsed[e]; ok {
			return v, nil
		}
		v, _, err := ctx.readVault(e)
		if err != nil {
			return nil, err
		}
		parsed[e] = v
		return v, nil
	}

	wts, err := git.Worktrees(cwd)
	if err != nil {
		return err
	}

	p.printf("vault: %s\n", ctx.vaultDir())
	switch {
	case len(envs) > 0:
		line := "  environments: " + joinEnvs(envs)
		if !ctx.DefaultEnv.IsUnnamed() {
			line += " (default: " + ctx.DefaultEnv.String() + ")"
		}
		p.printf("%s\n", line)
	default:
		if _, ok := mtimeNanos(ctx.vaultPath(env.Unnamed)); !ok {
			p.printf("  (no vault yet — run 'envkeep push' from a worktree)\n")
		}
	}
	for _, wt := range wts {
		if wt.Bare {
			continue
		}
		e, label := worktreeStatus(ctx, wt, loadVault)
		p.printf("  %-24s %-8s %s\n", worktreeName(wt), envLabel(e), label)
	}
	return p.err
}

func worktreeName(wt git.Worktree) string {
	if wt.Branch != "" {
		return wt.Branch
	}
	return filepath.Base(wt.Path)
}

// joinEnvs renders environment names as a comma-separated list for the header.
func joinEnvs(envs []env.Name) string {
	names := make([]string, len(envs))
	for i, e := range envs {
		names[i] = e.String()
	}
	return strings.Join(names, ", ")
}

// envLabel renders an environment name for the status column; the unnamed
// (legacy) environment shows as a dash.
func envLabel(e env.Name) string {
	if e.IsUnnamed() {
		return "-"
	}
	return e.String()
}

// worktreeStatus resolves a worktree's active environment and maps its shared
// drift assessment to the one-word status label.
func worktreeStatus(ctx *Context, wt git.Worktree, loadVault func(env.Name) (envfile.Env, error)) (env.Name, string) {
	w, err := ctx.worktreeAt(wt.Path)
	if err != nil {
		return env.Unnamed, "error: " + err.Error()
	}
	d, err := assessWorktree(ctx.Repo, w, loadVault)
	if err != nil {
		return d.env, "error: " + err.Error()
	}
	switch {
	case !d.vaultOK:
		if d.localOK {
			return d.env, "unsynced (no vault)"
		}
		return d.env, "absent"
	case !d.localOK:
		return d.env, "absent"
	case !d.hasMark:
		return d.env, "unsynced"
	case d.clean:
		return d.env, "clean"
	default:
		return d.env, d.status.String()
	}
}
