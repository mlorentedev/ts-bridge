---
tags: [spec, verification, templates]
created: "2026-10-19"
---

# Verification - BUG-031-derp-socks-proxy

## Evidence

Map every acceptance criterion from `proposal.md` to concrete proof (commit hash, test name, or observed behavior).

- [x] AC1: `TestControlProxySDKConsumers` isolates the process-global SDK hook.
  It checks HTTPS control response bytes and the native DERP client's
  authenticated `ServerInfoMessage`, using normally verified fixture TLS and
  the original destination hostname observed by a real SOCKS server.
- [x] AC2: selector/fallback cases in `bootstrap_test.go`;
  `TestControlProxyHTTP`, `TestControlProxyRejectsUnrelatedDestinations` and
  `TestControlProxyUpstreamFailure` cover the forwarding boundary.
- [x] AC3: `TestControlProxyCancel`, `TestControlProxyCleanupActiveTunnel`,
  `TestControlProxyCleanupCanceledStart` and the failed hook-replacement case
  cover listener/tunnel ownership and error cleanup.
- [x] AC4: Windows tests/build pass (see Test status). Live DERP acceptance is
  deferred to #441 and is not claimed by this change.

## Test status

- Baseline on `06cfb87`: `go test .\internal\bootstrapssh .\cmd\cli` passed
  (13.441s and 20.536s).
- Initial native SDK regression failed before the adapter with
  `incompatible SOCKS version` and malformed HTTP response `"\x05\xff"`.
  It passed after the protocol correction.
- After lifecycle hardening, `go test .\internal\bootstrapssh .\cmd\cli
  -count=1` passed (7.287s / 17.070s), and
  `go vet .\internal\bootstrapssh .\cmd\cli` passed.
- `TestControlProxyBufferedCONNECT` passed with buffered input. Replacing that
  input with the raw client connection made it fail; the mutation was restored.
- Subsequently, `go test ./...` failed `TestStartEnforcesReadyTimeoutDuringDial`;
  the other packages passed. The original test counted Windows helper-process
  startup and termination against a 200ms assertion for a configured 20ms
  readiness dial, and a later measurement revision read a `0s` duration because
  of coarse Windows clock resolution around a blocking call.
- Final repair (test-only): the dial now records that its context carried a
  deadline and keeps `dialElapsed <= 200ms` only as an upper bound.
  `go test ./internal/bootstrapssh -run TestStartEnforcesReadyTimeoutDuringDial
  -count=20` -> ok (7.0s).
- At HEAD after the review fixes: `go test -count=1 ./...` -> every package ok
  (`cmd/cli` 15.985s, `internal/bootstrapssh` 5.403s), `go build ./...` exit 0,
  `go vet ./internal/bootstrapssh ./cmd/cli` clean, and
  `go list -deps ./cmd/ts-bridge` contains none of the four test-only modules
  nor `derp/derpserver`.
- Not run locally: `golangci-lint run` (installed v2.12.2 is built with go1.26,
  the module targets 1.27.1; CI pins v2.14.0) and `-race` (needs cgo, absent on
  this host). The PR's `lint` and Linux `test -race` jobs are the authority.
- No updated Windows candidate has been built, installed or live-tested; the
  live DERP acceptance is deferred to #441.

## Decisions made during implementation

Brief log of non-obvious trade-offs or course corrections taken during the work. Routine choices belong in commit messages, not here.

- Readiness test weakened deliberately: it moved from "whole `start()` <= 200ms"
  to "the dial carried a deadline and took <= 200ms". Helper spawn/teardown is
  no longer bounded by the test. Retention evidence: removing the
  `context.WithTimeout` in `waitUntilReady` still reddens the test.
- Review fixes: a `slog.Warn` on denied destinations, and the CONNECT dial now
  inherits the request context instead of a second 30s readiness constant.
- No module-cache edits, host changes, global proxy settings or credentials.
- Keep the correction independent of the draft profile feature.
- The adapter has its own child cancellation context so explicit close also
  closes hijacked tunnels. A canceled parent is rejected before startup.
- Existing `golang.org/x/net v0.59.0` becomes direct. Four SDK-pinned indirect
  modules support the native server fixture; no SDK version upgrade.
- ADR-005 and `docs/runbooks/guide-control-bootstrap.md` record the boundary.
- Live HTTPS previously completed verified TLS/HTTP2 but stalled before headers
  both over the mesh and over SSH SOCKS alone. The DERP repair is not proof
  that this separate application-response failure is solved.

## Promotion candidates

Answer each line `yes: <path>`, naming the file you promoted, or `no: <reason>`. `dotf spec archive` refuses a line left unanswered, a `no` without a reason, and a `yes` whose file does not exist; a `00_meta/` path is looked up in the vault.

- [x] Lesson for the repo's `docs/lessons/`? yes: docs/lessons/lesson-038-2026-10-20.md
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? yes: docs/adr/adr-005-headscale-compat.md
- [x] New pattern candidate for `00_meta/patterns/`? no: this is a repair of one pinned SDK integration, not verified cross-project recurrence.

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/BUG-031-derp-socks-proxy/` -> `specs/archive/BUG-031-derp-socks-proxy/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
