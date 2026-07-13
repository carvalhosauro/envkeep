package hook

import (
	"strings"
	"testing"
)

func TestSnippetZsh(t *testing.T) {
	s, err := Snippet("zsh")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"add-zsh-hook chpwd", "envkeep check"} {
		if !strings.Contains(s, want) {
			t.Errorf("zsh snippet missing %q", want)
		}
	}
}

func TestSnippetBash(t *testing.T) {
	s, err := Snippet("bash")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"PROMPT_COMMAND", "envkeep check", "$PWD"} {
		if !strings.Contains(s, want) {
			t.Errorf("bash snippet missing %q", want)
		}
	}
}

func TestSnippetUnknownShell(t *testing.T) {
	if _, err := Snippet("fish"); err == nil {
		t.Error("Snippet(fish): want error, got nil")
	}
}
