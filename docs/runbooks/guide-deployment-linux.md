---
id: "ts-bridge-deploy-linux"
type: runbook
status: active
tags: [deployment, linux, systemd, production]
created: "2026-02-23"
owner: manu
---

# Deployment Guide: Linux (systemd)

## Prerequisites
- ts-bridge binary (from [Releases](https://github.com/mlorentedev/ts-bridge/releases))
- Tailscale auth key ([generate](https://login.tailscale.com/admin/settings/keys))

## Installation

### 1. Install Binary

```bash
sudo cp ts-bridge /usr/local/bin/
sudo chmod +x /usr/local/bin/ts-bridge
```

### 2. Create Service User

```bash
sudo useradd -r -s /sbin/nologin ts-bridge
```

### 3. Create Directories

```bash
sudo mkdir -p /var/lib/ts-bridge /etc/ts-bridge
sudo chown ts-bridge:ts-bridge /var/lib/ts-bridge
sudo chmod 700 /var/lib/ts-bridge
```

### 4. Create Environment File

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

> **Security:** Use a secrets manager in production (env file with perms 600, or HashiCorp Vault / AWS SSM).

### 5. Install systemd Service

```bash
sudo cp scripts/host/ts-bridge.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable ts-bridge
sudo systemctl start ts-bridge
```

### 6. Verify

```bash
sudo systemctl status ts-bridge
journalctl -u ts-bridge -f
```

## Monitoring

### Health Endpoint

Enable with `TS_HEALTH_ADDR=127.0.0.1:8080`:

```bash
curl http://127.0.0.1:8080/health/live   # {"status":"ok"} (liveness)
curl http://127.0.0.1:8080/health/ready  # {"status":"ok"} (readiness: tsnet tunnel up)
curl http://127.0.0.1:8080/metrics       # Connection stats (JSON)
```

### Key Metrics

| Metric | Description | Alert Threshold |
|--------|-------------|-----------------|
| `active_connections` | Current open connections | > 80% of max |
| `total_errors` | Cumulative error count | Increasing rapidly |
| `rejected_connections` | Connections rejected due to limit | > 0 sustained |

### Prometheus Integration

Example scrape config (note: currently JSON format, Prometheus text format pending backlog item OBS-001):

```yaml
scrape_configs:
  - job_name: 'ts-bridge'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: /metrics
```

### Log Monitoring

```bash
# Enable JSON logs for parsing
export TS_LOG_FORMAT=json

# Example log entries
# {"time":"2024-01-15T10:30:00Z","level":"INFO","msg":"connection opened","client":"127.0.0.1:54321"}
# {"time":"2024-01-15T10:35:00Z","level":"INFO","msg":"connection closed","client":"127.0.0.1:54321","duration":"5m0s","bytes_tx":1048576,"bytes_rx":2097152}
```

## Performance Tuning

### Connection Limits

Default: 1000 concurrent connections.

```bash
export TS_MAX_CONNECTIONS=5000
```

### File Descriptor Limits

For high connection counts, add to `[Service]` section of systemd unit:

```ini
LimitNOFILE=65535
```

### Timeout Tuning

```bash
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

### Platform-Specific: SELinux

```bash
# Check audit log
ausearch -m avc -ts recent

# Temporarily disable (for testing only)
setenforce 0
```

### Platform-Specific: Firewall

```bash
sudo ufw allow from 127.0.0.1 to any port 33389
```

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
