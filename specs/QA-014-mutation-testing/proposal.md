---
id: "qa-014-mutation-testing"
type: spec
status: proposed
created: "2026-06-19"
tags: [spec, testing, qa, mutation, gremlins, ci]
issue: 189
---

# QA-014: Mutation Testing Harness — Proposal

## Problem

CI reports line/branch coverage, but coverage only proves a line *executed* —
not that a test would *fail* if that line were wrong. A test can run a branch
and assert nothing meaningful about it. Today nothing measures **assertion
strength**: weak asserts, missing edge cases, and dead-but-covered code all
pass green.

## Goal

Add a **mutation testing** harness using [go-gremlins/gremlins](https://gremlins.dev)
that applies to **all existing code**, not just one feature. Gremlins injects
small faults (mutants) — flipping `>` to `>=`, `&&` to `||`, removing
increments, negating conditions — then re-runs the tests. A *killed* mutant
means a test caught the fault; a *survived* mutant pinpoints a real gap in the
suite. The survived-mutant list becomes a prioritized backlog for hardening
tests.

## Scope

### In scope

- **`.gremlins.yaml`** — advisory config: thresholds 0, regex excludes for
  platform shell-exec wrappers, conservative mutant set.
- **`Makefile`** — `mutation`, `mutation-dry`, `mutation-install` targets for
  local runs (gremlins installed via `go install`, never added to `go.mod`).
- **`.github/workflows/mutation.yml`** — weekly cron + `workflow_dispatch`,
  advisory, uploads a mutation report artifact and a job summary.
- **Rollout plan** — order in which to act on survived mutants (below).

### Out of scope

- Making mutation score a **merge gate** (deferred until the suite is mature;
  premature gating produces churn against equivalent mutants).
- Per-PR mutation runs (too slow; would dominate CI time).
- Mutating platform shell-exec code (`*_linux.go` firewall/RDP/service calls,
  and the build-tag-excluded `*_windows.go` / `*_darwin.go`) — these shell out
  to PowerShell, `sc`, `ufw`, `iptables`, `powercfg` and cannot be killed by
  unit tests without real OS services.

## Architecture

```
.gremlins.yaml                          # advisory config (thresholds 0)
Makefile                                # mutation / mutation-dry / mutation-install
.github/workflows/mutation.yml          # weekly cron + manual dispatch
specs/QA-014-mutation-testing/          # this spec
```

`gremlins unleash` loads packages for the current GOOS, mutates non-test
source, and runs the affected package's tests per mutant. On the Linux runner,
build tags already exclude `*_windows.go` / `*_darwin.go`; the regex excludes
in `.gremlins.yaml` add belt-and-suspenders coverage (and apply on local
Windows/macOS runs too).

## CI Integration

A standalone workflow (not a job in `ci.yml`) so PR latency is unaffected:

1. Build the module (`go build ./...`) to fail fast on compile errors.
2. `go install github.com/go-gremlins/gremlins/cmd/gremlins@v0.6.0`.
3. Assert `git diff --exit-code go.mod go.sum` — the zero-dependency invariant.
4. `gremlins unleash --output mutation-report.json` (advisory; `continue-on-error`).
5. Publish a job summary and upload the report artifact.

## Rollout Plan

Act on survived mutants package-by-package, easiest signal first:

1. **`internal/telemetry`** — pure logic, fully unit-testable; expect a high
   kill rate, good baseline.
2. **`internal/config` / `internal/host` config merge** — flags > env >
   defaults precedence is branch-heavy and a frequent source of weak asserts.
3. **`internal/host/firewall_rule.go`** — the `sanitizeFirewallRule` allowlist
   is security-relevant; surviving mutants here are the highest priority.
4. **`internal/proxy`** — port/forwarding logic where reachable without real
   network I/O.

Each round: read the survived-mutant list, add or strengthen tests to kill the
*meaningful* mutants, and note any equivalent (un-killable) mutants so they are
not re-investigated next run.

## Acceptance Criteria

- [ ] `gremlins unleash` runs in CI on the whole module and uploads a report
- [ ] gremlins does **not** appear in `go.mod` / `go.sum` (guarded in CI)
- [ ] Workflow is advisory (green even when mutants survive)
- [ ] Workflow runs on weekly schedule **and** manual `workflow_dispatch`
- [ ] `make mutation-dry` lists mutants locally without running tests
- [ ] Platform shell-exec wrappers are excluded from mutation

## Non-functional

- **No new module dependencies** — gremlins is install-only, pinned to `v0.6.0`.
- **Reproducible** — pinned tool version means local and CI report identical mutants.
- **Non-blocking** — thresholds are 0; mutation score never fails a build.
- **Bounded runtime** — job `timeout-minutes: 60`.
