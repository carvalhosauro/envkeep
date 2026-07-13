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
	// chpwd trigger, the binary call, and the shell-side mtime guard (D7 layer 1):
	// a zstat read plus the per-directory clean cache that lets it skip the spawn.
	for _, want := range []string{"add-zsh-hook chpwd", "envkeep check", "zstat", "_envkeep_mtime", "_envkeep_clean"} {
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
	// PROMPT_COMMAND trigger, the $PWD guard, the binary call, and the shell-side
	// mtime guard (bash 4+ associative-array cache with a 3.x fallback).
	for _, want := range []string{"PROMPT_COMMAND", "envkeep check", "$PWD", "BASH_VERSINFO", "_ENVKEEP_MT"} {
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
