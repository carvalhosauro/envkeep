package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingReturnsDefault(t *testing.T) {
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if cfg.EnvFile != DefaultEnvFile {
		t.Errorf("EnvFile = %q, want default %q", cfg.EnvFile, DefaultEnvFile)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, Config{EnvFile: ".env.local"}); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.EnvFile != ".env.local" {
		t.Errorf("EnvFile = %q, want .env.local", cfg.EnvFile)
	}
}

func TestLoadEmptyValueFallsBackToDefault(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(Path(dir)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(dir), []byte("env_file=\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.EnvFile != DefaultEnvFile {
		t.Errorf("EnvFile = %q, want default for empty value", cfg.EnvFile)
	}
}

func TestPath(t *testing.T) {
	got := Path("/repo/.git")
	want := filepath.Join("/repo/.git", "envkeep", "config")
	if got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
}
