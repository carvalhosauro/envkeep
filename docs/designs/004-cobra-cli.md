# Design 004 — Cobra CLI + docker-hybrid surface (issue #3, phase 2)

> Status: **PLAN — decided, pending implementation.** Implements D29 (adopt
> `cobra`, docker-style hybrid) — step 2 of ROADMAP Phase 1.5 (named
> environments). Forks resolved with the maintainer 2026-07-14. No code yet.
>
> Read `docs/designs/003-named-environments.md` and DECISIONS D23–D30 first.

## 1. Goal

Swap the stdlib `flag` dispatch (`cmd/envkeep/main.go`) for `cobra`, and expose
the docker-style hybrid surface (environment ops as top-level verbs, config as a
noun group) plus shell completions. Existing commands keep identical behavior —
same names, flags, output, exit codes (the golden tests guard this).

## 2. Decisions taken (this session)

- **Command definitions live in `internal/cli`, not `cmd/envkeep`.** `cmd/envkeep`
  is excluded from coverage (D21); putting the cobra layer there would leave it
  unmeasured. So `internal/cli` owns the `*cobra.Command`s (+ an `Execute()`),
  and `cmd/envkeep/main.go` shrinks to `os.Exit(cli.Execute())`. → D31.
- **`use` re-points the current worktree only** — it sets that worktree's
  `marker.Env` (reusing pull's re-point path). It does **not** touch the repo
  `default_env` (that changes only on first env creation, D27). Repo-wide fan-out
  is the `cascade` behavior (D28), deferred to 2d.
- **Sub-phasing:** 2a (port existing verbs to cobra + completions) is the first
  cut; 2b (`use`/`envs`/`rm` + `config` group) and 2d (`use` cascade, `status
  --all-envs`) follow.

## 3. First cut — 2a (port + completions), scope

Framework swap with **no behavior change**, plus completions. No new env verbs
yet.

- Root `envkeep` + subcommands: `status`, `push`, `pull`, `check`, `hook`,
  `version` — each a thin `*cobra.Command` wrapping the existing
  `cli.Status/Push/Pull/Check` / `hook.Snippet` logic.
- Flags via `pflag`, preserved 1:1: `--file`, `--env`, `-c`/`--create`
  (push/pull), `--dry-run` (push/pull), `--porcelain` (check).
- `envkeep completion <bash|zsh|fish>` (cobra built-in) **plus dynamic `--env`
  completion** — a `ValidArgsFunction`/flag-completion backed by
  `vault.Environments` (complete existing env names). Useful already for the
  ported `--env` flag.
- Exit codes preserved: map cobra's error return to `exitError`/`exitUsage`
  (`cmd/envkeep` keeps the mapping, or `cli.Execute()` returns an int).
- Golden outputs of the ported verbs **must not change** (guard).

## 4. Later cuts

- **2b** — the docker-hybrid env surface: `use <env> [-c]` (re-point, §2),
  `envs` (list `vault/*/`), `rm <env>` (delete, guarded — refuse if a worktree's
  `marker.Env` points at it; confirm/`--force`, E5), and `config
  <get|set|list|unset>` (env_file / default_env / cascade).
- **2d (deferred)** — `use` cascade fan-out (D28, needs the `cascade` flag +
  worktree walk + skip-not-clobber); `status --all-envs` matrix (needs a per-env
  base the marker does not store today — marker schema change, or a limited
  clean/differs view).

## 5. Command surface (full target)

```
envkeep status [--env E]              # ported (2a)
envkeep push  [--env E] [-c] [--dry-run]
envkeep pull  [--env E] [-c] [--dry-run]
envkeep check [--porcelain]
envkeep hook  <zsh|bash>
envkeep version
envkeep completion <bash|zsh|fish>    # 2a

envkeep use <env> [-c]                # 2b — re-point current worktree
envkeep envs                          # 2b — list environments
envkeep rm  <env> [--force]           # 2b — delete env (guarded)
envkeep config <get|set|list|unset> [key] [value]   # 2b

# 2d: status --all-envs ; use cascade (config `cascade=true`)
```
`set` is **not** a top-level verb — reserved for `config set` (D29).

## 6. Mechanics

- **Dependency:** `github.com/spf13/cobra` (+ `spf13/pflag`) enters `go.mod` —
  the first runtime dependency (D6 was stdlib; D29 accepted the trade for the
  grouped surface + completions). Pin a version; keep it the only CLI dep.
- **Layout:** `internal/cli/root.go` (`Execute()` + root command + persistent
  flags), one file per command group (`cmd_sync.go` for push/pull/status, etc.)
  or per verb — decide during 2a. Commands call the already-tested logic funcs.
- **Version:** `rootCmd.Version = buildinfo.Version` (cobra `--version`), or keep
  an explicit `version` subcommand — decide in 2a (keep output stable).

## 7. Testing

- The function-level tests (`cli.Status/Push/Pull/Check`, `hook`, …) stay — they
  already cover the behavior.
- Add a few **root-command integration tests**: construct the root, execute with
  argv, assert output + exit. Command defs living in `internal/cli` count toward
  coverage (the D31 reason).
- Ported verbs' golden outputs unchanged.

## 8. Back-compat

Same command names, flags, and output as today; the shell hook (`envkeep check`)
is untouched; `--file`/`--env`/`--dry-run`/`--porcelain` all preserved. A user
upgrading sees no difference until they reach for the new (2b) verbs.

## 9. Open items (resolve during 2a)

- `envkeep` with no args: cobra shows help by default — decide exit code (old was
  exit 1 on no command; keep or accept cobra's 0).
- cobra version to pin in `go.mod`.
- Whether `hook`'s arg (`zsh|bash`) becomes a cobra positional with validation.

## 10. Proposed DECISIONS

- **D31** — cobra command definitions live in `internal/cli` (with `Execute()`),
  `cmd/envkeep/main.go` stays a thin entrypoint; rationale: testability +
  coverage (cmd/envkeep is excluded, D21). `use` re-points the current worktree
  only; repo-wide fan-out is `cascade` (D28).
