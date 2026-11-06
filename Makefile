# ts-bridge — developer task runner.
#
# Scope: mutation testing (go-gremlins/gremlins) and the coverage gates.
# Build/test/lint live in CI (.github/workflows/ci.yml) and are run directly with
# `go build|test|vet`.
#
# `cover` / `cover-check` replaced a Codecov upload that had been dead for months: the
# workflow referenced a secret the repo does not define, `fail_ci_if_error: false` swallowed
# the failure, and `codecov.yml` tuned a 70% patch target for a service this project left.
# A threshold nobody can observe is not a gate, so it lives here now, where CI runs it.
# See #335.
#
# gremlins is a CI-only / local-only dev tool. It is installed via `go install`
# and is deliberately kept OUT of go.mod / go.sum to preserve the project's
# zero-dependency design goal.

# Pin the gremlins version so local and CI runs report identical mutants.
# Bump deliberately; do not silently track latest.
GREMLINS_VERSION ?= v0.6.0
GREMLINS_PKG     := github.com/go-gremlins/gremlins/cmd/gremlins
GOBIN            := $(shell go env GOPATH)/bin
GREMLINS         := $(GOBIN)/gremlins

# Coverage floor, in percent of statements. Set below the measured value so it bars
# regression rather than pretending to be an aspiration; override per run with
# `make cover-check COVER_MIN=nn`.
COVER_MIN ?= 60
COVERAGE  ?= coverage.out

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help.
	@grep -hE '^[a-zA-Z0-9_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

.PHONY: cover
cover: ## Print total and per-package statement coverage from a fresh run.
	go test -coverprofile=$(COVERAGE) ./...
	@printf 'per package:\n'
	@go test -cover ./... 2>/dev/null | sed -n 's#^ok[[:space:]]*\([a-zA-Z0-9/._-]*\).*coverage: \([0-9.]*\)%.*#  \2%  \1#p' | sort -n
	@printf 'total:\n'
	@go tool cover -func=$(COVERAGE) | tail -1

.PHONY: cover-assert
cover-assert: ## Assert an existing coverage profile meets COVER_MIN, without re-running tests.
	@test -f $(COVERAGE) || { echo "cover-assert: no $(COVERAGE) profile -- run 'make cover-check'"; exit 2; }
	@go tool cover -func=$(COVERAGE) | awk -v min=$(COVER_MIN) ' \
	    /^total:/ { \
	        gsub(/%/, "", $$3); \
	        if ($$3 + 0 < min + 0) { printf "cover-assert: %.1f%% is below the %.0f%% floor\n", $$3, min; exit 1 } \
	        printf "cover-assert: %.1f%% >= %.0f%% floor\n", $$3, min; \
	    }'

.PHONY: cover-check
cover-check: ## Run the suite with coverage, then assert the coverage floor.
	go test -coverprofile=$(COVERAGE) ./...
	@$(MAKE) --no-print-directory cover-assert

# Not wired into CI yet: the hygiene job lives under .github/ and is the maintainer's change (#575).
.PHONY: specs-check
specs-check: ## Check that every active spec's go test -run selector still names a test, then that guard's own suite.
	bash scripts/check-features-selectors.sh
	bash scripts/tests/test-features-selectors.sh

.PHONY: mutation-install
mutation-install: ## Install the pinned gremlins binary (does not touch go.mod).
	go install $(GREMLINS_PKG)@$(GREMLINS_VERSION)

.PHONY: mutation-dry
mutation-dry: mutation-install ## List the mutants gremlins would generate (no tests run).
	$(GREMLINS) unleash --dry-run

.PHONY: mutation
mutation: mutation-install ## Run mutation testing on the whole module (advisory; writes mutation-report.json).
	$(GREMLINS) unleash --output mutation-report.json

# Local e2e on throwaway `dotf vm` guests (#606). Not run in CI: the runners cannot
# nest a VM. Each target brings a fresh session up and always deletes it; run before
# a release with a breaking or host-facing change. See docs/runbooks/e2e-vm.md.
.PHONY: e2e
e2e: ## Run the Linux, then the Windows e2e scenario on throwaway VMs.
	scripts/e2e/run.sh linux
	scripts/e2e/run.sh windows

.PHONY: e2e-linux
e2e-linux: ## Run the e2e scenario on a throwaway Ubuntu VM.
	scripts/e2e/run.sh linux

.PHONY: e2e-windows
e2e-windows: ## Run the e2e scenario on a throwaway Windows 11 VM.
	scripts/e2e/run.sh windows

.PHONY: e2e-clean
e2e-clean: ## Delete any e2e VM session left behind (idempotent).
	scripts/e2e/run.sh clean
