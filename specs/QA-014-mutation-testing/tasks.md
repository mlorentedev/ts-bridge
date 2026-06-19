---
id: "qa-014-mutation-testing"
type: spec
status: proposed
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

- [ ] `actionlint .github/workflows/mutation.yml` (or CI) parses the workflow.
- [ ] `gremlins unleash --dry-run` succeeds and lists mutants (run once via
      `workflow_dispatch` after merge, since gremlins cannot run in the local
      broken-toolchain environment).
- [ ] First scheduled/dispatched run uploads `mutation-report.json` and the job
      stays green despite survived mutants.

## Follow-up — act on survived mutants (separate issues/PRs)

Each package below becomes its own hardening PR once the first report lands.
Numbers are allocated when the issues are filed; do not pre-create them.

1. [ ] `internal/telemetry` — strengthen asserts to kill survived mutants.
2. [ ] `internal/config` + host config merge — cover flags > env > defaults edges.
3. [ ] `internal/host/firewall_rule.go` — security-relevant; highest priority.
4. [ ] `internal/proxy` — port/forwarding logic reachable without real network.

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
