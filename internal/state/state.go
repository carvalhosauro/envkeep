// Package state stores each worktree's per-worktree sync marker: the base — the
// shared env content at the worktree's last sync — plus the file mtimes seen
// then (for the cheap mtime cache). The marker lives in the worktree's own
// gitdir so every worktree tracks its own last sync independently (see
// docs/DECISIONS.md D5).
//
// The base is stored in full (not just a hash): the 3-way conflict check needs
// the actual values to tell a real per-key conflict from a mergeable
// divergence, and a hash cannot provide that. It is a small JSON file.
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/carvalhosauro/envkeep/internal/envfile"
	"github.com/carvalhosauro/envkeep/internal/fsutil"
)

const fileName = "envkeep.base"

// Marker is a worktree's record of its last sync.
type Marker struct {
	// Base is the shared vault content the worktree last synced to — the common
	// ancestor for the 3-way comparison.
	Base envfile.Env `json:"base"`
	// LocalMTime and VaultMTime are the file mtimes (unix nanoseconds) observed
	// at the last check, used to skip re-parsing when nothing has changed.
	LocalMTime int64 `json:"local_mtime"`
	VaultMTime int64 `json:"vault_mtime"`
}

// Path returns the marker file path inside a worktree's gitdir.
func Path(gitDir string) string {
	return filepath.Join(gitDir, fileName)
}

// Load reads the marker for a worktree gitdir. The bool is false (with a nil
// error) when no marker exists yet — the worktree has never synced.
func Load(gitDir string) (Marker, bool, error) {
	data, err := os.ReadFile(Path(gitDir))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Marker{}, false, nil
		}
		return Marker{}, false, fmt.Errorf("state: read: %w", err)
	}
	var m Marker
	if err := json.Unmarshal(data, &m); err != nil {
		return Marker{}, false, fmt.Errorf("state: %s: %w", Path(gitDir), err)
	}
	return m, true, nil
}

// markerStat is a projection of Marker holding only the cached mtimes. Decoding
// into it makes encoding/json skip the (potentially large) base object without
// allocating the map or its strings.
type markerStat struct {
	LocalMTime int64 `json:"local_mtime"`
	VaultMTime int64 `json:"vault_mtime"`
}

// LoadStat reads only the cached mtimes from a worktree's marker, skipping the
// base snapshot — the cheap read for the mtime fast path (D5/D7), where the base
// is never consulted. ok is false (with a nil error) when no marker exists yet.
// Callers that need the base (an mtime miss → 3-way compare) must fall back to
// Load.
func LoadStat(gitDir string) (localMTime, vaultMTime int64, ok bool, err error) {
	data, err := os.ReadFile(Path(gitDir))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, 0, false, nil
		}
		return 0, 0, false, fmt.Errorf("state: read: %w", err)
	}
	var s markerStat
	if err := json.Unmarshal(data, &s); err != nil {
		return 0, 0, false, fmt.Errorf("state: %s: %w", Path(gitDir), err)
	}
	return s.LocalMTime, s.VaultMTime, true, nil
}

// Save atomically writes the marker for a worktree gitdir.
func Save(gitDir string, m Marker) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("state: marshal: %w", err)
	}
	return fsutil.WriteFileAtomic(Path(gitDir), data, 0o600)
}
