---
id: lesson-019-tsnet-server-up-partial-start-must-close-on-e
type: lesson
status: active
created: "2026-05-28"
owner: manu
tags: [ts-bridge, lesson, go, tsnet, windows, file-locking, error-handling]
---

# tsnet.Server.Up() partial-start: must Close() on error or Windows cleanup leaks

**Context:** Alongside the auth failure, v1.5.0 emitted `failed to cleanup ephemeral state directory ... tailscaled.log1.txt: The process cannot access the file because it is being used by another process` (5 retries, all fail). Root cause was in `initTailscale` (PR #18).

**Problem:** `tsnet.Server.Up(ctx)` is **not** all-or-nothing. By the time it returns an error, it has already spawned background goroutines (logger, control client, NetMon, magicsock) and opened `tailscaled.log*.txt` for writing. The previous code returned the error without calling `server.Close()`. The deferred `cleanupEphemeralStateDir` then tried to `os.RemoveAll` the temp dir, but Windows file locks are *mandatory* (unlike POSIX advisory locks), so `unlinkat` on the log file fails for every retry until the process exits. On Linux the same code path succeeds because `unlink()` on an open file works.

**Fix:** Call `server.Close()` on the error path *before* the cleanup defer fires. This releases the file handles synchronously.

```go
status, err := server.Up(ctx)
if err != nil {
    _ = server.Close() // release tsnet workers holding tailscaled.log*
    return nil, fmt.Errorf("tailscale init failed (control=%s): %w", controlURLForError(cfg.ControlURL), err)
}
```

**Rule (generalized):** Any Go constructor that *might* have spawned background goroutines or opened OS handles before returning an error must be paired with an explicit `Close()` on the error path, even if a later API audit would suggest "we never started anything." The contract for `tsnet.Server.Up()` is implicit: if you call it, you must call `Close()` *regardless of return value*.

**Secondary UX win added in the same PR:** the error wrap now includes the effective control URL — `tailscale init failed (control=https://controlplane.tailscale.com (default)): ...` — so the operator immediately sees whether the key was sent to SaaS or a custom plane. Diagnosis time on the next occurrence drops from ~30min to ~5s.

**Diagnostic pattern matching:** Added `diagnoseTailscaleInitError` helper that maps known tsnet error substrings to actionable `WARN` log lines with a `remediation` field. Fail-silent on unknown patterns to avoid log noise. Table-driven tests in `main_test.go`.

**Tags:** `#go` `#tsnet` `#windows` `#file-locking` `#error-handling`
