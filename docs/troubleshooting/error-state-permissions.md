---
id: "ts-bridge-error-state-permissions"
type: troubleshooting
status: active
tags: [state, permissions, filesystem]
created: "2026-02-23"
owner: manu
---

# Error: State Directory Permissions

## Symptom
```
state directory has loose permissions
```
Or permission errors accessing the state directory.

## Cause
The state directory (`TS_STATE_DIR`, default `./ts-state`) must have `0700` permissions. tsnet stores Tailscale node state here and warns if other users can read it.

## Fix

```bash
# Reset state directory
rm -rf ./ts-state
./ts-bridge  # Will recreate with correct permissions

# Or fix permissions manually
chmod 700 ./ts-state
```

## Authentication Failures After Restart

### Symptom
Tailscale authentication fails after service restart.

### Causes
1. Auth key expired (ephemeral keys: 90 days default)
2. State directory not writable

### Fix
1. Check auth key validity and regenerate if needed
2. Ensure state directory is writable by the service user:
   ```bash
   sudo chown ts-bridge:ts-bridge /var/lib/ts-bridge
   sudo chmod 700 /var/lib/ts-bridge
   ```

## Verbose Mode for Debugging

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

## Log Analysis

### Text format (default)
```
time=2024-01-15T10:30:00Z level=INFO msg="connection opened" client=127.0.0.1:54321
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
