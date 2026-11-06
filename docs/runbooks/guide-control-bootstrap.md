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
- An abrupt exit is handled per OS: a panic, `kill -9`, or Task Manager's End task.
  - Linux: the kernel sends `SIGKILL` to `ssh` when ts-bridge dies.
  - Windows: `ssh` runs in a kill-on-close Job Object, and the job dies with ts-bridge.
    `ssh` joins the job just after it starts. If ts-bridge is killed inside that window,
    or `ssh` spawns a process before joining (a `ProxyCommand`), that process is not covered.
    If ts-bridge itself runs inside a job that forbids nesting, containment is skipped.
  - macOS: there is no equivalent. A leftover `ssh` keeps the SOCKS port bound until you
    stop it (`lsof -iTCP:<port> -sTCP:LISTEN` names it).

  Containment is best effort: if the OS refuses it, the bootstrap starts as before.

The local SOCKS and HTTP listeners are unauthenticated. Loopback binding prevents
remote access, not access by other local users/processes. Use a trusted client host;
never expose these listeners through port forwarding.

## Restricting the SSH account with PermitOpen

The adapter forwards any port on the control hostname, not only the port in the
control URL. An account limited with `PermitOpen <control-host>:443` therefore
refuses every other port the SDK dials, and ssh prints one
`channel N: open failed: administratively prohibited` line on stderr per refusal.
The bridge can still reach READY over the permitted port.

ssh does not name the refused destination, but ts-bridge does. Every dial through
the bootstrap SOCKS listener is logged before it is made as
`SSH bootstrap SOCKS dial target=<host:port>`, at debug level. The console shows it
with `--verbose`; the JSON log file under the platform log directory always keeps
it, so a start without `--verbose` can be inspected afterwards. Match the refused
channel to the `target` logged just before it and add that `host:port` to
`PermitOpen`, or accept the refusal if the start still reaches READY.

Which extra destination the SDK dials (#572) is not confirmed. A live start with
`--verbose` against a restricted account is what names it; this runbook declares
the destination once that start has been recorded.

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
