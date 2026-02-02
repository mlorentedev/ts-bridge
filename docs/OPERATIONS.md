# Operations Guide

This guide covers deploying and operating ts-bridge in production.

## Deployment

### Linux (systemd)

1. **Install binary:**
   ```bash
   sudo cp ts-bridge /usr/local/bin/
   sudo chmod +x /usr/local/bin/ts-bridge
   ```

2. **Create service user:**
   ```bash
   sudo useradd -r -s /sbin/nologin ts-bridge
   ```

3. **Create directories:**
   ```bash
   sudo mkdir -p /var/lib/ts-bridge /etc/ts-bridge
   sudo chown ts-bridge:ts-bridge /var/lib/ts-bridge
   sudo chmod 700 /var/lib/ts-bridge
   ```

4. **Create environment file:**
   ```bash
   sudo tee /etc/ts-bridge/ts-bridge.env << EOF
   TS_AUTHKEY=tskey-auth-...
   TS_TARGET=your-host:3389
   TS_LOCAL_ADDR=127.0.0.1:33389
   TS_HEALTH_ADDR=127.0.0.1:8080
   TS_STATE_DIR=/var/lib/ts-bridge
   EOF
   sudo chmod 600 /etc/ts-bridge/ts-bridge.env
   ```

5. **Install service:**
   ```bash
   sudo cp scripts/host/ts-bridge.service /etc/systemd/system/
   sudo systemctl daemon-reload
   sudo systemctl enable ts-bridge
   sudo systemctl start ts-bridge
   ```

6. **Verify:**
   ```bash
   sudo systemctl status ts-bridge
   journalctl -u ts-bridge -f
   ```

### Windows

See `scripts/host/install-service.ps1` for Windows service installation.

## Monitoring

### Health Endpoint

Enable with `TS_HEALTH_ADDR=127.0.0.1:8080`:

```bash
# Health check
curl http://127.0.0.1:8080/health
# Response: {"status":"ok"}

# Metrics
curl http://127.0.0.1:8080/metrics
# Response:
# {
#   "active_connections": 5,
#   "total_connections": 1234,
#   "total_bytes_tx": 1048576,
#   "total_bytes_rx": 2097152,
#   "total_errors": 3,
#   "rejected_connections": 0
# }
```

### Key Metrics

| Metric | Description | Alert Threshold |
|--------|-------------|-----------------|
| `active_connections` | Current open connections | > 80% of max |
| `total_errors` | Cumulative error count | Increasing rapidly |
| `rejected_connections` | Connections rejected due to limit | > 0 sustained |

### Prometheus Integration

Example scrape config:
```yaml
scrape_configs:
  - job_name: 'ts-bridge'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: /metrics
```

### Log Monitoring

Enable JSON logs for parsing:
```bash
export TS_LOG_FORMAT=json
```

Example log entries:
```json
{"time":"2024-01-15T10:30:00Z","level":"INFO","msg":"connection opened","client":"127.0.0.1:54321"}
{"time":"2024-01-15T10:35:00Z","level":"INFO","msg":"connection closed","client":"127.0.0.1:54321","duration":"5m0s","bytes_tx":1048576,"bytes_rx":2097152}
```

## Troubleshooting

### Connection Issues

**Symptom:** Connections timeout on dial

**Check:**
1. Tailscale connectivity: Is the target reachable via Tailscale?
2. Target service: Is the target port open?
3. Firewall: Check both ends for firewall rules

```bash
# Verbose mode shows tsnet internals
./ts-bridge -v
```

**Symptom:** Connections rejected

**Check:**
1. Connection limit: Check `rejected_connections` metric
2. Increase limit: `TS_MAX_CONNECTIONS=2000`

### Performance Issues

**Symptom:** High latency

**Check:**
1. DERP relay: Direct connection may have failed
2. Network path: Check for congestion
3. Target load: Is the target overloaded?

**Symptom:** High memory usage

**Check:**
1. Active connections: Many concurrent connections use memory
2. Buffer size: Each connection uses 32KB buffer per direction

### State Issues

**Symptom:** "state directory has loose permissions" warning

**Fix:**
```bash
chmod 700 /var/lib/ts-bridge
```

**Symptom:** Authentication failures after restart

**Check:**
1. Auth key validity: Ephemeral keys expire (90 days default)
2. State directory: Ensure writable

## Performance Tuning

### Connection Limits

Default: 1000 concurrent connections

```bash
# Increase for high-traffic deployments
export TS_MAX_CONNECTIONS=5000
```

### File Descriptor Limits

For high connection counts:
```bash
# Check current limit
ulimit -n

# Increase in systemd
# Add to [Service] section:
LimitNOFILE=65535
```

### Timeout Tuning

```bash
# Increase for slow networks
export TS_TIMEOUT=60s
```

## Security Hardening

### Network

- Bind to localhost only (default): `TS_LOCAL_ADDR=127.0.0.1:33389`
- Use firewall rules to restrict access
- Enable health endpoint on internal interface only

### Auth Keys

- Use short-lived keys (30 days recommended)
- Rotate keys before expiration
- Store keys in secrets manager when possible

### State Directory

- Permissions must be 0700
- Located on encrypted filesystem if possible
- Regular backups not needed (ephemeral nodes)

### Logging

- Consider log redaction for sensitive environments
- JSON format for SIEM integration
- Rotate logs with logrotate

## Backup & Recovery

### What to Backup

- Environment file (`/etc/ts-bridge/ts-bridge.env`)
- Service configuration

### What NOT to Backup

- State directory (ephemeral, recreated on start)

### Disaster Recovery

1. Install binary on new host
2. Restore environment file
3. Start service (new Tailscale node created)

## Upgrades

### Rolling Upgrade

1. Download new binary
2. Stop service: `systemctl stop ts-bridge`
3. Replace binary: `cp ts-bridge-new /usr/local/bin/ts-bridge`
4. Start service: `systemctl start ts-bridge`

### Version Verification

```bash
ts-bridge -version
# Output: ts-bridge v1.2.0 (abc1234)
```
