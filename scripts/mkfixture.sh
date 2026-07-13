#!/usr/bin/env bash
# mkfixture.sh — build a throwaway git repo with linked worktrees for envkeep
# tests. Uses REAL git (envkeep depends on real git-common-dir behaviour, so
# mocking git would test nothing — see docs/DECISIONS.md D18).
#
# Usage:
#   mkfixture.sh [--bare] <dest-dir>
#     --bare   bare repo in <dest>/.bare with worktrees alongside (the layout
#              some people use: .bare/ + a .git file pointing at it). Otherwise
#              a normal repo whose primary worktree is <dest>/main.
#
# On success prints KEY=VALUE lines on stdout for callers to parse:
#   ROOT, MODE, MAIN, WT_A, WT_B, COMMON_DIR
set -euo pipefail

# Be hermetic: a caller inside a git hook (or another git command) exports
# GIT_DIR/GIT_WORK_TREE/… which would otherwise hijack the git commands below
# and point them at the wrong repo. Clear them so this fixture is self-contained.
unset GIT_DIR GIT_WORK_TREE GIT_COMMON_DIR GIT_INDEX_FILE GIT_PREFIX GIT_NAMESPACE

# Deterministic, isolated git identity — never touch the user's global config.
git_() {
	git -c user.email=envkeep@test.local \
		-c user.name=envkeep-test \
		-c commit.gpgsign=false \
		-c init.defaultBranch=main \
		"$@"
}

seed_commit() { # <worktree-dir>
	printf 'seed\n' >"$1/README"
	git_ -C "$1" add README
	git_ -C "$1" commit -q -m "chore: seed"
}

mode=normal
if [ "${1:-}" = "--bare" ]; then
	mode=bare
	shift
fi
dest="${1:?usage: mkfixture.sh [--bare] <dest-dir>}"

mkdir -p "$dest"
dest="$(cd "$dest" && pwd)"

if [ "$mode" = "normal" ]; then
	git_ init -q -b main "$dest/main"
	seed_commit "$dest/main"
	git_ -C "$dest/main" worktree add -q "$dest/wt-a" -b wt-a
	git_ -C "$dest/main" worktree add -q "$dest/wt-b" -b wt-b
else
	# Build a seed repo, clone it bare, then attach worktrees to the bare repo.
	# This is robust across git versions (no reliance on --orphan semantics).
	seed="$dest/.seed"
	git_ init -q -b main "$seed"
	seed_commit "$seed"
	git_ clone -q --bare "$seed" "$dest/.bare"
	rm -rf "$seed"
	# Faithful ".bare/ + top-level .git pointer" layout.
	printf 'gitdir: ./.bare\n' >"$dest/.git"
	git_ -C "$dest/.bare" worktree add -q "$dest/main" main
	git_ -C "$dest/.bare" worktree add -q "$dest/wt-a" -b wt-a
	git_ -C "$dest/.bare" worktree add -q "$dest/wt-b" -b wt-b
fi

common_dir="$(git -C "$dest/wt-a" rev-parse --path-format=absolute --git-common-dir)"

cat <<EOF
ROOT=$dest
MODE=$mode
MAIN=$dest/main
WT_A=$dest/wt-a
WT_B=$dest/wt-b
COMMON_DIR=$common_dir
EOF
