# Troubleshooting Guide

## Quick Reference

| Issue | Solution |
|-------|----------|
| "dial failed: context deadline exceeded" | Check host Tailscale IP and firewall |
| "tailscale init failed" | Invalid auth key or network blocking |
| "Connection reset by peer" | Host service down or network issue |
| Certificate warnings | Use `/cert:ignore` with xfreerdp |
| Slow connection | DERP relay active; normal for restricted networks |

## Common Issues

### Connection Timeout

**Symptom:** `dial failed: context deadline exceeded`

**Causes:**
1. Wrong target IP/port
2. Host not running Tailscale
3. Firewall blocking on host side
4. Target service not running

**Debug steps:**
```bash
# Enable verbose mode
./ts-bridge -v

# Check if target is reachable from another Tailscale node
tailscale ping 100.x.x.x
```

### Authentication Failed

**Symptom:** `tailscale init failed`

**Causes:**
1. Invalid or expired auth key
2. Auth key doesn't start with `tskey-`
3. Network blocking Tailscale control plane

**Fix:**
1. Generate new key at https://login.tailscale.com/admin/settings/keys
2. Ensure key format: `tskey-auth-...`
3. Check if `login.tailscale.com` is accessible

### Connection Rejected

**Symptom:** `connection rejected: limit reached`

**Cause:** Too many concurrent connections

**Fix:**
```bash
# Increase limit
export TS_MAX_CONNECTIONS=2000
```

### Slow Performance

**Symptom:** High latency, sluggish RDP

**Causes:**
1. Using DERP relay (no direct connection)
2. Network congestion
3. Target machine overloaded

**Check:**
```bash
# Enable verbose to see connection path
./ts-bridge -v

# Look for "via DERP" in Tailscale logs
```

**Mitigations:**
- DERP adds 50-200ms - acceptable for RDP
- Try different network if possible
- Check target machine resources

### State Directory Errors

**Symptom:** Permission errors or `state directory has loose permissions` warning

**Fix:**
```bash
# Reset state directory
rm -rf ./ts-state
./ts-bridge  # Will recreate with correct permissions

# Or fix permissions manually
chmod 700 ./ts-state
```

### RDP Certificate Warnings

**Symptom:** Certificate errors when connecting

**Fix (Linux xfreerdp):**
```bash
xfreerdp /v:127.0.0.1:33389 /cert:ignore ...
```

**Fix (Windows):** Accept the certificate warning in mstsc

## Verbose Mode

Enable detailed logging for troubleshooting:

```bash
# Via flag
./ts-bridge -v

# Via environment
export TS_VERBOSE=true
./ts-bridge

# JSON format for parsing
export TS_LOG_FORMAT=json
./ts-bridge -v
```

Verbose mode shows:
- tsnet internal operations
- Connection establishment details
- Bytes transferred per connection
- Connection duration

## Health Endpoint

Enable monitoring endpoint:

```bash
export TS_HEALTH_ADDR=127.0.0.1:8080
./ts-bridge
```

Then check:
```bash
# Health status
curl http://127.0.0.1:8080/health

# Metrics
curl http://127.0.0.1:8080/metrics
```

Metrics include:
- `active_connections` - Current open connections
- `total_connections` - All-time connection count
- `total_bytes_tx/rx` - Bandwidth usage
- `total_errors` - Error count
- `rejected_connections` - Connections rejected due to limit

## Log Analysis

### Text format (default)
```
time=2024-01-15T10:30:00Z level=INFO msg="connection opened" client=127.0.0.1:54321
time=2024-01-15T10:35:00Z level=INFO msg="connection closed" client=127.0.0.1:54321 duration=5m0s bytes_tx=1048576 bytes_rx=2097152
```

### JSON format
```bash
export TS_LOG_FORMAT=json
./ts-bridge 2>&1 | jq .
```

### Filter errors
```bash
./ts-bridge 2>&1 | grep -E "level=(ERROR|WARN)"
```

## Platform-Specific Issues

### Linux

**SELinux blocking:**
```bash
# Check audit log
ausearch -m avc -ts recent

# Temporarily disable (for testing)
setenforce 0
```

**Firewall:**
```bash
# Allow local port
sudo ufw allow from 127.0.0.1 to any port 33389
```

### Windows

**Execution policy:**
```powershell
PowerShell -ExecutionPolicy Bypass -File .\scripts\client\run.ps1
```

**Antivirus:** Some AV may flag the binary. Add exception if needed.

### macOS

**Gatekeeper:**
```bash
# If blocked, allow in System Preferences > Security
# Or remove quarantine
xattr -d com.apple.quarantine ./ts-bridge
```

## Getting Help

1. Enable verbose mode and capture logs
2. Check metrics endpoint if enabled
3. Open issue at https://github.com/mlorentedev/ts-bridge/issues with:
   - OS and version
   - ts-bridge version (`./ts-bridge -version`)
   - Relevant log output (redact auth keys!)
   - Steps to reproduce
