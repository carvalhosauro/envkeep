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

	"github.com/carvalhosauro/envkeep/internal/env"
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
	// its own yet. Unnamed means the repo has not adopted environments (the
	// legacy unnamed vault, D27).
	DefaultEnv env.Name
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
		cfg.DefaultEnv = env.Name(v)
	}
	if v, ok := f.Get(keyCascade); ok {
		// Tolerant parse: an unparseable value is treated as the false default
		// rather than failing config load.
		cfg.Cascade, _ = strconv.ParseBool(v)
	}
	return cfg, nil
}

// Keys returns the configurable keys, in display order.
func Keys() []string { return []string{keyEnvFile, keyDefaultEnv, keyCascade} }

// Get returns the string value of a config key. ok is false if the key is unset.
func Get(commonDir, key string) (string, bool, error) {
	cfg, err := Load(commonDir)
	if err != nil {
		return "", false, err
	}
	switch key {
	case keyEnvFile:
		return cfg.EnvFile, cfg.EnvFile != "", nil
	case keyDefaultEnv:
		return cfg.DefaultEnv.String(), !cfg.DefaultEnv.IsUnnamed(), nil
	case keyCascade:
		if !cfg.Cascade {
			return "false", false, nil
		}
		return "true", true, nil
	default:
		return "", false, fmt.Errorf("config: unknown key %q", key)
	}
}

// Set assigns a config key and persists it.
func Set(commonDir, key, val string) error {
	cfg, err := Load(commonDir)
	if err != nil {
		return err
	}
	switch key {
	case keyEnvFile:
		cfg.EnvFile = val
	case keyDefaultEnv:
		cfg.DefaultEnv = env.Name(val)
	case keyCascade:
		b, perr := strconv.ParseBool(val)
		if perr != nil {
			return fmt.Errorf("config: %q must be a boolean: %w", keyCascade, perr)
		}
		cfg.Cascade = b
	default:
		return fmt.Errorf("config: unknown key %q", key)
	}
	return Save(commonDir, cfg)
}

// Unset clears a config key back to its default.
func Unset(commonDir, key string) error {
	switch key {
	case keyEnvFile, keyDefaultEnv, keyCascade:
		return Set(commonDir, key, defaultFor(key))
	default:
		return fmt.Errorf("config: unknown key %q", key)
	}
}

func defaultFor(key string) string {
	switch key {
	case keyEnvFile:
		return DefaultEnvFile
	case keyCascade:
		// Set's cascade branch requires a parseable bool, so the default must
		// be "false" rather than "" (which strconv.ParseBool rejects).
		return "false"
	default:
		return "" // default_env -> unnamed
	}
}

// Save writes the repo config. Only keys with a non-default value are written,
// so a repo that never adopts environments keeps a config identical to the
// pre-environments form (just env_file) — the R3 back-compat guarantee (D27).
func Save(commonDir string, cfg Config) error {
	f := envfile.New()
	f.Set(keyEnvFile, cfg.EnvFile)
	if !cfg.DefaultEnv.IsUnnamed() {
		f.Set(keyDefaultEnv, cfg.DefaultEnv.String())
	}
	if cfg.Cascade {
		f.Set(keyCascade, "true")
	}
	return fsutil.WriteFileAtomic(Path(commonDir), f.Render(), 0o644)
}
