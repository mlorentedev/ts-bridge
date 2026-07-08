---
tags: [spec, verification]
created: "2026-07-08"
---

# Verification - UX-004-structured-lifecycle-signals

## Evidence

All new tests in `cmd/cli/signal_test.go`. Emission wiring in `cmd/cli/run.go`; `--quiet` flag in `cmd/cli/connect.go`.

- [x] Criterion 1 (READY line, actual bound addr) -> `TestFormatReadyLine`, `TestEmitReady`, and `TestEmitReady_UsesBoundAddr` (binds a real `127.0.0.1:0` listener and asserts the READY line carries the OS-assigned port, not the nominal `:0`). Wiring: `Run` calls `emitReady(os.Stdout, listener.Addr().String(), cfg.Target)` before the accept loop.
- [x] Criterion 2 (`--quiet` suppresses banner, keeps READY) -> `TestWriteStartupBanner_QuietSuppresses` (quiet -> 0 bytes; non-quiet -> banner header). Wiring: `writeStartupBanner` returns early on `cfg.Quiet`; `emitReady` is called unconditionally right after, so READY survives `--quiet`. Flag registered in `connect.go`, resolved post-Merge onto `cfg.Quiet`.
- [x] Criterion 3 (ERROR reason token per category) -> `TestDiagnoseTailscaleInitError_Reason` (table: `bad_authkey` for api-key/invalid-key/expired; `control_plane_unreachable` for deadline/timeout), `TestFormatErrorLine`, `TestEmitError`. **End-to-end smoke:** the built binary run against a fake key emitted `ERROR reason=bad_authkey detail="tsnet.Up: backend: invalid key: unable to validate API key"` to stderr and exited `1`.
- [x] Criterion 4 (`unknown` fallback never silent) -> `TestDiagnoseTailscaleInitError_Reason/unrecognized` returns `reasonUnknown`; `initTailscale` emits the ERROR line for every non-nil `server.Up` error (reason falls back to `unknown`).
- [x] Criterion 5 (`detail=` escaping, single line) -> `TestEscapeSignalDetail` (plain, embedded quotes -> `\"`, backslashes -> `\\` incl. trailing-backslash Windows path, `\n`/`\r\n`/`\r` collapse to space, combined + trim).
- [x] Criterion 6 (no regression) -> `go test ./... -count=1` — all packages pass; existing `Run`/connect behavior unchanged without `--quiet`.

## Test status

- Test suite: `go test ./... -count=1` -> all 10 packages `ok`, 0 failures (CI runs `-race` on Linux)
- `go vet ./...` -> clean
- Manual smoke test: `connect` with a valid-format-but-fake `tskey-` key (4s timeout) -> `ERROR reason=bad_authkey detail="…invalid key…"` on stderr, exit 1 (see Criterion 3). `--quiet` present in `connect --help`.
- READY end-to-end not smoke-tested locally (needs a successful tsnet startup against a real tailnet); covered by unit tests incl. the real-listener bound-addr test, plus one-line wiring in `Run`.
- No regressions in existing test suite: yes

## Decisions made during implementation

- READY is emitted right after the banner and **before** the health server starts — grouped with the banner's direct stdout writes, ahead of any logger stdout output, to avoid the BUG-010 stdout-interleave race (the console logger writes to `os.Stdout`). The listener is already bound at that point, so the OS queues incoming connections until `AcceptLoop` runs — "READY" is accurate.
- `--quiet` is resolved post-Merge in `runConnect` (like `--profile`), not threaded through env/YAML precedence — it is a pure console-output concern with no env/YAML source.
- `escapeSignalDetail` escapes `\` -> `\\` (FIRST) then `"` -> `\"` and collapses newlines; `formatErrorLine` wraps `detail` in literal quotes (not `%q`) so the output is plain-readable rather than Go-quoted. Backslash-first ordering was added after a pre-merge review flagged that a `detail` ending in `\` (Windows state-dir paths in tsnet errors) would otherwise backslash the closing quote and break a backslash-aware parser.
- `diagnoseTailscaleInitError` signature changed from `(hint, remediation)` to `(reason, hint, remediation)`; `reason` is never empty for a non-nil error (falls back to `unknown`) so the ERROR signal is never silent, while `hint`/`remediation` stay empty for unrecognized errors to keep the human log quiet on noise.

## Promotion candidates

- [ ] Lesson for the repo's `docs/lessons.md`? no — the BUG-010 stdout-race reasoning is already documented; this reuses it.
- [ ] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? no — the `TOKEN key=value` grammar is documented in the README public contract; small enough to not warrant an ADR (revisit if a JSON event mode is added later).
- [ ] New pattern candidate for `00_meta/patterns/`? no — single-project CLI concern.

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/UX-004-structured-lifecycle-signals/` -> `specs/archive/`
- [ ] Bitácora tickets #203 and #204 closed with PR link (ADR-018)
- [ ] Promotions above executed (none)
