package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/carvalhosauro/envkeep/internal/config"
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

// TestUseCascadeSwitchesAllWorktrees verifies `use <env> --cascade` fans out to
// every worktree in the repo (D28), not just the current one — wt-b starts
// absent from prod, and the cascade must pull it in too.
func TestUseCascadeSwitchesAllWorktrees(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "DB=prod\n")
	t.Chdir(f["WT_A"])
	pushEnv(t, f["WT_A"], "prod", true)
	// wt-b starts absent; cascade should pull prod into it too
	if _, err := execRoot(t, "use", "prod", "--cascade"); err != nil {
		t.Fatalf("use --cascade: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(f["WT_B"], ".env"))
	if !strings.Contains(string(got), "DB=prod") {
		t.Errorf("cascade did not reach wt-b:\n%s", got)
	}
}

// TestUseCascadeSkipsWorktreeWithUnpushedEdits proves cascade reuses Pull's E4
// re-point guard instead of re-implementing it (D28): a worktree that is on a
// different environment with edits not yet pushed there must be skipped and
// reported, never clobbered.
func TestUseCascadeSkipsWorktreeWithUnpushedEdits(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "DB=prod\n")
	t.Chdir(f["WT_A"])
	pushEnv(t, f["WT_A"], "prod", true)

	// wt-b: cleanly on homo, then edited locally without pushing.
	writeFile(t, filepath.Join(f["WT_B"], ".env"), "DB=homo\n")
	pushEnv(t, f["WT_B"], "homo", true)
	writeFile(t, filepath.Join(f["WT_B"], ".env"), "DB=homo-EDITED\n")

	out, err := execRoot(t, "use", "prod", "--cascade")
	if err != nil {
		t.Fatalf("use --cascade: %v", err)
	}
	got, readErr := os.ReadFile(filepath.Join(f["WT_B"], ".env"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	if !strings.Contains(string(got), "DB=homo-EDITED") {
		t.Errorf("cascade clobbered wt-b's unpushed edit instead of skipping it:\n%s", got)
	}
	if !strings.Contains(out, f["WT_B"]) {
		t.Errorf("cascade output should report the skipped worktree %q:\n%s", f["WT_B"], out)
	}
	if !strings.Contains(out, "not pushed") {
		t.Errorf("cascade output should report the skip reason:\n%s", out)
	}
}

// TestUseCascadeDryRunPreviewsWithoutWriting verifies `--cascade --dry-run`
// leaves every worktree's local env untouched (D28's dry-run preview).
func TestUseCascadeDryRunPreviewsWithoutWriting(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "DB=prod\n")
	t.Chdir(f["WT_A"])
	pushEnv(t, f["WT_A"], "prod", true)

	if _, err := execRoot(t, "use", "prod", "--cascade", "--dry-run"); err != nil {
		t.Fatalf("use --cascade --dry-run: %v", err)
	}
	if _, err := os.Stat(filepath.Join(f["WT_B"], ".env")); !os.IsNotExist(err) {
		t.Errorf("dry-run cascade should not have written wt-b's .env, stat err = %v", err)
	}
}

// TestUseCascadeUnknownEnvRefused verifies cascade refuses to fan out to an
// environment that does not exist.
func TestUseCascadeUnknownEnvRefused(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "K=1\n")
	t.Chdir(f["WT_A"])
	if _, err := execRoot(t, "use", "ghost", "--cascade"); err == nil || !strings.Contains(err.Error(), "unknown environment") {
		t.Errorf("use ghost --cascade: err = %v, want an 'unknown environment' refusal", err)
	}
}

// TestUseHonorsConfigCascadeDefault verifies `use <env>` (no --cascade flag)
// fans out to every worktree when the repo config sets cascade=true (D28), so
// a repo can opt in once instead of the caller passing --cascade every time.
func TestUseHonorsConfigCascadeDefault(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "DB=prod\n")
	t.Chdir(f["WT_A"])
	pushEnv(t, f["WT_A"], "prod", true)

	cfg, err := config.Load(f["COMMON_DIR"])
	if err != nil {
		t.Fatal(err)
	}
	cfg.Cascade = true
	if err := config.Save(f["COMMON_DIR"], cfg); err != nil {
		t.Fatal(err)
	}

	if _, err := execRoot(t, "use", "prod"); err != nil { // no --cascade flag
		t.Fatalf("use prod: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(f["WT_B"], ".env"))
	if !strings.Contains(string(got), "DB=prod") {
		t.Errorf("use should have cascaded via the config default:\n%s", got)
	}
}
