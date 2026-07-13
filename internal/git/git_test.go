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

func TestCommonDirNormal(t *testing.T) {
	f := fixture(t, false)
	got, err := CommonDir(f["WT_A"])
	if err != nil {
		t.Fatal(err)
	}
	if got != f["COMMON_DIR"] {
		t.Errorf("CommonDir = %q, want %q", got, f["COMMON_DIR"])
	}
	if !filepath.IsAbs(got) {
		t.Errorf("CommonDir not absolute: %q", got)
	}
}

func TestCommonDirBareResolvesToDotBare(t *testing.T) {
	f := fixture(t, true)
	// The whole bare-repo premise (D13): common dir from a linked worktree must
	// resolve to the shared .bare directory.
	for _, from := range []string{f["WT_A"], f["WT_B"], f["MAIN"]} {
		got, err := CommonDir(from)
		if err != nil {
			t.Fatal(err)
		}
		if got != f["COMMON_DIR"] {
			t.Errorf("CommonDir(%q) = %q, want %q", from, got, f["COMMON_DIR"])
		}
		if filepath.Base(got) != ".bare" {
			t.Errorf("CommonDir = %q, want it to end in .bare", got)
		}
	}
}

func TestLocateMatchesIndividualQueries(t *testing.T) {
	for _, bare := range []bool{false, true} {
		f := fixture(t, bare)
		p, err := Locate(f["WT_A"])
		if err != nil {
			t.Fatal(err)
		}
		common, _ := CommonDir(f["WT_A"])
		dir, _ := Dir(f["WT_A"])
		top, _ := Toplevel(f["WT_A"])
		if p.CommonDir != common || p.GitDir != dir || p.Toplevel != top {
			t.Errorf("bare=%v Locate = %+v, want {%s %s %s}", bare, p, common, dir, top)
		}
		if p.CommonDir != f["COMMON_DIR"] {
			t.Errorf("bare=%v Locate.CommonDir = %q, want %q", bare, p.CommonDir, f["COMMON_DIR"])
		}
	}
}

func TestToplevel(t *testing.T) {
	f := fixture(t, false)
	got, err := Toplevel(f["WT_A"])
	if err != nil {
		t.Fatal(err)
	}
	if got != f["WT_A"] {
		t.Errorf("Toplevel = %q, want %q", got, f["WT_A"])
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
