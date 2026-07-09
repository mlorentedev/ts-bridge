---
id: "qa-004-smoke-test-suite"
type: spec
status: proposed
created: "2026-06-18"
tags: [spec, testing, qa, smoke, e2e, automation]
issue: 78
---

# QA-004: Tasks (TDD order)

## Issue mapping

Each phase maps to a separate GitHub issue for independent tracking and PRs:

| Issue | Title | Scope |
|-------|-------|-------|
| [#173](https://github.com/mlorentedev/ts-bridge/issues/173) | QA-004: BATS smoke test — CLI parsing | version, help, all subcommands |
| [#174](https://github.com/mlorentedev/ts-bridge/issues/174) | QA-005: BATS smoke test — init | all flags, formats, overwrite protection |
| [#175](https://github.com/mlorentedev/ts-bridge/issues/175) | QA-006: BATS smoke test — status | running/not-running, --json, --watch, --addr |
| [#176](https://github.com/mlorentedev/ts-bridge/issues/176) | QA-007: BATS smoke test — connect | flag parsing, error handling, graceful shutdown |
| [#177](https://github.com/mlorentedev/ts-bridge/issues/177) | QA-008: BATS smoke test — discover | Tailscale/Headscale, --json, --filter, --auto, --port |
| [#178](https://github.com/mlorentedev/ts-bridge/issues/178) | QA-009: BATS smoke test — host setup | Windows registry, firewall, UPnP, sleep; Linux xrdp, UFW |
| [#179](https://github.com/mlorentedev/ts-bridge/issues/179) | QA-010: BATS smoke test — host check | Windows/Linux/macOS, --json, read-only |
| [#181](https://github.com/mlorentedev/ts-bridge/issues/181) | QA-011: BATS smoke test — config precedence | flags > env > YAML > defaults |
| [#182](https://github.com/mlorentedev/ts-bridge/issues/182) | QA-012: BATS smoke test — error handling | missing auth key, invalid target, bad ports, timeouts |
| [#183](https://github.com/mlorentedev/ts-bridge/issues/183) | QA-013: Multi-device e2e validation | real Tailscale mesh, bidirectional forwarding |

## Execution order

Each issue is a self-contained PR. Execute in this order:

1. **QA-004 (#173)** — CLI parsing (foundation — everything depends on this)
2. **QA-005 (#174)** — init (simplest command with many flags)
3. **QA-006 (#175)** — status (bridge lifecycle)
4. **QA-007 (#176)** — connect (most flags, most complex)
5. **QA-008 (#177)** — discover (newest feature)
6. **QA-009 (#178)** — host setup (platform-specific, admin required)
7. **QA-010 (#179)** — host check (read-only, simpler than setup)
8. **QA-011 (#181)** — config precedence (cross-cutting, touches all commands)
9. **QA-012 (#182)** — error handling (cross-cutting, touches all commands)
10. **QA-013 (#183)** — multi-device e2e (requires hardware, manual)

## Shared infrastructure (applies to all issues)

All BATS issues share the same test file: `scripts/tests/smoke.bats`. Each issue adds tests to the same file in its PR.

- [x] **Create:** `scripts/tests/helpers/smoke_helpers.bash` — BATS helper functions *(QA-004 / #173)*
- [ ] **Create:** `scripts/tests/helpers/smoke_helpers.psm1` — PowerShell helper module *(deferred; smoke.ps1 uses inline helpers today)*
- [ ] **Create:** `scripts/tests/fixtures/` — test data files *(deferred to QA-005/#174 — first ticket that needs YAML fixtures)*
- [ ] **Expand:** `scripts/tests/smoke.ps1` — add discover, status, host check coverage *(QA-006/#175, QA-008/#177, QA-010/#179)*
- [x] **Update:** `.github/workflows/ci.yml` — add `smoke` job (Linux/BATS) *(QA-004 / #173; Windows PowerShell smoke job deferred)*
- [x] **Create:** `docs/qa-coverage.md` — feature coverage matrix *(QA-004 / #173)*

> **Decision:** BATS for POSIX because it's the de facto standard for shell test suites, well-documented, and CI-friendly. PowerShell for Windows because the existing smoke test is already in PS and Windows doesn't have a POSIX shell by default.
