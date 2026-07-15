package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/carvalhosauro/envkeep/internal/buildinfo"
)

// execRoot runs the root command with args, capturing combined stdout+stderr.
func execRoot(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	root := newRootCmd()
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(args)
	err := root.Execute()
	return out.String(), err
}

func TestRootVersion(t *testing.T) {
	out, err := execRoot(t, "version")
	if err != nil {
		t.Fatalf("version: %v", err)
	}
	if !strings.Contains(out, "envkeep") || !strings.Contains(out, buildinfo.Version) {
		t.Errorf("version output = %q, want it to contain envkeep + %q", out, buildinfo.Version)
	}
}

// TestRootVersionAliases verifies the pre-cobra `version`/`--version`/`-v`
// trio all still print byte-identical "envkeep <version>\n" output, matching
// what the old fmt.Println("envkeep", buildinfo.Version) produced.
func TestRootVersionAliases(t *testing.T) {
	want := "envkeep " + buildinfo.Version + "\n"

	cases := []struct {
		name string
		args []string
	}{
		{"subcommand", []string{"version"}},
		{"long-flag", []string{"--version"}},
		{"short-flag", []string{"-v"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := execRoot(t, tc.args...)
			if err != nil {
				t.Fatalf("%v: %v", tc.args, err)
			}
			if out != want {
				t.Errorf("%v output = %q, want %q", tc.args, out, want)
			}
		})
	}
}

// TestExecute exercises Execute()'s success and error branches directly,
// since it drives os.Args rather than accepting an injected arg slice.
func TestExecute(t *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	t.Run("success", func(t *testing.T) {
		os.Args = []string{"envkeep", "version"}
		if code := Execute(); code != exitOK {
			t.Errorf("Execute() = %d, want %d", code, exitOK)
		}
	})

	t.Run("error", func(t *testing.T) {
		os.Args = []string{"envkeep", "definitely-not-a-command"}
		if code := Execute(); code != exitError {
			t.Errorf("Execute() = %d, want %d", code, exitError)
		}
	})
}

// TestRootStatusPortsBehavior verifies the cobra `status` subcommand still
// reports "clean" for a worktree right after its .env is pushed to the vault.
// push isn't ported to cobra until A3, so the vault is seeded here via the
// existing mustPush function-level helper instead of execRoot(t, "push"),
// keeping A2 independently testable.
func TestRootStatusPortsBehavior(t *testing.T) {
	f := fixture(t)
	writeFile(t, filepath.Join(f["WT_A"], ".env"), "KEY=value\n")
	t.Chdir(f["WT_A"])

	mustPush(t, f["WT_A"])

	out, err := execRoot(t, "status")
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !strings.Contains(out, "clean") {
		t.Errorf("status output missing 'clean':\n%s", out)
	}
}

// TestProcessCwd asserts processCwd is just os.Getwd, wired for later
// subcommands (status/push/pull/check) in A2-A5.
func TestProcessCwd(t *testing.T) {
	got, err := processCwd()
	if err != nil {
		t.Fatalf("processCwd() error = %v", err)
	}
	want, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd() error = %v", err)
	}
	if got != want {
		t.Errorf("processCwd() = %q, want %q", got, want)
	}
}
