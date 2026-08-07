---
id: "qa-014-mutation-testing"
type: spec
status: archived # draft | implementing | verifying | archived
created: "2026-06-19"
tags: [spec, testing, qa, mutation, gremlins, ci]
issue: 189
---

# QA-014: Tasks

## This PR — harness setup (issue #189)

- [x] **Create:** `.gremlins.yaml` — advisory config (thresholds 0, regex
      excludes for `_linux\.go$` / `_windows\.go$` / `_darwin\.go$`, mutant set).
- [x] **Create:** `Makefile` — `mutation`, `mutation-dry`, `mutation-install`
      targets; gremlins pinned to `v0.6.0`, installed via `go install`.
- [x] **Create:** `.github/workflows/mutation.yml` — weekly cron +
      `workflow_dispatch`, builds, installs gremlins, asserts go.mod untouched,
      runs `gremlins unleash` (advisory), uploads report + summary.
- [x] **Create:** this spec (`proposal.md`, `tasks.md`, `verification.md`).

## Verification (this PR)

- [x] `actionlint .github/workflows/mutation.yml` (or CI) parses the workflow.
- [~] `gremlins unleash --dry-run` succeeds and lists mutants (run once via
      `workflow_dispatch` after merge, since gremlins cannot run in the local
      broken-toolchain environment).
      **Never run** — superseded by the full weekly `unleash`; see
      `verification.md`.
- [x] First scheduled/dispatched run uploads `mutation-report.json` and the job
      stays green despite survived mutants.

## Follow-up — act on survived mutants (separate issues/PRs)

Each package below becomes its own hardening PR once the first report lands.
Numbers are allocated when the issues are filed; do not pre-create them.

> **Re-ranked 2026-08-06 against the first report actually read.** The list below
> was written before any report existed, and two of its four entries are
> falsified by the data: `internal/telemetry` (`metrics.go`) has **0 survivors**
> — 3/3 killed — and `internal/host/firewall_rule.go`, listed as "highest
> priority", also has **0 survivors** (3/3 killed). Both are struck. One package
> the list never mentioned, `internal/logging`, is the third-largest source of
> survivors. Priority now follows survivor count.

1. [ ] `internal/config` — **74 survivors** (`merge.go` 43, `config.go` 31).
       All 43 in `merge.go` are conditionals (25 negation, 18 boundary), i.e.
       the precedence rules. Overlaps QA-011 (#181), which should be written in
       Go for exactly this reason.
2. [ ] `internal/logging` — **18 survivors** (`logging.go`), plus 7 not covered.
       Absent from the original list.
3. [ ] `internal/proxy` — **18 survivors** (`reconnect.go` 12, `proxy.go` 6).
       Port/forwarding logic reachable without a real network.
4. [ ] `internal/discover/env.go` — 5 survivors.
5. [ ] `cmd/cli` — **0 survivors but 205 not covered**: the `RunE` bodies are
       never reached from `go test`. Blocked on the command-tree constructor
       (#278); not an assertion-strength problem.
6. [~] ~~`internal/telemetry`~~ — 0 survivors (3/3 killed). Nothing to harden.
7. [~] ~~`internal/host/firewall_rule.go`~~ — 0 survivors (3/3 killed).
       Nothing to harden.

## Decisions

- **gremlins over `manbearpig`/`go-mutesting`:** gremlins is actively
  maintained (v0.6.0, Dec 2025), config-file driven, has a dry-run mode, JSON
  output, and first-class build-tag handling.
- **Standalone workflow, not a `ci.yml` job:** keeps PR latency unaffected;
  mutation runs are minutes-to-tens-of-minutes, unacceptable per-PR.
- **Advisory (thresholds 0), not a gate:** gating before the suite is mature
  produces churn against equivalent mutants. Revisit once kill rate stabilizes.
- **Install-only, never in go.mod:** preserves the zero-dependency design goal;
  guarded by `git diff --exit-code go.mod go.sum` in CI.
