package state

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/carvalhosauro/envkeep/internal/envfile"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	want := Marker{
		Base:       envfile.Env{"A": "1", "B": "hello world"},
		LocalMTime: 111,
		VaultMTime: 222,
	}
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
	if !reflect.DeepEqual(got, want) {
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
	if err := os.WriteFile(Path(dir), []byte("{not valid json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(dir); err == nil {
		t.Error("Load of malformed marker: want error, got nil")
	}
}

func TestLoadStatReadsMtimesSkippingBase(t *testing.T) {
	dir := t.TempDir()
	m := Marker{
		Base:       envfile.Env{"A": "1", "B": "hello world"},
		LocalMTime: 111,
		VaultMTime: 222,
	}
	if err := Save(dir, m); err != nil {
		t.Fatal(err)
	}
	lm, vm, ok, err := LoadStat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("LoadStat ok = false after Save")
	}
	if lm != 111 || vm != 222 {
		t.Errorf("LoadStat mtimes = (%d, %d), want (111, 222)", lm, vm)
	}
}

func TestLoadStatMissingIsNotError(t *testing.T) {
	_, _, ok, err := LoadStat(t.TempDir())
	if err != nil {
		t.Fatalf("LoadStat of missing marker: %v", err)
	}
	if ok {
		t.Error("LoadStat ok = true for missing marker")
	}
}

func TestLoadStatMalformedErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(Path(dir), []byte("{not valid json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := LoadStat(dir); err == nil {
		t.Error("LoadStat of malformed marker: want error, got nil")
	}
}

func TestPath(t *testing.T) {
	if got, want := Path("/x/.git"), filepath.Join("/x/.git", "envkeep.base"); got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
}
