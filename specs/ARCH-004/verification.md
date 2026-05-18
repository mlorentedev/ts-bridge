---
tags: [spec, verification, ARCH-004]
created: "2026-05-18"
---

# Verification - ARCH-004

## Evidence

- [x] `TS_DIAL_RETRIES` parsed, default 3, rejects negative/invalid → `TestLoadConfig/dial_retries_*` (5 cases)
- [x] `TS_DIAL_BACKOFF_BASE` parsed, default 1s, rejects negative → `TestLoadConfig/dial_backoff_base_negative_rejected`
- [x] `TS_DIAL_BACKOFF_MAX` parsed, default 30s, rejects negative + rejects max<base → `TestLoadConfig/dial_backoff_max_*` (2 cases)
- [x] Success first attempt → `TestReconnectDialer_SucceedsFirstAttempt`
- [x] Success after N transient failures → `TestReconnectDialer_SucceedsAfterTransientFailures`
- [x] Gives up after MaxRetries → `TestReconnectDialer_GivesUpAfterMaxRetries` (also verifies error wrap)
- [x] Permanent error short-circuits → `TestReconnectDialer_PermanentErrorShortCircuits`
- [x] Context cancellation aborts retry promptly → `TestReconnectDialer_ContextCancellationAbortsLoop`
- [x] Jitter bounded `[d, d+d/2]` → `TestComputeBackoff_JitterBounded` (200 iterations × 2 attempt levels)
- [x] Overflow protection at attempt=100 → `TestComputeBackoff_AttemptOverflowProtection`
- [x] MaxRetries=0 disables → `TestReconnectDialer_MaxRetriesZeroDisables`
- [x] Permanent error classifier covers DNS/AddrError/tsnet-terminal → `TestIsPermanentDialError` (8 cases)
- [x] Inner error preserved via `errors.Is` chain → `TestReconnectDialer_WrappedErrorPreservesInner`
- [x] No regressions → all existing tests (config, main, proxy, integration) still PASS

## Test status

- Test suite: `go test ./...` → PASS (4 packages, ~60 tests including 19 new for ARCH-004)
- Race detector (CI): pending — no cgo on Windows dev box, CI runs `-race`
- Manual smoke: not executed locally; deferred to operator (would require blocking the target with a firewall rule then unblocking to observe retry logs)
- No regressions: yes

## Decisions made during implementation

- **Used `math/rand/v2` instead of `math/rand`.** The v2 package (Go 1.22+) avoids the global-lock contention of v1 and removes the deprecation around `rand.Seed`. Jitter is not security-sensitive, so `crypto/rand` is overkill.
- **`isPermanentDialError` uses `errors.As` for structured types (`*net.AddrError`, `*net.DNSError`) and `strings.HasPrefix` for tsnet's text error.** tsnet does not expose a sentinel error or typed error for terminal-state failures (verified by reading `tsnet/tsnet.go:203`); string match is the only available option. Documented at the call site.
- **Attempt cap of 30 before shift.** `1 << 30 = 1<<30` nanoseconds ≈ 1s before any base multiplication — already far beyond any realistic `MaxBackoff`. The cap exists purely to prevent shift-overflow in pathological configurations, not to bound the wait time (the `maxBackoff` parameter does that).
- **Single attempt counter semantics.** `MaxRetries=2` means "1 initial attempt + 2 retries = 3 total attempts." This matches the user mental model expressed in `TS_DIAL_RETRIES` and is documented via test assertions.
- **No Tier 2 supervisor.** Resolved Q1 in proposal: `tsnet.Server.awaitRunning` self-heals during transient backend state changes. A persistent terminal state surfaces as `"tsnet: backend in state X"` which `isPermanentDialError` short-circuits to avoid infinite retries — operator must restart the bridge in that rare case.

## Promotion candidates

- [ ] Lesson for `10_projects/ts-bridge/90-lessons.md`? <tbd — likely something about `tsnet.awaitRunning` semantics worth crystallizing if non-obvious>
- [ ] ADR-worthy decision for `10_projects/ts-bridge/30-architecture/`? <tbd — the "no Tier 2 supervisor needed" finding might justify a short ADR>
- [ ] New pattern candidate for `00_meta/patterns/`? <tbd — `ReconnectDialer` is a generic decorator pattern but probably stays project-local>

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/ARCH-004/` → `specs/archive/ARCH-004/`
- [ ] Vault `11-tasks.md` ARCH-004 ticked with PR link
- [ ] Vault `11-tasks.md` ARCH-005 ticked as subsumed
- [ ] Promotions above executed (if any)
