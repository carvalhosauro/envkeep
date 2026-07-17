package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/carvalhosauro/envkeep/internal/config"
	"github.com/carvalhosauro/envkeep/internal/env"
	"github.com/carvalhosauro/envkeep/internal/vault"
)

func pushEnv(t *testing.T, cwd, env string, create bool) string {
	t.Helper()
	var b bytes.Buffer
	if err := Push(&b, cwd, "", env, create, false, false); err != nil {
		t.Fatalf("push --env %s (create=%v): %v\n%s", env, create, err, b.String())
	}
	return b.String()
}

func readEnvVault(t *testing.T, common, name string) map[string]string {
	t.Helper()
	m, err := vault.NewFileStore(vault.PathForEnv(common, env.Name(name), ".env")).Read()
	if err != nil {
		t.Fatalf("read vault %q: %v", name, err)
	}
	return m
}

// A key holds different values per environment, with no cross-environment leak.
func TestPerEnvironmentValues(t *testing.T) {
	f := fixture(t)
	common := f["COMMON_DIR"]

	writeFile(t, filepath.Join(f["WT_A"], ".env"), "DB=prod-db\nSHARED=x\n")
	pushEnv(t, f["WT_A"], "prod", true)

	writeFile(t, filepath.Join(f["WT_A"], ".env"), "DB=homo-db\nSHARED=x\n")
	pushEnv(t, f["WT_A"], "homo", true)

	prod := readEnvVault(t, common, "prod")
	homo := readEnvVault(t, common, "homo")
	if prod["DB"] != "prod-db" {
		t.Errorf("prod DB = %q, want prod-db", prod["DB"])
	}
	if homo["DB"] != "homo-db" {
		t.Errorf("homo DB = %q, want homo-db (prod's value must not leak)", homo["DB"])
	}
	if prod["SHARED"] != "x" || homo["SHARED"] != "x" {
		t.Errorf("SHARED should be present in both: prod=%v homo=%v", prod, homo)
	}
}

func TestUnknownEnvironmentRefusedWithoutCreate(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "K=1\n")
	err := Push(&bytes.Buffer{}, f["WT_A"], "", "ghost", false, false, false)
	if err == nil || !strings.Contains(err.Error(), "unknown environment") {
		t.Errorf("Push --env ghost error = %v, want an 'unknown environment' refusal", err)
	}
}

// Switching a worktree to another environment (a re-point) swaps the local file
// to that environment's values.
func TestSwitchEnvironmentRepoints(t *testing.T) {
	f := fixture(t)

	writeFile(t, filepath.Join(f["WT_A"], ".env"), "DB=prod\n")
	pushEnv(t, f["WT_A"], "prod", true)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "DB=homo\n")
	pushEnv(t, f["WT_A"], "homo", true) // wt-a now active on homo

	var b bytes.Buffer
	if err := Pull(&b, f["WT_A"], "", "prod", false, false); err != nil {
		t.Fatalf("re-point to prod: %v\n%s", err, b.String())
	}
	got, err := os.ReadFile(filepath.Join(f["WT_A"], ".env"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "DB=prod") {
		t.Errorf("re-point did not swap local to prod values:\n%s", got)
	}
}

// A re-point must not silently discard unpushed local edits in the current
// environment (E4).
func TestRepointGuardsUnpushedEdits(t *testing.T) {
	f := fixture(t)

	writeFile(t, filepath.Join(f["WT_A"], ".env"), "DB=prod\n")
	pushEnv(t, f["WT_A"], "prod", true)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "DB=homo\n")
	pushEnv(t, f["WT_A"], "homo", true)

	// Switch cleanly to prod, then edit locally without pushing.
	mustCmdEnv(t, f["WT_A"], "prod")
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "DB=prod-EDITED\n")

	err := Pull(&bytes.Buffer{}, f["WT_A"], "", "homo", false, false)
	if err == nil || !strings.Contains(err.Error(), "not pushed") {
		t.Errorf("switch with unpushed edits: err = %v, want a refusal (E4)", err)
	}
}

// TestRepointRefusalMessageUnchangedAndClassifiable proves wrapping the E4
// re-point guard with the ErrRefused sentinel (added for cascade, D28/C1) does
// not change what a direct `envkeep use`/`envkeep pull` caller sees: the exact
// refusal message is preserved, while errors.Is(err, ErrRefused) now lets
// cascade classify it as a skip instead of clobbering the worktree.
func TestRepointRefusalMessageUnchangedAndClassifiable(t *testing.T) {
	f := fixture(t)

	writeFile(t, filepath.Join(f["WT_A"], ".env"), "DB=prod\n")
	pushEnv(t, f["WT_A"], "prod", true)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "DB=homo\n")
	pushEnv(t, f["WT_A"], "homo", true)

	mustCmdEnv(t, f["WT_A"], "prod")
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "DB=prod-EDITED\n")

	err := Pull(&bytes.Buffer{}, f["WT_A"], "", "homo", false, false)
	const want = `local has changes not pushed to environment "prod"; push or discard before switching to "homo"`
	if err == nil || err.Error() != want {
		t.Fatalf("Pull error = %v, want byte-identical %q", err, want)
	}
	if !errors.Is(err, ErrRefused) {
		t.Errorf("errors.Is(err, ErrRefused) = false, want true")
	}
}

func mustCmdEnv(t *testing.T, cwd, env string) {
	t.Helper()
	if err := Pull(&bytes.Buffer{}, cwd, "", env, false, false); err != nil {
		t.Fatalf("pull --env %s: %v", env, err)
	}
}

// The first environment created migrates the legacy flat vault into it and sets
// default_env, leaving no stray flat vault (D27).
func TestLegacyMigrationOnFirstCreate(t *testing.T) {
	f := fixture(t)
	common := f["COMMON_DIR"]

	writeFile(t, filepath.Join(f["WT_A"], ".env"), "A=1\nB=2\n")
	mustPush(t, f["WT_A"]) // legacy flat vault, no environments yet
	if envs, _ := vault.Environments(common); len(envs) != 0 {
		t.Fatalf("expected no environments before adoption, got %v", envs)
	}

	pushEnv(t, f["WT_A"], "prod", true) // first env → migrate + set default

	if _, err := os.Stat(vault.PathForEnv(common, "", ".env")); !os.IsNotExist(err) {
		t.Error("legacy flat vault must be gone after migration")
	}
	prod := readEnvVault(t, common, "prod")
	if prod["A"] != "1" || prod["B"] != "2" {
		t.Errorf("prod vault did not inherit the legacy values: %v", prod)
	}
	cfg, err := config.Load(common)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultEnv != "prod" {
		t.Errorf("default_env = %q, want prod", cfg.DefaultEnv)
	}
}

// A repo that never adopts environments keeps the legacy flat layout untouched
// (R3): no environment dirs, vault at the flat path, no default_env written.
func TestLegacyRepoUnchanged(t *testing.T) {
	f := fixture(t)
	common := f["COMMON_DIR"]

	writeFile(t, filepath.Join(f["WT_A"], ".env"), "A=1\n")
	mustPush(t, f["WT_A"])

	if envs, _ := vault.Environments(common); len(envs) != 0 {
		t.Errorf("legacy repo must have no environment dirs, got %v", envs)
	}
	if _, err := os.Stat(vault.PathForEnv(common, "", ".env")); err != nil {
		t.Errorf("legacy flat vault must exist: %v", err)
	}
	var b bytes.Buffer
	if err := Status(&b, f["WT_A"], "", ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "clean") {
		t.Errorf("legacy worktree should read clean:\n%s", b.String())
	}
	if data, err := os.ReadFile(config.Path(common)); err == nil && strings.Contains(string(data), "default_env") {
		t.Errorf("legacy repo must not have default_env in config:\n%s", data)
	}
}

// A push targeting an env other than the worktree's own has no 3-way base, so
// it must not silently overwrite keys whose value differs in the target vault
// (#20); --force is the explicit opt-in.
func TestCrossEnvPushRefusesOverwriteWithoutForce(t *testing.T) {
	f := fixture(t)
	common := f["COMMON_DIR"]

	writeFile(t, filepath.Join(f["WT_A"], ".env"), "DB=prod-db\n")
	pushEnv(t, f["WT_A"], "prod", true) // wt-a active on prod
	writeFile(t, filepath.Join(f["WT_B"], ".env"), "DB=homo-db\n")
	pushEnv(t, f["WT_B"], "homo", true) // homo holds DB=homo-db

	var b bytes.Buffer
	err := Push(&b, f["WT_A"], "", "homo", false, false, false)
	if err == nil || !strings.Contains(err.Error(), "--force") {
		t.Fatalf("cross-env push = %v, want refusal mentioning --force", err)
	}
	if !strings.Contains(b.String(), "DB") || strings.Contains(b.String(), "prod-db") {
		t.Errorf("refusal output = %q, want key name without values", b.String())
	}
	if got := readEnvVault(t, common, "homo")["DB"]; got != "homo-db" {
		t.Errorf("homo DB = %q, want homo-db untouched after refusal", got)
	}

	if err := Push(&bytes.Buffer{}, f["WT_A"], "", "homo", false, false, true); err != nil {
		t.Fatalf("push --force: %v", err)
	}
	if got := readEnvVault(t, common, "homo")["DB"]; got != "prod-db" {
		t.Errorf("homo DB after --force = %q, want prod-db", got)
	}
}

// A cross-env push that only adds keys (no differing values) needs no --force.
func TestCrossEnvPushAddOnlyNeedsNoForce(t *testing.T) {
	f := fixture(t)
	common := f["COMMON_DIR"]

	writeFile(t, filepath.Join(f["WT_A"], ".env"), "DB=prod-db\n")
	pushEnv(t, f["WT_A"], "prod", true)
	writeFile(t, filepath.Join(f["WT_B"], ".env"), "OTHER=x\n")
	pushEnv(t, f["WT_B"], "homo", true)

	writeFile(t, filepath.Join(f["WT_A"], ".env"), "NEW=1\n")
	if err := Push(&bytes.Buffer{}, f["WT_A"], "", "homo", false, false, false); err != nil {
		t.Fatalf("add-only cross-env push: %v", err)
	}
	homo := readEnvVault(t, common, "homo")
	if homo["NEW"] != "1" || homo["OTHER"] != "x" {
		t.Errorf("homo = %v, want NEW=1 merged and OTHER=x kept", homo)
	}
}

// A default_env pre-set before any environment exists necessarily dangles;
// first-env adoption must re-point it at the env actually created (#21).
func TestFirstEnvAdoptionOverridesDanglingDefaultEnv(t *testing.T) {
	f := fixture(t)
	common := f["COMMON_DIR"]
	if err := config.Set(common, "default_env", "prod"); err != nil {
		t.Fatal(err)
	}

	writeFile(t, filepath.Join(f["WT_A"], ".env"), "K=1\n")
	pushEnv(t, f["WT_A"], "homo", true)

	cfg, err := config.Load(common)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.DefaultEnv != env.Name("homo") {
		t.Errorf("default_env = %q, want homo (pre-set prod dangles)", cfg.DefaultEnv)
	}
}
