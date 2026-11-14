---
id: "adr-018"
type: adr
status: accepted
owner: manu
date: "2026-11-14"
issue: "mlorentedev/ts-bridge#451"
tags: [architecture, control-plane, gui, mcp, lifecycle, cross-platform]
created: "2026-11-14"
---

# ADR-018: A Per-User Agent with a Local API, a Tray App and an MCP Server

## Status

Accepted by the operator on 2026-11-14. Tracked by #451.

## Context

ts-bridge can only be operated from the CLI, and `connect` runs in the
foreground (ADR-016). The operator wants to drive it from a GUI on macOS, Linux
and Windows alike, and from AI agents through an MCP server. Nothing can list
profiles or start, stop or list sessions programmatically. The only
machine-readable surfaces are `status --json` and the health and metrics
endpoints of a process that is already running.

ADR-016 rejected a background supervisor (its option B) with this reopen
trigger: "a concrete consumer requires a successful second invocation to reuse
an unattended session; specify ownership and recovery first." A tray app and
an MCP server are that consumer, so this ADR reopens option B and specifies
both.

### Constraints

| ID | Constraint | Origin |
| --- | --- | --- |
| G-C1 | The app lives in the tray or menu bar on all three OSes. Sessions outlive the window, and it can start at login | Owner, #451 |
| G-C2 | Minimalist: a profile list, connect/disconnect, status, and no other settings surface | Owner, #451 |
| G-C3 | First run takes one input (a `tsb://` descriptor or a discovered host) plus the credential, with defaults for everything else | Owner, #451 |
| G-C4 | The core stays one pure-Go binary that cross-compiles without cgo, with no new dependency without strong justification | AGENTS.md, ADR-002 rationale |
| G-C5 | Per-profile, OS-backed ownership: no duplicates, no takeover, no PID-only locks | ADR-016 C5 |
| G-C6 | ts-bridge installs no system service | ADR-017 Amendment A |
| G-C7 | The local control surface knows its caller, so no bare TCP | Audit: NetBird deprecated TCP for that reason |
| G-C8 | MCP: at most ~12 tools, with safety annotations | `pattern-mcp-tool-design` |
| G-C9 | Credentials stay in the ADR-015 store, and the GUI holds no secrets | ADR-015 |

### Reference audit

The full matrix is in the maintainer's store
(`prestudy/gui-control-plane-reference-audit-matrix.md`). Each reference was
read from primary sources on 2026-11-14.

| Reference | Who owns sessions | Local IPC and auth | GUI |
| --- | --- | --- | --- |
| Tailscale | `tailscaled` daemon | LocalAPI (HTTP) over a Unix socket with peer credentials; a Windows named pipe with an SDDL and a per-connection token check | Swift (macOS), Go + win32 (Windows), Go `fyne.io/systray` (Linux) |
| NetBird | daemon | gRPC over a Unix socket or named pipe with peer credentials; TCP deprecated as carrying "no caller identity" | Wails v3, as a separate binary (it replaced Fyne) |
| WireGuard (Windows) | manager service | inherited pipes into a UI process the manager launches | Go + walk |
| Syncthing | daemon | REST on 127.0.0.1 with an API key | web UI; separate tray apps |
| Ollama | the app spawns the server | HTTP on 127.0.0.1, no auth | Go + webview; no Linux app |

Four of the five references have one process own the sessions, with thin
clients reaching it over a local socket that identifies the caller. Every one
ships the GUI as a separate binary or process. On Linux the tray is SNI over
D-Bus, and GNOME needs the AppIndicator extension, which Ubuntu preinstalls.

Measured locally on 2026-11-14:

- `fyne.io/systray` v1.12.2 builds with `CGO_ENABLED=0` for Linux and Windows,
  and fails for macOS. Wails v3 uses Obj-C on macOS and gtk4 plus webkitgtk on
  Linux. Every macOS tray path therefore needs cgo, which G-C4 forbids in the
  core binary.
- `modelcontextprotocol/go-sdk` v1.8.0 adds four modules that ts-bridge does
  not already require (`google/jsonschema-go`, `segmentio/encoding`,
  `segmentio/asm`, `yosida95/uritemplate`). The rest of its graph (`x/oauth2`,
  `x/sync`, `x/time`, `x/sys`) is already in `go.mod`.

## Options Considered

| Option | Door | G-C1 | G-C3 | G-C4 | G-C6 | G-C7 | Verdict |
| --- | --- | --- | --- | --- | --- | --- | --- |
| A: the tray app owns sessions (Ollama) | 2 | yes | yes | yes | yes | n/a | rejected: the MCP server would need the tray running |
| B: a per-user agent with a local API; tray, CLI and MCP as clients | 1 | yes | yes | yes | yes | yes | **chosen** |
| B': B as a system service (root/SYSTEM) | 1 | yes | yes | yes | no | yes | rejected, see below |
| C: a native app per OS (SwiftUI / WinUI / GTK) | 1 | yes | yes | yes | yes | yes | rejected: three codebases, one maintainer |
| D: Fyne (full toolkit) | 2 | yes | yes | separate binary | yes | yes | rejected: cgo plus OpenGL on every OS; looks non-native; NetBird left it |
| E: a menu-only tray (`fyne.io/systray`), first run in the CLI | 2 | yes | no | separate binary | yes | yes | rejected on G-C3 |
| F: Wails v3 for the tray binary | 2 | yes | yes | separate binary | yes | yes | **chosen for the GUI** |

## Decision

### 1. `ts-bridge agent`: per-user, owns sessions

- A process of the logged-in user. It is not root, SYSTEM or a system service.
  tsnet runs in userspace and changes no system network state, so no privilege
  is needed. Credentials are per user (ADR-015), so a per-user owner reads them
  without crossing users.
- One agent per user, guarded by an OS-backed lock in the user's state
  directory. That is the same mechanism ADR-016 uses per profile, and never a
  PID file.
- Each session is a child process running the existing foreground
  `connect --profile <name>` path. The child holds the ADR-016 per-profile
  lock, so `connect --profile X` from a terminal and a session of X in the
  agent exclude each other, and each side fails with an actionable error that
  names the other. Children are tied to the agent's lifetime (`Pdeathsig` on
  Linux, a Job Object on Windows, a lifeline pipe on macOS), so an agent crash
  leaves no orphan holding a lock.
- Recovery: the agent records which profiles should be running in its state
  directory. After a restart it brings back the sessions that were running
  when it stopped unexpectedly. A session the user stopped stays stopped.

### 2. The local API: identity-checked, no TCP

- HTTP/JSON, versioned under `/v1/`. It covers profiles (list, import a `tsb://`
  descriptor with its credential), sessions (list, start, stop, status), and
  discovery. Importing writes the credential through the ADR-015 store code,
  so neither the GUI nor the MCP server ever persists a secret.
- Unix (Linux, macOS): a socket in a per-user directory with mode 0700
  (`$XDG_RUNTIME_DIR/ts-bridge/` on Linux, the user cache directory on macOS).
  The socket is 0600, and every connection's peer credentials (`SO_PEERCRED`,
  `getpeereid`) must match the agent's uid.
- Windows: a named pipe that includes the user's SID, with an SDDL granting
  only that SID. Each connection checks the client process token's SID.
- A stale socket is removed only after the agent lock is acquired.

### 3. Autostart: registered per user, never as a system service

`ts-bridge agent enable` and `agent disable` register and remove a per-user
login start, idempotently and without elevation, using a restart mechanism on
each OS:

| OS | Registration | Restart on crash |
| --- | --- | --- |
| Linux | systemd `--user` unit (XDG autostart where there is no user systemd) | `Restart=on-failure` |
| macOS | LaunchAgent in `~/Library/LaunchAgents` | `KeepAlive` |
| Windows | Scheduled Task at logon for the user | the task's restart-on-failure setting |

The tray also relaunches the agent when it finds the socket dead.

### 4. `ts-bridge-tray`: a separate binary, Wails v3

- A separate Go module (`gui/`, with its own `go.mod`), so Wails and cgo never
  reach the core's dependency graph (G-C4). It is built natively per OS and
  released as its own artifacts.
- It is a client of the local API only and owns no session or secret.
- Surface (G-C2): a tray or menu-bar icon whose state shows whether any
  session is up, and a menu listing the profiles with connect/disconnect. One
  small window holds the profile list, status, and the first-run form, which
  is the single field for a `tsb://` descriptor or a discovered host plus the
  credential (G-C3). It also offers "start at login" (`agent enable`).
- On GNOME without the AppIndicator extension, the window opens from the
  application launcher and the app stays usable without the tray.
- Pin Wails v3.1.x. Wails v2.17 is the fallback if v3 regresses. The GUI stays
  thin enough that swapping its toolkit is a Type 2 change.

### 5. `ts-bridge mcp`: stdio, official SDK, a client of the agent

- A subcommand in the core binary that uses `modelcontextprotocol/go-sdk`.
  Four new modules are justified because the SDK tracks protocol revisions
  that would otherwise be ours to follow.
- It is a client of the local API and owns no session. Its tools are
  `list_profiles`, `list_sessions`, `session_status`, `start_session`,
  `stop_session` and `discover_hosts`, with `readOnlyHint` on the reads and
  without it on start and stop (G-C8).

### 6. Unchanged

- `connect` keeps its foreground contract (ADR-016). The agent adds a mode
  and replaces nothing.
- `host serve` and ADR-017 Amendment A are unchanged. A host is still
  provisioned by the consumer, and ts-bridge still installs no system service.
  This ADR amends only the client side: registering a per-user login start is
  allowed.

## Rationale

The intersection of the references gives one session owner with thin clients
over an identity-checked local socket. That pattern is what lets a tray, a
terminal and an AI agent share the same sessions. A per-user agent keeps this
pattern while avoiding what made the references privileged: they create
system network interfaces, and tsnet does not. Running sessions as children
reuses the tested foreground path and the ADR-016 lock instead of building a
second session implementation. Wails gives one GUI codebase for three OSes,
with tray and autostart built in, and a real window for the first-run form.

## Rejected Alternatives and Reopen Triggers

| Alternative | Reason | Reopen trigger |
| --- | --- | --- |
| A: the tray owns sessions | The MCP server and the CLI could not reach a session without the tray | The MCP consumer is dropped |
| B': agent as a system service | Needs a privileged installer and crosses per-user credentials; tsnet needs no privilege | Agent crashes on Windows are seen in daily use despite the task's restart-on-failure, or a session must run with no user logged in |
| C: native app per OS | Three codebases for one maintainer | A second maintainer per OS, or App Store distribution becomes a requirement |
| D: Fyne | cgo plus OpenGL everywhere, non-native look | none |
| E: menu-only tray | Pushes first run back to the CLI (G-C3) | The owner accepts a CLI-only first run |
| Hand-rolled MCP JSON-RPC | Every protocol revision would be ours to track, to save four modules | The SDK's graph grows past what the core can justify, or it stops tracking the spec |
| A browser extension | Needs a native-messaging host and adds nothing over `ts-bridge browser` | A user needs something `ts-bridge browser` cannot do |
| A TCP local API (even on loopback) | Carries no caller identity (G-C7) | none |

## Consequences

### Positive

- One session model serves the tray, the terminal and AI agents.
- The core stays pure Go and gains four small modules, for MCP only.
- Sessions survive a closed window and an agent restart, and the user's
  explicit stops are respected.

### Negative

- New public surface to maintain: the agent, the `/v1/` API, `agent enable`,
  `mcp`, and a second release artifact family for the tray.
- The tray binary needs cgo and a native build per OS (a macOS runner). macOS
  signing and notarization become release work.
- A Linux desktop without SNI needs the AppIndicator extension for the tray;
  without it the app works from its window only.
- The agent's recovery depends on each OS's user-level restart; Windows has
  the weakest one (see the B' reopen trigger).

### Neutral

- `connect --profile` and the agent exclude each other per profile, so a
  profile runs in one place at a time.

## Follow-ups

- #782: the agent, the local API and the session CLI (`session list/start/stop`), with a spec.
- #783: `agent enable` / `agent disable`: per-user autostart on the three OSes.
- #784: `ts-bridge mcp`.
- #785: `ts-bridge-tray` (Wails v3), including the first-run form and GNOME without a tray.

## References

- #451; ADR-015; ADR-016 (option B reopened); ADR-017 Amendment A
- `tailscale/tailscale` `safesocket/`, `ipn/ipnauth/`, `client/systray/`
- `netbirdio/netbird` `client/internal/ipcauth/`, `client/ui/`
- `WireGuard/wireguard-windows` `docs/attacksurface.md`
- `fyne-io/systray`; `wailsapp/wails` v3; `modelcontextprotocol/go-sdk`
