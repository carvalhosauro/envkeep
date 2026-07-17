# envkeep

[![ci](https://github.com/carvalhosauro/envkeep/actions/workflows/ci.yml/badge.svg)](https://github.com/carvalhosauro/envkeep/actions/workflows/ci.yml)
[![release](https://img.shields.io/github/v/release/carvalhosauro/envkeep)](https://github.com/carvalhosauro/envkeep/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/carvalhosauro/envkeep)](https://goreportcard.com/report/github.com/carvalhosauro/envkeep)
[![license](https://img.shields.io/github/license/carvalhosauro/envkeep)](LICENSE)

Keep `.env` files in sync across the git worktrees of one repository — and
switch each worktree between named environments (`dev`, `staging`, …).

> Status: **v1 (MVP) complete** — `status`/`push`/`pull`/`check`, named
> environments (`envs`/`use`/`rm`) and the shell hook work end-to-end. See
> [`docs/STATUS.md`](docs/STATUS.md) for detail. AI agents: start at
> [`AGENTS.md`](AGENTS.md).

![envkeep demo](demo/demo.gif)

## The problem

`.env` is gitignored, so git does not propagate it between worktrees. Two
symptoms follow:

1. **A new worktree is born without `.env`.** (Already solved by many tools:
   copy-env, worktree-env-sync, envi, worktrunk.)
2. **Day to day, once worktrees already exist**, you update a variable in one
   worktree and forget to propagate it to the others. You lose track of which
   worktree holds the newest value. **This is the pain envkeep targets** — it
   has no good solution in existing tools, which almost all focus on a one-time
   copy at creation.

## The idea

- The git repository is the shared source of truth. Every worktree of a repo
  sees the same common `.git` (`git rev-parse --git-common-dir` works from any
  worktree, including linked ones and bare-repo `.bare/` layouts).
- A flat-file **vault** (`KEY=VALUE`) holds the shared env values, stored inside
  that common dir. No external registry, no database. Inspectable with `cat` and
  `git diff`.
- Per worktree, envkeep tells you whether the local `.env` is **synced**,
  **ahead**, **behind**, **conflicting**, or **absent** relative to the vault.
- `push` sends the local `.env` to the vault; `pull` writes the vault to the
  local `.env`. Order and comments in `.env` are preserved on write.
- A **per-worktree override file** (gitignored) holds values that must differ
  between worktrees — the common case is the dev-server port when you run
  several worktrees in parallel. Override keys are never pushed to the vault and
  are re-applied on every pull.
- A **shell hook** (`chpwd` / cd-trap) warns you discreetly when you enter a
  worktree whose `.env` has drifted, so you don't have to remember to run the
  command.

## Demo

**Switch environments.** `envkeep use` re-points the worktree and rewrites
`.env` from the target environment's vault — order and comments preserved.

![envkeep use](demo/use.gif)

**Create an environment like a branch.** `use -c` snapshots the current `.env`
into a new environment and switches to it — `git checkout -b` for env files.

![envkeep use -c](demo/create.gif)

**Switch every worktree at once.** `--cascade` fans the switch out to the whole
repo; a worktree with unpushed edits is skipped and reported, never clobbered.

![envkeep use --cascade](demo/cascade.gif)

**Propagate a change.** Rotate a key in one worktree and `push`; every other
worktree shows `behind` until a `pull` catches it up.

![push and pull between worktrees](demo/drift.gif)

**Or let the hook remember for you.** `cd` into a drifted worktree and get a
one-line warning — no command to remember.

![shell hook warning](demo/hook.gif)

## Install

**No Go needed** — download a prebuilt binary (Linux/macOS, amd64/arm64):

```sh
curl -sSfL https://raw.githubusercontent.com/carvalhosauro/envkeep/main/install.sh | sh
```

Installs to `~/.local/bin` (override with `ENVKEEP_INSTALL_DIR`). Requires a
published GitHub Release (see Releasing).

**With Go**, build from source:

```sh
go install github.com/carvalhosauro/envkeep/cmd/envkeep@latest
```

`envkeep version` reports the installed version. From a clone (development):
`make install` (version stamped from `git describe`) or `make build` → `./bin`.

## Releasing

Releases are automated with [goreleaser](https://goreleaser.com). Push a
semver tag and GitHub Actions builds the cross-platform binaries and publishes
the Release:

```sh
git tag v0.1.0
git push origin v0.1.0
```

Locally: `make release-check` validates the config; `make snapshot` dry-runs a
build without publishing.

## Commands

```
envkeep status                 # per-worktree: active env + clean / ahead / behind / diverged / conflict / absent
envkeep push [--env <env>]     # local .env -> vault (union merge; refuses on conflict)
envkeep pull [--env <env>]     # vault -> local .env (preserves order/comments, reapplies override)
envkeep envs                   # list the repo's environments; * marks the default
envkeep use <env> [-c]         # switch this worktree's environment (-c creates it)
envkeep use <env> --cascade    # switch every worktree of the repo at once
envkeep rm <env>               # delete an environment (refuses while a worktree is on it)
envkeep check                  # quiet drift check for the current worktree (used by the hook)
envkeep hook zsh|bash          # print the shell snippet to source in .zshrc / .bashrc
envkeep config get|set|unset|list   # repo config: env_file, default_env, cascade
```

All state-changing commands take `--dry-run` to preview without writing.

### Environments

The vault holds any number of named environments (`dev`, `staging`, …) —
parallel snapshots of the same env file. Each worktree points at one of them
(its *active env*); `status`, `push`, and `pull` operate against that env
unless `--env` says otherwise.

```sh
envkeep use -c dev              # first env: snapshot .env into 'dev' and point here
envkeep use staging             # switch: rewrites .env from staging's vault
envkeep use -c preview          # like `git checkout -b`: new env from the current .env
envkeep use staging --cascade   # switch every worktree in the repo
envkeep envs                    # list environments; * marks the default
envkeep rm preview              # delete (refuses while a worktree is on it)
```

Switching is guarded: if the local env file has edits not yet pushed to the
current environment, `use` refuses (`push or discard before switching`) instead
of silently discarding them. A cross-env `push --env <other>` likewise refuses
to overwrite keys whose value differs in the target environment, unless
`--force`.

Repo-level defaults live in `envkeep config` (stored next to the vault,
inspectable like everything else):

```sh
envkeep config set env_file .env.local  # track a different filename
envkeep config set default_env dev      # env assumed for worktrees that never chose one
envkeep config set cascade true         # make plain `use` fan out to all worktrees
envkeep config list
```

### Shell integration

```sh
# ~/.zshrc  (or ~/.bashrc with: bash)
eval "$(envkeep hook zsh)"
```

Warns discreetly when you `cd` into a worktree whose `.env` has drifted from the
vault. Silent when in sync.

The snippet carries a shell-side mtime guard: entering a worktree whose `.env`
has not moved since a clean check skips launching the binary entirely (in zsh
via a `stat` builtin — no process spawn at all). It guards the default `.env`;
repos with a custom `env_file` still spawn the binary, which resolves the config.

### Per-worktree overrides

Values that must differ between worktrees (e.g. the dev-server port) go in an
override file next to `.env`, named `<env-file>.override` (e.g. `.env.override`).
Override keys are never pushed to the shared vault and are re-applied on every
pull. **Gitignore the override file** — it is worktree-local:

```gitignore
.env.override
```

## Success criterion for the MVP

Running `push`/`pull`/`status` by hand already removes "I forgot to update the
other worktree." The shell hook removes "I forgot to run the command." Together
they solve the whole pain.

## Documentation

| File | Purpose |
|------|---------|
| [`AGENTS.md`](AGENTS.md) | Entrypoint for any agent/session — read first |
| [`docs/STATUS.md`](docs/STATUS.md) | Current phase + dated log of how we got here |
| [`docs/DECISIONS.md`](docs/DECISIONS.md) | Every design choice, its WHY, and reconsider-triggers |
| [`docs/DESIGN.md`](docs/DESIGN.md) | Architecture, on-disk layout, conflict model, cache |
| [`docs/ROADMAP.md`](docs/ROADMAP.md) | Phases, scope fences, unlock triggers |
| [`demo/README.md`](demo/README.md) | How the README gifs are recorded (vhs tapes) |

## Non-goals (deliberately fenced — see ROADMAP for triggers)

Encryption, background daemon/watch mode, SQLite/state index, multi-file
support, team/remote sync, and integrations (Vault/Doppler/1Password) are **out
of scope for v1**. Each is gated behind a concrete trigger recorded in the docs.
This is personal, single-machine tooling; it does not get built for a team that
does not exist yet.
