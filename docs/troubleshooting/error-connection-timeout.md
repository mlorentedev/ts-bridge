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

## Causes
1. Wrong target IP/port in `TS_TARGET`
2. Host machine not running Tailscale
3. Firewall blocking on host side
4. Target service (e.g., RDP) not running on host

## Debug Steps

```bash
# Enable verbose mode
./ts-bridge -v

# Check if target is reachable from another Tailscale node
tailscale ping 100.x.x.x
```

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
| `dial failed: context deadline exceeded` | Check host Tailscale IP and firewall |
| `Connection reset by peer` | Host service down or network issue |
| Slow connection | DERP relay active; normal for restricted networks |
