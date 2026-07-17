// Package cli wires the pure layers (env, envfile, vault, state, config)
// together with git discovery to implement the status/push/pull/check commands.
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/carvalhosauro/envkeep/internal/config"
	"github.com/carvalhosauro/envkeep/internal/env"
	"github.com/carvalhosauro/envkeep/internal/envfile"
	"github.com/carvalhosauro/envkeep/internal/git"
	"github.com/carvalhosauro/envkeep/internal/vault"
)

// overrideSuffix names the per-worktree override file: the tracked filename plus
// this suffix (e.g. .env -> .env.override). It is always distinct from the
// tracked file and must be gitignored by the user (D9).
const overrideSuffix = ".override"

// Repo is the repo-level context shared by every worktree of a repository:
// where the vaults live (CommonDir), the tracked filename, and the environment
// resolution inputs. Most command logic — vault access, env resolution, env
// creation — needs only this and a target environment, not any particular
// worktree, so it hangs off Repo rather than Context.
type Repo struct {
	CommonDir  string
	EnvFile    string
	EnvFlag    env.Name // raw --env value (Unnamed if unset)
	DefaultEnv env.Name // config default_env (Unnamed if unset / legacy)
	Cascade    bool     // config cascade default for `use` fan-out (D28)
}

// worktreePaths bundles the paths a single worktree's drift assessment needs:
// its gitdir (where the marker lives), its local env file, and its override
// file. It is the per-worktree axis, separate from Repo so assessWorktree can
// run over every worktree, not just the invoking one.
type worktreePaths struct {
	gitDir       string
	localPath    string
	overridePath string
}

// Context is a Repo plus the worktree the command was invoked in (self). Repo's
// methods and fields are promoted, so ctx.vaultPath / ctx.resolveEnv /
// ctx.CommonDir read naturally; self carries the invoking worktree's paths.
type Context struct {
	*Repo
	self worktreePaths
}

// Resolve discovers the repo from cwd and builds the command context. envFileFlag
// (may be "") takes precedence over the repo config, which defaults to .env.
// envFlag (may be "") is the raw --env value, resolved against the marker and
// default_env per command via resolveEnv.
func Resolve(cwd, envFileFlag, envFlag string) (*Context, error) {
	// One git call for all three paths — keeps the per-prompt hook check cheap.
	p, err := git.Locate(cwd)
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(p.CommonDir)
	if err != nil {
		return nil, err
	}
	envFile := cfg.EnvFile
	if envFileFlag != "" {
		envFile = envFileFlag
	}
	repo := &Repo{
		CommonDir:  p.CommonDir,
		EnvFile:    envFile,
		EnvFlag:    env.Name(envFlag),
		DefaultEnv: cfg.DefaultEnv,
		Cascade:    cfg.Cascade,
	}
	// The invoking worktree's gitdir came from Locate, so build self directly
	// (no extra git call — check stays cheap, D7).
	return &Context{
		Repo: repo,
		self: repo.worktreePathsAt(p.Toplevel, p.GitDir),
	}, nil
}

// worktree returns the paths of the worktree the command was invoked in.
func (c *Context) worktree() worktreePaths { return c.self }

// resolveEnv applies the environment precedence (D25): an explicit --env flag
// wins; otherwise the worktree's own active env (its marker); otherwise the repo
// default_env; otherwise Unnamed (the legacy environment). markerEnv is Unnamed
// when the worktree has no marker or a legacy marker.
func (r *Repo) resolveEnv(markerEnv env.Name) env.Name {
	switch {
	case !r.EnvFlag.IsUnnamed():
		return r.EnvFlag
	case !markerEnv.IsUnnamed():
		return markerEnv
	default:
		return r.DefaultEnv
	}
}

// vaultPath returns the vault file path for e in this repo.
func (r *Repo) vaultPath(e env.Name) string {
	return vault.PathForEnv(r.CommonDir, e, r.EnvFile)
}

// vaultStore returns a vault Store bound to e.
func (r *Repo) vaultStore(e env.Name) vault.Store {
	return vault.NewFileStore(r.vaultPath(e))
}

// vaultDir returns the directory that holds every environment's vault (the
// parent of the legacy flat vault path).
func (r *Repo) vaultDir() string {
	return filepath.Dir(r.vaultPath(env.Unnamed))
}

// readVault reads e's vault, mapping the fresh (not-yet-created) state to an
// empty set with exists=false.
func (r *Repo) readVault(e env.Name) (envfile.Env, bool, error) {
	got, err := r.vaultStore(e).Read()
	if errors.Is(err, vault.ErrNotFound) {
		return envfile.Env{}, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return got, true, nil
}

// worktreePathsAt builds a worktree's paths from its root and gitdir, with no
// git call (both are already known).
func (r *Repo) worktreePathsAt(toplevel, gitDir string) worktreePaths {
	return worktreePaths{
		gitDir:       gitDir,
		localPath:    filepath.Join(toplevel, r.EnvFile),
		overridePath: filepath.Join(toplevel, r.EnvFile+overrideSuffix),
	}
}

// worktreeAt resolves another worktree's paths, shelling out once for its
// gitdir — used by status to inspect every worktree. The invoking worktree is
// built directly in Resolve, so check never pays this git call (D7).
func (r *Repo) worktreeAt(toplevel string) (worktreePaths, error) {
	gitDir, err := git.Dir(toplevel)
	if err != nil {
		return worktreePaths{}, err
	}
	return r.worktreePathsAt(toplevel, gitDir), nil
}

// ensureTargetEnv enforces the git-branch existence rule (D26) and performs
// first-env adoption (D27). The unnamed env is always a no-op. For a named env:
// if it already exists, nil; if it does not exist and create is false, an
// "unknown environment" error; if create is true, the name is validated and
// (unless dryRun) the env is adopted.
func (r *Repo) ensureTargetEnv(e env.Name, create, dryRun bool) error {
	if e.IsUnnamed() || vault.EnvExists(r.CommonDir, e) {
		return nil
	}
	if !create {
		return fmt.Errorf("unknown environment %q; create it with --create (or --env with an existing environment)", e.String())
	}
	if err := e.Validate(); err != nil {
		return err
	}
	if dryRun {
		return nil // preview only — never mutate on a dry run
	}
	return r.adoptEnv(e)
}

// adoptEnv creates a new environment: if it is the repo's first, the legacy flat
// vault (if any) is migrated into it (D27, via vault.MigrateLegacy — vault owns
// the layout), and default_env is set when unset so later commands resolve the
// environment. It updates r.DefaultEnv to match.
func (r *Repo) adoptEnv(e env.Name) error {
	existing, err := vault.Environments(r.CommonDir)
	if err != nil {
		return err
	}
	if len(existing) == 0 {
		migrated, err := vault.MigrateLegacy(r.CommonDir, r.EnvFile, e)
		if err != nil {
			return err
		}
		if migrated {
			fmt.Fprintf(os.Stderr, "envkeep: migrated legacy vault to environment %q\n", e.String())
		}
	}
	cfg, err := config.Load(r.CommonDir)
	if err != nil {
		return err
	}
	if cfg.DefaultEnv.IsUnnamed() {
		cfg.DefaultEnv = e
		if err := config.Save(r.CommonDir, cfg); err != nil {
			return err
		}
		r.DefaultEnv = e
	}
	return nil
}

// readShared reads the worktree's local env and its override, returning the
// shared set (local minus override keys, D9) and whether the local file exists.
// It is the read half push/check/status share (pull needs the ordered file, so
// it reads its own).
func readShared(localPath, overridePath string) (shared envfile.Env, localExists bool, err error) {
	override, err := readEnvOrEmpty(overridePath)
	if err != nil {
		return nil, false, err
	}
	local, exists, err := readEnv(localPath)
	if err != nil {
		return nil, false, err
	}
	if !exists {
		local = envfile.Env{}
	}
	return local.Without(override), exists, nil
}

// readEnv reads an env file as a logical set. ok is false (nil error) if absent.
func readEnv(path string) (e envfile.Env, ok bool, err error) {
	f, ok, err := readFile(path)
	if err != nil || !ok {
		return nil, ok, err
	}
	return f.Map(), true, nil
}

// readEnvOrEmpty reads an env file, returning an empty set if it is absent.
func readEnvOrEmpty(path string) (envfile.Env, error) {
	e, ok, err := readEnv(path)
	if err != nil {
		return nil, err
	}
	if !ok {
		return envfile.Env{}, nil
	}
	return e, nil
}

// readFile reads and parses an env file, preserving layout. ok is false (nil
// error) if the file does not exist.
func readFile(path string) (*envfile.File, bool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	f, err := envfile.Parse(data)
	if err != nil {
		return nil, false, fmt.Errorf("%s: %w", path, err)
	}
	return f, true, nil
}

// mtimeNanos returns a file's mtime in unix nanoseconds; ok is false if the file
// cannot be stat'd.
func mtimeNanos(path string) (int64, bool) {
	fi, err := os.Stat(path)
	if err != nil {
		return 0, false
	}
	return fi.ModTime().UnixNano(), true
}

// printer writes formatted output while carrying the first write error, so
// callers check one error at the end instead of every Fprintf.
type printer struct {
	w   io.Writer
	err error
}

func (p *printer) printf(format string, a ...any) {
	if p.err != nil {
		return
	}
	_, p.err = fmt.Fprintf(p.w, format, a...)
}
