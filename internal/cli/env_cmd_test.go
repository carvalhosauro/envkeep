package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/carvalhosauro/envkeep/internal/config"
	"github.com/carvalhosauro/envkeep/internal/env"
	"github.com/carvalhosauro/envkeep/internal/git"
	"github.com/carvalhosauro/envkeep/internal/state"
	"github.com/carvalhosauro/envkeep/internal/vault"
)

// TestEnvsLists verifies `envs` lists every environment, marks this worktree's
// active env with "*", and labels the repo default with "(default)".
func TestEnvsLists(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "K=1\n")
	t.Chdir(f["WT_A"])
	pushEnv(t, f["WT_A"], "prod", true)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "K=2\n")
	pushEnv(t, f["WT_A"], "homo", true) // active on homo; prod remains default
	out, err := execRoot(t, "envs")
	if err != nil {
		t.Fatalf("envs: %v", err)
	}
	if !strings.Contains(out, "* homo") {
		t.Errorf("envs should mark the active env with *:\n%s", out)
	}
	if !strings.Contains(out, "prod (default)") {
		t.Errorf("envs should label the default:\n%s", out)
	}
	if strings.Contains(out, "* prod") {
		t.Errorf("envs must not mark the default as active when another is active:\n%s", out)
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

// TestUseCreateNewEnvSnapshotsFromCurrent verifies `use -c <new>` behaves like
// `git checkout -b`: it creates the new environment by snapshotting the
// CURRENT worktree's local .env into the new env's vault and re-points the
// worktree there, leaving local untouched — rather than emptying local and
// re-pointing to a phantom vault-less environment (the FU1 bug).
func TestUseCreateNewEnvSnapshotsFromCurrent(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "DB=prod-db\n")
	t.Chdir(f["WT_A"])
	pushEnv(t, f["WT_A"], "prod", true)

	// Local diverges from prod's vault before the checkout-b, so a snapshot of
	// "current local" is provably distinct from a reset to prod's content.
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "A=1\nB=2\n")

	if _, err := execRoot(t, "use", "-c", "staging"); err != nil {
		t.Fatalf("use -c staging: %v", err)
	}

	staging := readEnvVault(t, f["COMMON_DIR"], "staging")
	if staging["A"] != "1" || staging["B"] != "2" {
		t.Errorf("staging vault = %v, want a snapshot of current local (A=1, B=2)", staging)
	}

	got, err := os.ReadFile(filepath.Join(f["WT_A"], ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "A=1") || !strings.Contains(string(got), "B=2") {
		t.Errorf("local .env must stay intact after use -c, got:\n%s", got)
	}

	envsOut, err := execRoot(t, "envs")
	if err != nil {
		t.Fatalf("envs: %v", err)
	}
	if !strings.Contains(envsOut, "staging") {
		t.Errorf("envs should list staging:\n%s", envsOut)
	}

	statusOut, err := execRoot(t, "status")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !strings.Contains(statusOut, "staging") {
		t.Errorf("status should show the worktree re-pointed to staging:\n%s", statusOut)
	}
}

// TestUseCreateDoesNotEmptyLocal is a minimal explicit guard for the FU1
// regression: after `use -c <new>` the local .env must be non-empty and equal
// to what it was right before the command ran.
func TestUseCreateDoesNotEmptyLocal(t *testing.T) {
	f := fixture(t)
	t.Chdir(f["WT_A"])
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "X=9\nY=8\n")

	before, err := os.ReadFile(filepath.Join(f["WT_A"], ".env"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := execRoot(t, "use", "-c", "newenv"); err != nil {
		t.Fatalf("use -c newenv: %v", err)
	}

	after, err := os.ReadFile(filepath.Join(f["WT_A"], ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if len(after) == 0 {
		t.Fatal("use -c emptied the local .env — FU1 regression")
	}
	if string(after) != string(before) {
		t.Errorf("local .env changed after use -c newenv:\nbefore: %q\nafter:  %q", before, after)
	}
}

// TestUseCreateExistingEnvSwitches verifies `use -c <existing>` on an env that
// already exists just switches to it (pulls its content) rather than treating
// -c as re-creating/overwriting the target's vault from current local — -c
// only takes the checkout-b (snapshot-from-current) path when the target does
// not exist yet (D26).
func TestUseCreateExistingEnvSwitches(t *testing.T) {
	f := fixture(t)
	t.Chdir(f["WT_A"])
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "DB=prod-db\n")
	pushEnv(t, f["WT_A"], "prod", true)

	writeFile(t, filepath.Join(f["WT_A"], ".env"), "DB=homo-db\n")
	pushEnv(t, f["WT_A"], "homo", true) // now on homo, local DB=homo-db

	if _, err := execRoot(t, "use", "-c", "prod"); err != nil {
		t.Fatalf("use -c prod: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(f["WT_A"], ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "DB=prod-db") {
		t.Errorf("use -c on an existing env should pull its content, got:\n%s", got)
	}
	if strings.Contains(string(got), "DB=homo-db") {
		t.Errorf("local still holds homo's value after use -c prod:\n%s", got)
	}

	prod := readEnvVault(t, f["COMMON_DIR"], "prod")
	if prod["DB"] != "prod-db" {
		t.Errorf("use -c prod altered prod's vault: %v, want it unchanged (DB=prod-db)", prod)
	}
}

// TestUseCreateDryRunPreviewsNoCreate verifies `use -c <new> --dry-run`
// mutates nothing: the new environment is not created, local stays untouched,
// and the worktree marker is not re-pointed to it.
func TestUseCreateDryRunPreviewsNoCreate(t *testing.T) {
	f := fixture(t)
	t.Chdir(f["WT_A"])
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "DB=prod-db\n")
	pushEnv(t, f["WT_A"], "prod", true) // wt-a active on prod

	before, err := os.ReadFile(filepath.Join(f["WT_A"], ".env"))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := execRoot(t, "use", "-c", "staging", "--dry-run"); err != nil {
		t.Fatalf("use -c staging --dry-run: %v", err)
	}

	after, err := os.ReadFile(filepath.Join(f["WT_A"], ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Errorf("dry-run mutated local .env:\nbefore: %q\nafter:  %q", before, after)
	}

	if vault.EnvExists(f["COMMON_DIR"], env.Name("staging")) {
		t.Error("dry-run should not have created staging")
	}

	envsOut, err := execRoot(t, "envs")
	if err != nil {
		t.Fatalf("envs: %v", err)
	}
	if strings.Contains(envsOut, "staging") {
		t.Errorf("envs should not list staging after a dry run:\n%s", envsOut)
	}

	statusOut, err := execRoot(t, "status")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !strings.Contains(statusOut, "prod") {
		t.Errorf("status should still show the worktree on prod:\n%s", statusOut)
	}
	if strings.Contains(statusOut, "staging") {
		t.Errorf("dry-run should not re-point the marker to staging:\n%s", statusOut)
	}
}

// TestUseExistingSwitches verifies `use <existing>` (no -c) still pulls the
// existing environment's content into local — the checkout (not checkout -b)
// path is unchanged.
func TestUseExistingSwitches(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "V=X\n")
	t.Chdir(f["WT_A"])
	pushEnv(t, f["WT_A"], "prod", true)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "V=Y\n")
	pushEnv(t, f["WT_A"], "homo", true) // now on homo, local V=Y

	if _, err := execRoot(t, "use", "prod"); err != nil {
		t.Fatalf("use prod: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(f["WT_A"], ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "V=X") {
		t.Errorf("use prod should pull prod's content (V=X) into local, got:\n%s", got)
	}
	if strings.Contains(string(got), "V=Y") {
		t.Errorf("local still holds homo's value after switching to prod:\n%s", got)
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

// TestUseCreateCascadeCreatesNewEnvThenFansOut verifies `use -c <new>` still
// creates the environment (checkout -b from the invoking worktree) when the
// repo's cascade default is on, then fans the switch out to every worktree —
// rather than routing straight to the fan-out, which ignored -c and refused
// the not-yet-created env with a bare "unknown environment".
func TestUseCreateCascadeCreatesNewEnvThenFansOut(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "DB=prod\n")
	t.Chdir(f["WT_A"])

	cfg, err := config.Load(f["COMMON_DIR"])
	if err != nil {
		t.Fatal(err)
	}
	cfg.Cascade = true // opt the repo into cascade-by-default
	if err := config.Save(f["COMMON_DIR"], cfg); err != nil {
		t.Fatal(err)
	}

	if _, err := execRoot(t, "use", "-c", "prod"); err != nil { // no --cascade flag
		t.Fatalf("use -c prod under cascade default: %v", err)
	}

	// prod was created from wt-a's local content (checkout -b)...
	prod := readEnvVault(t, f["COMMON_DIR"], "prod")
	if prod["DB"] != "prod" {
		t.Errorf("prod vault = %v, want a snapshot of wt-a local (DB=prod)", prod)
	}
	// ...and the cascade then reached wt-b.
	got, _ := os.ReadFile(filepath.Join(f["WT_B"], ".env"))
	if !strings.Contains(string(got), "DB=prod") {
		t.Errorf("cascade did not switch wt-b to the new env:\n%s", got)
	}
}

// TestUseCreateCascadeDryRunCreatesNothing verifies `use -c <new> --cascade
// --dry-run` previews without creating the env or touching any worktree.
func TestUseCreateCascadeDryRunCreatesNothing(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "DB=prod\n")
	t.Chdir(f["WT_A"])

	if _, err := execRoot(t, "use", "-c", "prod", "--cascade", "--dry-run"); err != nil {
		t.Fatalf("use -c prod --cascade --dry-run: %v", err)
	}
	if vault.EnvExists(f["COMMON_DIR"], env.Name("prod")) {
		t.Error("dry-run should not have created prod")
	}
	if _, err := os.Stat(filepath.Join(f["WT_B"], ".env")); !os.IsNotExist(err) {
		t.Errorf("dry-run should not have written wt-b's .env, stat err = %v", err)
	}
}

// TestConfigSetGet verifies `config set` persists a value that `config get`
// then reads back.
func TestConfigSetGet(t *testing.T) {
	f := fixture(t)
	t.Chdir(f["WT_A"])
	if _, err := execRoot(t, "config", "set", "default_env", "prod"); err != nil {
		t.Fatalf("config set: %v", err)
	}
	out, err := execRoot(t, "config", "get", "default_env")
	if err != nil {
		t.Fatalf("config get: %v", err)
	}
	if strings.TrimSpace(out) != "prod" {
		t.Errorf("config get default_env = %q, want prod", out)
	}
}

// TestConfigListUnset verifies `config list` prints every key=value pair and
// `config unset` clears a key back to its default (visible in a later list).
func TestConfigListUnset(t *testing.T) {
	f := fixture(t)
	t.Chdir(f["WT_A"])
	if _, err := execRoot(t, "config", "set", "cascade", "true"); err != nil {
		t.Fatalf("config set cascade: %v", err)
	}
	out, err := execRoot(t, "config", "list")
	if err != nil {
		t.Fatalf("config list: %v", err)
	}
	for _, want := range []string{"env_file=", "default_env=", "cascade=true"} {
		if !strings.Contains(out, want) {
			t.Errorf("config list missing %q:\n%s", want, out)
		}
	}
	if _, err := execRoot(t, "config", "unset", "cascade"); err != nil {
		t.Fatalf("config unset cascade: %v", err)
	}
	out, err = execRoot(t, "config", "list")
	if err != nil {
		t.Fatalf("config list: %v", err)
	}
	if !strings.Contains(out, "cascade=false") {
		t.Errorf("config list after unset should show cascade=false:\n%s", out)
	}
}

// TestConfigGetUnknownKeyErrors verifies the config subcommands surface
// config.Get/Set/Unset's unknown-key errors rather than swallowing them.
func TestConfigGetUnknownKeyErrors(t *testing.T) {
	f := fixture(t)
	t.Chdir(f["WT_A"])
	if _, err := execRoot(t, "config", "get", "bogus"); err == nil {
		t.Error("config get bogus: want error for unknown key")
	}
}

// TestUseCascadeAbortReportsPartialProgressAndFailingWorktree verifies that
// when Pull returns a genuine (non-refusal) error partway through a cascade,
// UseCascade (a) still flushes the summary of worktrees already switched
// before the failure instead of discarding it, and (b) wraps the returned
// error with the failing worktree's path so the caller can tell which one
// broke the cascade.
//
// The genuine error is forced by corrupting wt-b's on-disk sync marker
// (envkeep.base) so its next Pull fails inside state.Load's JSON decode —
// a real I/O/parse failure, never one of Pull's guard refusals — rather than
// faking the abort.
func TestUseCascadeAbortReportsPartialProgressAndFailingWorktree(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "DB=prod\n")
	t.Chdir(f["WT_A"])
	pushEnv(t, f["WT_A"], "prod", true)
	mustPull(t, f["WT_B"]) // gives wt-b a real marker file to corrupt

	gitDir, err := git.Dir(f["WT_B"])
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, state.Path(gitDir), "not valid json")

	var buf bytes.Buffer
	ctx, err := Resolve(f["WT_A"], "", "prod")
	if err != nil {
		t.Fatal(err)
	}
	err = UseCascade(ctx, &buf, false)
	if err == nil {
		t.Fatal("UseCascade: want an error from wt-b's corrupted marker, got nil")
	}
	if !strings.Contains(err.Error(), f["WT_B"]) {
		t.Errorf("abort error should name the failing worktree %q: %v", f["WT_B"], err)
	}
	if !strings.Contains(buf.String(), f["WT_A"]) {
		t.Errorf("abort should still report the worktree switched before the failure:\n%s", buf.String())
	}
}
