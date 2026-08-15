---
id: "ts-bridge-deploy-windows"
type: runbook
status: active
tags: [deployment, windows, production]
created: "2026-02-23"
updated: "2026-06-12"
owner: manu
---

# Deployment Guide: Windows

## Prerequisites
- ts-bridge binary for Windows (from [Releases](https://github.com/mlorentedev/ts-bridge/releases))
- Tailscale auth key ([generate](https://login.tailscale.com/admin/settings/keys))
- PowerShell (for service installation, optional)

## Client Deployment (No Admin Required)

This is the primary use case — locked-down Windows machines where you cannot install Tailscale natively.

### 1. Extract Binary

```powershell
# Download from GitHub Releases and extract
Expand-Archive -Path ts-bridge-windows-amd64.zip -DestinationPath C:\ts-bridge
cd C:\ts-bridge
```

### 2. Configure

Create a `.env` file with the two required variables:

```powershell
# .env — same directory as ts-bridge.exe
TS_AUTHKEY=tskey-auth-kXXXXXXXXX   # From Tailscale admin console
TS_TARGET=100.82.151.104:3389       # Host Tailscale IP + port
```

### 3. Run

```powershell
# Run with .env config (auto-detected)
.\ts-bridge.exe connect

# Run with verbose logging
.\ts-bridge.exe connect -v

# Run with key file (secure alternative to plaintext .env / --auth-key)
.\ts-bridge.exe connect --target 100.82.151.104:3389 --auth-key-file C:\path\to\authkey

# Interactive setup wizard
.\ts-bridge.exe init
```

### 4. Connect

Once the bridge is running, read the local port from the banner and connect your RDP client:

```batch
mstsc /v:127.0.0.1:<LOCAL_PORT>
```

### 5. Run as Windows Service (Optional)

For persistent deployments (requires admin rights):

```powershell
# Install as Windows service (run as Administrator)
New-Service -Name "ts-bridge" `
  -BinaryPathName "C:\ts-bridge\ts-bridge.exe connect" `
  -StartupType Automatic

Start-Service ts-bridge
```

## Host Setup (Admin Required)

The target machine (host) needs Tailscale installed natively. Use the CLI to automate RDP configuration:

```powershell
# Configure host for RDP (run as Administrator)
ts-bridge host setup

# Verify host readiness (read-only)
ts-bridge host check

# Custom firewall rule name
ts-bridge host setup --firewall-rule "MyRDPRule"

# Skip disabling sleep mode
ts-bridge host setup --no-sleep
```

### Manual Host Setup

If the CLI cannot be run on the host, use the PowerShell helper script:

```powershell
# Run as Administrator
.\scripts\host\setup.ps1
```

## Version Verification

```powershell
.\ts-bridge.exe version
.\ts-bridge.exe version --short   # Just the semver
```

## Platform-Specific Issues

### Execution Policy
If script execution is restricted, the binary itself needs no execution policy — it's an `.exe`.

### Antivirus
Some AV products may flag the binary. Add an exception if needed.

### RDP Certificate Warnings
Accept the certificate warning in `mstsc` when connecting to `127.0.0.1:<LOCAL_PORT>`.

## macOS Notes

### Gatekeeper
```bash
# If blocked, allow in System Preferences > Security
# Or remove quarantine
xattr -d com.apple.quarantine ./ts-bridge
```