package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func run(t *testing.T, fn func(*bytes.Buffer) error) (string, error) {
	t.Helper()
	var b bytes.Buffer
	err := fn(&b)
	return b.String(), err
}

func TestPushDryRunDoesNotWriteVault(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "KEY=value\n")

	out, err := run(t, func(b *bytes.Buffer) error { return Push(b, f["WT_A"], "", "", false, true, false) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "dry run") || !strings.Contains(out, "+ KEY") {
		t.Errorf("dry-run output = %q, want delta + dry-run note", out)
	}
	if strings.Contains(out, "value") {
		t.Errorf("dry-run output = %q, must not leak secret values (#22)", out)
	}
	if _, err := os.Stat(f["COMMON_DIR"] + "/envkeep/vault/.env"); !os.IsNotExist(err) {
		t.Error("dry-run must not create the vault")
	}
}

func TestPushAlreadyInSync(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "KEY=value\n")
	mustPush(t, f["WT_A"])

	out := mustPush(t, f["WT_A"])
	if !strings.Contains(out, "already in sync") {
		t.Errorf("second push = %q, want 'already in sync'", out)
	}
}

func TestPushRefusesWhenBehind(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "KEY=v1\n")
	mustPush(t, f["WT_A"])
	mustPull(t, f["WT_B"]) // wt-b synced at v1

	writeFile(t, filepath.Join(f["WT_A"], ".env"), "KEY=v2\n")
	mustPush(t, f["WT_A"]) // vault -> v2; wt-b now behind

	err := Push(&bytes.Buffer{}, f["WT_B"], "", "", false, false, false)
	if err == nil || !strings.Contains(err.Error(), "pull") {
		t.Errorf("push when behind = %v, want refusal mentioning pull", err)
	}
}

func TestPushNoLocalEnv(t *testing.T) {
	f := fixture(t)
	err := Push(&bytes.Buffer{}, f["WT_A"], "", "", false, false, false)
	if err == nil || !strings.Contains(err.Error(), "no .env") {
		t.Errorf("push without .env = %v, want 'no .env' error", err)
	}
}

func TestPullDryRunDoesNotWriteLocal(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "KEY=value\n")
	mustPush(t, f["WT_A"])

	out, err := run(t, func(b *bytes.Buffer) error { return Pull(b, f["WT_B"], "", "", false, true) })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "dry run") {
		t.Errorf("pull dry-run = %q, want dry-run note", out)
	}
	if _, err := os.Stat(filepath.Join(f["WT_B"], ".env")); !os.IsNotExist(err) {
		t.Error("pull dry-run must not write local .env")
	}
}

func TestPullNoVault(t *testing.T) {
	f := fixture(t)
	err := Pull(&bytes.Buffer{}, f["WT_A"], "", "", false, false)
	if err == nil || !strings.Contains(err.Error(), "push") {
		t.Errorf("pull with no vault = %v, want refusal mentioning push", err)
	}
}

func TestPullRemovesStaleLocalKey(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "A=1\n")
	mustPush(t, f["WT_A"]) // vault = {A}

	// wt-b has an extra local-only key Z and no marker yet.
	writeFile(t, filepath.Join(f["WT_B"], ".env"), "A=1\nZ=9\n")
	out := mustPull(t, f["WT_B"])
	if !strings.Contains(out, "- Z") {
		t.Errorf("pull output = %q, want it to show Z removed", out)
	}
	got, _ := os.ReadFile(filepath.Join(f["WT_B"], ".env"))
	if strings.Contains(string(got), "Z=9") {
		t.Errorf("pull should have removed stale Z:\n%s", got)
	}
}

func TestStatusNoVault(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "KEY=value\n")

	out, err := run(t, func(b *bytes.Buffer) error { return Status(b, f["WT_A"], "", "") })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "no vault yet") {
		t.Errorf("status with no vault = %q, want 'no vault yet'", out)
	}
	if !strings.Contains(out, "unsynced (no vault)") {
		t.Errorf("status = %q, want the worktree with a .env marked unsynced", out)
	}
}

func TestStatusUnsynced(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "KEY=v1\n")
	mustPush(t, f["WT_A"]) // vault exists, wt-a synced

	// wt-b has a local .env but never synced.
	writeFile(t, filepath.Join(f["WT_B"], ".env"), "KEY=v1\n")
	out, err := run(t, func(b *bytes.Buffer) error { return Status(b, f["WT_B"], "", "") })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "unsynced") {
		t.Errorf("status = %q, want wt-b 'unsynced'", out)
	}
}

// Conflict output lists key names only — values are secrets (#22).
func TestConflictOutputRedactsValues(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "KEY=v1\n")
	mustPush(t, f["WT_A"])
	mustPull(t, f["WT_B"])

	writeFile(t, filepath.Join(f["WT_A"], ".env"), "KEY=v2\n")
	mustPush(t, f["WT_A"]) // vault -> v2
	writeFile(t, filepath.Join(f["WT_B"], ".env"), "KEY=v3\n")

	out, err := run(t, func(b *bytes.Buffer) error { return Push(b, f["WT_B"], "", "", false, false, false) })
	if err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Fatalf("push = %v, want conflict refusal", err)
	}
	if !strings.Contains(out, "KEY") || strings.Contains(out, "v2") || strings.Contains(out, "v3") {
		t.Errorf("conflict output = %q, want key name without values", out)
	}
}

// pull rewrites a group/world-readable local env at 0600 instead of
// preserving the wider mode (#23).
func TestPullTightensLocalEnvPerm(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "KEY=v1\n")
	mustPush(t, f["WT_A"])

	local := filepath.Join(f["WT_B"], ".env")
	writeFile(t, local, "")
	if err := os.Chmod(local, 0o644); err != nil {
		t.Fatal(err)
	}
	mustPull(t, f["WT_B"])

	fi, err := os.Stat(local)
	if err != nil {
		t.Fatal(err)
	}
	if got := fi.Mode().Perm(); got != 0o600 {
		t.Errorf("pulled .env mode = %o, want 0600", got)
	}
}
