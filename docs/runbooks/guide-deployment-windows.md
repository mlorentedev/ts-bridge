---
id: "ts-bridge-deploy-windows"
type: runbook
status: active
tags: [deployment, windows, production]
created: "2026-02-23"
owner: manu
---

# Deployment Guide: Windows

## Prerequisites
- ts-bridge binary for Windows (from [Releases](https://github.com/mlorentedev/ts-bridge/releases))
- Tailscale auth key ([generate](https://login.tailscale.com/admin/settings/keys))
- PowerShell (for service installation)

## Installation

### Windows Service

Use the provided install script:

```powershell
# See scripts/host/install-service.ps1
PowerShell -ExecutionPolicy Bypass -File .\scripts\host\install-service.ps1
```

### Client (No Admin)

```powershell
# Extract binary
# Edit .env with TS_AUTHKEY and TS_TARGET
# Run
PowerShell -ExecutionPolicy Bypass -File .\scripts\client\run.ps1
```

## Platform-Specific Issues

### Execution Policy
```powershell
PowerShell -ExecutionPolicy Bypass -File .\scripts\client\run.ps1
```

### Antivirus
Some AV products may flag the binary. Add exception if needed.

### RDP Certificate Warnings
Accept the certificate warning in `mstsc` when connecting to `127.0.0.1:33389`.

## macOS Notes

### Gatekeeper
```bash
# If blocked, allow in System Preferences > Security
# Or remove quarantine
xattr -d com.apple.quarantine ./ts-bridge
```
