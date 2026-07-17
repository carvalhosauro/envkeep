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

func TestGetSetByKey(t *testing.T) {
	dir := t.TempDir()
	if err := Set(dir, "default_env", "prod"); err != nil {
		t.Fatal(err)
	}
	v, ok, err := Get(dir, "default_env")
	if err != nil || !ok || v != "prod" {
		t.Errorf("Get(default_env) = %q,%v,%v; want prod,true,nil", v, ok, err)
	}
	if err := Set(dir, "bogus", "x"); err == nil {
		t.Error("Set(bogus): want error for unknown key")
	}
}

// TestKeysOrder verifies Keys returns the three configurable keys in the
// documented display order.
func TestKeysOrder(t *testing.T) {
	want := []string{"env_file", "default_env", "cascade"}
	got := Keys()
	if len(got) != len(want) {
		t.Fatalf("Keys() = %v, want %v", got, want)
	}
	for i, k := range want {
		if got[i] != k {
			t.Errorf("Keys()[%d] = %q, want %q", i, got[i], k)
		}
	}
}

// TestGetSetUnsetEnvFile round-trips the env_file key through Set, Get, and
// Unset (back to DefaultEnvFile).
func TestGetSetUnsetEnvFile(t *testing.T) {
	dir := t.TempDir()
	if err := Set(dir, "env_file", ".env.local"); err != nil {
		t.Fatal(err)
	}
	v, ok, err := Get(dir, "env_file")
	if err != nil || !ok || v != ".env.local" {
		t.Errorf("Get(env_file) = %q,%v,%v; want .env.local,true,nil", v, ok, err)
	}
	if err := Unset(dir, "env_file"); err != nil {
		t.Fatal(err)
	}
	v, ok, err = Get(dir, "env_file")
	if err != nil || !ok || v != DefaultEnvFile {
		t.Errorf("Get(env_file) after unset = %q,%v,%v; want %q,true,nil", v, ok, err, DefaultEnvFile)
	}
}

// TestGetSetUnsetCascade exercises both branches of the cascade key (false
// default, true after Set), Set's boolean-parse error, and Unset back to
// false.
func TestGetSetUnsetCascade(t *testing.T) {
	dir := t.TempDir()
	v, ok, err := Get(dir, "cascade")
	if err != nil || ok || v != "false" {
		t.Errorf("Get(cascade) default = %q,%v,%v; want false,false,nil", v, ok, err)
	}
	if err := Set(dir, "cascade", "true"); err != nil {
		t.Fatal(err)
	}
	v, ok, err = Get(dir, "cascade")
	if err != nil || !ok || v != "true" {
		t.Errorf("Get(cascade) after set = %q,%v,%v; want true,true,nil", v, ok, err)
	}
	if err := Set(dir, "cascade", "not-a-bool"); err == nil {
		t.Error("Set(cascade, not-a-bool): want error")
	}
	if err := Unset(dir, "cascade"); err != nil {
		t.Fatal(err)
	}
	v, ok, err = Get(dir, "cascade")
	if err != nil || ok || v != "false" {
		t.Errorf("Get(cascade) after unset = %q,%v,%v; want false,false,nil", v, ok, err)
	}
}

// TestUnsetUnknownKeyErrors and TestGetUnknownKeyErrors cover the unknown-key
// error branch on Unset and Get (Set's is already covered by TestGetSetByKey).
func TestUnsetUnknownKeyErrors(t *testing.T) {
	dir := t.TempDir()
	if err := Unset(dir, "bogus"); err == nil {
		t.Error("Unset(bogus): want error for unknown key")
	}
}

func TestGetUnknownKeyErrors(t *testing.T) {
	dir := t.TempDir()
	if _, _, err := Get(dir, "bogus"); err == nil {
		t.Error("Get(bogus): want error for unknown key")
	}
}

// TestGetSetLoadErrorPropagates forces Load to hit a genuine parse error (not
// the tolerated missing-file default) and verifies both Get and Set propagate
// it rather than swallowing it.
func TestGetSetLoadErrorPropagates(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Dir(Path(dir)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(dir), []byte("not a valid line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Get(dir, "env_file"); err == nil {
		t.Error("Get: want error propagated from Load's parse failure")
	}
	if err := Set(dir, "env_file", ".env"); err == nil {
		t.Error("Set: want error propagated from Load's parse failure")
	}
}

// TestLoadReadErrorNotExist verifies Load returns a real error (distinct from
// the tolerated missing-file default) when a path component of the config
// file cannot be read at all — here, a plain file sits where a directory is
// expected.
func TestLoadReadErrorNotExist(t *testing.T) {
	dir := t.TempDir()
	badParent := filepath.Join(dir, "envkeep")
	if err := os.WriteFile(badParent, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Error("Load: want error when a path component is not a directory")
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
