---
tags: [spec, verification, REL-003]
created: "2026-05-18"
---

# Verification - REL-003

## Evidence

- [x] `TS_IDLE_TIMEOUT` unset defaults to 0 → `TestLoadConfig/idle_timeout_defaults_to_disabled`
- [x] `TS_IDLE_TIMEOUT="5m"` parsed correctly → `TestLoadConfig/idle_timeout_parsed`
- [x] Invalid value rejected → `TestLoadConfig/idle_timeout_invalid`
- [x] Negative value rejected → `TestLoadConfig/idle_timeout_negative_rejected`
- [x] Idle read returns timeout error after deadline → `TestIdleConnReadTimesOutWithoutActivity`
- [x] Continuous traffic does NOT trip idle → `TestIdleConnReadSucceedsWithTraffic`
- [x] Disabled (idle=0) returns conn unchanged → `TestWithIdleTimeoutDisabled`
- [x] Enabled (idle>0) returns `*idleConn` wrapper → `TestWithIdleTimeoutEnabled`
- [x] Timeout error correctly classified → `TestIsIdleTimeoutErr` (uses real `net.Pipe` deadline error)
- [x] Idle close path emits info, not warn — code review of `proxyConnections` switch (see `internal/proxy/proxy.go`)
- [x] No regressions in existing tests → `go test ./...` 100% PASS on master + branch

## Test status

- Test suite: `go test ./...` → all packages PASS (35 existing + 9 new = 44 tests, table-driven cases not counted individually).
- Manual smoke: not executed locally (idle timeout in real RDP needs a 30-min session); deferred to operator on first deploy with `TS_IDLE_TIMEOUT=30m`.
- No regressions: yes, all previously-green tests remain green.

## Decisions made during implementation

- **Per-direction deadline (A1) over shared-state bidirectional (A2).** Simpler (~20 lines vs ~60), opt-in default 0, and users set the timeout high enough (30 min+) that false positives on RDP are unlikely. If reported in practice, escalate to A2 in a separate spec.
- **Wrap both `client` AND `remote` in `handleConn`.** Each direction's `Read` happens inside `io.CopyBuffer(dst, src, ...)` reading from `src`. Wrapping only one end leaves the opposite direction without enforcement. Symmetric wrap is safer.
- **`isIdleTimeoutErr` uses `errors.As` against `net.Error`.** Avoids string matching on error messages (which differ between Linux `i/o timeout` and Windows). Verified with a real `net.Pipe` deadline error in the unit test, not a synthetic stand-in — keeps the matcher honest against stdlib drift.
- **Did not consume any third-party deps.** Pattern is pure stdlib; no `go.mod` change.

## Promotion candidates

To be evaluated at archive time.

- [ ] Lesson for `10_projects/ts-bridge/90-lessons.md`? <tbd>
- [ ] ADR-worthy decision for `10_projects/ts-bridge/30-architecture/`? <tbd>
- [ ] New pattern candidate for `00_meta/patterns/`? <tbd>

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/REL-003/` → `specs/archive/REL-003/`
- [ ] Backlog entry in vault `11-tasks.md` ticked with PR link
- [ ] Promotions above executed (if any)
