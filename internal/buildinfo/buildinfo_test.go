package buildinfo

import "testing"

func TestVersionNotEmpty(t *testing.T) {
	if Version == "" {
		t.Fatal("Version must never be empty; expected a fallback like \"dev\"")
	}
}
