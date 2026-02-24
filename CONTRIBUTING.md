# Contributing to ts-bridge

## Development Setup

### Prerequisites

- Go 1.21+
- A Tailscale account (for integration testing)

### Quick Start

```bash
# Clone
git clone https://github.com/mlorentedev/ts-bridge.git
cd ts-bridge

# Setup environment
cp .env.example .env
# Edit .env with your Tailscale auth key and target

# Run in development mode (verbose logging)
./scripts/dev.sh

# Or build manually
go build -o ts-bridge .
./ts-bridge -v
```

### Building

```bash
# Simple build
go build -o ts-bridge .

# Build with version info
VERSION=v1.2.0
COMMIT=$(git rev-parse --short HEAD)
go build -ldflags="-s -w -X main.version=${VERSION} -X main.commit=${COMMIT}" -o ts-bridge .

# Verify
./ts-bridge -version
# Output: ts-bridge v1.2.0 (abc1234)
```

### Testing

```bash
# Run all tests
go test -v ./...

# Run with race detector
go test -race ./...

# Run with coverage
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Run specific test
go test -v -run TestHealthEndpoints ./...
```

### Linting

```bash
go vet ./...
golangci-lint run
```

## Project Structure

```text
ts-bridge/
├── main.go                    # Core application
├── main_test.go               # Unit tests
├── main_integration_test.go   # Integration tests
├── scripts/
│   ├── dev.sh                 # Development launcher
│   ├── client/                # Client launchers (run.sh, run.ps1)
│   └── host/                  # Host setup (setup.ps1, ts-bridge.service)
├── .github/workflows/
│   ├── ci.yml                 # Build, test, lint, security
│   └── release.yml            # Automatic releases
└── [config files]             # go.mod, .env.example, etc.
```

## Code Style

- Follow standard Go conventions
- Use `gofmt` for formatting
- Use `slog` for logging (not `log` or `fmt.Print`)
- Add tests for new functionality
- Keep functions under 50 lines when possible
- Document exported types and functions

### Logging Convention

```go
// Use structured logging
logger.Info("connection opened", "client", addr)
logger.Debug("tunnel established", "client", addr, "target", cfg.Target)
logger.Warn("copy error", "client", addr, "error", err)
logger.Error("dial failed", "client", addr, "error", err)
```

## Commit Messages

This project uses [Conventional Commits](https://www.conventionalcommits.org/) for automatic versioning:

| Prefix | Description | Version Bump |
|--------|-------------|--------------|
| `feat:` | New feature | Minor (0.1.0 → 0.2.0) |
| `fix:` | Bug fix | Patch (0.1.0 → 0.1.1) |
| `docs:` | Documentation only | None |
| `chore:` | Maintenance | None |
| `refactor:` | Code refactoring | None |
| `test:` | Adding tests | None |
| `feat!:` | Breaking change | Major (0.1.0 → 1.0.0) |

Examples:
```
feat: add connection rate limiting
fix: handle ECONNRESET errors gracefully
docs: update README with health endpoint info
chore: update dependencies
```

## Pull Request Process

1. Create a feature branch from `master`
   ```bash
   git checkout -b feat/my-feature
   ```

2. Make changes with appropriate tests

3. Ensure all checks pass:
   ```bash
   go test -race ./...
   go vet ./...
   ```

4. Commit using Conventional Commits

5. Push and open a Pull Request

6. CI must pass before merge

## Release Process

Releases are automated via [release-please](https://github.com/googleapis/release-please):

1. Push commits to `master` using Conventional Commits
2. release-please creates/updates a Release PR
3. Merging the Release PR triggers:
   - Git tag creation (e.g., `v1.2.0`)
   - `CHANGELOG.md` update
   - Binary builds for 6 platforms
   - GitHub Release with assets and checksums

### Configuration Files

| File | Purpose |
|------|---------|
| `release-please-config.json` | Release behavior configuration |
| `.release-please-manifest.json` | Current version tracker |

## CI Pipeline

On every push/PR:

| Job | Description |
|-----|-------------|
| `test` | Build, vet, tests with race detector and coverage |
| `lint` | golangci-lint static analysis |
| `security` | gosec security scanner |
| `shellcheck` | Validate bash scripts |
| `build-matrix` | Cross-compile for linux/windows/darwin × amd64/arm64 |

## Testing Guidelines

- Unit tests for config parsing and error handling
- Integration tests for proxy behavior (use loopback, no real Tailscale)
- Table-driven tests for multiple cases
- Test file naming: `*_test.go` for unit, `*_integration_test.go` for integration
