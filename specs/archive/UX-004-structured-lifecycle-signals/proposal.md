---
id: "UX-004-structured-lifecycle-signals"
type: spec
status: archived # draft | implementing | verifying | archived
created: "2026-07-08"
issue: "ts-bridge#203"   # primary; also closes ts-bridge#204 (combined PR)
tags: [spec, proposal, ux, cli, observability]
template_version: "1.0"
---

# UX-004: Structured Lifecycle Signals

<!-- from issue #203: emit structured READY line to stdout when tunnel is accepting connections -->
<!-- from issue #204: structured exit reason when connect fails — documented exit codes or stderr signal -->

## Why

Programmatic callers that spawn `ts-bridge connect` (the kubelab toolkit's `ts_bridge_tunnel` context manager, discovered in TOOL-015 / ADR-052) have no event-driven way to observe the bridge lifecycle. To detect "tunnel up" they poll the local port in a loop with a 20-second worst-case timeout, and get no signal if the process dies silently. To explain a failure they can only guess — every early exit collapses to a generic non-zero code, so the toolkit raises `"check auth in .env"` even when the real cause is an unreachable control plane. This blocks clean lifecycle management and forces every consumer to reinvent brittle port-polling and blind error guessing.

## What

`connect` (and the underlying `Run`) emit two structured, line-oriented signals using a stable `TOKEN key=value` grammar:

1. **Success → stdout**, emitted once immediately before the accept loop blocks:
   `READY local=127.0.0.1:16443 target=100.64.0.11:6443`
   `local` is the **actual bound listener address** (`listener.Addr()`), which is authoritative in auto-port mode where the requested port differs from the one bound.
2. **Startup failure → stderr**, emitted once immediately before the non-zero exit:
   `ERROR reason=bad_authkey detail="invalid preauth key: hskey-…"`
   `reason` is one of a closed, stable set of tokens — `bad_authkey`, `control_plane_unreachable`, `unknown` — derived from the existing `diagnoseTailscaleInitError` classifier. `detail` carries the underlying error message (single-line, escaped) for logging.

A `--quiet` flag suppresses the decorative ASCII banner and the "Waiting for connections…" human text; the structured `READY`/`ERROR` lines always emit regardless of `--quiet`.

The process exit code stays a generic non-zero (`1`) — the machine-readable cause travels in the stderr line, not in the exit code (option (b) of #204).

## Out of scope

- **Runtime death signaling** — already shipped in BUG-001 (#208 / PR #226); that covers a session dying mid-run. UX-004 covers startup only (ready signal + init-failure signal).
- **Stable per-category exit codes** (option (a) of #204) — explicitly rejected in favor of the stderr line; exit stays generic non-zero. Callers read the `reason` token, not the code.
- **`target_unreachable` as a startup signal** — target dial failures happen per-connection at runtime (in the proxy), not at tsnet init, so they never cause an early startup exit. Out of scope for the startup-failure surface (the issue #204 table lists it as a caller concern, but it maps to runtime, not startup).
- **JSON output mode** — the `KEY=value` single-line grammar is the contract; a richer JSON event stream is a separate future concern.

## Risks / open questions

- **[RESOLVED] `--quiet` semantics.** Suppresses the decorative banner + "Waiting…" text; keeps the structured `READY`/`ERROR` lines. This slightly reinterprets #203's suggestion ("suppress the line for interactive use") toward the more useful "machine-clean output" direction — a human who finds `READY` noisy can ignore one line, whereas a programmatic caller genuinely benefits from banner-free output. **Flag for user review.**
- **[RESOLVED] stdout is no longer banner-only.** Adding `READY` to stdout means callers must line-match the `READY ` prefix, not assume stdout structure. Documented in the flag help + `.env.example`/README.
- **[RESOLVED] `local=` source.** Must use `listener.Addr().String()`, never `cfg.LocalAddr` — in auto-port mode `cfg.LocalAddr` may carry a `:0`/range placeholder while the bound address is authoritative.
- **[OPEN] `detail=` sanitization.** The underlying error string may contain `"` or newlines; it must be escaped so the emitted line stays single-line and parseable. MUST be resolved before code (defines the escaping helper).
- **[RESOLVED] token stability.** The `reason` token set is a machine ABI: `bad_authkey`, `control_plane_unreachable`, `unknown`. New categories may be added later but existing tokens never change meaning.

## Acceptance criteria

- [ ] On successful startup, `connect` writes exactly one `READY local=<addr> target=<addr>` line to stdout before accepting connections, with `local` reflecting the **actual bound** listener address (verified where the requested port differs from the bound port).
- [ ] `--quiet` suppresses the decorative banner and "Waiting for connections…" text; the `READY` line still emits.
- [ ] On tsnet init failure, `connect` writes one `ERROR reason=<token> detail="…"` line to stderr and exits non-zero, with `token` = `bad_authkey` / `control_plane_unreachable` / `unknown` matching the classified cause (table-driven test over the three categories).
- [ ] An unrecognized init failure still emits `ERROR reason=unknown detail="…"` (never silent).
- [ ] `detail="…"` is a single line with embedded quotes/newlines escaped so the line is machine-parseable — unit-tested.
- [ ] No `--profile`/`.env`/existing-flag behavior changes; existing `Run` tests pass unchanged (regression).

## References

- Bitácora board: ts-bridge#203 (READY line) and ts-bridge#204 (structured exit reason) — combined PR
- Related spec: `specs/archive/BUG-001-tsnet-session-death-signal/` (runtime death; this is the startup complement it names)
- Touch points: `cmd/cli/run.go` (`Run`, `initTailscale`, `diagnoseTailscaleInitError`, `printBanner`), `cmd/cli/connect.go` (`--quiet` flag)
- Downstream consumer: kubelab TOOL-015 `ts_bridge_tunnel` context manager (ADR-052)
