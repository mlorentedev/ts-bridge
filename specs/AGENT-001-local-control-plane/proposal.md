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
and inspect sessions, and that something must know who is asking. ADR-018 (accepted and merged
2026-11-14, revised after two adversarial reviews) decides the shape: one per-user agent owns the
sessions, reached over an identity-checked local socket, and every other surface is a client. This
spec covers ADR-018 §1 and §2 plus the `session` CLI that is the first client (#782).

## What

A per-user agent process (`ts-bridge agent`) and a versioned HTTP/JSON API under `/v1/`, delivered in
five PRs (see `tasks.md`):

1. **Agent and Unix transport (PR 1).** The base directory from the OS user identity, one agent per
   user under an OS-backed lock, a Unix socket whose directory and peer credentials are checked on
   both ends, `GET /v1/meta`, `GET /v1/status`, the `{code,message,detail}` error envelope, and
   `ts-bridge agent` in the foreground. On Windows the agent refuses to start until PR 2.
2. **Windows transport (PR 2).** The named pipe with an SDDL for the user's SID,
   `FILE_FLAG_FIRST_PIPE_INSTANCE` and `PIPE_REJECT_REMOTE_CLIENTS`, a per-connection token check, and
   a client that opens with SQOS identification and checks the server's SID through
   `GetNamedPipeServerProcessId`.
3. **Session supervisor (PR 3).** Children running `connect --profile <name> --lifeline <fd|handle>
   --quiet`, the lifeline on every OS plus a Job Object on Windows, `READY`/`ERROR` parsing, the
   holder metadata in the per-profile lock, and `/v1/sessions` list, start, stop and status.
4. **Recovery and profiles (PR 4).** The state machine (`stopped`, `starting`, `ready`, `backoff`,
   `failed`, `stopping`) with a budget of three attempts, persisted desired state, the audit line per
   mutation, and `/v1/profiles` list and import through the ADR-015 transition.
5. **Session CLI (PR 5).** `ts-bridge session list|start|stop|status` as an API client, and the
   mutual-exclusion errors between `connect --profile X` and an agent session of X.

### PR 1 detail

- `internal/oslock` holds the advisory file lock (`flock` on Unix, `LockFileEx` on Windows) that
  `internal/profile` already uses for ownership and for the store lock. It moves there unchanged, so
  the agent lock and the profile locks are one primitive (ADR-018 §1).
- `internal/agent` owns the paths, the lock, the listener, the identity check and the handler. Its
  project-local import is `oslock` only.
- **Base directory (ADR-018 §1).** Resolved from the OS user identity, never from the environment:
  the home directory from the user database (`os/user.LookupId` of the effective uid, which is
  `getpwuid_r` on macOS and `/etc/passwd` on a cgo-less Linux build), never `$HOME`, `$XDG_*` or
  `$TMPDIR`. Linux: `<home>/.local/state/ts-bridge`. macOS: `<home>/Library/Application
  Support/ts-bridge`. Windows: the LocalAppData known folder of the process token plus `ts-bridge`.
  It holds `agent.lock`. A user the database cannot name is refused with that cause.
- **Socket directory (ADR-018 §2).** Linux: `/run/user/<uid>/ts-bridge` when `/run/user/<uid>` exists,
  is a real directory, is owned by the user and has mode 0700; otherwise `/tmp/ts-bridge-<uid>`.
  macOS: `<base>/run`. Created 0700. It must be a real directory (not a symlink) owned by the
  effective uid; a directory we own with looser permissions is tightened, one owned by anyone else is
  refused.
- **Identity.** An `Identity` is a string: the decimal uid on Unix, the SID string on Windows. One
  comparison, `Authorize(peer, owner)`, serves both and is tested directly with fake identities,
  because a cross-user connection cannot be made in CI. A wrapped listener identifies each accepted
  connection and closes the ones that fail, without stopping the accept loop.
- **Peer credentials.** Linux `SO_PEERCRED`, macOS `LOCAL_PEERCRED`, both through `x/sys/unix`. Other
  Unix systems fail closed ("peer credentials are not available").
- **Client checks the server (ADR-018 §2).** Before sending a request, the client checks that the
  socket directory is a real directory owned by its uid with mode 0700, then reads the server's peer
  credentials on the connected socket and requires its own uid. A failed check closes the connection
  before any byte is written.
- **Single instance.** The agent takes `agent.lock` in the base without waiting; a second agent fails
  with "another ts-bridge agent is already running for this user". The kernel drops the lock with its
  holder, so a crash leaves nothing to clear. The lock file is never removed. (The bounded wait and
  holder metadata in ADR-018 §1 belong to the per-profile lock, PR 3.)
- **Stale socket.** Lock first, then remove a leftover socket and listen. Only a socket is removed;
  anything else at that path is an error.
- **Socket path length.** A path at or above `sun_path` (104 bytes on macOS, 108 on Linux) is refused
  at startup with the path named.
- **Contract.** `GET /v1/meta` answers `{"agent_version","api_min","api_max"}` (both `1` for now).
  `GET /v1/status` answers `{"pid","started"}` (RFC 3339). Every error, including an unknown path and
  a wrong method, is `{"code","message","detail"}` with a stable `code` (`not_found`,
  `method_not_allowed`). The skew rule (a client outside the range keeps reads, refuses mutations) has
  nothing to apply to until the first mutation in PR 3, so PR 1 ships the range and PR 3 the check.
- **`ts-bridge agent`** runs the agent in the foreground until interrupted. It exists in PR 1 so the
  listener can be exercised through the real binary; it adds no session behaviour.

## Decisions

- **Windows named pipes: `github.com/tailscale/go-winio` (PR 2).** `golang.org/x/sys/windows` alone
  has no overlapped-I/O pipe listener, so a pipe server on it would be a hand-written `net.Listener` of
  roughly 200 lines of handle and overlapped-I/O code. The fork is already in `go.mod` as an indirect
  requirement with its `go.sum` hashes (tailscaled's `safesocket` listens with it). Importing it
  directly adds no module and no download; `go mod tidy`, which CI enforces, only removes its
  `// indirect` marker. `go.mod` is on the frozen list; the operator allowed that one-line change on
  2026-11-14. `go.sum` must not change.
- **Windows in its own PR.** The pipe server, the token check and the client's SQOS and
  `GetNamedPipeServerProcessId` check are new Windows-only code that runs only in the `test-windows`
  job. Keeping them out of PR 1 keeps each PR under the cap and lets the Unix contract land first.
- **Lock primitive: extracted, not duplicated.** `oslock` is a ~30-line package moved out of
  `internal/profile`. Exporting a wrapper from `profile` instead would make the agent depend on the
  profile store for a file lock.
- **Peer credentials through `x/sys/unix`, not `tailscale/peercred`.** Ten lines per OS on a security
  boundary, and no new direct dependency edge. Reopen if more Unix targets are added.
- **No TCP, ever.** Not even loopback (ADR-018 G-C7).
- **The base never follows the environment.** A desktop session and a login supervisor see different
  environments; a base that followed `$XDG_STATE_HOME` would give them two locks and two agents
  (ADR-018 §1, G-C5). The Linux socket path depends on the uid alone for the same reason.

## Out of scope

- Windows transport (PR 2); sessions, recovery, profiles and the `session` CLI (PR 3 to 5). `connect`
  is untouched in PR 1.
- `agent enable` / `disable` and the recorded base (#783), `ts-bridge mcp` (#784), the tray (#785).
- Authorization beyond "same user" (ADR-018 §2 trust boundary).
- Discovery: ADR-018 keeps it out of the API.

## Risks / open questions

- A cgo-less Linux build reads `/etc/passwd` for the home directory, so a user known only through
  NSS (LDAP, SSSD) cannot start the agent. The error names the cause. Reopen if such a user appears;
  the fix is a cgo build, not a fallback to `$HOME`.
- `LOCAL_PEERCRED` on macOS returns the uid of the process that called `connect` or `listen`, which is
  the right answer on both ends of a same-user socket.
- A home directory on a network filesystem may not give a reliable `flock` (as for SEC-391). The agent
  state is per machine; documented, not handled.

## Acceptance criteria

- [ ] AC1: a second agent for the same user fails to start while the first runs, and starts after it
  stops; a crashed agent's leftover socket does not block the next start, a held lock never loses its
  socket to a second starter, and a non-socket file at the socket path is never removed.
- [ ] AC2: the base comes from the user database and ignores `$HOME` and `$XDG_*`; the Linux socket
  directory depends on the uid only; the socket directory is 0700 and owned by the user, the socket is
  0600, a symlinked or foreign-owned directory is refused, and a socket path at the `sun_path` limit
  is refused.
- [ ] AC3: a connection whose peer identity differs from the agent's is closed before any request is
  read, a peer that cannot be identified is refused, and the accept loop keeps serving the owner
  (fake identities on every OS; the same-user path through the real transport on Linux and macOS).
- [ ] AC4: the client refuses a socket directory it does not own or that is not 0700, and a server
  whose peer identity is not its own, before writing a byte.
- [ ] AC5: `GET /v1/meta` and `GET /v1/status` answer over the real transport, and every error is the
  `{code,message,detail}` envelope; `ts-bridge agent` serves until interrupted.
- [ ] AC6: the Windows pipe admits only the user's SID, and the client verifies the server's SID (PR 2).
- [ ] AC7: sessions are children of the agent tied to it by the lifeline on every OS (PR 3).
- [ ] AC8: the state machine retries three times then holds `failed`; a user stop is kept; import
  stores the credential only through the ADR-015 transition (PR 4).
- [ ] AC9: `session` commands and `connect --profile` name each other when they collide (PR 5).

## References

- Issue #782; ADR-018 (§1, §2); ADR-016 (ownership lock); ADR-015 (credential store); ADR-007
  (package table)
- `tailscale/tailscale` `safesocket/`, `ipn/ipnauth/`
