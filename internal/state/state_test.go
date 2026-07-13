package state

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/carvalhosauro/envkeep/internal/envfile"
)

func TestHashEnvStableAndOrderIndependent(t *testing.T) {
	a := envfile.Env{"A": "1", "B": "2"}
	b := envfile.Env{"B": "2", "A": "1"} // same content, different insertion
	if HashEnv(a) != HashEnv(b) {
		t.Error("HashEnv not order-independent")
	}
	if HashEnv(a) == HashEnv(envfile.Env{"A": "1", "B": "3"}) {
		t.Error("HashEnv collided on differing values")
	}
}

func TestHashEnvNoBoundaryCollision(t *testing.T) {
	// Without length-prefixing, {"A":"B=1"} and {"A":"B", "":"1"}-style shifts
	// could collide. These two must differ.
	x := HashEnv(envfile.Env{"AB": "C"})
	y := HashEnv(envfile.Env{"A": "BC"})
	if x == y {
		t.Error("HashEnv collided across the key/value boundary")
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := Marker{VaultHash: "deadbeef", LocalMTime: 111, VaultMTime: 222}
	if err := Save(dir, want); err != nil {
		t.Fatal(err)
	}
	got, ok, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("Load ok = false after Save")
	}
	if got != want {
		t.Errorf("Load = %+v, want %+v", got, want)
	}
}

func TestLoadMissingIsNotError(t *testing.T) {
	_, ok, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("Load of missing marker: %v", err)
	}
	if ok {
		t.Error("Load ok = true for missing marker")
	}
}

func TestLoadMalformedErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(Path(dir), []byte("local_mtime=not-a-number\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(dir); err == nil {
		t.Error("Load of malformed marker: want error, got nil")
	}
}

func TestPath(t *testing.T) {
	if got, want := Path("/x/.git"), filepath.Join("/x/.git", "envkeep.base"); got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
}
