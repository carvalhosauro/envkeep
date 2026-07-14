# Design 003 — Named environments (issue #3)

> Status: **DECIDED — ready to implement.** The "define everything" pass for
> issue #3 (+ its environment-switch follow-up comment, now `use`): on-disk and command changes,
> every ambiguity, and the forks — all resolved with the maintainer (§0, §12).
> Proposes DECISIONS entries D23–D30 (§13) and a ROADMAP phase (§15), pending a
> commit pass into `docs/DECISIONS.md` + `docs/ROADMAP.md`.
>
> Read `docs/DECISIONS.md` (D-refs below) and `docs/DESIGN.md` first.

---

## 0. Decisions taken (2026-07-13)

All forks resolved with the maintainer over two rounds; the rest of the doc is
reconciled to these. Nothing below is still open.

- **DP1 = A — independent per-env vaults.** No `shared` layer in this design.
  Each `vault/<env>/<file>` is a complete, self-contained value-set. This
  removes the push-routing problem (§7) entirely. The layout stays B-compatible
  (a future `shared` layer is a zero-migration add, §4), but B is **not** built
  and is trigger-gated on the common-key-duplication pain becoming real.
- **DP2 = per-worktree active env.** Each worktree's active env lives in
  `marker.Env` — the direct analogue of a **git worktree's own HEAD** (each git
  worktree already checks out its own branch). `default_env` (config) is the
  fallback for a worktree with no marker yet. `use` (§10) is the explicit
  repo-wide override.
- **DP3 = git-branch model.** Targeting/switching to an env **validates
  existence** (`--env prod` on a non-existent env errors, like `git checkout
  nonexistent`); **creating** an env is explicit (`--create` / `-c`, analogous to
  `git checkout -b`). Consequence: **the set of environments is discovered live
  from `vault/*/` on disk** (D4 "query git/fs live, no persisted index" ethos) —
  the filesystem is the registry, exactly as `.git/refs/heads/` is for branches —
  so the proposed `environments` config key is **dropped** (§5). An env "exists"
  iff its vault dir exists, just as a branch exists iff its ref does.
- **Local layout = SWAP, single tracked file** (round 2). Each worktree has ONE
  tracked file (`.env`, or whatever `env_file` names — D12); switching env
  rewrites its contents (`marker.Env` records which). Environment files do **not**
  coexist framework-style (`.env.prod`+`.env.homo` side by side was rejected — it
  would revert DP2/DP3). The app always reads the one tracked file; envkeep swaps
  what's in it.
- **Multi-*file* = NO / deferred (D12).** Environment is the *only* new dimension:
  one tracked file per repo, N environments. Tracking several *distinct* files at
  once (N files × M envs) stays behind D12's trigger; the `vault/<env>/<file>`
  layout already receives it with no migration. (A repo whose convention is a
  single `.env.local` still works today — that's the configurable `env_file`, D12,
  not multi-file.)
- **DP5 = single, env-agnostic override.** One `.env.override` per worktree
  (machine-local, e.g. `PORT`), applied identically under every env (D9
  unchanged). Per-env overrides (`.env.override.<env>`) rejected — trigger-gated
  on a real need for a machine-local value that varies by environment.
- **DP4 = opt-in migration.** On first env creation, the legacy flat vault
  `vault/<file>` is moved to `vault/<default_env>/<file>` (atomic `rename` +
  notice). A repo that never creates an env is untouched (R3). Detail in §9(a).
- **DP6 = cascade fan-out is phase 2.** The core (`--env`/`-c`, `use`,
  `default_env`, `marker.Env`) delivers the primary pain relief; the `use`
  cascade (fan-out to every worktree) lands after.
- **DP9 = docker-style hybrid CLI + `cobra`** (round 3). Environment is
  envkeep's *primary* domain, so its ops are **top-level verbs** (`use`, `envs`,
  `rm`) — an `env` noun-group would be redundant with the tool's own name (like
  `docker ps`, not `docker container ps`). Only config, the secondary domain, is
  a noun group: `envkeep config <get|set|list|unset>` (git-config style). The
  active-env switch verb is **`use`** (not `set`); **`set` is reserved for
  `config set`** — no overload. This flat-primary-verbs + one-grouped-secondary +
  completions shape is docker's, and it fires D6's reconsider-trigger, so **adopt
  `cobra`** (D29, revises D6). The `cascade` config flag (renamed from
  `set_propagate`) governs whether `use` fans out.

---

## 1. Problem & goal

One `.env` forces one value per key. The same logical key (`DATABASE_URL`,
`API_HOST`) needs different values per deployment target (`local` / `homo` /
`prod`). Today users duplicate keys in one file — ambiguous, and unsyncable
because the vault maps a key to exactly one value.

**Goal:** model **named environments** as a first-class dimension so one logical
key holds a distinct value per environment, while `push`/`pull`/`status`/`check`
keep working per selected environment and existing single-env repos keep working
untouched.

**What "robust" means here (the bar this design is held to):**

- **R1 — No silent data loss.** All D20 refuse-not-merge guarantees hold *per
  environment*. Selecting the wrong env never clobbers another env's values.
- **R2 — No accidental cross-env leak.** A value pushed under `prod` never
  appears under `homo` unless explicitly shared.
- **R3 — Zero-config back-compat.** A repo that never opts into environments
  behaves exactly as today, byte-for-byte (same vault path, same output).
- **R4 — The core promise survives the new dimension.** envkeep exists to kill
  "I forgot to propagate to the other X." The env dimension must not quietly
  re-introduce a *new* "I forgot to propagate to the other environment" gap
  (this is the crux tension — see §4 and DP1).
- **R5 — Inspectable & dependency-light** (D2/D6 ethos): flat files, no DB, no
  new runtime deps, `cat`-able vaults.
- **R6 — The 3-way conflict engine (`envfile.Classify`) is reused unchanged**,
  operating on *effective* env sets; only the write-back path grows.

## 2. Scope

**In scope (this design):** an environment dimension for the vault, marker, and
config; env selection for push/pull/status/check; a default env; back-compat +
migration; the `use` command + opt-in cascade (issue comment).

**Out of scope / fenced (with triggers, D-style):**

- **Multi-*file* env** (`env_files` list — D12/ROADMAP Phase 3). Orthogonal
  dimension (filename × environment). This design keeps them orthogonal so both
  can coexist later; it does not build multi-file.
- **Encryption per env** (D14) — unchanged, still deferred.
- **Team/remote per env** (D16) — unchanged.
- **Per-env override files** (`.env.override.prod`) — see DP5; default keeps the
  single env-agnostic override, trigger-gated.

## 3. The dimension: where "environment" lives in the stack

Today the composition rule is one layer deep (D9):

```
effective .env = vault ⊕ override            (override wins)
```

The environment slots **between** the shared vault and the per-worktree
override. **This design (Model A) builds two layers:**

```
effective .env = env ⊕ override               (right wins)
  └─ env      : this environment's full value-set (vault/<env>/<file>)
  └─ override : per-worktree machine-local (PORT, …)  (D9, unchanged)
```

The stack reserves room for an optional third layer (Model B, trigger-gated on
the D24 common-key pain), which would slot `shared` under `env` with no
migration:

```
effective .env = shared ⊕ env ⊕ override      (future — Model B, D24 trigger)
```

DP1 (below) is the fork that chose Model A over B; it was the single most
consequential decision:

- **DP1** — is the `shared` layer present, or is each env a full independent
  value-set (no sharing)? ✅ Resolved: **A, independent** (§0/§4).
- The rest (paths, marker, selection, migration) is largely mechanical *given*
  DP1.

## 4. DP1 — the storage/sharing model (THE decision)

### Model A — independent per-env vaults (flat, no sharing)

Each environment is a complete, self-contained value-set. `vault/prod/.env`
holds *every* key prod needs; `vault/homo/.env` likewise.

- ✅ Kills the routing ambiguity entirely (see §7) — a flat local maps to one
  flat env vault, `push`/`pull`/`Classify` are today's code parameterized by env
  path. Smallest, most on-ethos change.
- ✅ R2 free (envs physically separate), R1/R6 unchanged.
- ❌ **Violates R4 for common keys.** Adding a key shared by all envs means
  touching every env vault. Forgetting one env is exactly the "forgot to
  propagate" bug envkeep fights — now across envs instead of worktrees.
- Mitigation without a real layer: `pull --from <env>` to seed a new env from an
  existing one; `push --to <env>[,<env>…]` / `--all-envs` to fan a common change
  out explicitly. Propagation stays a conscious action, not automatic.

### Model B — shared base + per-env overlay (inheritance)

A `shared` pseudo-env holds common keys; each env overlays only what differs.
`effective = shared ⊕ env`. Mirrors the override model (D9) one level up.

- ✅ **R4 for common keys:** change a shared key once, every env sees it.
  Structurally kills the cross-env propagation gap.
- ✅ Compact vaults (no duplicated commons), matches the tool's philosophy.
- ❌ Introduces the **push-routing problem** (§7): a flat local `.env` carries no
  layer annotation, so on push envkeep cannot *know* whether an edited/new key
  belongs to `shared` or to the env overlay. Needs an explicit routing rule.
- ❌ New mental model + likely a `promote`/`--shared` verb.

### Recommendation (phased, mirrors how D12 pre-wired multi-file)

Ship **Model A's on-disk layout and semantics first** — it fully solves the
*stated primary pain* (different values per env) and is a minimal, robust
delta — **but choose the path layout so the `shared` layer drops in with no
migration**, exactly as D12 named the vault after the file so multi-file needs
no migration. Concretely: reserve `vault/<env>/…` now and reserve a `shared`
(or `_base`) env name; if/when R4's common-key pain is felt, add the `shared`
layer as `vault/shared/…` + one composition change + the routing rule — no
vault move.

This is the on-ethos answer (ship smallest robust thing, trigger the rest). **But
R4 is a real philosophical hit**, so DP1 was put to the maintainer.

**DECIDED (§0): Model A.** Independent per-env vaults; no `shared` layer built.
Layout kept B-compatible so the layer is a zero-migration add if the common-key
duplication pain (R4) materializes — recorded as D24's reconsider-trigger.

## 5. On-disk layout changes

### Vault path (`internal/vault/vault.go:35`)

Today: `<common>/envkeep/vault/<envFilename>` (e.g. `vault/.env`).

Env-aware: `<common>/envkeep/vault/<env>/<envFilename>` (e.g. `vault/prod/.env`).
The issue's preferred layout; composes with future multi-file (each env dir can
hold several tracked filenames — filename × env stays orthogonal). Preferred
over the `.env.<env>` suffix because a directory keeps the two dimensions clean
and avoids suffix collisions with real filenames like `.env.local`.

`vault.Path` gains an `env` parameter (or a sibling `PathForEnv`); the legacy
2-arg form maps to the unnamed/legacy env for back-compat (§9).

### Config (`internal/config/config.go`) — new keys

Flat `KEY=VALUE`, parsed by the existing `envfile.Parse`. Add:

| key | meaning | default |
|-----|---------|---------|
| `default_env`   | env used when a worktree has no active env yet | *(unset → prompt for `--env`, E6)* |
| `cascade` | `use` fans out to all worktrees (issue comment) | `false` (safer — DP4/comment) |

`Config` struct grows `DefaultEnv string`, `Cascade bool`. Config is read/written
through the `envkeep config <get|set|list|unset>` subcommand (git-config style,
§0/D29), not hand-edited.

**No `environments` list key** (DP3, §0): the set of environments is discovered
live from the vault directories (`vault/*/`), the D4-style single-source-of-
truth — the filesystem is the registry, like `.git/refs/heads/` for branches. An
env exists iff `vault/<env>/` exists; `envs` (or `status --all-envs`) walks
those dirs. This avoids config↔disk drift and needs no declared-set bookkeeping.
Reserved names (rejected at *create* time): `shared`/`_base` (future Model B
layer), empty, and anything not filesystem-safe (E1).

### Marker (`internal/state/state.go`) — env-keyed

The marker records the last sync for *the env the local file currently holds*.
It must record **which env** that is, or `check`/`status` compare against the
wrong vault.

Chosen shape (back-compat friendly, one file): add an `env` field to `Marker`.

```go
type Marker struct {
    Env        string      `json:"env,omitempty"`   // env the local file holds; "" = legacy/default
    Base       envfile.Env `json:"base"`
    LocalMTime int64       `json:"local_mtime"`
    VaultMTime int64       `json:"vault_mtime"`
}
```

- Old markers lack `env` → unmarshal to `""` → interpreted as legacy/default env
  (same tolerance as D22's schema evolution — no migration needed).
- `LoadStat`'s mtime fast path is unchanged; `VaultMTime` now means "the *active
  env's* vault mtime." A single base suffices: switching envs rewrites the local
  file wholesale and the marker with it, so we never need to remember a
  non-active env's base (it would be stale anyway; a later `Classify` re-derives
  correctly — worked example in §8).

*Alternative rejected:* one marker file per env (`envkeep.base.<env>`) — needed
only if a worktree must remember several envs' bases at once; single-active-env
per worktree makes that unnecessary. Reconsider if the local file ever holds
more than one env simultaneously.

## 6. Environment selection — precedence & the active-env question (DP2)

**Which env a command targets:**

```
--env FLAG  >  worktree active env (marker.Env)  >  repo default_env (config)  >  legacy unnamed
```

**DP2 — is "active env" per-worktree or repo-wide?** Two coherent models:

- **Per-worktree active env** (recommended): worktree `hotfix/` runs `prod`,
  `feature/` runs `homo`, simultaneously. This is arguably the *point* — different
  worktrees for different targets. Active env stored in `marker.Env`; a worktree
  with no marker falls back to `default_env`.
- **Repo-wide active env:** one env for the whole repo (stored in config). Matches
  the switch comment's "flip everyone" framing but forbids concurrent per-worktree
  envs.

These interact with `use` (§10). **DECIDED (§0/DP2): per-worktree active env**,
with `default_env` as the repo-wide fallback for unset worktrees, and `use` as
the *explicit* repo-wide override that fans out. This keeps both use cases and
mirrors git worktrees, which already each carry their own HEAD.

## 7. The push-routing problem (the hard part — only under Model B)

A pulled local `.env` is **flat**: it has lost the shared-vs-overlay distinction.
On `push --env prod` (after stripping override, D9) we hold a flat set `S` and
must split it back into `(new-shared, new-prod-overlay)`. envkeep cannot read
intent for two cases:

1. **A new key** in `S`, absent from both layers: shared (all envs) or prod-only?
2. **An edited key** that currently lives in `shared`: did the user mean "change
   it for all envs" or "fork it to prod only"?

The override model (D9) escapes this because override keys are a *separate
physical file* — envkeep knows them by presence. There is no physical separation
inside one flat `.env`, so Model B **must** define a routing rule. Options:

- **B-rule-1 "preserve homes + value inference"** (recommended for B): a key keeps
  its current layer. On push env=prod: if a key's local value **equals** the
  shared value, it stays shared (not forked); if it differs (or the key is absent
  from shared), it is written to the prod overlay. New keys default to the env
  overlay (env-specific by default → satisfies R2). To create/change a key **for
  all envs**, an explicit `push --shared` or `envkeep promote KEY`. This resolves
  the common case automatically and only asks for intent on the genuinely
  ambiguous "make this global" action.
- **B-rule-2 "explicit shared file":** the user maintains commons via a separate
  file/flow; normal push only ever writes the env overlay. Simpler code, more
  user bookkeeping.

**Model A has no routing problem** — this entire section is why A is the smaller
step and why B is trigger-gated in the recommendation.

Reused-engine note (R6): under either model, `envfile.Classify` runs on the
**effective** env set (`shared ⊕ env` for B, or the env vault for A) vs local vs
base — unchanged. Only push's *write-back* (decompose `S` into layers) is new,
and only under B.

## 8. Command semantics (per environment)

Let `E` = the resolved target env (§6). `V(E)` = the vault set for `E` =
contents of `vault/E/.env` (Model A / DP1). Any command that targets an env
first **validates that `E` exists** (`vault/E/` present) unless `--create`/`-c`
is given (DP3/E1); an unknown env without `--create` errors.

- **`push [--env E] [--dry-run]`** — merge local (minus override) into `E`.
  Union semantics (D8) and refuse-on-Behind/Conflict (D20) unchanged, judged
  against `V(E)` and `marker.Base`. Model B adds layer decomposition (§7) before
  the write. Writes `vault/E/.env`, saves marker with `Env=E`.
- **`pull [--env E] [--dry-run]`** — write `V(E) ⊕ override` into local (D9/D11
  order+comment preservation unchanged). If the worktree's `marker.Env != E`
  (switching env), it is **not** a conflict — it's a full re-point: rewrite local
  to `E`, save marker `Env=E`. Guards still protect *unpushed* work in the
  current env (Ahead/Conflict vs `marker.Env`'s vault) — see edge E4.
- **`status [--env E]`** — default: one line per worktree showing its **active
  env** + state vs that env's vault; `--env E` forces the comparison env for
  every worktree. The header lists the existing environments (+ default). Absent
  env vault → `absent`. (`--all-envs`, the full worktree × env matrix, is phase 2
  — a true matrix needs a per-env base the marker does not store today.)
- **`check`** — quiet per-worktree drift for the hook. Uses `marker.Env` (the
  active env) to pick the vault; mtime fast path unchanged (stats the active
  env's vault). Shell snippet (`internal/hook`) needs **no change**: it only
  stats the local `.env` filename, which does not vary by env.

### Worked example — env switch is Behind, not Conflict (validates single-base)

Worktree on `prod`, in sync: `marker={Env:prod, Base:P}`, local=`P`, vault/prod=`P`.
User runs `pull --env homo` (vault/homo=`H`). Selection → `E=homo`. Marker's
`Env=prod ≠ homo`: this is a re-point. We compare against `homo`'s reality:
`Classify(base=P, local=P, vault=H)` → local==base → **Behind** → pull applies
`H`, saves `marker={Env:homo, Base:H}`. No stale-base false conflict. Switching
back to prod later: `Classify(base=H, local=H, vault=P)` → **Behind** → pulls
`P`. Correct both ways with a single stored base. ✅

*(If local had unpushed prod edits — local `P'≠P` — the switch must not silently
discard them: see E4.)*

## 9. Back-compat & migration (R3, DP4)

**Legacy repos (no env ever created) change nothing.** `vault.Path` keeps
returning `vault/<envFilename>`; marker `Env=""`; output identical. This is the
R3 guarantee and must be covered by a golden test that a pre-env repo's
`status`/`push`/`pull` output is byte-identical.

**Opting in** = the first env creation (`use -c <env>`). DP4
decides how the legacy flat `vault/<file>` becomes an env vault. **DECIDED: (a)
opt-in migration.** On that first create, move `vault/<file>` →
`vault/<default_env>/<file>` (atomic `rename`, same dir) with a one-line notice;
single clean code path afterward. Requires `default_env` to be set at that point
(the first-created env is a natural default). Rejected: (b) dual-path forever
(two code paths, `status --all-envs` special-cased) and (c) lazy read-through
(middle ground, still two paths transiently). Migration is reversible (it is one
`rename`).

## 10. `use` — switch the active environment (+ opt-in cascade)

`envkeep use <env>` — switches the active env (must exist, DP3); `use -c <env>`
creates then switches (git-`checkout -b` shape). On the per-worktree model (DP2),
`use` in a worktree flips that worktree's `marker.Env`; with cascade on it also
updates the repo `default_env` and fans out. (The issue #3 comment called this
`set`; renamed to `use` so `set` stays exclusive to `config set` — see §0/DP9.)

- **`cascade=false`** (default): only flips the active env (repo
  `default_env`, and/or the current worktree's `marker.Env`). Other worktrees
  pick it up lazily on next `pull`. No mass write.
- **`cascade=true`**: walks every worktree (`git.Worktrees`, same as
  `status`) and pulls `<env>` into each local file. One command → whole repo on
  one env.
- **Safety (mirrors pull guards, R1):** a worktree that is `ahead`/`conflict` for
  its *current* env is **skipped + reported**, never clobbered.
- **`--dry-run`:** preview which worktrees would change, write nothing.

**Scoping:** the cascade fan-out is a *second* feature phase — the core (`--env`
on push/pull/status + `use` + default_env + marker.Env) delivers the primary pain
relief without it. Build cascade after the core lands and is validated.

**Command architecture (D6 → D29): docker-style hybrid.** Environment is
envkeep's *primary* domain, so its operations are **top-level verbs** (like
`docker ps`/`run`, not `docker container ps`) — a `env` noun-group prefix would
be redundant with the tool's own name. Only the *secondary* domain, config, is a
noun group (like `git config`). Surface:

```
envkeep use <env> [-c]        switch active env (-c: create + switch)
envkeep envs                  list environments (the vault/*/ dirs)
envkeep rm <env>              delete an environment (guarded — E5)
envkeep push|pull [--env e] [-c] [--dry-run]
envkeep status [--all-envs]
envkeep check   ·   hook <zsh|bash>   ·   version
envkeep config <get|set|list|unset>   # the one noun group (git-config style)
```

`set` is **not** a top-level verb — it belongs only to `config set` (avoids the
overload). This mix (flat primary verbs + one grouped secondary domain + shell
completions) is docker's exact shape and is what fires D6's reconsider-trigger,
so **adopt `cobra`** (D29). `rm` is destructive (E5) — guard it (confirm /
`--force`).

## 11. Ambiguities & edge cases (the checklist)

- **E1 — env name validation (DP3, git-branch model).** Targeting an env
  (`--env prod`, `use prod`, `pull --env prod`) **requires it to already exist**
  (`vault/prod/` present); an unknown env errors like `git checkout nonexistent`
  — this is what kills the typo-creates-silently hazard. **Creating** an env is
  a distinct, explicit act: `--create`/`-c` (mirrors `git checkout -b`) or
  `use -c <name>`. Reserved names rejected at create time:
  `shared`/`_base` (future Model B), empty. Charset must be filesystem-safe
  (`[A-Za-z0-9._-]`, no `/`, no `..`) since it becomes a directory name. The env
  set for iteration (`status --all-envs`, `envs`) is `vault/*/` on disk (§5).
- **E2 — first push to a brand-new env.** No vault yet = "fresh" (like today's
  first push). Optionally seed from another env: `push`/`pull --from <env>`.
- **E3 — override × env.** Override stays single & env-agnostic in v1 (DP5). A key
  present in override shadows the env value for that worktree (intended — it's
  machine-local). Reconsider per-env overrides only on real need.
- **E4 — switching env with unpushed local edits.** `pull --env other` while the
  current env is `Ahead` (unpushed local changes) must **refuse** (or require
  `--force`), not silently discard — same D20 spirit. The re-point in §8 applies
  only when the current env is clean/behind.
- **E5 — deleting/renaming an env.** `env rm <env>` deletes `vault/<env>/` —
  destructive, so guard it (confirm / `--force`) and **refuse** if any worktree's
  `marker.Env` still points at it (would strand them → E7). Rename = create new +
  migrate + rm old; a single `rename` is later polish. Never auto-delete.
- **E6 — `default_env` unset but envs exist.** Commands with no `--env` and no
  `marker.Env`: error asking for `--env` or to set a `default_env`, not a guess.
- **E7 — a worktree's `marker.Env` names an env that no longer exists** (its
  `vault/<env>/` was removed). Treat as unsynced/needs re-point; `status` flags it
  (e.g. `env 'x' gone — pick another`).
- **E8 — mixed-env `status` output width.** Env name column must not break the
  existing aligned layout; budget a column.
- **E9 — legacy marker (`Env:""`) in a now-multi-env repo.** After migration (§9a)
  the default env's worktrees have `Env:""` markers; treat `""` == `default_env`
  so they read as clean, then normalize `Env` on next write.
- **E10 — the shell hook guard cache.** Layer-1 guard keys on `$PWD` + local
  mtime only; env switches rewrite the local file (mtime moves) so the guard
  correctly re-checks. No hook change needed — verify with a test.
- **E11 — `check --porcelain`** (prompt segment): should it surface the active
  env (e.g. `prod:behind`)? Nice-to-have; decide with the starship segment work
  (STATUS open item). Keep bare token by default.
- **E12 — concurrent worktrees on the same env** pushing: unchanged from today —
  union merge + refuse-on-behind already serialize safely per vault (D8/D20),
  now per env vault.

## 12. Forks — all resolved (decision log)

| # | Decision | Options | Outcome |
|---|----------|---------|---------|
| **DP1** | Sharing model | A independent per-env / B shared+overlay | ✅ **A** — independent; layout pre-wired for B; B trigger-gated (§0, §4) |
| **DP2** | Active-env scope | per-worktree / repo-wide | ✅ **per-worktree** (`marker.Env` = worktree HEAD) + `default_env` fallback (§0, §6) |
| **DP3** | Env set | declared / open / **git-branch** | ✅ **git-branch** — target validates existence, `--create`/`-c` to make new; set = `vault/*/` on disk (§0, E1) |
| **DP4** | Legacy vault migration | (a) migrate / (b) dual-path / (c) lazy | ✅ **(a) opt-in migrate** on first env create (§9) |
| **DP5** | Override per-env? | single env-agnostic / per-env | ✅ **single, env-agnostic**; per-env trigger-gated (E3) |
| **DP6** | cascade fan-out timing | core / phase 2 | ✅ **phase 2** (§10, §15) |
| **DP7** | Local layout | swap single file / coexist / hybrid | ✅ **swap, single tracked file** (§0) |
| **DP8** | Multi-file (N files × M envs) | yes / no / deferred | ✅ **no — deferred to D12** (§0) |
| **DP9** | Command architecture | flat / noun-grouped / docker-hybrid | ✅ **docker-hybrid** — top-level env verbs (`use`/`envs`/`rm`) + `config` group + cobra; switch verb `use`, `set`=config only (D29 revises D6) |

All forks resolved. Ready to materialize DECISIONS + ROADMAP and implement (§15).

## 13. Proposed DECISIONS entries (ready to commit — all forks resolved)

- **D23** — Environment is a dimension between vault and override. In this design
  (Model A) `effective = env ⊕ override`; the stack reserves room for a future
  `shared` layer (`shared ⊕ env ⊕ override`, D24 trigger). Layout
  `vault/<env>/<file>`; orthogonal to multi-file (D12). Local side = one tracked
  file that **swaps** on env switch (not coexisting per-env files).
- **D24** — Sharing model = **A, independent per-env vaults** (DP1). Layout
  `vault/<env>/<file>` pre-wired so a future `shared` layer (Model B) is a
  zero-migration add. Reconsider-trigger: re-declaring common keys across envs
  becomes a real maintenance pain (then add `vault/shared/…` + §7 routing rule).
- **D25** — Active env is per-worktree (`marker.Env`, the worktree's HEAD
  analogue), `default_env` fallback; selection precedence
  `--env > marker.Env > default_env > legacy`.
- **D26** — **git-branch env model** (DP3): the env set is discovered live from
  `vault/*/` (no `environments` config key, D4 ethos); targeting validates
  existence; `--create`/`-c` creates. Reserved/filesystem-safe names only.
- **D27** — Opt-in migration of the legacy flat vault; `Env:""` == legacy/default;
  no change for never-opt-in repos (R3).
- **D28** — `use` + `cascade` (default false), skip-not-clobber ahead/
  conflict worktrees, `--dry-run`. Cascade fan-out is phase 2 (after the core).
- **D29 (revises D6)** — docker-style hybrid CLI on `cobra`: environment ops are
  top-level verbs (`use`, `envs`, `rm`) since env is the primary domain (no
  redundant `env` prefix), and only config is a noun group
  (`envkeep config <get|set|list|unset>`, git-config style). Switch verb is
  `use`; `set` is reserved for `config set`. Fires D6's own reconsider-trigger
  (grouped subcommands + shell completions).
- **D30** — single, env-agnostic override (DP5): one `.env.override` per worktree,
  applied under every env; per-env overrides trigger-gated.

## 14. Test / fixture plan (D18, D21)

- Extend `scripts/mkfixture.sh` with: multi-env repo (prod/homo values), env
  switch, cross-env `status --all-envs`, legacy→multi-env migration, first push
  to a new env, override × env, and the R3 byte-identical legacy golden.
- New golden outputs under `testdata/golden/` (+ `-update`).
- Coverage tiers (D21): `config` ≥80, `vault` ≥85, `state` ≥80 must hold with the
  new fields; add unit tests for `PathForEnv`, live `vault/*/` discovery,
  env-existence validation + `--create` (git-branch model), and marker `Env`
  round-trip + legacy (`Env:""`) tolerance.

## 15. Phasing (proposed build order)

1. **Core env dimension (Model A)** — no cobra needed yet, just flags on the
   existing verbs: `vault.PathForEnv` + live env discovery from `vault/*/`;
   `marker.Env` + legacy (`Env:""`) tolerance; `--env` (+ `--create`/`-c`) on
   push/pull/status; env-existence validation (git-branch model); selection
   precedence; `check` uses `marker.Env`; opt-in migration (§9a); config
   `default_env`/`cascade` parse; `status` shows each worktree's active env +
   state (with `--env` to force a comparison env); fixtures + goldens + R3
   back-compat golden. ✅ **DONE (2026-07-14):** all layers, lint clean,
   coverage tiers pass (total 84.1%), driven end-to-end against the real binary.
2. **CLI restructure → cobra, docker-hybrid (D29):** top-level env verbs (`use`,
   `envs`, `rm`) + the one `envkeep config <get|set|list|unset>` group + shell
   completions. Also `status --all-envs` — the full worktree × environment
   matrix, deferred from step 1 because a true matrix needs a per-env base the
   marker does not store today. (Can precede or follow step 1's flags, but must
   land before the surface grows further.)
3. **`use` cascade fan-out** (§10) with `--dry-run` and skip-not-clobber (phase 2,
   D28).
4. **(trigger-gated) `shared` layer (Model B):** compose `shared ⊕ env`; push
   routing rule (§7 B-rule-1) + `promote`/`--shared`; only if the common-key
   duplication pain (D24 trigger) materializes.
