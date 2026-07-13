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

// Path returns the vault file path for a repo's common dir and tracked env
// filename. The vault is named after the tracked file so future multi-file
// support (deferred, D12) can add more vaults with no migration.
func Path(commonDir, envFilename string) string {
	return filepath.Join(commonDir, dirName, "vault", envFilename)
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
	// 0600: the vault holds secrets, so it is owner-only.
	return fsutil.WriteFileAtomic(s.path, f.Render(), 0o600)
}
