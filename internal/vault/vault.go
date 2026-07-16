// Package vault stores the shared env values for a repo. The v1 backend is a
// local flat file inside the git common dir (D2, D3); it sits behind a small
// Store interface so encrypted-file or remote backends (deferred, D14/D16) can
// be added as new implementations rather than a rewrite (D17).
package vault

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/carvalhosauro/envkeep/internal/env"
	"github.com/carvalhosauro/envkeep/internal/envfile"
	"github.com/carvalhosauro/envkeep/internal/fsutil"
)

// ErrNotFound is returned by Read when the vault does not exist yet — the repo
// has never been pushed to (the "fresh" state).
var ErrNotFound = errors.New("vault: not found")

// Store reads and writes the shared env set. Kept intentionally tiny (D17).
type Store interface {
	Read() (envfile.Env, error)
	Write(env envfile.Env) error
}

// dirName is the subdirectory under the git common dir that holds envkeep's
// shared state (D3). Being inside .git, it is never tracked by git.
const dirName = "envkeep"

// vaultSubdir is the directory under dirName that holds the per-environment
// vaults.
const vaultSubdir = "vault"

// Path returns the vault file path for the unnamed (legacy) environment. It is
// the pre-environments layout, kept so repos that never adopt environments are
// byte-identical (R3, D27). Equivalent to PathForEnv(commonDir, env.Unnamed, …).
func Path(commonDir, envFilename string) string {
	return PathForEnv(commonDir, env.Unnamed, envFilename)
}

// PathForEnv returns the vault file path for a given environment (D23). The
// unnamed environment keeps the legacy flat layout <vault>/<envFilename>; a
// named environment nests under its own directory <vault>/<name>/<envFilename>,
// which keeps the environment axis orthogonal to the deferred multi-file axis
// (D12) — a future filename #2 becomes a second file inside the same dir with
// no migration.
func PathForEnv(commonDir string, e env.Name, envFilename string) string {
	if e.IsUnnamed() {
		return filepath.Join(vaultDir(commonDir), envFilename)
	}
	return filepath.Join(vaultDir(commonDir), e.String(), envFilename)
}

// vaultDir is the directory holding every environment's vault.
func vaultDir(commonDir string) string {
	return filepath.Join(commonDir, dirName, vaultSubdir)
}

// Environments lists the named environments that exist for a repo — the
// subdirectories of the vault dir. The filesystem is the environment registry
// (D26), exactly as .git/refs/heads/ is for branches, so there is no config
// list to drift. The legacy flat vault file is not a directory and is skipped.
// Returns an empty slice (nil error) when no vault dir exists yet.
func Environments(commonDir string) ([]env.Name, error) {
	entries, err := os.ReadDir(vaultDir(commonDir))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("vault: list environments: %w", err)
	}
	var envs []env.Name
	for _, ent := range entries {
		if ent.IsDir() {
			envs = append(envs, env.Name(ent.Name()))
		}
	}
	sort.Slice(envs, func(i, j int) bool { return envs[i] < envs[j] })
	return envs, nil
}

// EnvExists reports whether the named environment has a vault directory — the
// existence check that gates targeting/switching an environment (D26,
// git-branch model: you can only switch to an env that already exists). The
// unnamed environment always "exists" (it is the legacy default).
func EnvExists(commonDir string, e env.Name) bool {
	if e.IsUnnamed() {
		return true
	}
	fi, err := os.Stat(filepath.Join(vaultDir(commonDir), e.String()))
	return err == nil && fi.IsDir()
}

// RemoveEnv deletes a named environment's vault directory. The unnamed (legacy)
// environment cannot be removed.
func RemoveEnv(commonDir string, e env.Name) error {
	if e.IsUnnamed() {
		return errors.New("vault: cannot remove the unnamed environment")
	}
	dir := filepath.Join(vaultDir(commonDir), e.String())
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("vault: remove environment %q: %w", e.String(), err)
	}
	return nil
}

// MigrateLegacy moves the legacy flat vault into the environment to, if a flat
// vault exists (the opt-in migration on first env creation, D27). It reports
// whether a file was moved. vault owns the on-disk layout, so the rename lives
// here rather than in the caller.
func MigrateLegacy(commonDir, envFilename string, to env.Name) (bool, error) {
	legacy := PathForEnv(commonDir, env.Unnamed, envFilename)
	if _, err := os.Stat(legacy); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, fmt.Errorf("vault: migrate legacy: %w", err)
	}
	target := PathForEnv(commonDir, to, envFilename)
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return false, fmt.Errorf("vault: migrate legacy: %w", err)
	}
	if err := os.Rename(legacy, target); err != nil {
		return false, fmt.Errorf("vault: migrate legacy: %w", err)
	}
	return true, nil
}

// FileStore is the flat-file vault backend.
type FileStore struct {
	path string
}

var _ Store = (*FileStore)(nil)

// NewFileStore returns a vault backed by the file at path.
func NewFileStore(path string) *FileStore {
	return &FileStore{path: path}
}

// Path returns the backing file path.
func (s *FileStore) Path() string {
	return s.path
}

// Read loads the vault. It returns ErrNotFound if the file does not exist.
func (s *FileStore) Read() (envfile.Env, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("vault: read %s: %w", s.path, err)
	}
	f, err := envfile.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("vault: parse %s: %w", s.path, err)
	}
	return f.Map(), nil
}

// Write atomically replaces the vault with env. Keys are written sorted so the
// file has a stable, diff-friendly form; values are quoted as needed via the
// shared envfile renderer.
func (s *FileStore) Write(env envfile.Env) error {
	f := envfile.New()
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		f.Set(k, env[k])
	}
	// The vault holds secrets, so it is owner-only (fsutil.SecretFilePerm).
	return fsutil.WriteFileAtomic(s.path, f.Render(), fsutil.SecretFilePerm)
}
