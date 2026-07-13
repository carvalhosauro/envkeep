# DX.md — developer environment

Consistency is enforced from day one so it never has to be retrofitted. See
[`DECISIONS.md`](DECISIONS.md) D19 for *why* this stack was chosen.

## One-time setup

```
make setup
```

Installs pinned tools into `./bin`, points git at the committed hooks, and runs
`go mod tidy`. Requires Go (see `go.mod`), `git`, `make`, and `curl`.

## Daily commands

| Command | Does |
|---------|------|
| `make all` | format → lint → test (the local loop) |
| `make fmt` | format code (gofumpt + goimports, via golangci-lint) |
| `make fmt-check` | fail if unformatted (what the hook/CI run) |
| `make lint` | run all configured linters |
| `make test` | `go test -race ./...` |
| `make cover` | tests + coverage profile + total % |
| `make cover-check` | enforce tiered coverage thresholds (`.testcoverage.yml`) |
| `make release-check` / `make snapshot` | validate / dry-run the release build |
| `make cover-html` | render `coverage.html` |
| `make build` | build the CLI into `./bin/envkeep` |
| `make help` | list all targets |

## The stack

- **Formatter + linter: `golangci-lint` v2**, pinned in the `Makefile`. v2 does
  *both* formatting (gofumpt + goimports as "formatters") and linting, so there
  is one tool, not three. Config: `.golangci.yml`.
- **Tools live in `./bin`, not `go.mod`.** golangci-lint drags a large
  dependency tree; installing it into the module would pollute `go.mod`/`go.sum`
  for a project whose whole ethos is minimal and inspectable. `make tools`
  installs a pinned binary into `./bin` (gitignored) instead.
- **Git hooks via `lefthook`** (Go single binary, pinned in `./bin`, config in
  `lefthook.yml`); installed by `make hooks` / `bin/lefthook install`:
  - `pre-commit`: formats staged Go files and re-stages them (`stage_fixed`),
    then lint + `go test`. Scoped to staged `*.go`, so docs-only commits skip it.
  - `pre-push`: `go test -race` + build.
  - Bypass once with `LEFTHOOK=0 git commit …` or `git commit --no-verify`.
- **CI** (`.github/workflows/ci.yml`) runs the *same* `make` targets with the
  *same* pinned linter version — CI and local can't drift. `git` is preinstalled
  on runners, so the real-git fixture tests will run there too.
- **`.editorconfig`** enforces tabs for Go, LF, trailing-whitespace trim.

## Bumping the linter

Edit `GOLANGCI_LINT_VERSION` in the `Makefile`, then `rm -rf bin && make tools`.
CI picks it up automatically (it calls `make tools`).
