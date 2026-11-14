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

Accepted by the operator on 2026-11-14. Tracked by #451. Revised the same day after two
independent adversarial reviews (`nan/deepseek-v4-flash`: PASS-WITH-GAPS;
`agy/gemini-3.8-flash-high`: FAIL on the same blockers). Both kept option B. The reviews and
the disposition of each finding are in
[`docs/design/adr-018-adversarial-reviews.md`](../design/adr-018-adversarial-reviews.md).

## Context

ts-bridge can only be operated from the CLI, and `connect` runs in the foreground
(ADR-016). The operator wants to drive it from a GUI on macOS, Linux and Windows alike,
and from AI agents through an MCP server. Nothing can list profiles or start, stop or
list sessions programmatically. The only machine-readable surfaces are `status --json`
and the health and metrics endpoints of a process that is already running.

ADR-016 rejected a background supervisor (its option B) with this reopen trigger: "a
concrete consumer requires a successful second invocation to reuse an unattended
session; specify ownership and recovery first." A tray app and an MCP server are that
consumer, so this ADR reopens option B and specifies both.

### Constraints

| ID | Constraint | Origin |
| --- | --- | --- |
| G-C1 | The app lives in the tray or menu bar on all three OSes. Sessions outlive the window, and it can start at login | Owner, #451 |
| G-C2 | Minimalist: a profile list, connect/disconnect, status, and no other settings surface | Owner, #451 |
| G-C3 | First run takes one input (a `tsb://` descriptor) plus the credential, with defaults for everything else | Owner, #451 |
| G-C4 | The core binary stays pure Go and cross-compiles without cgo, with no new dependency without strong justification. It binds the core, not the separate tray binary | AGENTS.md, ADR-002 rationale |
| G-C5 | Per-profile, OS-backed ownership: no duplicates, no takeover, no PID-only locks | ADR-016 C5 |
| G-C6 | ts-bridge installs no system service | ADR-017 Amendment A |
| G-C7 | The local control surface knows its caller, so no bare TCP | Audit: NetBird deprecated TCP for that reason |
| G-C8 | MCP: at most ~12 tools, with safety annotations | `pattern-mcp-tool-design` |
| G-C9 | Credentials persist only in the ADR-015 store; no GUI or MCP component persists or renders a secret | ADR-015 |

### Reference audit

The full matrix, with its divergence log, is
[`docs/design/gui-control-plane-reference-audit.md`](../design/gui-control-plane-reference-audit.md).
Each reference was read from primary sources on 2026-11-14.

| Reference | Who owns sessions | Local IPC and auth | GUI |
| --- | --- | --- | --- |
| Tailscale | `tailscaled` daemon | LocalAPI (HTTP) over a Unix socket with peer credentials on Linux; a Windows named pipe with an SDDL and a per-connection token check. The sandboxed macOS app cannot reach a socket in the user's directories, so it uses loopback TCP plus a `sameuserproof` token | Swift (macOS), Go + win32 (Windows), Go `fyne.io/systray` inside the CLI binary (Linux) |
| NetBird | daemon | gRPC over a Unix socket or named pipe with peer credentials; TCP deprecated as carrying "no caller identity" | Wails v3 (its own fork) as a separate binary, plus its own XEmbed tray host on Linux; it replaced Fyne |
| WireGuard (Windows) | manager service | inherited pipes into a UI process the manager launches | Go + walk |
| Syncthing | daemon | REST on 127.0.0.1 with an API key | web UI; separate tray apps |
| Ollama | the app spawns the server | HTTP on 127.0.0.1, no auth | Go + webview; no Linux app |

Two facts are shared by four of the five references: one process owns the sessions,
and clients reach it over a local socket that identifies the caller. ts-bridge-tray is
not sandboxed and is not distributed through the App Store, so the macOS socket
restriction that pushed Tailscale to TCP does not apply.

Measured locally on 2026-11-14:

- `fyne.io/systray` v1.12.2 builds with `CGO_ENABLED=0` for Linux and Windows and needs
  cgo for macOS. Wails v3 uses Obj-C on macOS and gtk4 plus webkitgtk-6.0 on Linux. A
  tray needs cgo on macOS whichever library draws it, so cgo does not separate the
  candidates. It decides only where the tray lives: outside the core binary.
- `modelcontextprotocol/go-sdk` v1.8.0 adds four modules that ts-bridge does not
  already require (`google/jsonschema-go`, `segmentio/encoding`, `segmentio/asm`,
  `yosida95/uritemplate`). The rest of its graph (`x/oauth2`, `x/sync`, `x/time`,
  `x/sys`) is already in `go.mod`.
- Not measured: which tray protocol Wails v3.1 registers on Linux (SNI over D-Bus or
  XEmbed), and whether its icon appears on GNOME with AppIndicator and on KDE under
  Wayland. NetBird ships its own XEmbed host next to its Wails fork, which is evidence
  that it may not. A spike (§4) settles it before the tray is built.

## Options Considered

| Option | Door | G-C1 | G-C3 | G-C4 | G-C6 | G-C7 | Verdict |
| --- | --- | --- | --- | --- | --- | --- | --- |
| A: the tray app owns sessions (Ollama) | 2 | yes | yes | yes | yes | n/a | rejected: the MCP server would need the tray running |
| B: a per-user agent with a local API; tray, CLI and MCP as clients | 1 | yes | yes | yes | yes | yes | **chosen** |
| B': B as a system service (root/SYSTEM) | 1 | yes | yes | yes | no | yes | rejected, see below |
| C: a native app per OS (SwiftUI / WinUI / GTK) | 1 | yes | yes | yes | yes | yes | rejected: three codebases, one maintainer |
| D: Fyne (full toolkit) | 2 | yes | yes | separate binary | yes | yes | rejected: OpenGL on every OS, a non-native look; NetBird left it |
| E: a menu-only tray (`fyne.io/systray`), first run in a terminal (`ts-bridge import` + `auth set`) | 2 | yes | partly | separate binary | yes | yes | rejected: first run leaves the app for a terminal |
| E': `fyne.io/systray` + a setup page served on loopback and opened in the browser (Syncthing) | 2 | yes | yes | separate binary | yes | no | rejected: the page needs a loopback HTTP listener with a token, which is the TCP surface G-C7 excludes |
| E'': the tray inside the core binary behind a build tag (Tailscale on Linux) | 2 | yes | no | no | yes | yes | rejected: the macOS tray needs cgo in the core, and the core gains a D-Bus dependency |
| F: Wails v3 for the tray binary | 2 | yes | yes | separate binary | yes | yes | **chosen for the GUI**, subject to the Linux tray spike (§4) |

## Decision

### 1. `ts-bridge agent`: per-user, owns sessions

**Identity and single instance.**

- The agent is a process of the logged-in user. It is not root, SYSTEM or a system
  service. tsnet runs in userspace and changes no system network state, so no privilege
  is needed. Credentials are per user (ADR-015), so a per-user owner reads them without
  crossing users.
- Its base directory, which holds the agent lock, the recovery state and the socket
  directory where §2 places it there, is resolved from the OS user identity, never from
  the environment:
  - Unix: the home directory from the user database (`getpwuid`), not `$HOME`, and never
    `$XDG_*` or `$TMPDIR`.
  - Windows: the LocalAppData known folder of the process token.
  A desktop session and a login supervisor see different environments, and a base that
  followed `$XDG_STATE_HOME` would give them two locks and two agents (G-C5).
- One agent per user is guarded by an OS-backed lock in that base: the mechanism
  ADR-016 uses per profile, never a PID file.
- `agent enable` records the resolved base in the registration. An agent whose resolved
  base differs from the recorded one refuses to start and names both paths.

**Sessions are child processes under a declared contract.**

- Each session is a child running `connect --profile <name>`. This is a new, documented
  invocation of `connect`, not an unchanged reuse of it. Without the flag below,
  `connect` keeps its foreground contract byte for byte.
- `--lifeline <fd|handle>` is a hidden flag, listed in the CLI reference as reserved
  for the agent:
  - Unix: an inherited pipe through `ExtraFiles`.
  - Windows: an inheritable anonymous pipe passed through
    `SysProcAttr.AdditionalInheritedHandles`, whose handle value goes in the flag (Go's
    `ExtraFiles` is not supported on Windows).
- When the lifeline is set, the child's stdin is the null device. The child reads the
  lifeline in its own goroutine:
  - A `stop` line cancels its context and runs the normal drain (`--drain-timeout`).
    This is the graceful stop on every OS, including Windows, where a console-less
    child receives no signal.
  - EOF means the agent died, which the kernel guarantees. On Unix the child runs the
    same drain and exits. On Windows the agent's death also closes the Job Object
    handle, and kill-on-close ends the child at once, with no drain. That is accepted:
    an agent that died cannot report the drain anyway, and tsnet's state survives an
    abrupt stop.
- Windows also creates each child suspended, assigns it to a Job Object with
  kill-on-close, then resumes it, so nothing the child spawns starts outside the job. A
  parent job that forbids nesting makes the assignment fail. That failure is logged, and
  the lifeline remains the guarantee. `Pdeathsig` on Linux may only be added on top of
  the lifeline, because it fires on parent-thread exit and races with an early parent
  death.
- Spawn environment:
  - The working directory is the agent's state directory.
  - The environment is an allow-list, not `os.Environ`.
  - `.env` loading is off. The agent passes the base resolved above, so the child's
    profile store, per-profile lock and logs are the ones a terminal `connect` uses on
    the same base.
  - `SSH_AUTH_SOCK` is passed only when the agent itself has it.
- Session ABI:
  - Children run with `--quiet`. The agent reads their stdout for the `READY` and
    `ERROR` tokens that `cmd/cli/signal.go` already declares as a machine contract.
  - Session state comes from those tokens and the exit code. The bound address comes
    from `READY`, never from the profile.
- Address and identity:
  - Before spawning, the agent refuses a second session on a local address another of
    its sessions already holds. A bind failure is an `ERROR`, with no retry on another
    port.
  - When a profile names no hostname, the agent passes one derived from the auto seed
    without the process id. A restart therefore returns as the same tailnet machine
    instead of registering a new one.
- Logs: each session writes its own log file, named after the profile. A process that
  exits never compresses or removes a file another process writes (#788).

**Terminal and agent exclusion.**

- The child holds the ADR-016 per-profile lock, so `connect --profile X` from a terminal
  and a session of X in the agent exclude each other.
- The holder writes its kind (terminal or agent), pid and start time into the lock
  file. The holder's metadata is informational: the OS lock remains the authority. The
  refusal on the other side names the holder, replacing today's "close this profile's
  existing ts-bridge terminal".
- When the agent starts a session, it waits up to the drain timeout plus a margin for a
  lock still held by its previous generation's child before reporting the profile busy.

**Non-interactive SSH bootstrap.**

- Sessions started by the agent have no terminal. A profile that bootstraps over SSH
  (ADR-005) runs it with `BatchMode=yes`. The key must be in `ssh-agent` or unencrypted,
  and the host key must already be in `known_hosts`.
- A login supervisor usually starts the agent without `SSH_AUTH_SOCK`, so SSH bootstrap
  in the agent is a degraded mode, not an equal one. When it fails, the error names the
  cause and says to run `connect --profile <name>` in a terminal.
- The agent never captures or relays a passphrase or a host-key prompt.

**Session state machine and recovery.**

- Each profile the agent manages is in one of these states: `stopped`, `starting`,
  `ready`, `backoff`, `failed` or `stopping`.
- The desired state (running or stopped), the attempt count and the last `ERROR` are
  persisted in the agent's state directory.
- A child that exits while desired running enters `backoff` and is retried with
  exponential backoff, up to a budget of three attempts. This applies whether the child
  dies while the agent lives or the agent restarts.
- When the budget is exhausted, the profile becomes `failed` with its last error. It
  stays `failed` until an explicit start, which resets the count.
- A session the user stopped stays stopped. A `ready` session that has held for a
  minute resets the count.

### 2. The local API: identity-checked, no TCP

**Trust boundary.**

- The boundary is the user. Peer credentials prove that the caller runs as the same
  user, nothing stronger.
- Any process of that user can list profiles, import a descriptor with a credential, and
  start and stop sessions. That process can already read the ADR-015 store and run
  `ts-bridge` itself, so the API adds no capability that the user's processes lack.
- A token in a 0600 file would not narrow this, since the same user can read it, so
  none is added (see Rejected).
- Every mutation is written to an append-only audit line in the agent's log, with the
  operation, the peer's pid where the OS reports one, and the time.

**Transport.**

- Linux:
  - The path is derived from the uid alone, never from the environment, so an agent
    and a client started with different `XDG_RUNTIME_DIR` values still meet at one
    endpoint.
  - The socket goes in `/run/user/<uid>/ts-bridge/` when `/run/user/<uid>` exists and
    is owned by the user with mode 0700 (the directory systemd-logind creates and
    `XDG_RUNTIME_DIR` normally names).
  - Otherwise it goes in `/tmp/ts-bridge-<uid>/`, created 0700. The agent refuses to use
    that directory if it is a symlink, owned by another user, or more permissive than
    0700.
- macOS: the socket goes in `run/` under the base from §1 (`~/Library/Application
  Support/ts-bridge/run/`), with mode 0700.
- Socket rules on both Unix OSes:
  - The socket itself is 0600.
  - A path at or above the platform's `sun_path` limit (104 bytes on macOS, 108 on
    Linux) is refused at startup with the path named.
  - Every connection's peer credentials (`SO_PEERCRED`, `getpeereid`) must match the
    agent's uid.
- Windows:
  - The agent creates a named pipe whose name includes the user's SID, with an SDDL
    granting only that SID, `FILE_FLAG_FIRST_PIPE_INSTANCE` and
    `PIPE_REJECT_REMOTE_CLIENTS`. If another process already owns the name, the agent
    fails, says so, and never retries under another name.
  - Each connection checks the client token's SID.
- The client authenticates the server before sending anything:
  - Unix: it checks the directory's owner and mode and the server's peer credentials.
  - Windows: it opens the pipe with `SECURITY_SQOS_PRESENT | SECURITY_IDENTIFICATION`,
    so a squatting server cannot impersonate it, then resolves
    `GetNamedPipeServerProcessId` to a token SID that must equal its own.
  - A client never sends a credential to an endpoint that failed this check.
- A stale socket is removed only after the agent lock is acquired.

**Contract.**

- HTTP/JSON under `/v1/`. `GET /v1/meta` returns `{agent_version, api_min, api_max}`.
- Every error is `{code, message, detail}`, with a stable `code`.
- The tray and the CLI are released separately from the agent, so version skew is the
  normal case. A client outside the agent's range keeps the reads, refuses mutations,
  and says which side to upgrade.
- Resources:
  - Profiles: list, and import a `tsb://` descriptor with its credential.
  - Sessions: list, start, stop and status, with the state of §1.
- Import runs the same ADR-015 transition as `import` and `auth set`: store lock,
  re-read under the lock, refuse when the control URL moved, and restore the previous
  key on failure. `force` is an explicit boolean with ADR-015's exact meaning, never a
  default.
- Discovery is not in `/v1` until ADR-015 decides how an administrative credential is
  held. Discovery needs a control-plane API key, which is not the node key.

**Upgrade.**

- Children run the agent's own executable, resolved at agent start.
- An upgrade is `agent stop` (which drains the sessions and keeps the desired set),
  then a replacement of the binary, then a start. On Windows the stop is required,
  because a running image cannot be replaced.
- `status` marks a session whose child runs a different build from the agent.

### 3. Autostart: one supervisor, registered per user, never as a system service

`ts-bridge agent enable` and `agent disable` register and remove a per-user login start,
idempotently and without elevation. The registration names one supervisor, which is
the only thing that restarts the agent:

| OS | Registration | Restart on crash |
| --- | --- | --- |
| Linux | systemd `--user` unit | `Restart=on-failure`, `RestartSec=5`, with `StartLimitIntervalSec` and `StartLimitBurst` set in `[Unit]` |
| Linux without user systemd | XDG autostart entry | none: the agent starts at login only |
| macOS | LaunchAgent in `~/Library/LaunchAgents` | `KeepAlive` with `SuccessfulExit=false`, so a clean `agent stop` stays stopped; `ThrottleInterval` set |
| Windows | Scheduled Task at logon for the user | the task's restart-on-failure setting. It fires only when the task ends in failure, its attempts are bounded, and it needs the user logged on. Its exact behaviour against an externally killed process is unverified until #783 measures it |
| Windows, task creation refused by policy | HKCU `Run` key | none: the agent starts at login only |

- `agent stop` exits 0, so no supervisor restarts it.
- When a supervisor-backed registration exists (the systemd, launchd or Scheduled
  Task rows), the tray never executes the agent. It asks the supervisor
  instead (`systemctl --user start`, `launchctl kickstart`, `schtasks /run`), because a
  second starter that wins the lock would turn the supervisor's instance into a crash
  loop.
- The tray starts the agent directly only when no supervisor-backed registration
  exists, which includes the XDG autostart and HKCU `Run` fallbacks.

### 4. `ts-bridge-tray`: a separate binary, Wails v3

- It is a separate Go module (`gui/`, with its own `go.mod`), so Wails and cgo never
  reach the core's dependency graph (G-C4). It is built natively per OS and released as
  its own artifacts.
- It is a client of the local API only and owns no session or secret.
- Surface (G-C2): a tray or menu-bar icon whose state shows whether any session is up,
  and a menu listing the profiles with connect/disconnect. One small window holds the
  profile list, status, "start at login" (`agent enable`), and the first-run form with
  a single field for a `tsb://` descriptor (G-C3).
- The credential never enters the webview. The tray's Go side collects it through the
  OS's native masked prompt and sends it straight to the agent's `import` over the
  verified endpoint. Where no native masked prompt exists (a Linux desktop without one),
  the tray opens a terminal running `ts-bridge import <name> <descriptor>` (which
  needs no credential) followed by `ts-bridge auth set --profile <name>`, the same
  two-step sequence the CLI documents.
- On GNOME without the AppIndicator extension, the window opens from the application
  launcher and the app stays usable without the tray.
- Wails v3.1.x is pinned. It was tagged on 2026-11-01, and NetBird, the reference that
  uses it, runs its own fork.
- Before any tray code, a spike (#789) answers three questions and records the
  measurements:
  - which tray protocol Wails v3.1 registers on Linux;
  - whether the icon appears on GNOME with AppIndicator and on KDE under Wayland;
  - which webkitgtk the supported distributions ship (Ubuntu 22.04 has webkit2gtk-4.1,
    not webkitgtk-6.0).
- If the Linux tray fails the spike, the icon is drawn with `fyne.io/systray` and the
  Wails window is kept. The GUI stays thin enough that changing its toolkit is a Type 2
  change.

### 5. `ts-bridge mcp`: stdio, official SDK, a client of the agent

- It is a subcommand in the core binary that uses `modelcontextprotocol/go-sdk`. Four
  new modules are justified because the SDK tracks protocol revisions that would
  otherwise be ours to follow.
- It is a client of the local API and owns no session. Its tools are:
  - reads, with `readOnlyHint`: `list_profiles`, `list_sessions`, `session_status`;
  - mutations: `start_session`, and `stop_session` with `destructiveHint`.
- It exposes no import, no credential and no discovery.
- Annotations are hints that let a client auto-approve a call; they are not
  authorization. A prompt-injected caller can do what those five tools allow and
  nothing more, inside the same-user boundary of §2.

### 6. What else changes and what does not

- `connect` without `--lifeline` keeps its foreground contract (ADR-016).
- ADR-017 gains Amendment C: registering a per-user login start for the client agent is
  allowed. `host serve` and Amendment A are unchanged, and ts-bridge still installs no
  system service.

## Rationale

One session owner with thin clients over an identity-checked local socket is what lets
a tray, a terminal and an AI agent share the same sessions. A per-user agent keeps this
pattern while avoiding what made the references privileged: they create system network
interfaces, and tsnet does not. Running sessions as children of the foreground path
reuses the tested connection code and the ADR-016 lock, at the price of a declared child
contract (§1) rather than a second session implementation. Wails was chosen because the
first-run form and the status view need a real window, and Wails gives one window
codebase for three OSes. cgo does not decide it, since every macOS tray needs cgo. The
Linux tray is the unverified part, and the spike settles it.

## Rejected Alternatives and Reopen Triggers

| Alternative | Reason | Reopen trigger |
| --- | --- | --- |
| A: the tray owns sessions | The MCP server and the CLI could not reach a session without the tray | The MCP consumer is dropped |
| B': agent as a system service | Needs a privileged installer and crosses per-user credentials; tsnet needs no privilege | Agent crashes on Windows are seen in daily use despite the task's restart-on-failure, or a session must run with no user logged in |
| C: native app per OS | Three codebases for one maintainer | A second maintainer per OS, or App Store distribution becomes a requirement |
| D: Fyne | OpenGL everywhere, a non-native look | none |
| E: menu-only tray, first run in a terminal | First run leaves the app | The Linux tray spike fails and so does the native prompt |
| E': systray + loopback setup page | Needs a loopback listener with a token (G-C7) and a browser round trip | A webview is unavailable on a supported OS |
| E'': tray in the core binary | cgo and D-Bus in the core (G-C4) | none |
| A capability token for mutations | A same-user process can read it, so it adds nothing over peer credentials | The agent gains a caller outside the user's processes |
| Hand-rolled MCP JSON-RPC | Every protocol revision would be ours to track, to save four modules | The SDK's graph grows past what the core can justify, or it stops tracking the spec |
| A browser extension | Needs a native-messaging host and adds nothing over `ts-bridge browser` | A user needs something `ts-bridge browser` cannot do |
| A TCP local API (even on loopback) | Carries no caller identity (G-C7) | none |

## Consequences

### Positive

- One session model serves the tray, the terminal and AI agents.
- The core stays pure Go and gains four small modules, for MCP only.
- Sessions survive a closed window and an agent restart, and come back as the same
  tailnet machine. The user's explicit stops are respected, and a profile that keeps
  failing stops in `failed` instead of looping.

### Negative

- New public surface to maintain:
  - the agent, `/v1/` with its version range, `agent enable`, `mcp`, and `--lifeline`;
  - a second release artifact family for the tray.
- The tray binary needs cgo and a native build per OS (a macOS runner). macOS signing
  and notarization become release work (#785 splits it out).
- A Linux desktop without SNI needs the AppIndicator extension for the tray; without
  it the app works from its window only.
- On Linux without user systemd, and on Windows under the `Run` fallback, nothing
  restarts a crashed agent until the next login, unless the tray finds it dead (there
  is no supervisor-backed registration, so the tray may start it).
- A profile whose SSH bootstrap needs a passphrase, an unknown host key or an
  `ssh-agent` the supervisor did not inherit cannot run in the agent and stays a
  foreground `connect --profile`.
- The agent's recovery depends on each OS's user-level restart; Windows has the
  weakest one (see the B' reopen trigger).
- Same-user processes, including a prompt-injected MCP client, can start and stop
  sessions. That is the trust boundary, stated in §2 and §5.

### Neutral

- `connect --profile` and the agent exclude each other per profile, so a profile runs
  in one place at a time.

## Follow-ups

- #782: the agent, the local API, the `--lifeline` child contract, and the session CLI
  (`session list/start/stop`), with a spec.
- #783: `agent enable` / `agent disable` on the three OSes, measuring the Windows task
  restart.
- #784: `ts-bridge mcp`.
- #785: the `ts-bridge-tray` shell and window. Native credential prompt (#790) and
  packaging, signing and CI (#791) are split out.
- #789: the Linux tray spike (Wails v3.1 protocol, GNOME, KDE/Wayland, webkitgtk per
  distribution).
- #788: one process exiting deletes the shared daily log.

## References

- #451; ADR-015; ADR-016 (option B reopened); ADR-017 Amendments A and C
- `tailscale/tailscale` `safesocket/`, `ipn/ipnauth/`, `client/systray/`
- `netbirdio/netbird` `client/internal/ipcauth/`, `client/ui/`
- `WireGuard/wireguard-windows` `docs/attacksurface.md`
- `fyne-io/systray`; `wailsapp/wails` v3; `modelcontextprotocol/go-sdk`
- Go `os/exec` (`ExtraFiles` unsupported on Windows), `syscall.SysProcAttr.AdditionalInheritedHandles`
