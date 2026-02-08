# ts-bridge — Backlog

> TCP bridge for tunneling connections through Tailscale's mesh network.

## P0 — High Priority

- [ ] **Split main.go into packages**: Single 492-line file handles config, server, health, metrics, and connection logic. Extract into `internal/config`, `internal/bridge`, `internal/health`.
- [ ] **Add Prometheus metrics export**: Current `/metrics` endpoint returns JSON. Add `/metrics` in Prometheus exposition format for production monitoring.
- [ ] **Graceful drain on shutdown**: Currently closes listener immediately. Implement connection draining with configurable timeout before forced close.

## P1 — Medium Priority

- [ ] **Multi-target support**: Currently bridges to a single `TS_TARGET`. Support config file (YAML/TOML) with multiple local:remote port mappings.
- [ ] **Reconnection logic**: If tsnet connection drops mid-session, the bridge doesn't attempt reconnection. Add exponential backoff reconnect for the Tailscale tunnel itself.
- [ ] **Connection idle timeout**: No idle timeout on established connections. Add `TS_IDLE_TIMEOUT` to close stale connections (important for RDP sessions left open).
- [ ] **Systemd service file for Linux**: Only Windows service installer exists (`scripts/host/install-service.ps1`). Add Linux systemd unit + install script.
- [ ] **Integration test coverage**: `main_integration_test.go` exists but needs expansion — test health endpoints, metrics accuracy, connection limits, and error paths.

## P2 — Low Priority / Nice-to-Have

- [ ] **Docker image**: Publish multi-arch Docker image for containerized deployments.
- [ ] **TLS for health endpoint**: Health/metrics endpoint has no TLS option — sensitive in production.
- [ ] **Config file support**: Load config from YAML/TOML in addition to env vars.
- [ ] **Bandwidth rate limiting**: No per-connection or global bandwidth throttling.
- [ ] **Access logging**: Structured access log (separate from app log) for audit trails.

## Done

_None yet._
