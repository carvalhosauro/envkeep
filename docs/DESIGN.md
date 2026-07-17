# DESIGN.md — how envkeep works

Technical map: architecture, on-disk layout, the conflict state machine, the
cache, and the one composition rule. For *why* any of this is shaped the way it
is, see [`DECISIONS.md`](DECISIONS.md) (entries referenced as `D#`).

## Core model

- The git repository is shared truth. All worktrees of a repo share one
  **common dir** (`git rev-parse --path-format=absolute --git-common-dir`,
  resolved absolute — see D13). This holds for normal repos, linked worktrees,
  and bare-repo `.bare/` layouts alike.
- Shared env values live in flat **vault** files inside that common dir (D2,
  D3). The vault holds one file per named **environment** (`dev`, `staging`, …
  — D23); each environment's vault is a complete, self-contained value set
  (no shared/inherited layer in v1, D24). A repo that never adopts
  environments keeps the single legacy flat vault (D27).
- Each worktree records a small **base marker** in its own per-worktree gitdir:
  its **active environment** (the analog of a git worktree's HEAD, D25) plus
  the base for 3-way conflict detection, doubling as the cache (D5).
- The effective local file is composed from the active environment's vault and
  a single, environment-agnostic per-worktree override (D9, D30):
  **effective `.env` = env-vault ⊕ override**.

## On-disk layout

Shared, in the common dir (never tracked by git, since it's inside `.git/`):

```
<git-common-dir>/envkeep/
  config                 # repo config, KEY=VALUE: env_file (D12), default_env (D25), cascade (D28)
  vault/
    .env                 # legacy flat vault — a repo that never adopted environments (D27)
    dev/.env             # one directory per named environment: vault/<env>/<envFilename> (D23)
    staging/.env
```

The environment set is discovered live from the `vault/*/` directories — the
filesystem is the registry, as `.git/refs/heads/` is for branches (D26). On the
first environment creation the legacy flat vault is migrated once into
`vault/<env>/` (D27).

Per worktree, in its own gitdir (`git rev-parse --git-dir`):

```
.git/envkeep.base                        # main worktree
.git/worktrees/<name>/envkeep.base       # each linked worktree
```

`envkeep.base` is a small JSON file (the base is stored in full, not hashed —
the 3-way check needs actual values to tell a real per-key conflict from a
mergeable divergence):

```json
{
  "env": "dev",                  // active environment; omitted = legacy/unnamed (D25, D27)
  "local_mtime": 1710000000000,  // .env mtime (unix ns) at last check — the cache
  "vault_mtime": 1710000000000,  // active env's vault mtime at last check
  "base": {"KEY": "value"}       // full vault snapshot at this worktree's last sync
}
```

In each worktree's working tree:

```
<worktree>/.env                  # the tracked file (name from config)
<worktree>/.env.<override>       # gitignored per-worktree override (e.g. .env.override)
```

## Component layout (Go packages)

```
cmd/envkeep/main.go     # entrypoint: os.Exit(cli.Execute()) — thin dispatch, excluded from coverage (D21, D29, D31)
internal/buildinfo/     # build-time version metadata
internal/config/        # per-repo config: env_file (D12), default_env (D25), cascade (D28)
internal/git/           # common-dir (absolute), worktree list, per-worktree gitdir
internal/env/           # env.Name: environment naming, validation, reserved names (D26)
internal/envfile/       # order/comment-preserving parser, merge, 3-way classify
internal/vault/         # Store interface (D17) + per-env flatfile layout, live env discovery, legacy migration (D23, D26, D27)
internal/state/         # base-marker (JSON) read/write: active env + base + mtime cache
internal/fsutil/        # shared atomic file write
internal/cli/           # cobra command tree (D29, D31) + logic: init/status/push/pull/check/hook/envs/use/rm/config
internal/hook/          # emits the zsh chpwd / bash cd-trap snippet (D7)
scripts/mkfixture.sh    # golden-set fixture generator (D18)
```

The entrypoint lives at `cmd/envkeep/main.go` (standard Go layout) and shrinks
to `os.Exit(cli.Execute())`: the `*cobra.Command` definitions live in
`internal/cli` next to the command logic, so the command surface stays tested
and coverage-counted while `cmd/envkeep` remains trivial dispatch (D31). The
CLI is a docker-style hybrid: environment operations are top-level verbs
(`use`, `envs`, `rm`); only config is a noun group (`config get|set|…`, D29).

## Store interface (D17)

```go
type Store interface {
    Read() (envfile.Env, error)
    Write(env envfile.Env) error
}
```

v1 ships one implementation: local flat file, bound to one environment's vault
path. Encrypted-file and remote backends (deferred, D14/D16) become new
implementations of these two methods — not a rewrite.

## The composition rule (D9, D23, D30)

There is exactly one rule, and push/pull are symmetric around it:

```
effective .env = env-vault ⊕ override      (override keys win)
```

- **pull:** write the active environment's vault values into `.env`, then
  re-apply override keys on top. Override-managed keys always keep their
  worktree-local values.
- **push:** take local `.env`, **strip** override keys, then merge the rest into
  the active environment's vault. Worktree-specific values (e.g. `PORT`) never
  enter shared truth.

Override keys are defined by their presence in the worktree's override file.
The override is a single file applied identically under every environment —
machine-local values are about the worktree, not the deployment target (D30).

## Conflict model (D5)

State is decided by a 3-way comparison against the **base** (vault content at
this worktree's last sync). Comparison is on the semantic key/value set
(order- and comment-insensitive); writing still preserves layout (D11).

| local vs base | vault vs base | State        | Meaning / action                            |
|---------------|---------------|--------------|---------------------------------------------|
| same          | same          | **clean**    | nothing to do                               |
| changed       | same          | **ahead**    | `push` fast-forwards the vault              |
| same          | changed       | **behind**   | `pull` fast-forwards the local `.env`       |
| changed       | changed, disjoint keys | **diverged** | both moved on different keys; `push` union-merges safely (D8) |
| changed       | changed, same key(s)   | **conflict** | refuse auto-merge; show 3-way; manual fix |

Plus states outside the table (no base to compare against):
- **absent** — no local `.env` exists yet.
- **unsynced** — local `.env` exists but the worktree has no marker (never
  pushed/pulled), or its environment's vault does not exist yet.
- **fresh** — no vault exists yet (first use in the repo).

**Conflict UX (v1):** detect → refuse → show the conflicting keys' 3-way
(`base | local | vault`) → the user edits the local file (or pulls a fresh
copy) and pushes again. No automatic merge in v1 — detection + refusal is the
safety. (`push --force` exists only for the cross-env push, where no 3-way
base exists; it is not a conflict override.)

## Cache: two-layer skip (D5, D7)

The base marker's stored mtimes drive a cache so the common case costs almost
nothing:

1. **Shell-side (in the hook, implemented #10):** `stat` the local `.env` mtime;
   if unchanged since the last check of this directory *and* that check was clean
   (a conservative per-directory cache the snippet keeps), never even spawn the
   binary. In zsh the stat is the `zsh/stat` builtin — a genuinely zero-spawn
   prompt (~85× cheaper than launching `envkeep check` on the clean path).
   Scope/limits: guards the default `.env` only (a custom `env_file` repo the
   shell can't cheaply resolve falls through to the binary); mtime is
   second-resolution, so an in-place edit within the same wall-clock second as a
   clean check is caught on the next `cd`, not immediately; a vault-only change
   while the local `.env` is untouched is likewise re-detected on the next `cd`,
   `.env` edit, or fresh shell — layer 2 remains the source of truth.
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

- zsh: registers a `chpwd` function; bash: guards `PROMPT_COMMAND` by comparing
  `$PWD` (acts only on a directory change).
- The snippet applies the shell-side mtime guard (layer 1 above) before calling
  `envkeep check`, so unchanged, previously-clean worktrees never spawn the
  binary. bash's per-directory cache needs associative arrays (bash 4+); the
  macOS system bash 3.2 transparently falls back to a plain per-`$PWD` guard.
- On drift, print one discreet line (`envkeep check`'s own message).
- Editing `.env` in place without leaving the directory is caught on the next
  `cd`; a true every-prompt variant remains a possible opt-in follow-up.
