// Package state stores each worktree's per-worktree sync marker: the vault
// content hash at the last sync (for 3-way conflict detection) plus the file
// mtimes seen then (for the cheap mtime cache). The marker lives in the
// worktree's own gitdir so every worktree tracks its own last sync
// independently (see docs/DECISIONS.md D5).
package state

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/carvalhosauro/envkeep/internal/envfile"
	"github.com/carvalhosauro/envkeep/internal/fsutil"
)

const fileName = "envkeep.base"

// Marker is a worktree's record of its last sync.
type Marker struct {
	VaultHash  string // HashEnv of the vault content at the last sync
	LocalMTime int64  // local .env mtime (unix nanoseconds) at the last check
	VaultMTime int64  // vault file mtime (unix nanoseconds) at the last check
}

// Path returns the marker file path inside a worktree's gitdir.
func Path(gitDir string) string {
	return filepath.Join(gitDir, fileName)
}

// HashEnv returns a stable, order-independent content hash of env. Length-
// prefixing each key and value keeps the encoding unambiguous, so no pair of
// distinct sets can collide by shifting characters across the '=' boundary.
func HashEnv(env envfile.Env) string {
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	for _, k := range keys {
		v := env[k]
		b.WriteString(strconv.Itoa(len(k)))
		b.WriteByte(':')
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(strconv.Itoa(len(v)))
		b.WriteByte(':')
		b.WriteString(v)
		b.WriteByte('\n')
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
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
	m, err := parseMarker(data)
	if err != nil {
		return Marker{}, false, fmt.Errorf("state: %s: %w", Path(gitDir), err)
	}
	return m, true, nil
}

// Save atomically writes the marker for a worktree gitdir.
func Save(gitDir string, m Marker) error {
	content := fmt.Sprintf("vault_hash=%s\nlocal_mtime=%d\nvault_mtime=%d\n",
		m.VaultHash, m.LocalMTime, m.VaultMTime)
	return fsutil.WriteFileAtomic(Path(gitDir), []byte(content), 0o600)
}

func parseMarker(data []byte) (Marker, error) {
	var m Marker
	for line := range strings.SplitSeq(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			return Marker{}, fmt.Errorf("malformed line %q", line)
		}
		var err error
		switch key {
		case "vault_hash":
			m.VaultHash = val
		case "local_mtime":
			m.LocalMTime, err = strconv.ParseInt(val, 10, 64)
		case "vault_mtime":
			m.VaultMTime, err = strconv.ParseInt(val, 10, 64)
		}
		if err != nil {
			return Marker{}, fmt.Errorf("field %q: %w", key, err)
		}
	}
	return m, nil
}
