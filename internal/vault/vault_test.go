package vault

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/carvalhosauro/envkeep/internal/envfile"
)

func TestReadMissingReturnsNotFound(t *testing.T) {
	s := NewFileStore(filepath.Join(t.TempDir(), "vault", ".env"))
	_, err := s.Read()
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Read() error = %v, want ErrNotFound", err)
	}
}

func TestWriteThenReadRoundTrip(t *testing.T) {
	s := NewFileStore(filepath.Join(t.TempDir(), "envkeep", "vault", ".env"))
	want := envfile.Env{"A": "1", "MSG": "hello world", "URL": "http://x?a=b"}
	if err := s.Write(want); err != nil {
		t.Fatal(err)
	}
	got, err := s.Read()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round trip = %v, want %v", got, want)
	}
}

func TestWriteIsSortedAndQuoted(t *testing.T) {
	s := NewFileStore(filepath.Join(t.TempDir(), ".env"))
	if err := s.Write(envfile.Env{"B": "2", "A": "a b", "C": "3"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(s.Path())
	if err != nil {
		t.Fatal(err)
	}
	want := "A=\"a b\"\nB=2\nC=3\n"
	if string(data) != want {
		t.Errorf("vault content = %q, want %q", data, want)
	}
}

func TestWriteEmptyEnvIsNotFresh(t *testing.T) {
	s := NewFileStore(filepath.Join(t.TempDir(), ".env"))
	if err := s.Write(envfile.Env{}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Read() // must succeed: the vault exists, just empty
	if err != nil {
		t.Fatalf("Read() after empty Write: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("empty vault Read = %v, want empty", got)
	}
}

func TestWriteOverwritesAtomically(t *testing.T) {
	s := NewFileStore(filepath.Join(t.TempDir(), ".env"))
	if err := s.Write(envfile.Env{"A": "1", "OLD": "x"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Write(envfile.Env{"A": "2"}); err != nil {
		t.Fatal(err)
	}
	got, err := s.Read()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, envfile.Env{"A": "2"}) {
		t.Errorf("after overwrite = %v, want {A:2}", got)
	}
	// No stray temp files left behind.
	entries, _ := os.ReadDir(filepath.Dir(s.Path()))
	if len(entries) != 1 {
		t.Errorf("dir has %d entries, want 1 (the vault)", len(entries))
	}
}

func TestReadCorruptVaultErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("this line has no equals sign\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	s := NewFileStore(path)
	_, err := s.Read()
	if err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("Read() of corrupt vault error = %v, want a parse error", err)
	}
}

func TestPathLayout(t *testing.T) {
	got := Path("/repo/.git", ".env.local")
	want := filepath.Join("/repo/.git", "envkeep", "vault", ".env.local")
	if got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
}
