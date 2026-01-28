# ts-bridge

TCP bridge for tunneling connections through Tailscale's encrypted mesh network without requiring administrator privileges on the client machine.

## Overview

Connect via RDP/SSH from a **non-admin machine** to an **admin machine** through restrictive firewalls using Tailscale's userspace networking.

| Machine | Admin Rights | Tailscale | Role |
|---------|--------------|-----------|------|
| **Client** | No | Not installed (uses tsnet) | Initiates connection |
| **Host** | Yes | Installed natively | Receives connection |

## Architecture

```text
┌─────────────────────────────────────────────────────────────────────────────────┐
│  CLIENT (Non-Admin Machine)                                                     │
│  ══════════════════════════                                                     │
│                                                                                 │
│   ┌────────────┐      ┌─────────────────────────────────────────────────────┐  │
│   │ RDP Client │      │                    ts-bridge                        │  │
│   │            │─────▶│  ┌─────────────┐         ┌────────────────────┐     │  │
│   └────────────┘      │  │ TCP Listener│────────▶│ tsnet (ephemeral)  │     │  │
│         │             │  │ :33389      │         │ WireGuard userspace│     │  │
│         ▼             │  └─────────────┘         └─────────┬──────────┘     │  │
│  127.0.0.1:33389      │                                    │                │  │
│                       │  No admin rights required          │                │  │
│                       │  No Tailscale installation         │                │  │
│                       └────────────────────────────────────┼────────────────┘  │
│                                                            │                   │
│  ┌──────────────────────────────────────────┐              │                   │
│  │ Restrictive Firewall                     │              │                   │
│  │ ✗ UDP blocked                            │              │                   │
│  │ ✗ Software installation blocked          │◀─────────────┘                   │
│  │ ✓ HTTPS allowed (DERP relay)             │   Tunnels via HTTPS              │
│  └──────────────────────────────────────────┘                                   │
└─────────────────────────────────────────────────────────────────────────────────┘
                                        │
                                        │ Tailscale Network
                                        │ (WireGuard encrypted)
                                        │
                                        ▼
┌─────────────────────────────────────────────────────────────────────────────────┐
│  HOST (Admin Machine)                                                           │
│  ════════════════════                                                           │
│                                                                                 │
│   ┌─────────────────────────────────────────────────────────────────────────┐  │
│   │                         Tailscale (Native Install)                      │  │
│   │                         100.x.x.x                                       │  │
│   └───────────────────────────────────┬─────────────────────────────────────┘  │
│                                       │                                        │
│                                       ▼                                        │
│                          ┌────────────────────────┐                            │
│                          │    RDP Server (:3389)  │                            │
│                          └────────────────────────┘                            │
│                                                                                 │
└─────────────────────────────────────────────────────────────────────────────────┘
```

## Why ts-bridge?

| Requirement | Native Tailscale | ts-bridge |
|-------------|------------------|-----------|
| Admin rights on client | **Yes** | **No** |
| Kernel module | Yes | No (userspace) |
| Software installation | Required | Portable binary |
| Leaves traces | Yes | No (ephemeral) |
| Works on locked-down machines | No | **Yes** |

## Installation

### Host Machine (Admin Rights Required)

1. Install [Tailscale](https://tailscale.com/download) normally
2. Run the setup script:

```powershell
# Run as Administrator
cd scripts\host
PowerShell -ExecutionPolicy Bypass -File .\setup.ps1

# Optional: Auto-run at boot
PowerShell -ExecutionPolicy Bypass -File .\install-service.ps1
```

Note the Tailscale IP shown (e.g., `100.82.151.104`).

### Client Machine (No Admin Rights)

#### Windows (via Scoop - no admin required)

```powershell
# Install Scoop (one-time, no admin)
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
Invoke-RestMethod -Uri https://get.scoop.sh | Invoke-Expression

# Install Go
scoop install go

# Clone and configure
git clone https://github.com/mlorentedev/ts-bridge.git
cd ts-bridge
cp .env.example .env
# Edit .env with your settings
```

#### Linux (no admin required)

```bash
# Install Go to home directory (no sudo)
curl -LO https://go.dev/dl/go1.21.0.linux-amd64.tar.gz
mkdir -p ~/go-sdk
tar -C ~/go-sdk -xzf go1.21.0.linux-amd64.tar.gz
export PATH=$PATH:~/go-sdk/go/bin
echo 'export PATH=$PATH:~/go-sdk/go/bin' >> ~/.bashrc

# Clone and configure
git clone https://github.com/mlorentedev/ts-bridge.git
cd ts-bridge
go mod tidy
cp .env.example .env
# Edit .env with your settings
```

#### Pre-built Binaries (no Go required)

Download from [Releases](https://github.com/mlorentedev/ts-bridge/releases):

```bash
# Linux
curl -LO https://github.com/mlorentedev/ts-bridge/releases/latest/download/ts-bridge-linux-amd64
chmod +x ts-bridge-linux-amd64
./ts-bridge-linux-amd64
```

```powershell
# Windows (PowerShell)
Invoke-WebRequest -Uri "https://github.com/mlorentedev/ts-bridge/releases/latest/download/ts-bridge-windows-amd64.exe" -OutFile "ts-bridge.exe"
.\ts-bridge.exe
```

## Usage

### Client - Linux/macOS

```bash
./scripts/client/run.sh

# Keep state between runs
./scripts/client/run.sh --keep-state
```

### Client - Windows

```powershell
PowerShell -ExecutionPolicy Bypass -File .\scripts\client\run.ps1

# Keep state between runs
PowerShell -ExecutionPolicy Bypass -File .\scripts\client\run.ps1 -KeepState
```

### Connect via RDP

```bash
# Linux
xfreerdp /v:127.0.0.1:33389 /u:Username /cert:ignore /dynamic-resolution /bpp:16 +compression -themes -wallpaper

# Linux (fixed size)
xfreerdp /v:127.0.0.1:33389 /u:Username /cert:ignore /size:80% /bpp:16 +compression

# Windows
mstsc /v:127.0.0.1:33389
```

## Configuration

Create `.env` from `.env.example`:

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `TS_AUTHKEY` | Yes | - | Tailscale auth key ([generate](https://login.tailscale.com/admin/settings/keys)) |
| `TS_TARGET` | Yes | - | Host Tailscale IP:PORT (e.g., `100.x.x.x:3389`) |
| `TS_LOCAL_ADDR` | No | `127.0.0.1:33389` | Local bind address |
| `TS_HOSTNAME` | No | `ts-bridge` | Ephemeral node name |
| `TS_STATE_DIR` | No | `./ts-state` | State directory |
| `TS_TIMEOUT` | No | `30s` | Connection timeout |

## How It Works

### Pattern: Userspace Network Tunnel (TCP-over-WireGuard Proxy)

1. **Client** runs ts-bridge with an ephemeral Tailscale node via [tsnet](https://pkg.go.dev/tailscale.com/tsnet)
2. tsnet runs WireGuard in **userspace** (no kernel module, no admin rights)
3. Firewall blocks UDP/WireGuard but **allows HTTPS**
4. Tailscale uses **DERP relay** (encrypted WebSocket over HTTPS)
5. Bridge accepts local TCP and **dials target through Tailscale**
6. All traffic is **end-to-end encrypted** (WireGuard inside HTTPS)
7. **Host** receives connection on Tailscale IP, forwards to RDP
8. Ephemeral node **auto-deletes** from Tailscale admin on exit

## Use Cases

- Access admin machine from locked-down client
- Development on restricted networks
- IT support without VPN infrastructure
- Remote access through restrictive firewalls

## Limitations

| Limitation | Impact | Mitigation |
|------------|--------|------------|
| TCP only | No UDP (VoIP, games) | Use for RDP, SSH, HTTP |
| DERP latency | +50-200ms when relayed | Acceptable for RDP |
| Auth key expiry | Default 90 days | Use long-lived keys |
| Single target | One host per instance | Run multiple instances |

## Project Structure

```text
ts-bridge/
├── .github/workflows/
│   ├── ci.yml                      # Build, test, lint, security scan
│   └── release.yml                 # Automatic releases via release-please
├── scripts/
│   ├── client/                     # For non-admin machines
│   │   ├── run.sh                  # Linux/macOS launcher (loads .env, runs bridge)
│   │   └── run.ps1                 # Windows launcher (loads .env, runs bridge)
│   └── host/                       # For admin machines (Windows)
│       ├── setup.ps1               # Configure Tailscale, RDP, firewall, power
│       └── install-service.ps1     # Register setup.ps1 as scheduled task
├── main.go                         # Bridge source (~210 lines)
├── main_test.go                    # Unit tests for config validation
├── go.mod                          # Go module definition
├── go.sum                          # Dependency checksums
├── .env.example                    # Configuration template (copy to .env)
├── .gitignore                      # Excludes .env, ts-state/, binaries
├── release-please-config.json      # Semantic versioning config
├── .release-please-manifest.json   # Current version tracker
├── CHANGELOG.md                    # Auto-generated on releases
├── LICENSE                         # MIT license
└── README.md
```

### File Descriptions

| File/Folder | Purpose |
|-------------|---------|
| `main.go` | TCP bridge using tsnet (Tailscale userspace). Validates config, creates ephemeral node, proxies connections. |
| `main_test.go` | Tests for `loadConfig()`: validates TS_TARGET format, TS_AUTHKEY prefix, TS_TIMEOUT parsing. |
| `.env.example` | Template with all environment variables. Copy to `.env` and fill in your values. |
| `scripts/client/*` | Launchers that load `.env`, clear state, and run the bridge. No admin required. |
| `scripts/host/*` | Windows scripts to configure RDP access. Require admin. Run once on host machine. |
| `release-please-*.json` | Config for [release-please](https://github.com/googleapis/release-please). Automates semver based on [Conventional Commits](https://conventionalcommits.org). |
| `ts-state/` | (gitignored) Tailscale ephemeral node state. Cleared on each run by default. |

## Security

- **No admin footprint**: Runs entirely in userspace
- **Ephemeral nodes**: Auto-delete from Tailscale, no traces
- **E2E encryption**: WireGuard encryption even through DERP
- **Local only**: Binds to 127.0.0.1 by default
- **Auth key**: Keep `.env` private, never commit

## Troubleshooting

| Issue | Solution |
|-------|----------|
| "dial failed: context deadline exceeded" | Check host Tailscale IP and firewall |
| "tailscale init failed" | Invalid auth key or network blocking |
| "Connection reset by peer" | Host service down or network issue |
| Certificate warnings | Use `/cert:ignore` with xfreerdp |
| Slow connection | DERP relay active; normal for restricted networks |

## Development

### Building from Source

```bash
# Clone
git clone https://github.com/mlorentedev/ts-bridge.git
cd ts-bridge

# Build
go build -o ts-bridge .

# Build with version info
go build -ldflags="-s -w -X main.version=v1.0.0" -o ts-bridge .

# Run tests
go test -v ./...
go test -race -coverprofile=coverage.out ./...

# Lint
go vet ./...
golangci-lint run
```

### Automatic Releases

This project uses [release-please](https://github.com/googleapis/release-please) by Google for automatic semantic versioning. No manual version bumping required.

#### How It Works

1. **Write commits using [Conventional Commits](https://conventionalcommits.org)**:
   ```
   feat: add new feature        → minor bump (0.1.0 → 0.2.0)
   fix: fix a bug               → patch bump (0.1.0 → 0.1.1)
   feat!: breaking change       → major bump (0.1.0 → 1.0.0)
   docs: update readme          → no release
   chore: update dependencies   → no release
   ```

2. **Push to main** → release-please analyzes commits and creates a Release PR

3. **Merge the Release PR** → automatically:
   - Creates git tag (e.g., `v1.2.0`)
   - Generates `CHANGELOG.md`
   - Builds binaries for 6 platforms
   - Publishes GitHub Release with assets

#### Configuration Files

| File | Purpose |
|------|---------|
| `release-please-config.json` | Release behavior (type, changelog path, versioning) |
| `.release-please-manifest.json` | Tracks current version (updated automatically) |

### CI Pipeline

On every push/PR:

| Job | Description |
|-----|-------------|
| `test` | Build, vet, run tests with race detector and coverage |
| `lint` | golangci-lint static analysis |
| `security` | gosec (security scanner) + govulncheck (vulnerability check) |
| `shellcheck` | Validate bash scripts |
| `build-matrix` | Cross-compile for linux/windows/darwin × amd64/arm64 |

### Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b feat/my-feature`
3. Commit using Conventional Commits: `git commit -m "feat: add my feature"`
4. Push and open a Pull Request
5. CI must pass before merge

## Support

For questions, bugs, or feature requests, please [open an issue](https://github.com/mlorentedev/ts-bridge/issues).

## License

[MIT](LICENSE)
