package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileAtomic(t *testing.T) {
	// Nested path exercises the MkdirAll branch.
	path := filepath.Join(t.TempDir(), "sub", "f.txt")

	if err := WriteFileAtomic(path, []byte("hi"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hi" {
		t.Errorf("content = %q, want hi", got)
	}
	if fi, _ := os.Stat(path); fi.Mode().Perm() != 0o600 {
		t.Errorf("perm = %v, want 0600", fi.Mode().Perm())
	}

	// Overwrite atomically, no temp file left behind.
	if err := WriteFileAtomic(path, []byte("bye"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got, _ := os.ReadFile(path); string(got) != "bye" {
		t.Errorf("after overwrite = %q, want bye", got)
	}
	if entries, _ := os.ReadDir(filepath.Dir(path)); len(entries) != 1 {
		t.Errorf("dir has %d entries, want 1 (no temp leftover)", len(entries))
	}
}

func TestWriteFileAtomicMkdirError(t *testing.T) {
	// A regular file stands where a parent directory would go, so MkdirAll fails.
	blocker := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(blocker, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteFileAtomic(filepath.Join(blocker, "sub", "x.txt"), []byte("y"), 0o600); err == nil {
		t.Error("expected an error when the parent path is a file")
	}
}
