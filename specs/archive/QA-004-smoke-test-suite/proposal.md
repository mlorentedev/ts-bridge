---
id: "qa-004-smoke-test-suite"
type: spec
status: proposed
created: "2026-06-18"
tags: [spec, testing, qa, smoke, e2e, automation]
issue: 78
---

# QA-004: Automated Smoke Test Suite — Proposal

## Problem

The existing smoke test (`scripts/tests/smoke.ps1`) covers only ~20% of the CLI surface, is Windows-only, and doesn't include `discover` or `status` deep testing. The three open QA tickets (QA-001, QA-002, QA-003) are manual checklists with no automation path. There's no cross-platform test infrastructure.

## Goal

Create a **cross-platform automated smoke test suite** that:

1. Covers **all CLI commands and flags** (version, init, connect, status, host, discover)
2. Runs on **both Windows (PowerShell) and Linux/macOS (BATS)**
3. **Automates** what can be automated (CLI parsing, help text, error messages, config precedence)
4. **Flags** what requires real hardware (actual proxy forwarding, host setup with admin privileges)
5. Produces a **CI-ready** test that can run in `.github/workflows/`

## Scope

### In scope

- **POSIX/BATS smoke test** — full CLI coverage on Linux/macOS
- **PowerShell smoke test expansion** — add `discover` and `status` testing, match BATS coverage
- **Test infrastructure** — shared test helpers, fixture files, CI integration
- **Feature coverage matrix** — document which features are tested vs. manual-only

### Out of scope (deferred to sub-tickets)

- Real Tailscale mesh e2e (requires physical devices) — stays in QA-013
- Host setup actual execution (requires admin privileges) — stays in QA-009
- Independent code audit — stays in QA-001

## Architecture

```
scripts/tests/
├── smoke.bats         # POSIX — the single smoke suite
├── helpers/           # shared test utilities
│   └── smoke_helpers.bash  # BATS helper functions
└── fixtures/          # test data
    ├── invalid.yaml   # YAML with unknown fields
    └── sample.yaml    # valid YAML config
```

> Revised by #271: the original layout paired `smoke.bats` with a PowerShell
> mirror (`smoke.ps1` + `smoke_helpers.psm1`). The mirror was retired — see the
> superseded decision in `tasks.md`.

## CI Integration

Add a `smoke` job to the existing CI workflow (`.github/workflows/ci.yml`) that:

1. Builds the binary for each platform
2. Runs `smoke.bats` on Linux runner
3. Fails the build if any test fails

Windows CLI coverage is not a smoke-suite job: the existing `test-windows` job
runs `go test ./...` on `windows-latest`, so Go tests under `cmd/cli/` are
cross-platform at no extra CI cost (#271).

## Acceptance Criteria

- [ ] BATS smoke test covers all CLI commands and flags
- [ ] PowerShell smoke test covers all CLI commands and flags
- [ ] Both tests produce structured output (PASS/FAIL with counts)
- [ ] CI runs smoke tests on every PR (Linux + Windows)
- [ ] Feature coverage matrix documents 100% CLI flag coverage
- [ ] Tests run in < 30 seconds

## Non-functional

- No new dependencies (BATS is a package manager dep, not a code dep)
- Tests use dummy auth keys (no real Tailscale network required)
- Tests are idempotent (clean temp dirs, no state leakage)
- Tests skip host setup/check on non-admin (with clear skip message)
