---
id: "AGENT-001-local-control-plane"
type: spec
status: implementing # draft | implementing | verifying | archived
created: "2026-11-14"
issue: "mlorentedev/ts-bridge#782"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal]
template_version: "1.0"
owner: manu
---

# AGENT-001-local-control-plane

## Why

ts-bridge can only be operated from a terminal, and `connect` runs in the foreground (ADR-016). A tray
app (#785) and an MCP server (#784) need something they can ask to list profiles and to start, stop
and inspect sessions, and that something must know who is asking. ADR-018 (accepted 2026-11-14, on the
branch `docs/adr-018-local-control-plane`, not yet on master) decides the shape: one per-user agent
owns the sessions, reached over an identity-checked local socket, and every other surface is a client.
This spec covers ADR-018 §1 and §2 plus the `session` CLI that is the first client (#782).

## What

A per-user agent process (`ts-bridge agent`) and a versioned HTTP/JSON API under `/v1/`, delivered in
four PRs (see `tasks.md`):

1. **Listener and identity (PR 1).** One agent per user, guarded by an OS-backed lock in the user's
   state directory. A listener that only accepts the agent's own user: a Unix socket in a 0700
   directory (mode 0600, peer credentials checked per connection) on Linux and macOS, a named pipe
   with an SDDL for the user's SID and a per-connection token check on Windows. A stale socket is
   removed only while holding the lock. `GET /v1/status`.
2. **Session supervisor (PR 2).** Sessions are child processes running `connect --profile <name>`,
   tied to the agent's lifetime, with a desired-running set in the state directory so an unexpected
   stop is restored and a user stop is respected. `/v1/sessions` list, start, stop and status.
3. **Profiles and discovery (PR 3).** `/v1/profiles` list and import (a `tsb://` descriptor plus a
   credential, written through the ADR-015 store code and nowhere else), and discovery.
4. **Session CLI (PR 4).** `ts-bridge session list|start|stop|status` as an API client, and the
   mutual-exclusion errors between `connect --profile X` and an agent session of X.

### PR 1 detail

- `internal/oslock` holds the advisory file lock (`flock` on Unix, `LockFileEx` on Windows) that
  `internal/profile` already uses for ownership and for the store lock. It moves there unchanged, so
  the agent lock and the profile locks are one primitive, as ADR-018 §1 says ("the same mechanism
  ADR-016 uses per profile").
- `internal/agent` owns the state directory, the lock, the listener, the identity check and the
  handler. Its project-local import is `oslock` only.
- **State directory.** Linux: `$XDG_RUNTIME_DIR/ts-bridge`, falling back to the user cache directory
  when that is unset or not absolute. macOS and Windows: the user cache directory plus `ts-bridge`.
  Created 0700. On Unix it must be a real directory (not a symlink) owned by the effective uid; looser
  permissions on a directory we own are tightened, a directory owned by anyone else is refused.
- **Identity.** An `Identity` is a string: the decimal uid on Unix, the SID string on Windows. One
  comparison function, `Authorize(peer, owner)`, serves both and is tested directly with fake
  identities, because a cross-user connection cannot be made in CI. A wrapped listener identifies each
  accepted connection and closes the ones that fail, without stopping the accept loop.
- **Peer credentials.** Linux `SO_PEERCRED`, macOS `LOCAL_PEERCRED`, both through `x/sys/unix`. Other
  Unix systems are not supported targets and fail closed ("peer credentials are not available").
- **Windows pipe.** `\\.\pipe\ts-bridge-agent-<SID>-<hash of the state directory>`; the hash keeps a
  test agent from colliding with a real one and is stable for the default directory. Created with
  `PIPE_REJECT_REMOTE_CLIENTS` and `FILE_FLAG_FIRST_PIPE_INSTANCE` (both set by go-winio), and the
  SDDL `D:P(A;;GA;;;<SID>)`: a protected DACL granting only that SID. Each connection's client process
  token SID is compared too, as defence in depth.
- **Single instance.** The agent takes `agent.lock` in the state directory without waiting; a second
  agent fails with "another ts-bridge agent is already running for this user". The kernel drops the
  lock with its holder, so a crash leaves nothing to clear. The lock file is never removed.
- **Stale socket.** Lock first, then remove a leftover socket and listen. Only a socket is removed;
  anything else at that path is an error.
- **`GET /v1/status`** answers `{"api":"v1","pid":<n>,"started":"<RFC 3339>"}`.
- **`ts-bridge agent`** runs the agent in the foreground until interrupted. It exists in PR 1 so the
  listener can be exercised through the real binary; it adds no session behaviour.

## Decisions

- **Windows named pipes: `github.com/tailscale/go-winio`.** `golang.org/x/sys/windows` alone has no
  overlapped-I/O pipe listener, so a pipe server on it would be a hand-written `net.Listener` of
  roughly 200 lines of handle and overlapped-I/O code that nobody here can test on a Windows machine
  but CI. go-winio is already in the module graph: `tailscale.com` requires both
  `github.com/Microsoft/go-winio` v0.6.2 and `github.com/tailscale/go-winio` (the fork tailscaled's
  own `safesocket` listens with), and `go.mod` already lists the fork as an indirect requirement with
  its `go.sum` hashes. Importing it directly adds no module and no download; `go mod tidy` only moves
  its line from `// indirect` to the direct block. The fork is chosen over `Microsoft/go-winio`
  because only the fork is already a `go.mod` entry, so `go.sum` does not change. Everything else on
  Windows (process token, SID, `GetNamedPipeClientProcessId`) uses `x/sys/windows`, already direct.
  `go.mod` is the only dependency file that changes and `go.sum` must not.
- **Lock primitive: extracted, not duplicated.** `oslock` is a new 30-line package moved out of
  `internal/profile`. The alternative (export a wrapper from `profile` and import `profile` from
  `agent`) would make the agent depend on the profile store for a file lock.
- **Peer credentials through `x/sys/unix`, not `tailscale/peercred`.** The latter is in the module
  graph too, but this is ten lines per OS on a security boundary, and writing them avoids another
  direct dependency edge. Reopen if more Unix targets are added.
- **No TCP, ever.** Not even loopback (ADR-018 G-C7).
- **The runtime directory fallback.** `XDG_RUNTIME_DIR` is absent without a login session
  (containers, `su`); the cache directory is per user and gets the same 0700 and ownership checks.
- **Unix socket paths are short.** The limit is 104 bytes on macOS and 108 on Linux. The agent
  refuses a state directory whose socket path would exceed 100 bytes with an error naming the path,
  rather than letting `bind` fail with "invalid argument".

## Out of scope

- Sessions, profiles, discovery and the `session` CLI (PR 2 to 4). `connect` is untouched.
- `agent enable` / `disable` (#783), `ts-bridge mcp` (#784), the tray (#785).
- Authorization beyond "same user": any process of the owning user may call the API, as with
  tailscaled's LocalAPI for the owner. Per-client scopes are not in ADR-018.

## Risks / open questions

- The Windows listener and token check are compiled and vetted for `GOOS=windows` locally but run
  only in the `test-windows` job. The first CI run is the evidence for them.
- `LOCAL_PEERCRED` on macOS returns the uid of the process that called `connect` or `listen`, which
  is the right answer for a same-user agent.
- A user whose cache directory is on a network filesystem may not get a reliable `flock`
  (as for SEC-391). The agent state is per machine; documented, not handled.

## Acceptance criteria

- [ ] AC1: a second agent for the same user fails to start while the first runs, and starts after it
  stops; a crashed agent's leftover socket does not block the next start, a held lock never loses its
  socket to a second starter, and a non-socket file at the socket path is never removed.
- [ ] AC2: the state directory is 0700 and owned by the user, the socket is 0600 (Unix), and a
  symlinked or foreign-owned directory is refused.
- [ ] AC3: a connection whose peer identity differs from the agent's is closed before any request is
  read, a peer that cannot be identified is refused, and the accept loop keeps serving the owner
  (tested with fake identities on every OS; the same-user path through the real transport on every
  OS).
- [ ] AC4: `GET /v1/status` answers over the real transport on Linux, macOS and Windows.
- [ ] AC5: sessions are children of the agent and die with it on every OS; the desired set restores
  an unexpected stop and respects a user stop (PR 2).
- [ ] AC6: profile import stores the credential only through the ADR-015 store (PR 3).
- [ ] AC7: `session` commands and `connect --profile` name each other when they collide (PR 4).

## References

- Issue #782; ADR-018 (branch `docs/adr/adr-018-local-control-plane`); ADR-016 (ownership lock);
  ADR-015 (credential store); ADR-007 (package table)
- `tailscale/tailscale` `safesocket/`, `ipn/ipnauth/`
