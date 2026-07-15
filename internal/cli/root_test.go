package cli

import (
	"bytes"
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
