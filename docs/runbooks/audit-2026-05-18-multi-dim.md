---
id: "ts-bridge-audit-2026-05-18"
type: audit
status: active
tags: [audit, scalability, ux, dx, solid, clean-code]
created: "2026-05-18"
owner: manu
---

# Multi-Dimensional Audit — 2026-05-18 (post v1.7.0)

> Four parallel subagent passes (scalability / UX / DX / SOLID+clean-code) run after the v1.7.0 release. Each agent operated from a self-contained brief without context-pollution from the others. All four verdicts converged on **PASS-WITH-GAPS** — no Blockers, no FAIL.

## Verdicts

| Dimension | Verdict | Findings count |
|---|---|---|
| Scalability | PASS-WITH-GAPS | 4 Major + 4 Minor |
| UX | PASS-WITH-GAPS | 3 Major + 7 Minor |
| DX | PASS-WITH-GAPS | 4 Major + 6 Minor |
| SOLID / Clean Code | PASS-WITH-GAPS | 0 Major + 5 Minor |

## Majors fixed in this session

| Dim | Finding | Where | Fix landed in |
|---|---|---|---|
| Scalability | TOCTOU on `MaxConnections` check (check-then-act) | `internal/proxy/proxy.go:99-109` | PR #34 (see below) |
| Scalability | `TS_TIMEOUT` reused for target dial; with retries can hold slot ~3min | `internal/proxy/proxy.go:147-156` | PR #34 (`TS_DIAL_TIMEOUT` added) |
| Scalability | Half-close not honored — `closeAll` slams both sides on first EOF | `internal/proxy/proxy.go:218-223` | PR #34 (`CloseWrite` semantics) |
| UX | README `mstsc /v:127.0.0.1:33389` hardcoded — auto mode randomizes port | `README.md:76` | PR #32 |
| UX | "Run: `cp .env.example .env`" doesn't work on Windows cmd | `scripts/client/run.sh:39`, `run.ps1:37` | PR #32 |
| DX | Stale `CLAUDE.md` claims `main.go ~785 lines` (now ~250 + 4 internal/ packages) | `CLAUDE.md:18-19` | PR #33 |
| DX | `golangci-lint` pinned to `latest` in CI — local-green/CI-red drift | `.github/workflows/ci.yml:78` | PR #33 |

## Minors deferred (open tickets)

These remain in the backlog — none are urgent given the project's load profile and maintenance posture.

### Scalability
- **MIN-SCAL-01** No process-wide circuit breaker shared across `ReconnectDialer` instances. Under "target down + 50 inbound RDP attempts", 50 retry loops run in parallel. Acceptable for current load (≤10 clients).
- **MIN-SCAL-02** `health.Server` has no `WriteTimeout` / `IdleTimeout`. Slow `/metrics` reader can pin a goroutine.
- **MIN-SCAL-03** Accept-error backoff doesn't distinguish `EMFILE` (fd exhaustion) from transient. On fd exhaustion: spins log every 10s. Log loudly once would help diagnosis.
- **MIN-SCAL-04** `withIdleTimeout` defaults to disabled (0). Silent peers can hold fds forever. Operational choice; could ship sane default (30m) — but breaks opt-in design contract.

### UX
- **MIN-UX-01** No `troubleshooting.md` in Starlight site. Operators have nowhere to look up "key expired", "control plane unreachable" beyond log lines.
- **MIN-UX-02** Banner (`printBanner` in `main.go:~260`) doesn't show control plane URL or whether `TS_HEALTH_ADDR` is active.
- **MIN-UX-03** README "Quick Install" doesn't mention `bootstrap.sh` / `bootstrap.ps1` (the friendlier path).
- **MIN-UX-04** `TS_AUTHKEY` validation in `config.go:226` doesn't hint "did you paste the URL instead of the key?" for `http*` prefixes (common mistake).
- **MIN-UX-05** `.env.example:18` shows `TS_LOCAL_ADDR=127.0.0.1:33389` as commented default, but in auto mode this is NOT the default.
- **MIN-UX-06** Ephemeral mode lifecycle not surfaced in banner. Users wondering "why doesn't my node appear in admin?" need a one-liner.
- **MIN-UX-07** No `-help` documented in getting-started; only `flag.Parse` default.

### DX
- **MIN-DX-01** `scripts/dev.sh` requires a real `TS_AUTHKEY` / `TS_TARGET` before it'll build. No offline smoke path documented.
- **MIN-DX-02** No `scripts/dev.ps1` — Windows contributors have no equivalent of the Linux dev workflow.
- **MIN-DX-03** No CI `concurrency` cancellation; rapid pushes spawn duplicate jobs.
- **MIN-DX-04** `mockDialer` (proxy_test.go) and `recordingDialer` (reconnect_test.go) are package-private. Future tests in `main_integration_test.go` can't reuse them — that file currently has 3 hand-rolled proxy bodies.
- **MIN-DX-05** `make test-no-race` (workaround for Windows-without-cgo) not documented in `CONTRIBUTING.md`.
- **MIN-DX-06** `CONTRIBUTING.md` "Project Structure" tree is stale (predates ARCH-002 split).

### SOLID / Clean code
- **MIN-SOLID-01** `internal/telemetry` package-level singleton (`globalMetrics`). Forces `ResetMetrics()` to exist for tests. Could be `type Metrics struct{}` injected via `AcceptLoop` / `health.StartServer`.
- **MIN-SOLID-02** `handleConn` does 5 things (metrics, TCP tuning, dial, idle wrap, log). Still <40 lines but bumps SRP smell.
- **MIN-SOLID-03** `computeBackoff` has a defensive `base <= 0` branch unreachable in production (config parser already rejects negatives). Drop or convert to documented invariant.
- **MIN-SOLID-04** `health.go:43` has unnamed magic constant `ReadHeaderTimeout: 10 * time.Second`. Lift to named const with rationale.
- **MIN-SOLID-05** `IsExpectedCloseError` falls back to string matching. Necessary (tsnet doesn't expose sentinels for these) but worth a doc-comment naming each platform.

## Auditing methodology (reusable)

Reproducible recipe for the next multi-dim audit:

1. Identify dimensions worth auditing (here: scalability, UX, DX, SOLID/clean-code). Skip security if a separate threat-model exists ([security-audit.md](../troubleshooting/security-audit.md)).
2. Launch one subagent per dimension in parallel via `Agent` tool with `general-purpose` (or `pr-review-toolkit:code-reviewer` for SOLID).
3. Each prompt must include:
   - Project pitch (≤3 sentences)
   - Reading list (specific files, not "explore the repo")
   - Scope + out-of-scope
   - Output format: ≤350 words, bullet form, file:line citations, severity classification, "say PASS and stop if clean"
4. Consolidate cross-dim. Look for findings that overlap (same root cause from multiple angles).
5. Triage Majors → fix in current or next session. Minors → log here, defer.
6. Re-audit only when the project's posture changes (major version bump, refactor, new contributor onboarding).

## Related

- Changelog: PRs #32, #33, #34 land the fixes from this audit (see repo `CHANGELOG.md`)
- [security-audit.md](../troubleshooting/security-audit.md) — security dimension covered separately
- [lessons.md](../lessons.md) — any non-obvious finding promoted there
