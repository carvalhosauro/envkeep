# STATUS.md — where we are and how we got here

Any agent or session: **read this first.** It states the current phase, what is
done, what is next, and a dated log of how the project reached this point.
Append to the log at the end of any working session.

## Current phase

**Phase 0 (Design & docs) — complete. Phase 1 (v1 MVP) — in progress
(steps 1–2 of 6 done).**

Repo initialized (`git`, `go mod` = `github.com/carvalhosauro/envkeep`, Go
1.26). DX toolchain in place (D19). Golden-set fixture generator and the core
`internal/envfile` package (parser + merge + diff + 3-way classify) are done and
tested (envfile 96.1% coverage). The design is settled and every decision is
recorded in [`DECISIONS.md`](DECISIONS.md).

## Done

- Problem and scope fully defined; non-goals fenced with explicit triggers.
- All design decisions recorded with why + rejected alternatives +
  reconsider-triggers (D1–D18).
- Doc set written: `README.md`, `AGENTS.md`,
  `docs/{DECISIONS,DESIGN,ROADMAP,STATUS}.md`.
- Name locked: **envkeep** (verify channel availability before first release).

## Next action

Continue Phase 1 at build step 3 — see [`ROADMAP.md`](ROADMAP.md):

3. `internal/git` — common dir resolved absolute (D13), worktree list from
   `--porcelain` (D4), per-worktree gitdir. First package that uses real git;
   test it with `scripts/mkfixture.sh` (normal + bare).
4. `internal/vault` — `VaultStore` interface + flatfile impl, atomic write.
5. `internal/state` + `internal/cmd` — base marker, status/push/pull, mtime cache.
6. `internal/hook` — shell snippet emitter.

Done: step 1 (`scripts/mkfixture.sh`, normal + bare, verified), step 2
(`internal/envfile`, 96.1% coverage).

## Open items to settle at implementation time

- Exact override filename (e.g. `.env.override` vs `.env.local` — note the
  clash if `.env.local` is also the *tracked* file; pick a distinct override
  name).
- `.env` parser multiline-value behavior (reject vs best-effort) — decide in
  `internal/envfile`.
- Hash function for the base marker's `vault_hash` (any stable content hash;
  SHA-256 fine).
- Go module path / binary distribution details.

## Log — how we got here

*Newest last. Each entry: date · what changed · why it matters.*

- **2026-07-12 · Initial design session.** Started from a written spec: Go CLI
  to sync `.env` across git worktrees, targeting the *daily* "forgot to
  propagate to other worktrees" pain (not the well-solved new-worktree-copy
  case). Spec already fenced off encryption, daemon, SQLite, and team/remote as
  v2/v3 with triggers.
- **2026-07-12 · First Q&A round.** Settled: CLI framework = stdlib `flag` (D6);
  shell hook = `chpwd`/cd-trap + mtime guard, not git post-checkout (D7); `push`
  = union merge to avoid clobbering keys only in another worktree (D8); testing
  = real git + bash fixture (D18). Introduced the override composition rule
  (D9). Initially proposed then **dropped** the per-worktree sidecar as
  premature.
- **2026-07-12 · Second round — conflict & cache.** Taking conflict detection
  seriously forced a 3-way merge, which structurally needs a **base**. The
  dropped sidecar was **reinstated** (D5) and now does double duty: conflict
  detection + mtime cache. This reversal is recorded on purpose so it isn't
  re-dropped for the same wrong reason. Name evolved `envwt` → `envkeep` (D10):
  `envwt` caged the name to the worktree feature; the name should outlive
  worktree-only scope.
- **2026-07-12 · Third round — bare repos & config.** Confirmed the tool works
  with bare-repo `.bare/` layouts *because* it keys off `git-common-dir`, which
  resolves to `.bare` — with the caveat that the common dir must be resolved
  **absolute** to avoid the relative-path gotcha (D13). Env filename made
  configurable, single-file for now, with the vault named after the file as a
  zero-cost seam for future multi-file (D12). Order/comment preservation on
  write confirmed (D11).
- **2026-07-12 · Docs-first checkpoint.** Wrote the full doc set (this file plus
  README/AGENTS/DECISIONS/DESIGN/ROADMAP) before any implementation, so phase
  and rationale survive across sessions and agents. Phase 0 complete.
- **2026-07-12 · DX scaffold (D19).** `git init` + `go mod init` (Go 1.26,
  module `github.com/carvalhosauro/envkeep`). Added Makefile, `.golangci.yml`
  (golangci-lint v2, pinned v2.12.2, lint+format in one tool, installed to
  `./bin` to keep `go.mod` clean), native git hooks in `.githooks/`, CI workflow
  running the same make targets, `.editorconfig`, `.gitignore`, `docs/DX.md`.
  Consistency enforced before the first line of app code. Added a smoke package
  (`internal/buildinfo` + test) and a minimal `main.go` entrypoint (wires
  `buildinfo`, handles `envkeep version`) so every gate is provably green on an
  otherwise codeless repo — golangci-lint/`go test` both error on zero packages,
  which would have made CI red on the first commit. Verified: `make
  fmt-check/lint/cover/build` all green, `.githooks/pre-commit` passes
  end-to-end. Scaffold staged, not committed (awaiting your go-ahead).
- **2026-07-12 · Hook manager switched to lefthook (D19 revised).** Replaced the
  native `.githooks/` scripts with `lefthook` v2.1.10 (Go single binary, pinned
  in `./bin` via `go install …/v2@version`, so `go.mod` stays clean). Config in
  `lefthook.yml`: pre-commit formats + re-stages staged Go then lint+test;
  pre-push race-test + build. Verified `bin/lefthook run pre-commit` green
  (format/lint/test). D19 status = REVISED with the native-hooks history kept.
- **2026-07-12 · Phase 1 steps 1–2.** `scripts/mkfixture.sh` builds throwaway
  real-git repos (normal + bare `.bare/`) with linked worktrees and prints their
  paths + resolved common dir; verified bare resolves common dir to `.bare` and
  normal to `main/.git` (D13 confirmed end-to-end). `internal/envfile`
  implemented: layout-preserving parser (export/quotes/escapes/inline comments,
  D11), `Union` + `ExcludeKeys` (D8/D9), `Diff`/`Delta`, and 3-way `Classify`
  returning Clean/Ahead/Behind/Diverged/Conflict with per-key conflict detail
  (D5). Pure, no git. Lint clean, 96.1% coverage. Note: `Diverged` (both changed
  but mergeable) is a refinement beyond the DESIGN table's binary
  both-changed→conflict; kept because it's strictly more precise.
