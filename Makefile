# envkeep — developer tasks.
# Tools install into ./bin so nothing pollutes go.mod or the global environment.

BIN                    := $(CURDIR)/bin
GOLANGCI_LINT_VERSION  := v2.12.2
LEFTHOOK_VERSION       := v2.1.10
GORELEASER_VERSION     := v2.17.0
GOTESTCOVERAGE_VERSION := v2.18.8
GOLANGCI_LINT          := $(BIN)/golangci-lint
LEFTHOOK               := $(BIN)/lefthook
GORELEASER             := $(BIN)/goreleaser
GOTESTCOVERAGE         := $(BIN)/go-test-coverage
export PATH            := $(BIN):$(PATH)

VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -X github.com/carvalhosauro/envkeep/internal/buildinfo.Version=$(VERSION)

.PHONY: all setup tools hooks tidy fmt fmt-check lint vet test bench loadtest cover cover-check cover-html build install release-check snapshot demos clean help

## all: format, lint, test (default local loop)
all: fmt lint test

## setup: one-shot dev environment bootstrap (tools + git hooks + tidy)
setup: tools hooks tidy
	@echo ">> dev environment ready — run 'make all'"

## tools: install pinned dev tools into ./bin
tools: $(GOLANGCI_LINT) $(LEFTHOOK)

$(GOLANGCI_LINT):
	@mkdir -p $(BIN)
	@echo ">> installing golangci-lint $(GOLANGCI_LINT_VERSION) -> $(BIN)"
	@curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh \
		| sh -s -- -b $(BIN) $(GOLANGCI_LINT_VERSION)

$(LEFTHOOK):
	@mkdir -p $(BIN)
	@echo ">> installing lefthook $(LEFTHOOK_VERSION) -> $(BIN)"
	@GOBIN=$(BIN) go install github.com/evilmartians/lefthook/v2@$(LEFTHOOK_VERSION)

## hooks: install git hooks via lefthook
hooks: $(LEFTHOOK)
	@git config --unset core.hooksPath 2>/dev/null || true
	@$(LEFTHOOK) install
	@echo ">> git hooks installed (lefthook)"

## tidy: sync go.mod/go.sum
tidy:
	go mod tidy

## fmt: format code (gofumpt + goimports via golangci-lint)
fmt: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) fmt

## fmt-check: fail if code is not formatted (CI gate)
fmt-check: $(GOLANGCI_LINT)
	@diff="$$($(GOLANGCI_LINT) fmt --diff 2>/dev/null)"; \
	if [ -n "$$diff" ]; then echo "$$diff"; echo ">> not formatted — run 'make fmt'"; exit 1; fi

## lint: run all configured linters
lint: $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) run

## vet: go vet only (subset of lint; kept for quick checks)
vet:
	go vet ./...

## test: race-enabled test run
test:
	go test -race ./...

## bench: run load-test benchmarks (scale via ENVKEEP_LOADTEST_KEYS / _WT)
bench:
	go test -run '^$$' -bench . -benchmem ./internal/cli/

## loadtest: scale correctness + per-command timing (heavy; builds real git worktrees)
loadtest:
	ENVKEEP_LOADTEST=1 go test -run TestLoadScale -v ./internal/cli/

## cover: test with coverage profile + total summary
cover:
	go test -race -covermode=atomic -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out | tail -1

$(GOTESTCOVERAGE):
	@mkdir -p $(BIN)
	@echo ">> installing go-test-coverage $(GOTESTCOVERAGE_VERSION) -> $(BIN)"
	@GOBIN=$(BIN) go install github.com/vladopajic/go-test-coverage/v2@$(GOTESTCOVERAGE_VERSION)

## cover-check: enforce tiered coverage thresholds (.testcoverage.yml)
cover-check: cover $(GOTESTCOVERAGE)
	$(GOTESTCOVERAGE) --config=.testcoverage.yml

## cover-html: render coverage.html from the profile
cover-html: cover
	go tool cover -html=coverage.out -o coverage.html
	@echo ">> wrote coverage.html"

## build: build the CLI into ./bin (version stamped from git)
build:
	go build -ldflags "$(LDFLAGS)" -o $(BIN)/envkeep ./cmd/envkeep

## install: install envkeep into GOBIN (on PATH via your Go toolchain)
install:
	go install -ldflags "$(LDFLAGS)" ./cmd/envkeep

$(GORELEASER):
	@mkdir -p $(BIN)
	@echo ">> installing goreleaser $(GORELEASER_VERSION) -> $(BIN)"
	@GOBIN=$(BIN) go install github.com/goreleaser/goreleaser/v2@$(GORELEASER_VERSION)

## release-check: validate .goreleaser.yaml
release-check: $(GORELEASER)
	$(GORELEASER) check

## snapshot: dry-run a release build (no publish, no tag needed)
snapshot: $(GORELEASER)
	$(GORELEASER) build --snapshot --clean --single-target

## demos: re-record the README gifs with vhs (see demo/README.md)
demos: build
	./demo/record.sh

## clean: remove build + coverage artifacts (tools kept)
clean:
	rm -rf coverage.out coverage.html $(BIN)/envkeep

## help: list targets
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed 's/## //'
