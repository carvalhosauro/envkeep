package state

import (
	"os"
	"strings"
	"testing"

	"github.com/carvalhosauro/envkeep/internal/envfile"
)

func TestMarkerEnvRoundTrip(t *testing.T) {
	dir := t.TempDir()
	m := Marker{Stat: Stat{Env: "prod", LocalMTime: 5, VaultMTime: 7}, Base: envfile.Env{"A": "1"}}
	if err := Save(dir, m); err != nil {
		t.Fatal(err)
	}
	got, ok, err := Load(dir)
	if err != nil || !ok {
		t.Fatalf("Load = ok:%v err:%v", ok, err)
	}
	if got.Env != "prod" {
		t.Errorf("Marker.Env = %q, want prod", got.Env)
	}
	s, ok, err := LoadStat(dir)
	if err != nil || !ok {
		t.Fatalf("LoadStat = ok:%v err:%v", ok, err)
	}
	if s.Env != "prod" || s.LocalMTime != 5 || s.VaultMTime != 7 {
		t.Errorf("LoadStat = env:%q lm:%d vm:%d, want prod,5,7", s.Env, s.LocalMTime, s.VaultMTime)
	}
}

// A marker written before named environments existed has no "env" field; it
// must decode to the unnamed environment ("") with no migration (D27).
func TestMarkerLegacyNoEnvField(t *testing.T) {
	dir := t.TempDir()
	legacy := `{"base":{"A":"1"},"local_mtime":11,"vault_mtime":22}`
	if err := os.WriteFile(Path(dir), []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}
	m, ok, err := Load(dir)
	if err != nil || !ok {
		t.Fatalf("Load legacy = ok:%v err:%v", ok, err)
	}
	if m.Env != "" {
		t.Errorf("legacy Marker.Env = %q, want empty", m.Env)
	}
	s, ok, err := LoadStat(dir)
	if err != nil || !ok || s.Env != "" || s.LocalMTime != 11 || s.VaultMTime != 22 {
		t.Errorf("LoadStat legacy = env:%q lm:%d vm:%d ok:%v err:%v", s.Env, s.LocalMTime, s.VaultMTime, ok, err)
	}
}

// A saved legacy marker (Env: "") must not emit an "env" key, so a downgrade or
// diff stays clean (omitempty).
func TestMarkerOmitsEmptyEnv(t *testing.T) {
	dir := t.TempDir()
	if err := Save(dir, Marker{Base: envfile.Env{"A": "1"}}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(Path(dir))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"env"`) {
		t.Errorf("empty Env must be omitted from the marker JSON:\n%s", data)
	}
}
