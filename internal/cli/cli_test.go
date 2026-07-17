package cli

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/carvalhosauro/envkeep/internal/vault"
)

// fixture runs scripts/mkfixture.sh and returns its KEY=VALUE output as a map.
func fixture(t *testing.T) map[string]string {
	t.Helper()
	for _, bin := range []string{"git", "bash"} {
		if _, err := exec.LookPath(bin); err != nil {
			t.Skipf("%s not installed", bin)
		}
	}
	script, err := filepath.Abs(filepath.Join("..", "..", "scripts", "mkfixture.sh"))
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("bash", script, filepath.Join(t.TempDir(), "repo")).CombinedOutput()
	if err != nil {
		t.Fatalf("mkfixture failed: %v\n%s", err, out)
	}
	kv := map[string]string{}
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			kv[k] = v
		}
	}
	return kv
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func mustPush(t *testing.T, cwd string) string {
	t.Helper()
	var b bytes.Buffer
	if err := Push(&b, cwd, "", "", false, false, false); err != nil {
		t.Fatalf("Push(%s): %v\n%s", cwd, err, b.String())
	}
	return b.String()
}

func mustPull(t *testing.T, cwd string) string {
	t.Helper()
	var b bytes.Buffer
	if err := Pull(&b, cwd, "", "", false, false); err != nil {
		t.Fatalf("Pull(%s): %v\n%s", cwd, err, b.String())
	}
	return b.String()
}

func readVaultEnv(t *testing.T, f map[string]string) vault.Store {
	t.Helper()
	return vault.NewFileStore(vault.Path(f["COMMON_DIR"], ".env"))
}

func TestPushThenStatusClean(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "KEY=value\n")

	mustPush(t, f["WT_A"])

	var b bytes.Buffer
	if err := Status(&b, f["WT_A"], "", ""); err != nil {
		t.Fatal(err)
	}
	out := b.String()
	if !strings.Contains(out, "clean") {
		t.Errorf("status missing 'clean' for pushed worktree:\n%s", out)
	}
	if !strings.Contains(out, "absent") {
		t.Errorf("status missing 'absent' for the other worktrees:\n%s", out)
	}
}

func TestPropagateAcrossWorktrees(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "KEY=value\nOTHER=x\n")
	mustPush(t, f["WT_A"])
	mustPull(t, f["WT_B"])

	got, err := os.ReadFile(filepath.Join(f["WT_B"], ".env"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"KEY=value", "OTHER=x"} {
		if !strings.Contains(string(got), want) {
			t.Errorf("wt-b .env missing %q:\n%s", want, got)
		}
	}
}

func TestOverrideExcludedOnPushAndReappliedOnPull(t *testing.T) {
	f := fixture(t)
	// wt-a: shared KEY plus a worktree-local PORT marked as override.
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "KEY=shared\nPORT=3000\n")
	writeFile(t, filepath.Join(f["WT_A"], ".env.override"), "PORT=3000\n")
	mustPush(t, f["WT_A"])

	vaultEnv, err := readVaultEnv(t, f).Read()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := vaultEnv["PORT"]; ok {
		t.Errorf("override key PORT leaked into vault: %v", vaultEnv)
	}
	if vaultEnv["KEY"] != "shared" {
		t.Errorf("vault missing shared KEY: %v", vaultEnv)
	}

	// wt-b: its own override port; pull must apply KEY and keep PORT=3001.
	writeFile(t, filepath.Join(f["WT_B"], ".env.override"), "PORT=3001\n")
	mustPull(t, f["WT_B"])

	got, err := os.ReadFile(filepath.Join(f["WT_B"], ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "KEY=shared") || !strings.Contains(string(got), "PORT=3001") {
		t.Errorf("wt-b .env wrong after pull:\n%s", got)
	}
}

func TestPushConflictRefused(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "KEY=v1\n")
	mustPush(t, f["WT_A"])
	mustPull(t, f["WT_B"]) // wt-b now synced at KEY=v1

	// Both worktrees change KEY differently.
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "KEY=v2\n")
	mustPush(t, f["WT_A"]) // vault -> v2

	writeFile(t, filepath.Join(f["WT_B"], ".env"), "KEY=v3\n")
	var b bytes.Buffer
	err := Push(&b, f["WT_B"], "", "", false, false, false)
	if err == nil {
		t.Fatalf("expected conflict error, got success:\n%s", b.String())
	}
	if !strings.Contains(err.Error(), "conflict") {
		t.Errorf("error = %q, want it to mention conflict", err)
	}
}

func TestPullRefusesWhenLocalAhead(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "KEY=v1\n")
	mustPush(t, f["WT_A"])

	// Local edited after sync, vault unchanged -> ahead -> pull must refuse.
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "KEY=v2\n")
	err := Pull(&bytes.Buffer{}, f["WT_A"], "", "", false, false)
	if err == nil || !strings.Contains(err.Error(), "push") {
		t.Errorf("Pull error = %v, want a refusal telling to push first", err)
	}
}

// TestPullAheadRefusalMessageUnchangedAndClassifiable proves the ErrRefused
// sentinel (added for cascade, D28/C1) is purely additive: a direct
// `envkeep pull` caller still sees the exact refusal message Pull has always
// produced (message preservation, Deliverable A guarantee), while
// errors.Is(err, ErrRefused) now lets cascade classify it as a skip rather
// than string-matching.
func TestPullAheadRefusalMessageUnchangedAndClassifiable(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "KEY=v1\n")
	mustPush(t, f["WT_A"])

	writeFile(t, filepath.Join(f["WT_A"], ".env"), "KEY=v2\n")
	err := Pull(&bytes.Buffer{}, f["WT_A"], "", "", false, false)

	const want = "local has changes not in the vault; run 'envkeep push' first"
	if err == nil || err.Error() != want {
		t.Fatalf("Pull error = %v, want byte-identical %q", err, want)
	}
	if !errors.Is(err, ErrRefused) {
		t.Errorf("errors.Is(err, ErrRefused) = false, want true")
	}
}

// TestPullConflictRefusalMessageUnchangedAndClassifiable is the conflict-guard
// counterpart of TestPullAheadRefusalMessageUnchangedAndClassifiable.
func TestPullConflictRefusalMessageUnchangedAndClassifiable(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "KEY=v1\n")
	mustPush(t, f["WT_A"])
	mustPull(t, f["WT_B"]) // wt-b now synced at KEY=v1

	writeFile(t, filepath.Join(f["WT_A"], ".env"), "KEY=v2\n")
	mustPush(t, f["WT_A"]) // vault -> v2

	writeFile(t, filepath.Join(f["WT_B"], ".env"), "KEY=v3\n") // wt-b diverges too
	err := Pull(&bytes.Buffer{}, f["WT_B"], "", "", false, false)

	const want = "conflict: vault and local changed the same key(s); resolve, then pull"
	if err == nil || err.Error() != want {
		t.Fatalf("Pull error = %v, want byte-identical %q", err, want)
	}
	if !errors.Is(err, ErrRefused) {
		t.Errorf("errors.Is(err, ErrRefused) = false, want true")
	}
}
