---
id: "ts-bridge-gui-control-plane-audit"
type: design
status: active
created: "2026-11-14"
tags: [architecture, gui, control-plane, mcp, audit]
---
# GUI + local control plane: reference audit (#451)

Audited 2026-11-14 from primary sources (repos and vendor docs). Evidence for [ADR-018](../adr/adr-018-local-control-plane.md). Local library first: `00_meta/patterns/` has no GUI, tray or daemon pattern. `pattern-mcp-tool-design` applies to the MCP consumer: at most ~12 tools, safety annotations.

## Matrix

| Ref | Sessions owned by | IPC + local auth | GUI toolkit | cgo / separate binary | Linux without tray | Autostart |
|---|---|---|---|---|---|---|
| Tailscale core | `tailscaled` daemon | LocalAPI (HTTP) over a Unix socket with peercred; Windows named pipe with SDDL and a per-connection token check; macOS GUI uses localhost TCP plus a `sameuserproof` token | n/a | pure Go | n/a | n/a |
| Tailscale `systray` (Linux) | daemon (client of the LocalAPI) | as above; the user is made `--operator` | Go, `fyne.io/systray` v1.12.2 + godbus | inside the CLI binary, build tag `cgo \|\| !darwin` | needs SNI; GNOME needs the AppIndicator extension | `tailscale configure systray --enable-startup=freedesktop\|systemd` |
| Tailscale macOS / Windows | daemon (Network Extension / service) | token / named pipe | Swift / Go + win32 (closed source) | separate app | n/a | UNVERIFIED |
| Trayscale | `tailscaled` | LocalAPI | GTK4/libadwaita via puregotk (no cgo) | separate app | window-first | UNVERIFIED |
| NetBird | daemon | gRPC over a Unix socket / npipe with peer creds; TCP deprecated because it "carries no caller identity" | Wails v3 (own fork) + web UI; replaced Fyne | separate `client/ui`; cgo on Linux (gtk4, an XEmbed host) | ships its own XEmbed tray host | Wails autostart, on by default |
| WireGuard Windows | manager service spawns tunnel services | inherited unnamed pipes into a UI process the manager launches | Go + lxn/walk | one exe, several roles | n/a | the manager launches the UI |
| Syncthing + trays | daemon | REST on 127.0.0.1 + API key | web; Qt/.NET trays | separate apps | Qt SNI/XEmbed | trays supervise the daemon |
| Ollama desktop | app spawns the server | HTTP on 127.0.0.1, no auth | Go + webview (cgo), hand-rolled win32 tray | macOS/Windows only | no Linux app | installer |

## Divergence log

Template candidates (shared by two or more):
- A daemon owns the sessions, and unprivileged clients reach it over a local socket. Seen in Tailscale, NetBird, WireGuard and Syncthing. Only Ollama has the app own them, and it has a single consumer.
- Unix socket with kernel peer credentials, and a Windows named pipe with an SDDL plus a per-connection identity check (Tailscale, NetBird).
- Bare TCP is rejected for lacking caller identity (NetBird). Tailscale adds a token where it has to use TCP.
- The GUI is a separate binary or process (all of them).
- The Linux tray is SNI over D-Bus, and GNOME needs the AppIndicator extension (Tailscale, fyne/systray).

Unique, so not template candidates: WireGuard's inherited pipes; NetBird's XEmbed host; Trayscale's purego GTK; Ollama's win32 tray; Tailscale's `sameuserproof`.

## Measured locally (2026-11-14)

- `fyne.io/systray` v1.12.2 builds with `CGO_ENABLED=0` for linux and windows, and FAILS for darwin. Only macOS needs cgo.
- `modelcontextprotocol/go-sdk` v1.8.0 has 14 modules in its graph. Against ts-bridge's go.mod, 4 are new besides the SDK itself: `google/jsonschema-go`, `segmentio/encoding`, `segmentio/asm`, `yosida95/uritemplate`. The x/oauth2, x/sync, x/time and x/sys modules are already required.
- Wails v3.1.0 has been stable since 2026-11-01. On Linux it uses cgo with gtk4 and webkitgtk-6.0 (a `purego` tag exists but is unverified here). Windows needs no cgo (WebView2). macOS uses Obj-C.
- Fyne, the full toolkit, needs cgo plus OpenGL on every desktop OS.

## Per-decision evidence

| ADR-018 decision | References that back it |
|---|---|
| One process owns sessions, thin clients over a local socket | Tailscale, NetBird, WireGuard (Windows), Syncthing (Ollama is the counter-example, with a single consumer) |
| Unix socket + peer-credential uid check | Tailscale (`safesocket`, `ipnauth`), NetBird (`ipcauth`) |
| Windows named pipe, SDDL + per-connection identity check | Tailscale (`pipe_windows.go`), NetBird (`creds_windows.go`) |
| No TCP for the local API | NetBird (TCP deprecated for no caller identity), Tailscale (adds a token where it has to use TCP) |
| GUI as a separate binary | all five |
| Wails for the GUI | NetBird (moved from Fyne to Wails v3); Ollama (webview GUI). Measured: macOS tray needs cgo |
| Tray on Linux via SNI; GNOME needs AppIndicator | Tailscale `systray` (KB 1597), `fyne.io/systray` README |
| A per-user agent, not a system service | divergence: the references run privileged because they create system interfaces; tsnet does not (ADR-018 Rationale) |
| Official MCP Go SDK | measured graph against this repo's go.mod (above) |
