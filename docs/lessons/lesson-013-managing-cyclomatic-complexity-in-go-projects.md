---
id: lesson-013-managing-cyclomatic-complexity-in-go-projects
type: lesson
status: active
created: "2026-03-07"
owner: manu
tags: [ts-bridge, lesson, go, refactoring, clean-architecture, ci-cd, linter]
---

# Managing Cyclomatic Complexity in Go projects

**Context:** Refactoring main.go into multiple internal packages and implementing Graceful Shutdown.
**Problem:** Large single files (main.go > 600 lines) led to high cyclomatic complexity (19), causing golangci-lint failures (threshold 15) and making logic difficult to test or maintain. Initial implementation of Graceful Drain increased complexity further.
**Solution:** 1. Split main.go into internal packages: internal/config, internal/proxy, internal/health, internal/telemetry. 
2. Decoupled metrics into internal/telemetry to avoid circular dependencies between health and proxy.
3. Extracted complex setup/teardown logic into helper functions (initTailscale, handleShutdown, drainActiveConnections) reducing cyclomatic complexity from 19 down to 7.
4. Used interface-based mocking (Dialer) to enable unit testing of core proxy logic without real network dependencies.
**Tags:** `#go` `#refactoring` `#clean-architecture` `#ci-cd` `#linter`
