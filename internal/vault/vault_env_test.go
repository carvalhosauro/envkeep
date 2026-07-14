package vault

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPathForEnv(t *testing.T) {
	common := filepath.FromSlash("/repo/.git")
	legacy := filepath.Join(common, "envkeep", "vault", ".env")
	if got := PathForEnv(common, "", ".env"); got != legacy {
		t.Errorf("PathForEnv unnamed = %s, want %s", got, legacy)
	}
	named := filepath.Join(common, "envkeep", "vault", "prod", ".env")
	if got := PathForEnv(common, "prod", ".env"); got != named {
		t.Errorf("PathForEnv prod = %s, want %s", got, named)
	}
	if Path(common, ".env") != PathForEnv(common, "", ".env") {
		t.Error("Path must equal PathForEnv with the unnamed environment")
	}
}

func writeVaultFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("A=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestEnvironmentsAndExists(t *testing.T) {
	common := t.TempDir()

	// No vault dir yet: no environments, and only the unnamed env "exists".
	if envs, err := Environments(common); err != nil || len(envs) != 0 {
		t.Fatalf("Environments (fresh) = %v, %v; want empty", envs, err)
	}
	if EnvExists(common, "prod") {
		t.Error("prod must not exist yet")
	}
	if !EnvExists(common, "") {
		t.Error("the unnamed environment always exists")
	}

	// Two env dirs plus a legacy flat vault file that must be ignored.
	writeVaultFile(t, PathForEnv(common, "prod", ".env"))
	writeVaultFile(t, PathForEnv(common, "homo", ".env"))
	writeVaultFile(t, PathForEnv(common, "", ".env")) // flat file, not a dir

	envs, err := Environments(common)
	if err != nil {
		t.Fatal(err)
	}
	if len(envs) != 2 || envs[0] != "homo" || envs[1] != "prod" {
		t.Errorf("Environments = %v, want sorted [homo prod]", envs)
	}
	if !EnvExists(common, "prod") || !EnvExists(common, "homo") {
		t.Error("prod and homo must exist")
	}
	if EnvExists(common, "staging") {
		t.Error("staging must not exist")
	}
}

func TestValidEnvName(t *testing.T) {
	for _, ok := range []string{"prod", "homo-1", "a.b_c", "Dev2"} {
		if err := ValidEnvName(ok); err != nil {
			t.Errorf("ValidEnvName(%q) = %v, want nil", ok, err)
		}
	}
	for _, bad := range []string{"", ".", "..", "shared", "_base", "a/b", "a b", "x*", "up/../x"} {
		if err := ValidEnvName(bad); err == nil {
			t.Errorf("ValidEnvName(%q) = nil, want error", bad)
		}
	}
}
