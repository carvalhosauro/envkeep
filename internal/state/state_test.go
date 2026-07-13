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

func TestPath(t *testing.T) {
	if got, want := Path("/x/.git"), filepath.Join("/x/.git", "envkeep.base"); got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
}
