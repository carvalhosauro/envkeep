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

	"github.com/carvalhosauro/envkeep/internal/env"
	"github.com/carvalhosauro/envkeep/internal/envfile"
	"github.com/carvalhosauro/envkeep/internal/fsutil"
)

const fileName = "envkeep.base"

// Stat is the cheap projection of a Marker: the active environment and the
// cached file mtimes, without the (potentially large) base snapshot. Decoding
// into it makes encoding/json skip the base without allocating the map. Marker
// embeds it, so these fields and their JSON tags are defined once and a new stat
// field lands in both Stat and Marker (and LoadStat) at once.
type Stat struct {
	// Env is the environment the worktree's local file currently holds — its
	// active environment, the analog of a git worktree's HEAD (D25). Unnamed
	// means the legacy environment; a marker written before named environments
	// existed omits the field and decodes to Unnamed (D27, no migration).
	Env env.Name `json:"env,omitempty"`
	// LocalMTime and VaultMTime are the file mtimes (unix nanoseconds) observed
	// at the last check, used to skip re-parsing when nothing has changed.
	// VaultMTime is the mtime of Env's vault.
	LocalMTime int64 `json:"local_mtime"`
	VaultMTime int64 `json:"vault_mtime"`
}

// Marker is a worktree's record of its last sync: its Stat (active env + cached
// mtimes) plus the full base snapshot.
type Marker struct {
	Stat
	// Base is the shared vault content the worktree last synced to — the common
	// ancestor for the 3-way comparison. It is the content of Env's vault at the
	// last sync.
	Base envfile.Env `json:"base"`
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

// LoadStat reads only a marker's Stat (active env + cached mtimes), skipping the
// base snapshot — the cheap read for the mtime fast path (D5/D7), where the base
// is never consulted. Returning the Stat rather than its loose fields keeps the
// signature stable when a field is added and mirrors Load's (Marker, bool,
// error). ok is false (nil error) when no marker exists yet. Callers that need
// the base (an mtime miss → 3-way compare) must fall back to Load.
func LoadStat(gitDir string) (Stat, bool, error) {
	data, err := os.ReadFile(Path(gitDir))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Stat{}, false, nil
		}
		return Stat{}, false, fmt.Errorf("state: read: %w", err)
	}
	var s Stat
	if err := json.Unmarshal(data, &s); err != nil {
		return Stat{}, false, fmt.Errorf("state: %s: %w", Path(gitDir), err)
	}
	return s, true, nil
}

// Save atomically writes the marker for a worktree gitdir.
func Save(gitDir string, m Marker) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("state: marshal: %w", err)
	}
	return fsutil.WriteFileAtomic(Path(gitDir), data, fsutil.SecretFilePerm)
}
