package cli

import (
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
