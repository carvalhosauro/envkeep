// Package git wraps the handful of git queries envkeep needs, always shelling
// out to the real git binary (envkeep's behavior depends on real
// git-common-dir resolution, so this layer is not mocked — see
// docs/DECISIONS.md D18).
package git

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Worktree is one entry from `git worktree list`.
type Worktree struct {
	Path     string // absolute path to the worktree (or to the bare repo)
	Head     string // commit the worktree points at ("" for a bare entry)
	Branch   string // short branch name ("" if detached or bare)
	Bare     bool
	Detached bool
}

// Paths bundles the three repo locations a command needs.
type Paths struct {
	CommonDir string // shared git dir (vault lives here)
	GitDir    string // per-worktree git dir (sync marker lives here)
	Toplevel  string // worktree root (the .env lives here)
}

// Locate resolves all three paths for the worktree containing dir in a single
// git invocation, rather than three. This keeps the per-prompt hook check cheap
// (git process spawns dominate its cost). All paths are absolute (D13).
func Locate(dir string) (Paths, error) {
	const want = 3
	if out, err := run(dir, "rev-parse", "--path-format=absolute",
		"--git-common-dir", "--git-dir", "--show-toplevel"); err == nil {
		if lines := strings.Split(out, "\n"); len(lines) == want {
			return Paths{CommonDir: lines[0], GitDir: lines[1], Toplevel: lines[2]}, nil
		}
	}
	// Fallback for older git without --path-format: resolve relative paths.
	out, err := run(dir, "rev-parse", "--git-common-dir", "--git-dir", "--show-toplevel")
	if err != nil {
		return Paths{}, err
	}
	lines := strings.Split(out, "\n")
	if len(lines) != want {
		return Paths{}, fmt.Errorf("git rev-parse: expected %d paths, got %q", want, out)
	}
	return Paths{
		CommonDir: absUnder(dir, lines[0]),
		GitDir:    absUnder(dir, lines[1]),
		Toplevel:  absUnder(dir, lines[2]),
	}, nil
}

// CommonDir returns the absolute shared git directory for the repo containing
// dir. Every worktree of a repo — including linked worktrees and the bare-repo
// (.bare/) layout — resolves to the same value; it is where envkeep stores the
// vault (D3). The path is always absolute (D13).
func CommonDir(dir string) (string, error) {
	// Preferred: git 2.31+ can emit an absolute path directly.
	if out, err := run(dir, "rev-parse", "--path-format=absolute", "--git-common-dir"); err == nil {
		return out, nil
	}
	// Fallback for older git: the raw value may be relative to dir.
	out, err := run(dir, "rev-parse", "--git-common-dir")
	if err != nil {
		return "", err
	}
	return absUnder(dir, out), nil
}

// Dir returns the absolute per-worktree git directory for the worktree
// containing dir (e.g. .git/worktrees/<name> for a linked worktree, .git for
// the main one). This is where a worktree's private state — the sync base
// marker — belongs (D5).
func Dir(dir string) (string, error) {
	if out, err := run(dir, "rev-parse", "--path-format=absolute", "--git-dir"); err == nil {
		return out, nil
	}
	out, err := run(dir, "rev-parse", "--git-dir")
	if err != nil {
		return "", err
	}
	return absUnder(dir, out), nil
}

// Toplevel returns the absolute root of the worktree containing dir — the
// directory the tracked .env lives in.
func Toplevel(dir string) (string, error) {
	return run(dir, "rev-parse", "--show-toplevel")
}

// Worktrees lists every worktree of the repo containing dir, queried live so it
// is never stale (D4).
func Worktrees(dir string) ([]Worktree, error) {
	out, err := run(dir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	return parseWorktrees(out), nil
}

// parseWorktrees decodes the --porcelain format: blank-line-separated blocks of
// "key value" lines, plus bare "bare"/"detached" flag lines.
func parseWorktrees(out string) []Worktree {
	var wts []Worktree
	var cur *Worktree
	flush := func() {
		if cur != nil {
			wts = append(wts, *cur)
			cur = nil
		}
	}
	for line := range strings.SplitSeq(out, "\n") {
		if line == "" {
			flush()
			continue
		}
		key, val, _ := strings.Cut(line, " ")
		switch key {
		case "worktree":
			cur = &Worktree{Path: val}
		case "HEAD":
			if cur != nil {
				cur.Head = val
			}
		case "branch":
			if cur != nil {
				cur.Branch = strings.TrimPrefix(val, "refs/heads/")
			}
		case "bare":
			if cur != nil {
				cur.Bare = true
			}
		case "detached":
			if cur != nil {
				cur.Detached = true
			}
		}
	}
	flush()
	return wts
}

func absUnder(base, p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Clean(filepath.Join(base, p))
}

// sanitizedEnv strips inherited GIT_* variables that would override path-based
// repo discovery. Notably, git hooks export GIT_DIR/GIT_WORK_TREE/…, which any
// nested git command would otherwise pick up instead of honoring cmd.Dir.
// envkeep always resolves the repo from the directory it is pointed at.
func sanitizedEnv() []string {
	skip := map[string]bool{
		"GIT_DIR":        true,
		"GIT_WORK_TREE":  true,
		"GIT_COMMON_DIR": true,
		"GIT_INDEX_FILE": true,
		"GIT_PREFIX":     true,
		"GIT_NAMESPACE":  true,
	}
	src := os.Environ()
	out := make([]string, 0, len(src))
	for _, kv := range src {
		name, _, _ := strings.Cut(kv, "=")
		if skip[name] {
			continue
		}
		out = append(out, kv)
	}
	return out
}

func run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = sanitizedEnv()
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}
