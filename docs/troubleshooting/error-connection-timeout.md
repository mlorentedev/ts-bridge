---
id: "ts-bridge-error-connection-timeout"
type: troubleshooting
status: active
tags: [networking, timeout, dial]
created: "2026-02-23"
owner: manu
---

# Error: Connection Timeout

## Symptom
```
dial failed: context deadline exceeded
```

Windows Remote Desktop can surface the same failure as:

```text
Error code: 0x4
```

## Causes
1. Wrong target IP/port in `TS_TARGET`
2. Host machine not running Tailscale
3. Firewall blocking on host side
4. Target service (e.g., RDP) not running on host

## Debug Steps

### 1. Verify the local bridge first

```powershell
Get-NetTCPConnection -LocalAddress 127.0.0.1 -LocalPort 33389 -State Listen
# If health is enabled, use the configured TS_HEALTH_ADDR:
Invoke-WebRequest http://127.0.0.1:9090/health/ready
```

If no process is listening, start `ts-bridge` before opening the RDP client.
An RDP `0x4` error at this stage says nothing about the remote host.

### 2. Classify the remote dial failure

Run the bridge with verbose logging and inspect the log file:

```powershell
.\ts-bridge.exe -v connect --profile office --auth-key-file "$HOME\.ts-bridge\authkey"
```

| Evidence | Meaning |
|----------|---------|
| `connection refused` | The mesh peer is reachable, but the target port is closed or wrong. |
| `context deadline exceeded` | No connection reached the target service; continue with the WireGuard/peer checks below. |
| `Handshake did not complete` plus `DERP ... does not know about peer` | The target node is not currently connected to Tailscale, even if the machine has Internet access or remains listed in the admin console. |

On the target Windows host:

```powershell
$ts = "$env:ProgramFiles\Tailscale\tailscale.exe"
Get-Service Tailscale
Start-Service Tailscale
& $ts status
& $ts up --unattended
```

The target must appear connected in `tailscale status`; an old device entry in
the admin console is not proof that its current node is online.

### 3. Use a direct LAN path as a discriminator

When the client and target share a dock, Internet Connection Sharing network,
or other local LAN, test the service directly:

```powershell
Get-NetNeighbor -AddressFamily IPv4
Test-NetConnection <lan-ip> -Port <rdp-port>
mstsc /v:<lan-ip>:<rdp-port> /admin
```

If direct LAN RDP succeeds while the ts-bridge dial times out, RDP and the host
are healthy; the fault is isolated to the target's Tailscale service/session.

## Related: Slow Performance

### Symptom
High latency, sluggish RDP.

### Causes
1. Using DERP relay (no direct connection) — adds 50-200ms, acceptable for RDP
2. Network congestion
3. Target machine overloaded

### Check
```bash
# Enable verbose to see connection path
./ts-bridge -v
# Look for "via DERP" in Tailscale logs
```

## Related: Connection Rejected

### Symptom
```
connection rejected: limit reached
```

### Cause
Too many concurrent connections (default limit: 1000).

### Fix
```bash
export TS_MAX_CONNECTIONS=2000
```

## Quick Reference

| Issue | Solution |
|-------|----------|
| RDP error `0x4` | Confirm the local bridge is still listening before diagnosing the target |
| `dial failed: context deadline exceeded` | Check the target's live Tailscale service/session, not only its saved device entry |
| `DERP ... does not know about peer` | Start/reconnect Tailscale on the target host |
| `Connection reset by peer` | Host service down or network issue |
| Slow connection | DERP relay active; normal for restricted networks |
