package cli

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/carvalhosauro/envkeep/internal/git"
	"github.com/carvalhosauro/envkeep/internal/state"
)

// loadMarker returns the sync marker for a worktree, failing if none exists.
func loadMarker(t *testing.T, cwd string) state.Marker {
	t.Helper()
	gd, err := git.Dir(cwd)
	if err != nil {
		t.Fatalf("git.Dir(%s): %v", cwd, err)
	}
	m, ok, err := state.Load(gd)
	if err != nil {
		t.Fatalf("state.Load(%s): %v", gd, err)
	}
	if !ok {
		t.Fatalf("no marker for %s", cwd)
	}
	return m
}

// staleBaseAgreeing drives wt-b into the stuck state from issue #1: its marker
// base is older than the vault, yet its local content already equals the vault.
// Returns the fixture. After this, wt-b has base={KEY:v1}, local=vault={KEY:v2}.
func staleBaseAgreeing(t *testing.T) map[string]string {
	t.Helper()
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "KEY=v1\n")
	mustPush(t, f["WT_A"]) // vault = v1
	mustPull(t, f["WT_B"]) // wt-b marker base = v1, local = v1

	writeFile(t, filepath.Join(f["WT_A"], ".env"), "KEY=v2\n")
	mustPush(t, f["WT_A"]) // vault = v2; wt-b marker base still v1

	// wt-b reaches the same new value locally without pulling: local==vault,
	// but base is stale → the old code classifies this as diverged forever.
	writeFile(t, filepath.Join(f["WT_B"], ".env"), "KEY=v2\n")
	return f
}

// TestCheckStaleBaseAgreesIsCleanAndRetires covers the #4 outcome for the hook:
// when content already agrees with the vault, check reports clean regardless of
// base age, and retires the stale base so the fast path works next time.
func TestCheckStaleBaseAgreesIsCleanAndRetires(t *testing.T) {
	f := staleBaseAgreeing(t)

	if out := checkPorcelain(t, f["WT_B"]); out != "" {
		t.Errorf("porcelain check with agreeing content = %q, want empty (clean)", out)
	}

	m := loadMarker(t, f["WT_B"])
	if m.Base["KEY"] != "v2" {
		t.Errorf("marker base = %v, want it retired to {KEY:v2}", m.Base)
	}
	// Marker mtimes refreshed → the cheap fast path applies from now on.
	if lm, _ := mtimeNanos(filepath.Join(f["WT_B"], ".env")); m.LocalMTime != lm {
		t.Errorf("marker LocalMTime = %d, want refreshed to %d", m.LocalMTime, lm)
	}
}

// TestPullStaleBaseAgreesRetiresBase covers the #1 pointed fix on pull: the
// empty-delta "already in sync" path must refresh the stale marker.
func TestPullStaleBaseAgreesRetiresBase(t *testing.T) {
	f := staleBaseAgreeing(t)

	out := mustPull(t, f["WT_B"])
	if !bytes.Contains([]byte(out), []byte("already in sync")) {
		t.Errorf("pull = %q, want 'already in sync'", out)
	}
	if m := loadMarker(t, f["WT_B"]); m.Base["KEY"] != "v2" {
		t.Errorf("after pull marker base = %v, want retired to {KEY:v2}", m.Base)
	}
}

// TestPushStaleBaseAgreesRetiresBase is the symmetric #1 fix on push.
func TestPushStaleBaseAgreesRetiresBase(t *testing.T) {
	f := staleBaseAgreeing(t)

	out := mustPush(t, f["WT_B"])
	if !bytes.Contains([]byte(out), []byte("already in sync")) {
		t.Errorf("push = %q, want 'already in sync'", out)
	}
	if m := loadMarker(t, f["WT_B"]); m.Base["KEY"] != "v2" {
		t.Errorf("after push marker base = %v, want retired to {KEY:v2}", m.Base)
	}
}

// TestStatusStaleBaseAgreesShowsClean covers status: it must display clean (not
// a false diverged) once local and vault agree, even with a stale base.
func TestStatusStaleBaseAgreesShowsClean(t *testing.T) {
	f := staleBaseAgreeing(t)

	var b bytes.Buffer
	if err := Status(&b, f["WT_B"], ""); err != nil {
		t.Fatal(err)
	}
	if out := b.String(); bytes.Contains([]byte(out), []byte("diverged")) {
		t.Errorf("status = %q, want no 'diverged' (content agrees)", out)
	}
}

// TestCheckMtimeBumpNoContentChangeRefreshes covers the #4 acceptance: touching
// the file without changing its logical content stays clean and refreshes the
// stored mtime so the fast path works again.
func TestCheckMtimeBumpNoContentChangeRefreshes(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "KEY=v1\n")
	mustPush(t, f["WT_A"])

	// Re-render with a comment: mtime moves, logical env ({KEY:v1}) is unchanged.
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "# note\nKEY=v1\n")

	if out := checkPorcelain(t, f["WT_A"]); out != "" {
		t.Errorf("porcelain after comment-only edit = %q, want empty (clean)", out)
	}
	m := loadMarker(t, f["WT_A"])
	if lm, _ := mtimeNanos(filepath.Join(f["WT_A"], ".env")); m.LocalMTime != lm {
		t.Errorf("marker LocalMTime = %d, want refreshed to %d", m.LocalMTime, lm)
	}
}
