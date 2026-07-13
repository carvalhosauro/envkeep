# DESIGN.md — how envkeep works

Technical map: architecture, on-disk layout, the conflict state machine, the
cache, and the one composition rule. For *why* any of this is shaped the way it
is, see [`DECISIONS.md`](DECISIONS.md) (entries referenced as `D#`).

## Core model

- The git repository is shared truth. All worktrees of a repo share one
  **common dir** (`git rev-parse --path-format=absolute --git-common-dir`,
  resolved absolute — see D13). This holds for normal repos, linked worktrees,
  and bare-repo `.bare/` layouts alike.
- Shared env values live in a flat **vault** file inside that common dir (D2,
  D3).
- Each worktree records a small **base marker** in its own per-worktree gitdir,
  enabling 3-way conflict detection and doubling as the cache (D5).
- The effective local file is composed from the vault and a per-worktree
  override (D9): **effective `.env` = vault ⊕ override**.

## On-disk layout

Shared, in the common dir (never tracked by git, since it's inside `.git/`):

```
<git-common-dir>/envkeep/
  config                 # repo config, KEY=VALUE. e.g. env_file=.env.local
  vault/
    .env                 # the vault, named after the tracked file (D12)
    .env.local           # (only if that's the tracked filename)
```

Per worktree, in its own gitdir (`git rev-parse --git-dir`):

```
.git/envkeep.base                        # main worktree
.git/worktrees/<name>/envkeep.base       # each linked worktree
```

`envkeep.base` contents (flat, parseable with the same parser):

```
vault_hash=<hash of vault content at this worktree's last sync>
local_mtime=<.env mtime at last check>
vault_mtime=<vault mtime at last check>
```

In each worktree's working tree:

```
<worktree>/.env                  # the tracked file (name from config)
<worktree>/.env.<override>       # gitignored per-worktree override (e.g. .env.override)
```

## Component layout (Go packages)

```
main.go                 # subcommand dispatch (stdlib flag, D6)
internal/git/           # common-dir (absolute), worktree list, per-worktree gitdir
internal/envfile/       # order/comment-preserving parser, merge, 3-way diff
internal/vault/         # VaultStore interface (D17) + flatfile implementation
internal/state/         # base-marker read/write (conflict + cache)
internal/cmd/           # status / push / pull
internal/hook/          # emits the zsh chpwd / bash cd-trap snippet
scripts/mkfixture.sh    # golden-set fixture generator (D18)
testdata/golden/        # expected outputs
```

## VaultStore interface (D17)

```go
type VaultStore interface {
    Read() (map[string]string, error)
    Write(map[string]string) error
}
```

v1 ships one implementation: local flat file. Encrypted-file and remote backends
(deferred, D14/D16) become new implementations of these two methods — not a
rewrite.

## The composition rule (D9)

There is exactly one rule, and push/pull are symmetric around it:

```
effective .env = vault ⊕ override      (override keys win)
```

- **pull:** write vault values into `.env`, then re-apply override keys on top.
  Override-managed keys always keep their worktree-local values.
- **push:** take local `.env`, **strip** override keys, then merge the rest into
  the vault. Worktree-specific values (e.g. `PORT`) never enter shared truth.

Override keys are defined by their presence in the worktree's override file.

## Conflict model (D5)

State is decided by a 3-way comparison against the **base** (vault content at
this worktree's last sync). Comparison is on the semantic key/value set
(order- and comment-insensitive); writing still preserves layout (D11).

| local vs base | vault vs base | State        | Meaning / action                          |
|---------------|---------------|--------------|-------------------------------------------|
| same          | same          | **clean**    | nothing to do                             |
| changed       | same          | **ahead**    | `push` fast-forwards the vault            |
| same          | changed       | **behind**   | `pull` fast-forwards the local `.env`     |
| changed       | changed       | **conflict** | refuse auto-merge; show 3-way; manual fix |

Plus two states outside the table:
- **absent** — no local `.env` exists yet.
- **fresh** — no vault exists yet (first use in the repo).

**Conflict UX (v1):** detect → refuse → show per-key 3-way (`base | local |
vault`) → user resolves with `push --force`, `pull --force`, or interactive
per-key pick. No automatic merge in v1 — detection + refusal is the safety.

## Cache: two-layer skip (D5, D7)

The base marker's stored mtimes drive a cache so the common case costs almost
nothing:

1. **Shell-side (in the hook):** `stat` the local `.env` mtime; if unchanged
   since the last check, never even spawn the binary. Zero-cost prompt.
2. **Binary-side:** on spawn, `stat` both `.env` and the vault, compare to the
   stored mtimes; if neither moved, print `clean` and exit **without parsing**.
   Parse + 3-way diff only run when an mtime actually changed.

## Vault writes are atomic

Write to a temp file in the same dir, then `rename()` over the vault. Prevents a
corrupt vault if the process dies mid-write.

## `.env` parser scope (v1)

Handle: `KEY=VALUE`, optional `export ` prefix, single/double quotes, inline
`#` comments, blank/comment lines preserved for layout, `=` inside quoted
values. Multiline values: define behavior explicitly (v1 may reject or
best-effort — decide when implementing `internal/envfile`).

## Shell hook (D7)

`envkeep hook zsh|bash` prints a snippet to source in `.zshrc` / `.bashrc`:

- zsh: registers a `chpwd` function; bash: wraps `cd` (or guards
  `PROMPT_COMMAND` by comparing `$PWD`).
- The snippet applies the shell-side mtime guard before calling
  `envkeep status --quiet`, so unchanged worktrees never spawn the binary.
- On drift, print one discreet line; optionally offer to sync.
- `precmd`/`PROMPT_COMMAND` (every-prompt) variant is opt-in for users who edit
  `.env` in place without leaving the directory.
