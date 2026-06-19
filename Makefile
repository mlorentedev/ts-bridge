# ts-bridge — developer task runner.
#
# Scope: mutation testing (go-gremlins/gremlins). Build/test/lint live in CI
# (.github/workflows/ci.yml) and are run directly with `go build|test|vet`.
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

.DEFAULT_GOAL := help

.PHONY: help
help: ## Show this help.
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

.PHONY: mutation-install
mutation-install: ## Install the pinned gremlins binary (does not touch go.mod).
	go install $(GREMLINS_PKG)@$(GREMLINS_VERSION)

.PHONY: mutation-dry
mutation-dry: mutation-install ## List the mutants gremlins would generate (no tests run).
	$(GREMLINS) unleash --dry-run

.PHONY: mutation
mutation: mutation-install ## Run mutation testing on the whole module (advisory; writes mutation-report.json).
	$(GREMLINS) unleash --output mutation-report.json
