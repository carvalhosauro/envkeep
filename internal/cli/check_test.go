package cli

import (
	"bytes"
	"path/filepath"
	"testing"
)

func check(t *testing.T, cwd string) string {
	t.Helper()
	var b bytes.Buffer
	if err := Check(&b, cwd, false); err != nil {
		t.Fatalf("Check(%s): %v", cwd, err)
	}
	return b.String()
}

func checkPorcelain(t *testing.T, cwd string) string {
	t.Helper()
	var b bytes.Buffer
	if err := Check(&b, cwd, true); err != nil {
		t.Fatalf("Check(%s, porcelain): %v", cwd, err)
	}
	return b.String()
}

func TestCheckSilentWhenClean(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "KEY=value\n")
	mustPush(t, f["WT_A"])

	if out := check(t, f["WT_A"]); out != "" {
		t.Errorf("Check on a clean worktree should be silent, got %q", out)
	}
}

func TestCheckWarnsWhenAhead(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "KEY=v1\n")
	mustPush(t, f["WT_A"])

	// Local changes after sync → ahead → the hook should nudge to push.
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "KEY=v2\n")
	out := check(t, f["WT_A"])
	if !contains(out, "ahead") || !contains(out, "push") {
		t.Errorf("Check when ahead = %q, want it to mention ahead + push", out)
	}
}

func TestCheckReportsUnsynced(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "KEY=v1\n")
	mustPush(t, f["WT_A"]) // creates the vault

	// wt-b has a local .env but never synced → no marker → "not yet synced".
	writeFile(t, filepath.Join(f["WT_B"], ".env"), "KEY=v1\n")
	if out := check(t, f["WT_B"]); !contains(out, "not yet synced") {
		t.Errorf("Check on unsynced worktree = %q, want 'not yet synced'", out)
	}
}

func TestCheckPorcelain(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "KEY=v1\n")
	mustPush(t, f["WT_A"])

	if out := checkPorcelain(t, f["WT_A"]); out != "" {
		t.Errorf("porcelain when clean = %q, want empty", out)
	}

	writeFile(t, filepath.Join(f["WT_A"], ".env"), "KEY=v2\n")
	if out := checkPorcelain(t, f["WT_A"]); out != "ahead\n" {
		t.Errorf("porcelain when ahead = %q, want %q", out, "ahead\n")
	}
}

func TestCheckSilentOutsideRepo(t *testing.T) {
	if out := check(t, t.TempDir()); out != "" {
		t.Errorf("Check outside a repo should be silent, got %q", out)
	}
}

func contains(s, sub string) bool {
	return bytes.Contains([]byte(s), []byte(sub))
}
