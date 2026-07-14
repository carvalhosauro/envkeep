// Package config reads the per-repo envkeep configuration, stored at
// <common-dir>/envkeep/config as flat KEY=VALUE and shared by every worktree of
// the repo. v1 has a single key, env_file, which names the tracked env file
// (D12). A missing config yields defaults.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/carvalhosauro/envkeep/internal/envfile"
	"github.com/carvalhosauro/envkeep/internal/fsutil"
)

const (
	dirName  = "envkeep"
	fileName = "config"

	// keyEnvFile is the sole config key (v1): it names the tracked env file.
	keyEnvFile = "env_file"

	// DefaultEnvFile is the tracked filename when config sets none.
	DefaultEnvFile = ".env"
)

// Config is the per-repo configuration.
type Config struct {
	EnvFile string
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
	return cfg, nil
}

// Save writes the repo config.
func Save(commonDir string, cfg Config) error {
	f := envfile.New()
	f.Set(keyEnvFile, cfg.EnvFile)
	return fsutil.WriteFileAtomic(Path(commonDir), f.Render(), 0o644)
}
