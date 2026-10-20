---
id: "ts-bridge-control-bootstrap"
type: runbook
status: active
created: "2026-10-19"
tags: [operations, networking, ssh, headscale, windows]
---

# SSH Control Bootstrap

Use the opt-in `--bootstrap-ssh` setting when the configured control hostname
must be reached through an authorized OpenSSH endpoint. Keep the existing
profile/config and managed credential; neither a new application token nor
administrator privileges are required.

## Routing and ownership

- OpenSSH owns identity selection, strict host verification and interactive
  key unlocking. Do not capture passphrases or start an agent automatically.
- The configured `--bootstrap-socks-addr` remains a dedicated loopback listener
  owned by that foreground connection.
- ts-bridge also creates an ephemeral loopback HTTP proxy for SDK consumers.
  It translates HTTP CONNECT to SOCKS without terminating or weakening TLS.
  Ordinary HTTP control URLs continue to work.
- Only the configured control hostname uses bootstrap. This includes a custom
  DERP relay on the same hostname; other hosts keep their existing proxy behavior.
- Ctrl+C, parent-context cancellation and failed startup close the adapter's
  listener and active tunnels. The SSH child remains under the normal bootstrap
  lifecycle; do not kill or reuse an unrelated process.

The local SOCKS and HTTP listeners are unauthenticated. Loopback binding prevents
remote access, not access by other local users/processes. Use a trusted client host;
never expose these listeners through port forwarding.

## Verification

`READY` and `/health/ready` prove startup, not destination reachability. Check a
real service through its bridge listener, preserving the original origin:

```powershell
curl.exe --noproxy "*" --fail --silent --show-error --max-time 15 `
  --connect-to "service.example:443:127.0.0.1:34443" `
  "https://service.example/health"
```

Substitute the configured origin, local port and documented health endpoint;
require its documented success body, not just an accepted TCP connection.
Never use `--insecure`, change system DNS/proxies or rotate credentials to hide
a transport failure. A normally verified TLS handshake followed by an HTTP
timeout is a distinct failure from relay establishment.

## Relay failure diagnosis

In the affected version, `CONNECT ... unexpected EOF` or a malformed response
starting with SOCKS bytes can mean the SDK sent HTTP CONNECT directly to the
SSH SOCKS listener. The adapter fixes that protocol boundary in application
code (ADR-005 / #443); a proxy URL assertion alone did not detect it.

Compare mesh and SSH-only service probes before blaming DERP for an application
stall. If both routes establish verified TLS but receive no HTTP response,
record that separate boundary and investigate it without altering the network
or disrupting existing RDP sessions.
