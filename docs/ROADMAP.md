# ROADMAP.md — what gets built, in what order

Phases are scope fences, not just a schedule. A later-phase feature is **out of
scope until its trigger fires**, and the trigger is written down. Building ahead
of a trigger is over-engineering (see `DECISIONS.md`). For the *why* behind each
fence, follow the `D#` references into `DECISIONS.md`.

Current phase: see [`STATUS.md`](STATUS.md).

---

## Phase 0 — Design & docs  ✅ complete

- Problem defined, scope fenced, every decision recorded with its why and
  reconsider-trigger.
- Deliverables: `README.md`, `AGENTS.md`, `docs/{DECISIONS,DESIGN,ROADMAP,STATUS}.md`.

## Phase 1 — v1 MVP  ✅ complete

The whole point of v1. Build order matches risk (riskiest logic + golden set
first):

1. **`scripts/mkfixture.sh`** — golden-set fixture generator. Builds canonical
   repo states: normal layout **and** bare `.bare/` layout, seeding every sync
   scenario (fresh, clean, ahead, behind, conflict, `.env` absent, override
   stripped-on-push, override reapplied-on-pull, common-dir-from-linked-worktree).
   (D18)
2. **`internal/envfile`** — order/comment-preserving parser, union merge, and
   3-way diff. Pure, unit-tested, no git. This is the riskiest logic. (D11, D8)
3. **`internal/git`** — common dir resolved absolute (D13), worktree list from
   `--porcelain` (D4), per-worktree gitdir.
4. **`internal/vault`** — `VaultStore` interface + flatfile impl, atomic write,
   vault named after tracked file. (D17, D12)
5. **`internal/state`** + **`internal/cmd`** — base-marker read/write; `status`,
   `push` (union, `--prune`, `--force`), `pull` (compose with override,
   `--force`), with conflict detection and the two-layer mtime cache. (D5, D8, D9)
6. **`internal/hook`** — `envkeep hook zsh|bash` emits the shell snippet with the
   mtime guard. (D7)

Cross-cutting: configurable env filename via `--file` > repo config > `.env`
default (D12); integration tests driven by the fixture, golden outputs in
`testdata/golden/` with an `-update` regeneration flag (D18).

**Exit / success criterion:** running `push`/`pull`/`status` by hand eliminates
"I forgot to update the other worktree"; the shell hook eliminates "I forgot to
run the command." Both together = the pain is solved.

## Phase 2 — Encryption (deferred; trigger-gated)  🔒

Only when D14's conditions make plaintext uncomfortable. Pattern: **per-machine
key in the OS keychain**, not a typed password (no friction on push/pull).
Lands as a new `VaultStore` implementation (D17) behind the existing interface —
not a rewrite.

## Phase 3 — Scale-out (each independently trigger-gated)  🔒

None of these get built speculatively. Each has a concrete trigger in
`DECISIONS.md`:

| Feature | Trigger (from DECISIONS) |
|---------|--------------------------|
| Multi-file env support (`env_files` list, one vault each) | A project needs several env files tracked at once (D12) |
| SQLite / cross-repo state index; `status --all` | Scope moves from per-repo to all-repos, or auditable push/pull history is wanted (D2) |
| Team / remote sync; Vault/Doppler/1Password adapters | A real person asks to use it in a team — then a new `VaultStore` adapter, not a redesign (D16, D17) |
| Daemon / `fsnotify` watch mode | The shell hook proves insufficient in practice, e.g. IDE terminals that don't source `.zshrc` leave you stale often (D15) |
| `cobra` CLI framework + self-generated completions | Command count grows past ~10, or the binary should emit its own completions (D6) |

## Guiding principle

Ship the smallest thing that removes the daily pain, keep it inspectable and
dependency-light, and let concrete triggers — not speculation — decide when the
project grows.
