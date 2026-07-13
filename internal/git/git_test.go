package git

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fixture runs scripts/mkfixture.sh and returns its KEY=VALUE output as a map.
func fixture(t *testing.T, bare bool) map[string]string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not installed")
	}
	script, err := filepath.Abs(filepath.Join("..", "..", "scripts", "mkfixture.sh"))
	if err != nil {
		t.Fatal(err)
	}
	args := []string{script}
	if bare {
		args = append(args, "--bare")
	}
	args = append(args, filepath.Join(t.TempDir(), "repo"))

	out, err := exec.Command("bash", args...).CombinedOutput()
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

func TestLocateResolvesAllThreePaths(t *testing.T) {
	for _, bare := range []bool{false, true} {
		f := fixture(t, bare)
		p, err := Locate(f["WT_A"])
		if err != nil {
			t.Fatal(err)
		}
		gitDir, err := Dir(f["WT_A"])
		if err != nil {
			t.Fatal(err)
		}
		if p.CommonDir != f["COMMON_DIR"] || p.GitDir != gitDir || p.Toplevel != f["WT_A"] {
			t.Errorf("bare=%v Locate = %+v, want {common:%q gitdir:%q top:%q}",
				bare, p, f["COMMON_DIR"], gitDir, f["WT_A"])
		}
		if !filepath.IsAbs(p.CommonDir) || !filepath.IsAbs(p.GitDir) || !filepath.IsAbs(p.Toplevel) {
			t.Errorf("bare=%v Locate paths not all absolute: %+v", bare, p)
		}
	}
}

func TestLocateBareResolvesToDotBare(t *testing.T) {
	f := fixture(t, true)
	// Bare-repo premise (D13): every worktree resolves to the same shared .bare
	// common dir.
	for _, from := range []string{f["WT_A"], f["WT_B"], f["MAIN"]} {
		p, err := Locate(from)
		if err != nil {
			t.Fatal(err)
		}
		if p.CommonDir != f["COMMON_DIR"] {
			t.Errorf("Locate(%q).CommonDir = %q, want %q", from, p.CommonDir, f["COMMON_DIR"])
		}
		if filepath.Base(p.CommonDir) != ".bare" {
			t.Errorf("CommonDir = %q, want it to end in .bare", p.CommonDir)
		}
	}
}

func TestGitDirIsPerWorktree(t *testing.T) {
	f := fixture(t, false)
	main, err := Dir(f["MAIN"])
	if err != nil {
		t.Fatal(err)
	}
	linked, err := Dir(f["WT_A"])
	if err != nil {
		t.Fatal(err)
	}
	if main == linked {
		t.Errorf("main and linked worktree share a gitdir: %q", main)
	}
	if !strings.Contains(filepath.ToSlash(linked), "worktrees/wt-a") {
		t.Errorf("linked GitDir = %q, want .../worktrees/wt-a", linked)
	}
}

func TestWorktreesNormal(t *testing.T) {
	f := fixture(t, false)
	wts, err := Worktrees(f["MAIN"])
	if err != nil {
		t.Fatal(err)
	}
	branches := map[string]bool{}
	for _, w := range wts {
		branches[w.Branch] = true
		if w.Bare {
			t.Errorf("normal layout should have no bare worktree, got %+v", w)
		}
	}
	for _, b := range []string{"main", "wt-a", "wt-b"} {
		if !branches[b] {
			t.Errorf("missing worktree for branch %q (got %v)", b, branches)
		}
	}
}

func TestWorktreesBareHasBareEntry(t *testing.T) {
	f := fixture(t, true)
	wts, err := Worktrees(f["WT_A"])
	if err != nil {
		t.Fatal(err)
	}
	var bareSeen bool
	branches := map[string]bool{}
	for _, w := range wts {
		if w.Bare {
			bareSeen = true
		} else {
			branches[w.Branch] = true
		}
	}
	if !bareSeen {
		t.Errorf("bare layout should include a bare worktree entry, got %+v", wts)
	}
	for _, b := range []string{"main", "wt-a", "wt-b"} {
		if !branches[b] {
			t.Errorf("missing worktree for branch %q (got %v)", b, branches)
		}
	}
}
