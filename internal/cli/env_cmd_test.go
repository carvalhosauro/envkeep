package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEnvsLists verifies `envs` lists every environment and marks the repo's
// default (the first one created, D27) with "*".
func TestEnvsLists(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "K=1\n")
	t.Chdir(f["WT_A"])
	pushEnv(t, f["WT_A"], "prod", true)
	pushEnv(t, f["WT_A"], "homo", true)
	out, err := execRoot(t, "envs")
	if err != nil {
		t.Fatalf("envs: %v", err)
	}
	if !strings.Contains(out, "homo") || !strings.Contains(out, "prod") {
		t.Errorf("envs missing environments:\n%s", out)
	}
	if !strings.Contains(out, "* prod") { // default (first created) marked
		t.Errorf("envs should mark the default:\n%s", out)
	}
}

// TestEnvsEmptyRepoHintsCreate verifies `envs` prints a helpful hint instead
// of an empty list when the repo has no named environments yet (legacy flat
// vault or brand-new repo).
func TestEnvsEmptyRepoHintsCreate(t *testing.T) {
	f := fixture(t)
	t.Chdir(f["WT_A"])
	out, err := execRoot(t, "envs")
	if err != nil {
		t.Fatalf("envs: %v", err)
	}
	if !strings.Contains(out, "no environments yet") {
		t.Errorf("envs on empty repo should hint at creation:\n%s", out)
	}
}

// TestUseRepointsCurrentWorktree verifies `use <env>` re-points the current
// worktree to that environment (Pull already does the re-point, D31).
func TestUseRepointsCurrentWorktree(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "DB=prod\n")
	t.Chdir(f["WT_A"])
	pushEnv(t, f["WT_A"], "prod", true)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "DB=homo\n")
	pushEnv(t, f["WT_A"], "homo", true) // now on homo
	if _, err := execRoot(t, "use", "prod"); err != nil {
		t.Fatalf("use prod: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(f["WT_A"], ".env"))
	if !strings.Contains(string(got), "DB=prod") {
		t.Errorf("use did not re-point to prod:\n%s", got)
	}
}

// TestUseUnknownEnvRefused verifies `use` refuses to switch to an environment
// that does not exist (unless -c/--create is passed).
func TestUseUnknownEnvRefused(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "K=1\n")
	t.Chdir(f["WT_A"])
	if _, err := execRoot(t, "use", "ghost"); err == nil {
		t.Error("use ghost: want error, got nil")
	}
}

// TestRmRefusesWhenWorktreeOnIt verifies `rm <env>` refuses to delete an
// environment while any worktree's active env still points at it, unless
// --force is passed (E5).
func TestRmRefusesWhenWorktreeOnIt(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "K=1\n")
	t.Chdir(f["WT_A"])
	pushEnv(t, f["WT_A"], "prod", true) // wt-a active on prod
	if _, err := execRoot(t, "rm", "prod"); err == nil {
		t.Error("rm prod: want refusal (a worktree is on it), got nil")
	}
	// --force deletes anyway
	if _, err := execRoot(t, "rm", "prod", "--force"); err != nil {
		t.Fatalf("rm prod --force: %v", err)
	}
}

// TestRmUnknownEnvRefused verifies `rm` refuses to delete an environment that
// does not exist.
func TestRmUnknownEnvRefused(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "K=1\n")
	t.Chdir(f["WT_A"])
	if _, err := execRoot(t, "rm", "ghost"); err == nil || !strings.Contains(err.Error(), "unknown environment") {
		t.Errorf("rm ghost: err = %v, want an 'unknown environment' refusal", err)
	}
}
