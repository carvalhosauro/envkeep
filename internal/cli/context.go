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

// Context is the resolved environment for the worktree a command runs in.
type Context struct {
	CommonDir    string
	GitDir       string // per-worktree gitdir (holds the sync marker)
	Toplevel     string // worktree root
	EnvFile      string // tracked filename
	LocalPath    string // Toplevel/EnvFile
	OverridePath string // Toplevel/EnvFile+overrideSuffix
	VaultPath    string
	Vault        vault.Store
}

// Resolve discovers the repo from cwd and builds the command context. envFileFlag
// (may be "") takes precedence over the repo config, which defaults to .env.
func Resolve(cwd, envFileFlag string) (*Context, error) {
	commonDir, err := git.CommonDir(cwd)
	if err != nil {
		return nil, err
	}
	gitDir, err := git.Dir(cwd)
	if err != nil {
		return nil, err
	}
	top, err := git.Toplevel(cwd)
	if err != nil {
		return nil, err
	}
	cfg, err := config.Load(commonDir)
	if err != nil {
		return nil, err
	}
	envFile := cfg.EnvFile
	if envFileFlag != "" {
		envFile = envFileFlag
	}
	vp := vault.Path(commonDir, envFile)
	return &Context{
		CommonDir:    commonDir,
		GitDir:       gitDir,
		Toplevel:     top,
		EnvFile:      envFile,
		LocalPath:    filepath.Join(top, envFile),
		OverridePath: filepath.Join(top, envFile+overrideSuffix),
		VaultPath:    vp,
		Vault:        vault.NewFileStore(vp),
	}, nil
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
