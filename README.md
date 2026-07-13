# envkeep

Keep `.env` files in sync across the git worktrees of one repository.

> Status: **design complete, implementation not started.** See
> [`docs/STATUS.md`](docs/STATUS.md) for the current phase. AI agents: start at
> [`AGENTS.md`](AGENTS.md).

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

## Command sketch (v1, not yet built)

```
envkeep status     # per-worktree: synced / ahead / behind / conflict / absent
envkeep push       # local .env -> vault (union merge by default; --prune to delete)
envkeep pull       # vault -> local .env (preserves order/comments, reapplies override)
envkeep hook zsh   # print the shell snippet to source in .zshrc / .bashrc
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

## Non-goals (deliberately fenced — see ROADMAP for triggers)

Encryption, background daemon/watch mode, SQLite/state index, multi-file
support, team/remote sync, and integrations (Vault/Doppler/1Password) are **out
of scope for v1**. Each is gated behind a concrete trigger recorded in the docs.
This is personal, single-machine tooling; it does not get built for a team that
does not exist yet.
