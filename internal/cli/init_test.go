package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/carvalhosauro/envkeep/internal/config"
	"github.com/carvalhosauro/envkeep/internal/vault"
)

func mustInit(t *testing.T, cwd, envFile string) string {
	t.Helper()
	var b bytes.Buffer
	if err := Init(&b, cwd, envFile, false); err != nil {
		t.Fatalf("Init(%s): %v\n%s", cwd, err, b.String())
	}
	return b.String()
}

func TestInitFresh(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "KEY=value\n")

	out := mustInit(t, f["WT_A"], "")

	if _, err := os.Stat(config.Path(f["COMMON_DIR"])); err != nil {
		t.Errorf("config not written: %v", err)
	}
	cfg, err := config.Load(f["COMMON_DIR"])
	if err != nil || cfg.EnvFile != ".env" {
		t.Errorf("config.Load = %+v, %v; want env_file=.env", cfg, err)
	}
	got, err := readVaultEnv(t, f).Read()
	if err != nil {
		t.Fatal(err)
	}
	if got["KEY"] != "value" {
		t.Errorf("vault not seeded from local .env: %v", got)
	}
	if !strings.Contains(out, "initialized envkeep") {
		t.Errorf("output missing 'initialized envkeep':\n%s", out)
	}

	// The first push must set the marker, so status settles to clean at once.
	var b bytes.Buffer
	if err := Status(&b, f["WT_A"], "", ""); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(b.String(), "clean") {
		t.Errorf("status after init not clean:\n%s", b.String())
	}
}

func TestInitEnvFileFlag(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env.local"), "KEY=value\n")

	mustInit(t, f["WT_A"], ".env.local")

	cfg, err := config.Load(f["COMMON_DIR"])
	if err != nil || cfg.EnvFile != ".env.local" {
		t.Errorf("config.Load = %+v, %v; want env_file=.env.local", cfg, err)
	}
	got, err := vault.NewFileStore(vault.Path(f["COMMON_DIR"], ".env.local")).Read()
	if err != nil || got["KEY"] != "value" {
		t.Errorf("vault for .env.local not seeded: %v, %v", got, err)
	}
}

func TestInitTwiceDoesNotClobber(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "KEY=v1\n")
	mustInit(t, f["WT_A"], "")

	// A local edit after init must not be pushed by a re-run.
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "KEY=v2\n")
	out := mustInit(t, f["WT_A"], "")

	if !strings.Contains(out, "already initialized") {
		t.Errorf("second init output = %q, want 'already initialized'", out)
	}
	got, err := readVaultEnv(t, f).Read()
	if err != nil {
		t.Fatal(err)
	}
	if got["KEY"] != "v1" {
		t.Errorf("second init clobbered the vault: %v", got)
	}
}

func TestInitRefusesToClobberExistingVault(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "KEY=v1\n")
	mustPush(t, f["WT_A"]) // vault seeded the manual way; no config written

	writeFile(t, filepath.Join(f["WT_A"], ".env"), "KEY=v2\n")
	out := mustInit(t, f["WT_A"], "")

	if !strings.Contains(out, "already initialized") {
		t.Errorf("init over an existing vault = %q, want 'already initialized'", out)
	}
	got, err := readVaultEnv(t, f).Read()
	if err != nil {
		t.Fatal(err)
	}
	if got["KEY"] != "v1" {
		t.Errorf("init clobbered a pre-existing vault: %v", got)
	}
}

func TestInitEnvFileMismatchRefused(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "KEY=v1\n")
	mustInit(t, f["WT_A"], "")

	err := Init(&bytes.Buffer{}, f["WT_A"], ".env.local", false)
	if err == nil || !strings.Contains(err.Error(), "config set env_file") {
		t.Errorf("Init with mismatched --env-file = %v, want redirect to 'config set env_file'", err)
	}
}

func TestInitNoLocalEnvWritesNothing(t *testing.T) {
	f := fixture(t)

	err := Init(&bytes.Buffer{}, f["WT_A"], "", false)

	if err == nil || !strings.Contains(err.Error(), ".env") {
		t.Fatalf("Init without a local env = %v, want error naming .env", err)
	}
	if _, serr := os.Stat(config.Path(f["COMMON_DIR"])); serr == nil {
		t.Error("failed init left a config behind")
	}
}

func TestInitDryRunWritesNothing(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "KEY=v1\n")

	var b bytes.Buffer
	if err := Init(&b, f["WT_A"], "", true); err != nil {
		t.Fatalf("Init dry-run: %v\n%s", err, b.String())
	}

	if _, err := os.Stat(config.Path(f["COMMON_DIR"])); err == nil {
		t.Error("dry-run wrote the config")
	}
	if _, err := readVaultEnv(t, f).Read(); !errors.Is(err, vault.ErrNotFound) {
		t.Errorf("dry-run wrote the vault (read err = %v, want ErrNotFound)", err)
	}
}
