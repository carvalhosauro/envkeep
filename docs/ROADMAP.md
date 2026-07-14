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

## Phase 1.5 — Named environments (issue #3)  ▶ next

The trigger fired: one logical key must carry different values per deployment
target (`local` / `homo` / `prod`) without duplicated keys. Full design +
resolved decisions in [`designs/003-named-environments.md`](designs/003-named-environments.md)
(D23–D30). Model A (independent per-env vaults), per-worktree active env
(git-worktree-HEAD analogue), git-branch env model (validate-to-switch,
`--create`/`-c` to make), single env-agnostic override. Build order:

1. **Core dimension (no cobra yet — flags on existing verbs).** `vault/<env>/<file>`
   layout + live env discovery from `vault/*/`; `marker.Env` + legacy (`Env:""`)
   tolerance; `--env` (+ `--create`/`-c`) on `push`/`pull`/`status`;
   existence-validated switching; selection precedence; `check` reads `marker.Env`;
   opt-in migration of the legacy flat vault (D27); config `default_env` /
   `cascade`; `status --all-envs`. Fixtures + goldens + the R3 byte-identical
   legacy back-compat golden. (D23–D27, D30)
2. **CLI restructure → `cobra`, docker-style hybrid (D29, revises D6).**
   Top-level env verbs `use` / `envs` / `rm` (env is the primary domain — no
   redundant `env` prefix), the one `envkeep config <get|set|list|unset>` group,
   shell completions. Switch verb is `use`; `set` is reserved for `config set`.
3. **`use` cascade fan-out (D28).** Repo-wide switch fanning out to every
   worktree, `--dry-run`, skip-not-clobber for `ahead`/`conflict` worktrees.

**Trigger-gated within this feature:** the `shared` layer (Model B) — added only
if re-declaring common keys across envs becomes real pain (D24 trigger). The
`vault/<env>/…` layout already receives it with no migration.

**Exit / success criterion:** a key holds different values under `prod` vs `homo`
with no duplication; `push`/`pull`/`status` operate against a selectable
environment defaulting to a configured one; existing single-env repos are
byte-identical (R3).

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
| ~~`cobra` CLI framework + self-generated completions~~ → **triggered, moved to Phase 1.5** | Trigger fired: named environments add noun-grouped `env`/`config` subcommands + completions (D6 → D29) |

## Guiding principle

Ship the smallest thing that removes the daily pain, keep it inspectable and
dependency-light, and let concrete triggers — not speculation — decide when the
project grows.
