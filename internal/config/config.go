// Package config reads the per-repo envkeep configuration, stored at
// <common-dir>/envkeep/config as flat KEY=VALUE and shared by every worktree of
// the repo. Keys: env_file names the tracked env file (D12); default_env names
// the environment a worktree falls back to when it has no active env yet (D25);
// cascade governs whether switching the active environment fans out to every
// worktree (D28). A missing config yields defaults.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/carvalhosauro/envkeep/internal/envfile"
	"github.com/carvalhosauro/envkeep/internal/fsutil"
)

const (
	dirName  = "envkeep"
	fileName = "config"

	// Config keys.
	keyEnvFile    = "env_file"    // names the tracked env file (D12)
	keyDefaultEnv = "default_env" // fallback environment for an unset worktree (D25)
	keyCascade    = "cascade"     // whether `use` fans out to every worktree (D28)

	// DefaultEnvFile is the tracked filename when config sets none.
	DefaultEnvFile = ".env"
)

// Config is the per-repo configuration.
type Config struct {
	EnvFile string
	// DefaultEnv is the environment used when a worktree has no active env of
	// its own yet. Empty means the repo has not adopted environments (the legacy
	// unnamed vault, D27).
	DefaultEnv string
	// Cascade, when true, makes an environment switch (`use`) fan out to every
	// worktree instead of only the current one (D28). Default false.
	Cascade bool
}

// Path returns the config file path for a repo's common dir.
func Path(commonDir string) string {
	return filepath.Join(commonDir, dirName, fileName)
}

// Load reads the repo config, returning defaults if the file does not exist.
func Load(commonDir string) (Config, error) {
	cfg := Config{EnvFile: DefaultEnvFile}
	data, err := os.ReadFile(Path(commonDir))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return cfg, fmt.Errorf("config: read: %w", err)
	}
	f, err := envfile.Parse(data)
	if err != nil {
		return cfg, fmt.Errorf("config: parse: %w", err)
	}
	if v, ok := f.Get(keyEnvFile); ok && v != "" {
		cfg.EnvFile = v
	}
	if v, ok := f.Get(keyDefaultEnv); ok {
		cfg.DefaultEnv = v
	}
	if v, ok := f.Get(keyCascade); ok {
		// Tolerant parse: an unparseable value is treated as the false default
		// rather than failing config load.
		cfg.Cascade, _ = strconv.ParseBool(v)
	}
	return cfg, nil
}

// Save writes the repo config. Only keys with a non-default value are written,
// so a repo that never adopts environments keeps a config identical to the
// pre-environments form (just env_file) — the R3 back-compat guarantee (D27).
func Save(commonDir string, cfg Config) error {
	f := envfile.New()
	f.Set(keyEnvFile, cfg.EnvFile)
	if cfg.DefaultEnv != "" {
		f.Set(keyDefaultEnv, cfg.DefaultEnv)
	}
	if cfg.Cascade {
		f.Set(keyCascade, "true")
	}
	return fsutil.WriteFileAtomic(Path(commonDir), f.Render(), 0o644)
}
