// Package cli wires the pure layers (envfile, vault, state, config) together
// with git discovery to implement the status/push/pull commands.
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/carvalhosauro/envkeep/internal/config"
	"github.com/carvalhosauro/envkeep/internal/envfile"
	"github.com/carvalhosauro/envkeep/internal/git"
	"github.com/carvalhosauro/envkeep/internal/vault"
)

// overrideSuffix names the per-worktree override file: the tracked filename plus
// this suffix (e.g. .env -> .env.override). It is always distinct from the
// tracked file and must be gitignored by the user (D9).
const overrideSuffix = ".override"

// Context is the resolved repo location for the worktree a command runs in.
// The vault path depends on the active environment, which is resolved per
// command (it needs the worktree's marker), so Context exposes env-aware
// accessors rather than a fixed vault path.
type Context struct {
	CommonDir    string
	GitDir       string // per-worktree gitdir (holds the sync marker)
	Toplevel     string // worktree root
	EnvFile      string // tracked filename
	LocalPath    string // Toplevel/EnvFile
	OverridePath string // Toplevel/EnvFile+overrideSuffix
	EnvFlag      string // raw --env value ("" if unset)
	DefaultEnv   string // repo config default_env ("" if unset / legacy)
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
	return &Context{
		CommonDir:    p.CommonDir,
		GitDir:       p.GitDir,
		Toplevel:     p.Toplevel,
		EnvFile:      envFile,
		LocalPath:    filepath.Join(p.Toplevel, envFile),
		OverridePath: filepath.Join(p.Toplevel, envFile+overrideSuffix),
		EnvFlag:      envFlag,
		DefaultEnv:   cfg.DefaultEnv,
	}, nil
}

// resolveEnv applies the environment precedence (D25): an explicit --env flag
// wins; otherwise the worktree's own active env (its marker); otherwise the repo
// default_env; otherwise "" (the legacy unnamed environment). markerEnv is ""
// when the worktree has no marker or a legacy marker.
func (c *Context) resolveEnv(markerEnv string) string {
	switch {
	case c.EnvFlag != "":
		return c.EnvFlag
	case markerEnv != "":
		return markerEnv
	default:
		return c.DefaultEnv
	}
}

// vaultPath returns the vault file path for env in this repo.
func (c *Context) vaultPath(env string) string {
	return vault.PathForEnv(c.CommonDir, env, c.EnvFile)
}

// vaultStore returns a vault Store bound to env.
func (c *Context) vaultStore(env string) vault.Store {
	return vault.NewFileStore(c.vaultPath(env))
}

// vaultDir returns the directory that holds every environment's vault (the
// parent of the legacy flat vault path).
func (c *Context) vaultDir() string {
	return filepath.Dir(c.vaultPath(""))
}

// readVaultFor reads env's vault, mapping the fresh (not-yet-created) state to
// an empty set with exists=false.
func readVaultFor(ctx *Context, env string) (envfile.Env, bool, error) {
	e, err := ctx.vaultStore(env).Read()
	if errors.Is(err, vault.ErrNotFound) {
		return envfile.Env{}, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return e, true, nil
}

// ensureTargetEnv enforces the git-branch existence rule (D26) and performs
// first-env adoption (D27). The unnamed env ("") is always a no-op. For a named
// env: if it already exists, nil; if it does not exist and create is false, an
// "unknown environment" error; if create is true, the name is validated and
// (unless dryRun) the env is adopted — migrating the legacy flat vault into it
// when it is the first env, and setting default_env when unset.
func ensureTargetEnv(ctx *Context, env string, create, dryRun bool) error {
	if env == "" || vault.EnvExists(ctx.CommonDir, env) {
		return nil
	}
	if !create {
		return fmt.Errorf("unknown environment %q; create it with --create (or --env with an existing environment)", env)
	}
	if err := vault.ValidEnvName(env); err != nil {
		return err
	}
	if dryRun {
		return nil // preview only — never mutate on a dry run
	}
	return adoptEnv(ctx, env)
}

// adoptEnv creates a new environment: if it is the repo's first, the legacy flat
// vault (if any) is migrated into it (D27), and default_env is set when unset so
// later commands resolve the environment. It mutates ctx.DefaultEnv to match.
func adoptEnv(ctx *Context, env string) error {
	existing, err := vault.Environments(ctx.CommonDir)
	if err != nil {
		return err
	}
	if len(existing) == 0 {
		// First environment: migrate the legacy flat vault into it if present, so
		// the shared values carry over and no stray flat vault is left behind.
		legacy := ctx.vaultPath("")
		if _, statErr := os.Stat(legacy); statErr == nil {
			target := ctx.vaultPath(env)
			if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
				return fmt.Errorf("migrate legacy vault: %w", err)
			}
			if err := os.Rename(legacy, target); err != nil {
				return fmt.Errorf("migrate legacy vault: %w", err)
			}
			fmt.Fprintf(os.Stderr, "envkeep: migrated legacy vault to environment %q\n", env)
		}
	}
	cfg, err := config.Load(ctx.CommonDir)
	if err != nil {
		return err
	}
	if cfg.DefaultEnv == "" {
		cfg.DefaultEnv = env
		if err := config.Save(ctx.CommonDir, cfg); err != nil {
			return err
		}
		ctx.DefaultEnv = env
	}
	return nil
}

// readEnv reads an env file as a logical set. ok is false (nil error) if absent.
func readEnv(path string) (env envfile.Env, ok bool, err error) {
	f, ok, err := readFile(path)
	if err != nil || !ok {
		return nil, ok, err
	}
	return f.Map(), true, nil
}

// readEnvOrEmpty reads an env file, returning an empty set if it is absent.
func readEnvOrEmpty(path string) (envfile.Env, error) {
	env, ok, err := readEnv(path)
	if err != nil {
		return nil, err
	}
	if !ok {
		return envfile.Env{}, nil
	}
	return env, nil
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
