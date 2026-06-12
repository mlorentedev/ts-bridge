---
id: "ts-bridge-launcher-parity"
type: runbook
status: done
tags: [parity, scripts, linux, windows, audit]
created: "2026-05-18"
owner: manu
---

# Launcher Script Parity (run.sh vs run.ps1)

> Static audit + maintenance checklist. The Go binary is OS-agnostic (cross-compiled by CI's build-matrix). What can diverge is the *launcher* shell logic that reads `.env`, validates required vars, and invokes the binary. This doc captures the audit performed on 2026-05-18 and the rules for keeping the two scripts in lockstep.

## Why this exists

OPS-004 was originally framed as "validate Linux client parity" — requiring an actual Linux machine to smoke-test. In practice, the binary is portable Go; what genuinely differs between platforms is the launcher script syntax. A static audit catches the meaningful divergences without needing hardware, and a maintenance checklist prevents drift in future PRs.

## Audit performed 2026-05-18

Compared `scripts/client/run.sh` (122 lines, Bash) and `scripts/client/run.ps1` (119 lines, PowerShell) line-by-line.

### Findings

| Concern | run.sh | run.ps1 | Verdict |
|---|---|---|---|
| Arg parsing (`--reset`, `--instance`) | Manual `case` loop | `param()` block | Equivalent semantics |
| `.env` file existence check | `[[ ! -f ]]` + exit 1 | `Test-Path` + exit 1 | Equivalent |
| `.env` parsing | `set -a; source` | Regex `^\s*([^#][^=]+)=(.*)$` + `SetEnvironmentVariable` | **Different security model** — Bash sources arbitrary code; PowerShell parses literals only. Documented assumption, not a bug. |
| `TS_AUTHKEY` / `TS_TARGET` validation | `[[ -z ]]` + exit | `if (-not $env:VAR)` + exit | Equivalent |
| Instance override | `export TS_INSTANCE_NAME` | `[Environment]::SetEnvironmentVariable` | Equivalent |
| **Truthy parsing for `TS_AUTO_INSTANCE` / `TS_MANUAL_MODE`** | `case` against `1\|true\|TRUE\|yes\|YES\|on\|ON` | `@("true","1","yes","on") -contains $env:VAR.ToLowerInvariant()` | **Bug found**: PowerShell case-insensitive, bash case-sensitive on specific patterns. `TS_AUTO_INSTANCE=True` (capital T) silently fell to false on Linux. **Fixed in PR #29** by adding a `lc()` helper using `tr '[:upper:]' '[:lower:]'`. |
| Auto-mode echo | Same text output | Same text output | Equivalent |
| Manual mode state dir handling | `rm -rf` + `mkdir -p` | `Remove-Item -Recurse -Force` + `New-Item -ItemType Directory -Force` | Equivalent |
| Path resolution | `cd "$(dirname "${BASH_SOURCE[0]}")" && pwd` | `Split-Path -Parent $MyInvocation.MyCommand.Path` | Both robust to symlinks/spaces |
| Binary detection | `./ts-bridge` (or `.exe` on msys/cygwin) | `ts-bridge.exe` first, fallback `ts-bridge` | Equivalent for native platforms |
| Fallback to `go run` | `command -v go` + `[[ -f main.go ]]` | `Get-Command "go"` + `Test-Path "main.go"` | Equivalent |

### State directory permissions

Neither script sets `0700` on creation — the Go binary does that the first time it touches the dir (`main.go` `ensureStateDir`). Code is gated by `runtime.GOOS != "windows"` for the warning, but `os.MkdirAll(dir, 0700)` runs on all OSes. On Windows the permission bits are interpreted by Go's stdlib but NTFS ACLs are OS-default — a known limitation documented in `50-troubleshooting/security-audit.md`.

### What was NOT audited

- `bootstrap.sh` / `bootstrap.ps1` — install/setup helpers. Lower-risk surface, not part of OPS-004.
- `scripts/host/setup.ps1` (Windows-only) — no Linux counterpart by design; the target host running native Tailscale on Linux is administered differently.
- `scripts/dev.sh` — dev helper, no PowerShell equivalent. Linux/macOS-only by intent.

---

## Parity maintenance checklist

> Run this when touching either `run.sh` or `run.ps1`. The other must mirror the change.

- [ ] Any new env var read by one script must be read by the other.
- [ ] Truthy-value parsing uses case-insensitive matching in BOTH (PowerShell `.ToLowerInvariant()` vs bash `lc()` helper).
- [ ] Any new validation (exit 1 on missing/bad value) added to one is added to the other with the same error string.
- [ ] Same banner output ("TAILSCALE BRIDGE (Client)", "Target: ...", separators).
- [ ] Binary fallback to `go run` is symmetric.
- [ ] Path resolution stays robust to spaces and symlinks (`BASH_SOURCE[0]` / `MyInvocation.MyCommand.Path`).
- [ ] CI's `shellcheck` step passes on `run.sh` (run via `.github/workflows/ci.yml` shellcheck job).
- [ ] No new POSIX-bash-3.2-incompatible syntax introduced in `run.sh` (e.g. `${var,,}`, `mapfile`, `[[ =~ ]]` with PCRE) — macOS default bash is 3.2.

## Known acceptable differences (not bugs)

- **`.env` parsing security**: bash `source` executes arbitrary code (including `$(...)` command substitution and side effects); PowerShell regex extracts literal `KEY=VALUE` pairs. Both assume operator-controlled `.env`. Documented in `security-audit.md` § Attack Surface (input vector: `.env`).
- **Default shell on macOS is bash 3.2** (very old). The script uses `#!/bin/bash` but stays within bash-3.2-compatible syntax to support out-of-the-box macOS without Homebrew bash 4+.
- **Windows `set -euo pipefail` equivalent is `$ErrorActionPreference = "Stop"`** — both abort on first error.

## Why no automated CI parity check?

A perfect parity test would require running both scripts under a controlled environment with the same `.env` and comparing the side-effects (env vars set, files created, binary invoked). That's a non-trivial test harness with cross-platform CI matrix. For a 119-line script pair owned by a single maintainer, the static checklist above + shellcheck on the bash side is the right ROI. Revisit if either script grows beyond 200 lines or accumulates >5 env-var reads.

## Related

- [security-audit.md](../troubleshooting/security-audit.md) — `.env` parsing trust assumption
- [lessons.md](../lessons.md) — entry 2026-05-18 on case-sensitivity divergence (if promoted)
- Changelog: PR #29 entry under v1.7.x patch (see repo `CHANGELOG.md`)
