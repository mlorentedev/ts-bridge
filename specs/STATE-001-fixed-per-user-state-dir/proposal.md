---
id: "STATE-001"
type: spec
status: draft # draft | implementing | verifying | archived
created: "2026-06-24"
issue: "ts-bridge#207"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal, bug, reliability]
template_version: "1.0"
---

# STATE-001: Fixed per-user state directory (stop leaking node identity into the CWD)

## Why

<!-- from issue #207: connect stores tsnet state in a CWD-relative ./ts-state -->

`ts-bridge connect` writes its tsnet state — including `tailscaled.state`, which holds the
node's **private identity** — to a **CWD-relative `./ts-state`** directory. Running the bridge
from any directory drops a `ts-state/` there. If that directory is a git working tree (especially
one with auto-commit, e.g. an Obsidian vault with `obsidian-git`), the node identity gets staged,
committed, and pushed: a **secret leak**. Logs already land in a fixed per-user location
(`LogDirForPlatform`); state does not — the two are inconsistent.

Root cause (traced): `connect` resolves config via `config.Merge` (precedence flags > env > yaml >
defaults). With no `--state-dir`/`TS_STATE_DIR`, `StateDir` stays `""` and falls through to the
final fallback `defaultStateDir = "./ts-state"` (`merge.go`), which is CWD-relative and
**non-ephemeral**. There is no `StateDirForPlatform()` analogue to `LogDirForPlatform()`.

## What

1. Add `StateDirForPlatform()` — a fixed, per-user, CWD-independent, **always-absolute** state
   directory, mirroring `LogDirForPlatform()`:
   - Windows: `%LOCALAPPDATA%\ts-bridge\state`
   - macOS:   `~/Library/Application Support/ts-bridge/state`
   - Linux/other: `$XDG_STATE_HOME/ts-bridge/state`, falling back to `~/.local/state/ts-bridge/state`
   - Hard guarantee: if the platform base resolves empty (no `HOME`/`LOCALAPPDATA`), fall back to
     `os.TempDir()/ts-bridge/state` so the result is **never CWD-relative**.
2. Use `StateDirForPlatform()` as the default (the manual-mode fallback) in both config loaders
   (`config.go` legacy env path and `merge.go` connect path), replacing the `./ts-state` constant.
3. Make the auto-instance **ephemeral** branch in `merge.go` write under a temp dir (mirroring the
   legacy loader) instead of `./ts-state`, so even the throwaway-identity path never touches the CWD.
4. Defense-in-depth: when the **resolved** state dir is not absolute (only reachable via an explicit
   relative `--state-dir`/`TS_STATE_DIR`/yaml value), emit a non-blocking `Warn` about CWD-relative
   node-identity leakage.

## Out of scope

- **Auto-migration of an existing `./ts-state`.** Moving a live `tailscaled.state` is risky
  (in-use file, cross-filesystem). Users who relied on `./ts-state` keep it explicitly via
  `--state-dir ./ts-state`. The default change is documented; no silent move.
- **Refusing (hard error) on a CWD-relative state dir.** Chose warn-only (non-blocking) so explicit
  relative overrides remain usable; a hard refuse + git-tree detection was considered and deferred.
- The #185 profiles model / `--state-dir` redesign. This PR only fixes the unsafe **default**.

## Risks / open questions

- **[RESOLVED] Default-location change is a behavior change.** Existing users with a node identity in
  a binary-adjacent `./ts-state` get a *fresh* identity at the new path → a new device in the control
  plane and a possible orphan. Accepted: documented in `docs/` + troubleshooting; `--state-dir
  ./ts-state` restores prior behavior. No auto-migration (see Out of scope).
- **[RESOLVED] Path convention.** Chose semantic XDG state dirs (Linux `~/.local/state`, macOS
  `Application Support`) over mirroring logging's base (`~/.local/share`, `~/Library/Logs`). State ≠
  logs; honors `$XDG_STATE_HOME`.
- **[RESOLVED] Empty-base edge case.** `LogDirForPlatform` can return a relative path if `HOME`/
  `LOCALAPPDATA` is empty — the latent form of this very bug. `StateDirForPlatform` guards with an
  absolute `os.TempDir()` fallback.

## Acceptance criteria

- [ ] `connect` with no `--state-dir`/`TS_STATE_DIR`/yaml resolves `StateDir` to an **absolute**,
      per-user path containing `ts-bridge` — never `./ts-state` (unit test on `Merge`).
- [ ] `StateDirForPlatform()` returns the correct per-OS path, honors `$XDG_STATE_HOME` on Linux,
      and is **always absolute** even when `HOME`/`LOCALAPPDATA`/`XDG_STATE_HOME` are all unset.
- [ ] Auto-instance ephemeral state (auto mode + derived hostname) resolves under a temp dir, not a
      CWD-relative path, with `EphemeralState == true`.
- [ ] Explicit overrides still win and are preserved verbatim: `--state-dir`, `TS_STATE_DIR`, and
      yaml `state_dir` (including relative values, for back-compat).
- [ ] A resolved **relative** state dir emits a non-blocking `Warn` naming the leakage risk; an
      absolute one does not.
- [ ] No regression: manual mode keeps `EphemeralState == false`; the existing config/merge suites
      pass with assertions updated for the new default.
- [ ] `docs/` (getting-started, configuration, cli-reference) and `docs/troubleshooting` reflect the
      new default state location; the legacy `./ts-state` default is corrected.

## References

- Bitácora board: ts-bridge#207
- Mirrors: `internal/logging/logging.go` `LogDirForPlatform()`
- Touch points: `internal/config/{config.go,merge.go}`, `cmd/cli/run.go` (`ensureStateDir`)
