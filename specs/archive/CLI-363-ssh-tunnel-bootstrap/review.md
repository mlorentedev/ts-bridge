---
spec: "CLI-363-ssh-tunnel-bootstrap"
verdict: "PASS WITH GAPS"
reviewed_sha: "fe571e476cef5c6d63a0d2dc895ea776f3301846"
reviewer: "agy/gemini-3.1-pro-high"
date: "2026-09-29"
---

## Adversarial review

**Scope**: CLI-363-ssh-tunnel-bootstrap
**Sources**: `specs/CLI-363-ssh-tunnel-bootstrap/{proposal,tasks,verification}.md` + `git diff 1c5a966e5104eeb485d1cae7ebe918abbb708582...HEAD`

### Spec and task alignment
- `proposal.md` acceptance criteria are cleanly mapped to tests and `features.json`.
- Proxy hook configuration properly isolates control-plane traffic via `socks5h` proxy, preserving other traffic fallback behavior (native WPAD/PAC hooks are not overwritten, as `tshttpproxy` natively drops to `sysProxyFromEnv` if `localProxyFunc` yields nil).
- Scope remains perfectly constrained to the proposal.

### Findings

| Severity | Reality | Area | Finding | Evidence | Test (named, or UNTESTED) | Fix location (code / tests / spec / vault) |
|----------|---------|------|---------|----------|---------------------------|---------------------------------------------|
| Minor | THEORETICAL | Resilience | Unhandled process orphaning on ungraceful shutdown. If `ts-bridge` panics or is killed via `SIGKILL`/Task Manager, the `ssh` child process orphans and continues running indefinitely (holding the SOCKS port open). `exec.CommandContext` on Windows does not use Job Objects by default to tie the child to the parent's lifecycle. | `cmd.Start()` usage in `internal/bootstrapssh/bootstrap.go` without Windows Job Object/`Pdeathsig` equivalent wrapping. | UNTESTED | code |
| Minor | SPECULATIVE | Configuration | Collision between `BootstrapSOCKSAddr` and `HealthAddr` is not validated. If they overlap, `ssh` binds the port first, and the health server fails to start, breaking the `/metrics` endpoint silently. | `validateBootstrapListenerCollision` checks `LocalAddr` and `SOCKS5Addr`, but omits `HealthAddr`. | UNTESTED | code + tests |

### Evaluator rubric

| Dimension | Grade (A-D) | Rationale (one line) |
|-----------|-------------|----------------------|
| Correctness        | B | Criteria met on happy path and almost all edge cases, minor orphan gap. |
| Verification       | A | Evidence proves each criterion with precise test mappings and reproducible states. |
| Scope              | A | Diff matches proposal exactly; no unrelated changes mixed in. |
| Reliability        | B | Clean graceful shutdown, but OS-level ungraceful kill leaves a daemonized SOCKS proxy. |
| Maintainability    | A | Code is clean, functions are small, and proxy setup logic is highly readable. |
| Handoff-readiness  | A | Spec updates, features.json, and `docs/lessons` accurately captured. |

### Verdict
PASS WITH GAPS

### Recommended next steps
- Add `HealthAddr` to `validateBootstrapListenerCollision` to reject overlaps upfront.
- Track the orphaned SSH process behavior in `docs/troubleshooting/` or evaluate if a Windows Job Object API (or equivalent termination wrapper) is worth introducing in a separate reliability pass.
