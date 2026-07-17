# DECISIONS.md — why envkeep is shaped this way

This is the reasoning record. Each entry states the **decision**, the **why**,
the **alternatives rejected**, and a concrete **reconsider-trigger** — the
specific condition that would justify revisiting it. If no trigger has fired,
the decision stands; treat a fenced-off option as a decision, not an oversight.

Format per entry: Decision · Why · Rejected · Reconsider-trigger · Status.

Statuses: `ACCEPTED` (in force), `SUPERSEDED` (replaced — kept for history),
`REVISED` (changed mid-design — the journey is recorded on purpose).

---

## D1 — Language: Go

**Decision:** Build in Go.
**Why:** Single static binary, trivial distribution, first-class subprocess
handling for shelling out to `git`, cross-platform.
**Rejected:** Shell script (too fragile for parsing/merge/conflict logic);
Rust (fine, but Go's simpler subprocess + string handling wins for a tool that
is mostly "shell out to git and diff text files").
**Reconsider-trigger:** None foreseen.
**Status:** ACCEPTED.

## D2 — Vault is a flat `KEY=VALUE` file, not a database

**Decision:** Shared env values live in a flat file inside the git common dir.
**Why:** Real volume is tiny (a handful of worktrees, a few keys). No
performance need for a relational store. A flat file is inspectable directly
with `cat` / `git diff` and adds no runtime dependency to the binary.
**Rejected:** SQLite or any embedded DB — solves a scale/index problem this
project does not have in v1.
**Reconsider-trigger:** Scope changes from "per repo" to "every repo I have
ever used" (e.g. `envkeep status --all`), which becomes a genuine cross-repo
index problem; **or** we want an auditable history of every push/pull, not just
current state. Neither is a v1 need.
**Status:** ACCEPTED.

## D3 — State lives in the git common dir; no external registry

**Decision:** Vault and config live under `<git-common-dir>/envkeep/`.
**Why:** Every worktree of a repo shares one common dir, so it is the natural
shared-truth location with zero external coordination. Bonus safety: anything
inside `.git/` is **never tracked by git**, so the vault can't be accidentally
committed.
**Rejected:** A central registry in `$HOME` or `$XDG_CONFIG_HOME` keyed by repo
path (needs a path index, goes stale when worktrees/repos move).
**Reconsider-trigger:** Cross-repo scope (see D2 trigger).
**Status:** ACCEPTED.

## D4 — No persisted worktree path index; query git live

**Decision:** Get the worktree list from `git worktree list --porcelain` every
time it is needed.
**Why:** Fast enough, and never stale. A saved index diverges the moment a
worktree is removed manually.
**Rejected:** Caching the worktree list to disk.
**Reconsider-trigger:** Measured performance problem from repeatedly shelling
out (not expected at this scale).
**Status:** ACCEPTED.

## D5 — Per-worktree base marker (sidecar) for conflict detection AND cache

**Decision:** Each worktree keeps a tiny state file recording the vault content
hash at its last sync, plus the last-seen mtimes of the local `.env` and the
vault. Stored in the **per-worktree gitdir** (`git rev-parse --git-dir` →
`.git/worktrees/<name>/envkeep.base` for linked worktrees, `.git/envkeep.base`
for the main one).
**Why:** "Synced vs divergent" is not decidable from two files alone — you need
a **base** (what the vault looked like at this worktree's last sync) to run a
3-way comparison, exactly like git. The same file doubles as the cache: compare
current mtimes to the stored ones and skip parsing entirely when nothing moved.
**Rejected:** Deriving state from only local `.env` + vault (can't tell who
diverged, so can't distinguish "I'm ahead" from "there's a conflict").
**Reconsider-trigger:** None — this is load-bearing for both conflict detection
and cache.
**Status:** REVISED. *History (kept on purpose): during design this sidecar was
first proposed, then dropped as premature ("status can just parse both files
every time"), then reinstated once conflict detection was taken seriously — a
3-way merge structurally requires a base. The drop was wrong; this is the
correction. Recorded so no future session re-drops it for the same wrong
reason.*

*Follow-up refinement (step 5): the marker stores the base as the full env
snapshot (a small JSON file), not merely a hash. Building the commands showed a
hash can only answer "did this side change?" — it cannot drive the per-key
conflict-vs-mergeable-divergence distinction, nor let push detect that another
worktree changed a specific key. The base content is required for both. The
original spec explicitly sanctioned "a small JSON alongside the vault" for this
state. mtimes still live in the same marker for the cache.*

## D6 — CLI framework: stdlib `flag` + manual subcommand dispatch

**Decision:** Use the standard library, one `FlagSet` per subcommand.
**Why:** ~5 subcommands. Matches the minimal-dependency, inspectable ethos of
the whole project. No third-party dep for something this small.
**Rejected:** `cobra` (earns its keep at 10+ commands or when you want it to
generate shell completions itself — heavy here); `urfave/cli` (lighter than
cobra, still an unneeded dep).
**Reconsider-trigger:** Command count grows past ~10, **or** we want the binary
to emit its own bash/zsh completions → adopt `cobra` then.
**Status:** REVISED. *The reconsider-trigger has now fired — see D29. Named
environments (design 003) add noun-grouped `env`/`config` subcommand trees plus
shell completions, so `cobra` is adopted. The stdlib `flag` choice was right for
the 5-command v1; recorded so the switch reads as a triggered decision, not a
reversal.*

## D7 — Shell hook: `chpwd` / cd-trap by default, with mtime guard

**Decision:** Default hook fires on directory change (zsh `chpwd`, bash cd
wrapper). A shell-side mtime guard skips spawning the binary when `.env` hasn't
changed. `precmd`/`PROMPT_COMMAND` (every-prompt) offered as opt-in. Git
`post-checkout` rejected.
**Why:** The core pain is "forgot to propagate to the *other* worktrees." That
drift matters exactly when you `cd` into the other worktree — which is when
`chpwd` fires. Firing every prompt adds overhead; only the opt-in in-place-edit
crowd needs it. `post-checkout` fires only on checkout/worktree-add — it misses
both `cd` and in-place edits, so it can't cover the daily case.
**Rejected:** `direnv` as the default (adds a dependency and an `.envrc` per
worktree; fine as a power-user path, not the baseline).
**Reconsider-trigger:** If the hook proves insufficient in practice — e.g. IDE
terminals that don't source `.zshrc` leave you in stale state often — that is
the signal (and only then) to reconsider a daemon/watch mode (see D14).
**Status:** ACCEPTED.

## D8 — `push` is a union merge by default, not a replace

**Decision:** `push` merges local `.env` into the vault. Local values win on
shared keys; keys present in the vault but absent locally are **kept**, not
deleted. Deletion only via explicit `--prune` or interactive confirm.
**Why:** Directly kills the stated fear: worktree B pushing without `KEY_X`
must not wipe `KEY_X` that worktree A added to the vault. Replace-semantics
would clobber it; union-semantics can't. Solves the problem with semantics
rather than relying on the user to catch it in a diff review.
**Rejected:** Replace-then-review (safe only if the user reads every diff every
time — fragile).
**Reconsider-trigger:** None foreseen; `--prune` covers deliberate deletion.
**Status:** ACCEPTED.

## D9 — Per-worktree override file (gitignored); composition rule

**Decision:** A gitignored per-worktree override file holds values that must
differ between worktrees (e.g. dev-server `PORT`). Override keys are **stripped
before push** (never pollute the shared vault) and **re-applied after pull**.
The one composition rule everything obeys: **effective `.env` = vault ⊕
override**.
**Why:** Running several worktrees in parallel needs per-worktree ports without
those leaking into shared truth. A single, explicit composition rule keeps push
and pull symmetric and predictable.
**Rejected:** Marking keys as "local" inside the shared vault (pollutes shared
truth; ambiguous across worktrees).
**Reconsider-trigger:** None foreseen.
**Status:** ACCEPTED.

## D10 — Name: `envkeep`

**Decision:** The project/binary is `envkeep`.
**Why:** Names the durable core (something that *keeps* env), not the v1 trick.
Room to grow toward a robust vault-like tool without the name fighting the
scope.
**Rejected:** `envwt` (env + worktree) — used in early drafts, then rejected:
it cages the name to the worktree feature. Worktree is the v1 *selling* feature,
but the *name* should outlive worktree-only scope. Also considered `envault`,
`enclave`, `envkeep` (chosen), `parcel`.
**Reconsider-trigger:** Name collision on a distribution channel (npm / gh /
brew / domain) discovered before first release → pick among the alternatives.
**Status:** ACCEPTED. *Verify availability on npm/gh/brew before first public
release.*

## D11 — Preserve key order and comments when writing `.env`

**Decision:** `pull` parses `.env` to an ordered structure and patches values
in place, preserving comment lines and key ordering.
**Why:** Users are hostile to tools that silently reorder or strip comments from
their files. Comparison/diff logic works on the semantic key/value set
(order-insensitive), but *writing* preserves layout.
**Rejected:** Normalizing the file on write (simpler, but user-hostile).
**Reconsider-trigger:** None foreseen.
**Status:** ACCEPTED.

## D12 — Configurable single env filename; vault named after the file

**Decision:** The tracked filename is configurable (some projects use `.env`,
others `.env.local`), defaulting to `.env`, resolved as
`--file` flag > repo config (`<common-dir>/envkeep/config`) > default. All
worktrees of one repo track the same filename. The vault is named after the
tracked file (`.../vault/.env`, `.../vault/.env.local`).
**Why:** Covers the real "`.env` here, `.env.local` there" need without
multi-file complexity. Naming the vault after the file is a zero-cost seam: the
day multi-file support lands, file #2 becomes a second vault with no migration
and no collision.
**Rejected:** Hardcoding `.env`; supporting multiple env files simultaneously
now (out of scope — see ROADMAP).
**Reconsider-trigger:** A project needs several env files tracked at once →
promote config key `env_file` (string) to `env_files` (list), one vault per
entry.
**Status:** ACCEPTED.

## D13 — Resolve git common dir as an absolute path

**Decision:** Always obtain the common dir via
`git rev-parse --path-format=absolute --git-common-dir` (git 2.31+), with a
manual cwd-resolution fallback for older git.
**Why:** `--git-common-dir` can return a path *relative to cwd* depending on git
version and layout (notably bare-repo `.bare/` setups). Using it raw would write
the vault to the wrong place when run from a subdirectory. This is also what
makes the bare-repo (`.bare/`) layout work: the tool keys off the common dir,
which resolves to `.bare`, so linked worktrees and bare setups behave
identically.
**Rejected:** Using the raw `--git-common-dir` output.
**Reconsider-trigger:** None foreseen.
**Status:** ACCEPTED.

## D14 — Encryption OUT of v1

**Decision:** The vault is plaintext in v1.
**Why:** The exposure risk is no greater than the `.env` already carries today.
Encryption's complexity is not justified now.
**Rejected:** Encrypting from day one.
**Reconsider-trigger:** When encryption does land (v2/v3), the pattern is a
**per-machine key stored in the OS keychain**, not a typed password — avoids
friction on every push/pull. This mirrors `vsync` (market reference seen during
design).
**Status:** ACCEPTED (deferred).

## D15 — Daemon / `fsnotify` watch mode OUT of v1

**Decision:** No background process. The shell hook covers the use case.
**Why:** A persistent daemon is complexity the hook makes unnecessary.
**Rejected:** Background watcher from the start.
**Reconsider-trigger:** If, even with the hook, you keep landing in stale state
frequently (e.g. IDE terminals that don't load `.zshrc`), that is the signal to
reconsider. Absent that signal, do not build it.
**Status:** ACCEPTED (deferred).

## D16 — Team / remote sync / secret-manager integrations OUT of v1

**Decision:** Personal, single-machine tooling. No team sharing, no remote
backend, no Vault/Doppler/1Password integration.
**Why:** Designing today for a team that does not exist is over-engineering.
Market note: `vsync` grew from the same `.env`+worktree pain into a full
encrypted vault with S3, multi-language runtime libs, and cloud fanout — a great
product, but it solves a *different* problem (secret distribution to
teams/production) and never became worktree-aware in the sense envkeep needs.
Borrow its good techniques (keychain, lean onboarding UX), not its scope.
**Rejected:** Building any of it now.
**Reconsider-trigger:** A real person asks to use it in a team. Then the path is
a new adapter behind the `VaultStore` interface (see D17), not a redesign.
**Status:** ACCEPTED (deferred).

## D17 — `VaultStore` interface, kept tiny

**Decision:** Vault read/write sits behind a minimal interface —
`Read() (map[string]string, error)` and `Write(map[string]string) error`. The
only v1 implementation is the local flat file.
**Why:** Leaves the door open for v2 (encrypted file) and future backends
(reading from a real Vault/1Password) as *new adapters*, not a rewrite. Keeping
it to two methods resists premature additions (`Lock()`, `History()`) that are
themselves the deferred v2/v3 signals.
**Rejected:** A fat interface anticipating locking/history now.
**Reconsider-trigger:** Encryption (D14) or remote backends (D16) land → add
adapters implementing this same interface.
**Status:** ACCEPTED.

## D18 — Tests use real git; a bash fixture script is the golden set

**Decision:** Integration tests shell out to real `git`. A
`scripts/mkfixture.sh` builds canonical repo states (normal **and** bare `.bare/`
layouts, plus every sync scenario) and is the reusable "golden set" generator.
Pure logic (parser, merge, 3-way diff) is unit-tested with no git.
**Why:** The whole tool depends on real `git rev-parse` / worktree behavior;
mocking git would test nothing. The bash fixture makes multi-worktree setups
reproducible in CI, where git is always available.
**Rejected:** Mocking the git layer.
**Reconsider-trigger:** None foreseen.
**Status:** ACCEPTED.

## D19 — DX toolchain: golangci-lint v2 + lefthook, both pinned in `./bin`

**Decision:** Formatting + linting via a single pinned `golangci-lint` v2
(gofumpt + goimports as its formatters, plus a curated linter set). Git hooks
managed by `lefthook` (Go, single binary) via `lefthook.yml` (`pre-commit`:
format staged Go + re-stage, then lint + test; `pre-push`: race test + build).
Both tools install to `./bin` — **not** into `go.mod` — pinned by version in the
`Makefile`. CI runs the same `make` targets with the same pinned versions.
Established before the first line of application code.
**Why:** Consistency is cheapest to enforce at day-1 and painful to retrofit.
golangci-lint v2 collapses three tools (gofumpt, goimports, linter) into one.
Neither tool goes through `go tool`/`go.mod`: golangci-lint would drag its large
dependency tree into this project's `go.sum`, and lefthook is installed with
`go install …@version` (which does not touch the module) into `./bin`. Keeping
both as pinned `./bin` binaries preserves the minimal, inspectable ethos while
giving lefthook's ergonomics: staged-file scoping (`{staged_files}`), automatic
re-staging of formatted files (`stage_fixed`), and one declarative `lefthook.yml`
instead of hand-rolled shell. CI calling `make` keeps local and CI from drifting.
**Rejected:** `go tool`-pinned tools (pollutes go.mod); the python `pre-commit`
framework (drags a Python runtime); separate gofmt/goimports/staticcheck
binaries (golangci-lint v2 subsumes them).
**Reconsider-trigger:** None foreseen for the hook manager. Linter/hook versions
bump by editing the `Makefile` pins.
**Status:** REVISED. *History (kept on purpose): the first implementation used
native committed git hooks under `.githooks/` via `core.hooksPath`, chosen for a
strict no-new-deps stance. Switched to `lefthook` at the user's request — it is
the Go-native hook manager, installs to `./bin` under the same pinned pattern as
golangci-lint (so `go.mod` stays clean), and its staged-file scoping + re-staging
give better ergonomics than the shell scripts. The no-dep purity was traded for
a small, well-contained tool that fits the intended long-term direction.*

## D20 — Command safety semantics, diff review via `--dry-run`, override filename

**Decision:** `push`/`pull` **refuse** rather than silently resolve when they
would lose work: `push` refuses when the vault is `Behind`-relative (has changes
the local env lacks) or in `Conflict`; `pull` refuses when local is `Ahead` or in
`Conflict`. A mergeable `Diverged` (non-overlapping changes) proceeds via union.
Diff review before overwriting is provided by `--dry-run` (prints the +/~/-
delta, writes nothing), not an interactive prompt. The per-worktree override file
is the tracked filename + `.override` (e.g. `.env.override`), gitignored by the
user.
**Why:** Resolves the original open question "review the diff before overwriting
the vault" without a fragile interactive stdin prompt — `--dry-run` keeps the
commands non-interactive, scriptable, and easy to test, while still letting you
preview. Refuse-not-merge on conflict is the D5 safety made concrete: the tool
never picks a winner for a real conflict. The `.override` suffix guarantees the
override file is always distinct from the tracked file (avoids the clash noted in
STATUS when the tracked file is itself `.env.local`).
**Rejected:** Interactive y/N confirmation in v1 (stdin coupling, harder to
test); auto-merging conflicts by picking a side (silent data loss).
**Reconsider-trigger:** Interactive per-key conflict resolution and a real
confirm prompt are future polish; `--prune` (letting push delete vault keys the
local env dropped) is deferred — union never deletes today (D8).
**Status:** ACCEPTED.

## D21 — Tiered coverage thresholds + release gating

**Decision:** Coverage is enforced by `go-test-coverage` (`.testcoverage.yml`,
`make cover-check`) with **tiered per-package thresholds**, not one global
number: pure logic is held high (envfile 90, hook 95, vault 85, state/config
80), glue where coverage is naturally bounded is held lower (git 60 — fallback
branches need an ancient git; fsutil 65), and the thin CLI entrypoint
(`cmd/envkeep`) plus the trivial `buildinfo` var are excluded entirely. CI runs
`cover-check` and validates the goreleaser config; the release workflow runs the
test suite before goreleaser publishes.
**Why:** A single total-% bar is blunt — it lets well-testable logic rot while
punishing glue whose remaining paths need exotic conditions (a missing binary, a
read-only FS). Tiers encode the intended bar per package, so the gate means
something. Excluding dispatch + a version var keeps the number honest rather than
padded with trivial lines. Gating the release on tests prevents a bad tag from
shipping binaries.
**Rejected:** Total-only threshold (hides per-package rot); a uniform per-file
hard bar (too rigid for glue); chasing 100% on error paths that need fault
injection (low value, high test complexity).
**Reconsider-trigger:** If a package's realistic ceiling moves (new testable
surface, or a fallback path becomes reachable), adjust its override in
`.testcoverage.yml`.
**Status:** ACCEPTED.

## D22 — Content agreement is Clean; stale bases are retired, not hashed

**Decision:** `envfile.Classify` returns `Clean` whenever local and vault agree
as logical sets, before the 3-way comparison against the base runs — if the two
sides are identical there is nothing to merge, so the base's age is irrelevant.
On top of that, the write commands (`pull`/`push`) and the per-worktree hook
(`check`) *retire a stale base*: when they observe agreement they rewrite the
marker with the current vault as the base (and refreshed mtimes) so the false
state cannot recur and the mtime fast path is restored. `status` stays
read-only — it reports the now-correct `clean` but does not mutate any marker;
the worktree self-heals on its next `check`/`pull`/`push`.
**Why:** A marker base that predates a value both sides now share made `Classify`
return `Diverged` even though local and vault were identical, and the empty-delta
early returns in `pull`/`push` bailed without refreshing the marker — a
stuck-forever `diverged` (issue #1). Judging agreement first makes that state
structurally impossible for every command in one place; retiring the base keeps
a *subsequent* edit from being misjudged against the stale base (which would look
like a conflict).
**Rejected:** Adding `LocalHash`/`VaultHash` to the marker as a git-style
stat+hash cache (issue #4's proposed mechanism). Git stores only the blob hash
in its index, so a hash is its *only* content record; envkeep already stores the
**full** base (D5), which it needs for the per-key conflict check a hash cannot
drive. On an mtime miss the file must be parsed either way, and once parsed
`env.Equal(base)` answers exactly what a hash would — the parse already
normalizes comments/order/whitespace. A hash field would duplicate information
derived from the base it sits beside, for zero behavioural gain, plus a schema
bump and legacy-marker handling. It delivers #4's *acceptance criteria* (touch-
without-change stays clean and refreshes the cache; content-agrees never
diverges; old markers keep working with no migration) via the full base instead.
**Reconsider-trigger:** If the marker ever stops storing the full base (e.g. to
shrink it, keeping full values only for the conflict path), a stored content
hash for the cheap "did this side change?" check becomes worth its keep.
**Status:** ACCEPTED.

---

## D23 — Named environments are a dimension between vault and override

**Decision:** Model a named environment (`local` / `homo` / `prod` / …) as a
first-class dimension. The vault gains a per-environment file at
`<common-dir>/envkeep/vault/<env>/<envFilename>`. The composition rule extends to
`effective .env = env ⊕ override` — a worktree holds one environment's values at
a time, override still wins (D9). The local side stays a **single tracked file
whose contents swap** when the environment changes; per-environment files do not
coexist in the worktree. See `docs/designs/003-named-environments.md`.
**Why:** One `.env` forces one value per key, so users duplicate keys in a single
file for prod/homolog variants — ambiguous and unsyncable (a key maps to exactly
one value). A per-env vault gives each key a distinct value per target with no
duplication. The `vault/<env>/<file>` directory keeps the environment axis
orthogonal to the deferred multi-file axis (D12): filename × environment compose
cleanly with no migration when multi-file lands.
**Rejected:** Duplicate keys in one flat file (the status-quo pain); a single
annotated multi-section file (re-introduces the ambiguity and breaks flat-file
inspectability, D2); coexisting `.env.<env>` files in the worktree
(framework-style) — that drops the single-active-environment switch model (D25)
and was declined by the maintainer.
**Reconsider-trigger:** None for the dimension itself; the sharing and selection
sub-decisions carry their own triggers (D24 / D25).
**Status:** ACCEPTED (design 003 — pending implementation).

## D24 — Sharing model: independent per-env vaults (no shared layer in v1)

**Decision:** Each environment's vault is a complete, self-contained value-set
(`vault/prod/.env` holds *every* key prod needs). There is no inherited `shared`
base layer in this design; `effective = env ⊕ override`. The `vault/<env>/…`
layout is chosen so a future `shared` layer (`shared ⊕ env ⊕ override`) is a
zero-migration add.
**Why:** Independent vaults remove the push-routing problem entirely: a flat
local `.env` maps to one flat env vault, so `push` / `pull` / `Classify` are the
v1 code parameterized by environment. It is the smallest change that solves the
stated primary pain (different values per environment) and fits the
smallest-thing ethos.
**Rejected:** Shared-base-plus-per-env-overlay (Model B) now. It is DRY for keys
common to every environment, but a flat local file carries no layer annotation,
so on push envkeep cannot know whether an edited or new key belongs to `shared`
or to the environment overlay. Resolving that needs an explicit routing rule
(value-equality inference plus a `promote` / `--shared` verb) and a new mental
model — complexity not justified until the pain is real.
**Reconsider-trigger:** Re-declaring keys identical across every environment
becomes a real maintenance burden — the "I forgot to add the common key to prod
too" failure recurs (the same class envkeep exists to kill). Then add
`vault/shared/<file>`, the `shared ⊕ env` composition, and the push routing rule;
no vault move.
**Status:** ACCEPTED (design 003 — pending implementation).

## D25 — Active environment is per-worktree; selection precedence

**Decision:** Each worktree's active environment lives in its sync marker
(`marker.Env`) — the direct analogue of a git worktree's own HEAD — so worktree
`hotfix/` can run `prod` while `feature/` runs `homo` at the same time. A
repo-wide `default_env` (config) is the fallback for a worktree with no marker
yet. Selection precedence for any command:
`--env flag > marker.Env > default_env > legacy (unnamed)`.
**Why:** Different worktrees for different targets is the natural use of the
feature; per-worktree active env mirrors how each git worktree already checks out
its own branch. A repo-wide-only active env would forbid concurrent per-worktree
environments.
**Rejected:** A single repo-wide active environment (simpler, but blocks the
concurrent-worktree use case). The repo-wide flip stays available explicitly via
`use` (D28), which layers on top of the per-worktree model rather than replacing
it.
**Reconsider-trigger:** None foreseen.
**Status:** ACCEPTED (design 003 — pending implementation).

## D26 — git-branch model for the environment set; discovered live from disk

**Decision:** Environments behave like git branches. Targeting/switching to one
(`--env prod`, `use prod`, `pull --env prod`) **validates that it exists** and
errors otherwise (like `git checkout nonexistent`); **creating** one is explicit
(`--create` / `-c`, e.g. `use -c prod`, analogous to `git checkout -b`: `use -c
<new>` snapshots the **current** worktree's env into the new environment's vault
and re-points to it, leaving the local file intact — it creates *from* the
current state, it never resets the local file). The set
of environments is discovered live from the vault directories (`vault/*/`) — the
filesystem is the registry, exactly as `.git/refs/heads/` is for branches — so
there is no `environments` config list to drift. Names must be filesystem-safe
(`[A-Za-z0-9._-]`, no `/` or `..`); `shared` / `_base` and empty are reserved.
**Why:** Existence-validated switching kills the typo-creates-a-silent-environment
hazard an open set would carry, while explicit `--create` keeps creation
deliberate. Live discovery follows the "query git/fs live, no persisted index"
ethos (D4) and removes a config↔disk drift class of bug. The git-branch mental
model fits a git-worktree-shaped tool.
**Rejected:** An open set (any `--env` string auto-creates — typos become silent
environments); a declared `environments` config list (a second source of truth
that drifts from the actual vault dirs).
**Reconsider-trigger:** None foreseen.
**Status:** ACCEPTED (design 003 — pending implementation).

## D27 — Opt-in migration of the legacy flat vault; `Env:""` == default

**Decision:** A repo that never creates an environment is untouched — the vault
stays at `vault/<envFilename>` and output is byte-identical to pre-env envkeep.
On the *first* environment creation, the legacy flat vault is moved once to
`vault/<default_env>/<envFilename>` (atomic `rename`, same dir, plus a one-line
notice). Legacy markers (no `env` field → `Env:""`) are read as the default
environment and normalized on the next write.
**Why:** Zero disruption for non-adopters (the back-compat guarantee), and a
single clean code path for adopters (no permanent dual-path). The `rename` is
atomic and reversible. Tolerating a missing `env` field mirrors how the marker
already evolved its schema with no migration (D5 follow-up, D22).
**Rejected:** Dual-path forever (default env keeps the legacy flat path — two code
paths and a special-cased `status --all-envs` layout); lazy read-through
(migrate-on-write — still two transient paths).
**Reconsider-trigger:** None foreseen.
**Status:** ACCEPTED (design 003 — pending implementation).

## D28 — `use` switches the active environment, with opt-in cascade

**Decision:** `envkeep use <env>` switches the active environment (must exist,
D26); `use -c <env>` creates then switches. A `cascade` config flag governs
fan-out: `cascade=false` (default) flips only the active environment (repo
`default_env` and/or the current worktree's `marker.Env`), and other worktrees
pick it up lazily on their next `pull`; `cascade=true` walks every worktree
(`git.Worktrees`, as `status` does) and pulls `<env>` into each. Cascade respects
the D20 guards — a worktree that is `ahead` / `conflict` for its current
environment is skipped and reported, never clobbered. `--dry-run` previews. The
cascade fan-out is the *second* implementation phase, after the core selection
lands. (The issue #3 comment named this `set`; renamed to `use` so `set` stays
exclusive to `config set` — D29.)
**Why:** Covers both "flip just here" and "make the whole repo this environment
in one command" without either being a silent default; the safe default is no
mass write. Reusing the `status` worktree walk and the `pull` guards keeps it
consistent with existing safety semantics.
**Rejected:** Always fanning out (a surprising mass write); an interactive confirm
(stdin coupling, harder to test — same reasoning as D20's `--dry-run` choice).
**Reconsider-trigger:** None foreseen.
**Status:** ACCEPTED (design 003 — pending implementation; phase 2 of the
feature).

## D29 — Adopt `cobra`; docker-style hybrid command surface (revises D6)

**Decision:** With named environments the CLI moves to `cobra` in a **docker-style
hybrid** shape. Environment is envkeep's *primary* domain, so its operations are
**top-level verbs** — `use <env>` (switch; `-c` creates + switches), `envs`
(list), `rm <env>` (delete) — the way docker exposes `ps`/`run`/`rm` at top level
rather than `docker container …`. A dedicated `env` noun-group is rejected as
**redundant with the tool's own name** (`envkeep env list` repeats "env"). Only
the *secondary* domain, config, is a noun group: `envkeep config
<get|set|list|unset>` (git-config style). The active-env switch verb is `use`;
**`set` is reserved for `config set`** (no top-level `set`, so no overload). Full
surface: `use`, `envs`, `rm`, `push`, `pull`, `status`, `check`, `hook`,
`version`, `config <…>`, plus `--env`/`-c` flags and self-generated completions.
**Why:** This is D6's written reconsider-trigger firing — the command count grows
past the stdlib comfort zone and the tool now wants subcommand trees plus
completions. `cobra` earns its keep at that size. The hybrid (flat primary verbs +
one grouped secondary domain + completions) is docker's proven shape and keeps the
common env operations short and non-redundant.
**Rejected:** A `env` noun-group for environment ops (redundant with the binary
name); a top-level `set` for env switch (overloads `config set`); staying on
stdlib `flag` (grouped subcommands + completions get painful to hand-roll);
`urfave/cli` (cobra's completion + subcommand-tree support is the reason to
switch).
**Reconsider-trigger:** None (this *is* the D6 trigger resolved).
**Status:** ACCEPTED (design 003 — pending implementation). Revises D6.

## D30 — Override stays single and environment-agnostic

**Decision:** The per-worktree override (`.env.override`, D9) remains a single
file applied identically under every environment. Its keys are machine-local
(e.g. `PORT`) and do not vary by deployment target.
**Why:** Machine-local values are about the worktree/machine, not about
prod-vs-homo, so one override serves every environment. Keeps the composition
rule (`env ⊕ override`) simple and symmetric with D9.
**Rejected:** Per-environment override files (`.env.override.<env>`) — more files
and a three-way composition for a need that does not exist yet.
**Reconsider-trigger:** A real machine-local value must differ by environment;
then add `.env.override.<env>` layered over the single override.
**Status:** ACCEPTED (design 003 — pending implementation).

## D31 — Cobra command definitions live in `internal/cli`; `use` re-points one worktree

**Decision:** When the CLI moves to `cobra` (D29, phase 2), the `*cobra.Command`
definitions and an `Execute()` entry live in `internal/cli` (which also owns the
command logic), and `cmd/envkeep/main.go` shrinks to `os.Exit(cli.Execute())`.
The `use <env>` verb re-points **only the current worktree** (sets its
`marker.Env`, reusing pull's re-point path); it does not change the repo
`default_env` (that moves only on first env creation, D27). Repo-wide fan-out is
the `cascade` behavior (D28). See `docs/designs/004-cobra-cli.md`.
**Why:** `cmd/envkeep` is excluded from coverage (D21) as thin dispatch; keeping
the cobra layer there would leave a growing command surface unmeasured and
awkward to unit-test. Homing the commands in `internal/cli` keeps them tested and
counted, and leaves `main.go` a trivial entrypoint. Per-worktree `use` matches
the per-worktree active-env model (D25) and keeps the common switch predictable
and non-destructive; the repo-wide sweep stays an explicit, opt-in `cascade`.
**Rejected:** Command definitions in `cmd/envkeep` (untested/uncounted, D21); a
`use` that also flips `default_env` (surprising, couples a local switch to repo
state); a top-level `set` verb (overloads `config set`, D29).
**Reconsider-trigger:** None foreseen.
**Status:** ACCEPTED (design 004 — pending implementation).

## D32 — `status --all-envs` (worktree × environment matrix) is deferred, not built

**Decision:** `status` stays **active-env-only** — it reports each worktree's
current environment and its sync state against that environment's vault. The
full worktree × environment **matrix** (`status --all-envs`) is **not built**
now. Of the three options weighed — (a) a limited `clean`/`differs` view per
env with no 3-way base, (b) extending the marker to store a base per synced env
(full 3-way per cell, but a marker schema change + migration + larger markers),
(c) defer — we take **(c)**.
**Why:** A truthful per-cell state (`ahead`/`behind`/`conflict`) needs a 3-way
base for every (worktree, env) pair, but the marker stores a base only for the
worktree's **active** env (D5). Option (b) pays for a schema change and a
migration to serve a feature no one has asked for — speculative growth the
project explicitly refuses ("build ahead of a trigger is over-engineering",
ROADMAP). Option (a) is cheap but ships a deliberately coarse, easily-misread
state and still adds surface to maintain. Deferring keeps `status` honest and
the marker simple; the capability is one focused follow-up plan away if a real
need appears.
**Rejected:** (a) coarse clean/differs view — honest-but-misleading, still
carries maintenance cost for a speculative need; (b) per-env bases in the marker
— a schema/migration cost with no concrete demand today.
**Reconsider-trigger:** Someone actually needs to see, in one place, every
worktree's state across **all** environments (e.g. auditing drift before a
release across many envs) → then spin a follow-up plan choosing (a) or (b) with
full TDD; (b) if per-cell `ahead`/`behind`/`conflict` precision is required, (a)
if a coarse clean/differs view suffices.
**Status:** ACCEPTED — deferred (design 004 §4; ROADMAP 2d).
