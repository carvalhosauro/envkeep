package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDefaultEnvAndCascadeRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, Config{EnvFile: ".env.local", DefaultEnv: "prod", Cascade: true}); err != nil {
		t.Fatal(err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.EnvFile != ".env.local" || got.DefaultEnv != "prod" || !got.Cascade {
		t.Errorf("round trip = %+v", got)
	}
}

func TestCascadeParseTolerant(t *testing.T) {
	dir := t.TempDir()
	// An unparseable cascade value must not fail Load; it defaults to false.
	if err := os.MkdirAll(filepath.Dir(Path(dir)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(dir), []byte("env_file=.env\ncascade=nonsense\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Load(dir)
	if err != nil {
		t.Fatalf("Load must tolerate a bad cascade value: %v", err)
	}
	if got.Cascade {
		t.Error("cascade should default to false on an unparseable value")
	}
}

func TestSaveOmitsEmptyEnvKeys(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, Config{EnvFile: ".env"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(Path(dir))
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if strings.Contains(s, "default_env") || strings.Contains(s, "cascade") {
		t.Errorf("a repo that never adopts environments must keep a bare config:\n%s", s)
	}
}
