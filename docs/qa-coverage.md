# QA Coverage Matrix

Tracks what the automated smoke suites cover versus what still requires a human
with real hardware. This is a **living** document: each QA ticket
(QA-004..QA-013) updates the row(s) it lands.

- **POSIX / Linux / macOS** → [`scripts/tests/smoke.bats`](../scripts/tests/smoke.bats) (BATS)
- **Windows** → [`scripts/tests/smoke.ps1`](../scripts/tests/smoke.ps1) (PowerShell)

Both run in CI: the `smoke` job (Linux) executes the BATS suite on every PR.

## Legend

| Mark | Meaning |
|------|---------|
| ✅ | Automated (green in CI) |
| 🔶 | Partial — parsing/help only, behaviour deferred |
| ⬜ | Not yet automated |
| 👤 | Manual only (needs real mesh / admin privileges) |

## Coverage by command

| Command | Surface | BATS | PowerShell | Ticket |
|---------|---------|------|------------|--------|
| `version` | output, `--short`, `--version` flag | ✅ | ✅ | QA-004 (#173) |
| *(root)* | `--help`, `-h`, `help`, no-args, `-v`, unknown command, unknown flag | ✅ | 🔶 | QA-004 (#173) |
| `connect` | `--help` reachable | 🔶 | 🔶 | QA-004 (#173) |
| `init` | `--help` reachable | 🔶 | 🔶 | QA-004 (#173) |
| `status` | `--help` reachable | 🔶 | 🔶 | QA-004 (#173) |
| `discover` | `--help` reachable | 🔶 | 🔶 | QA-004 (#173) |
| `import` | `--help` reachable | 🔶 | ⬜ | QA-004 (#173) |
| `host` | `--help`, subcommand `--help` reachable | 🔶 | 🔶 | QA-004 (#173) |
| `init` | all flags, formats, overwrite protection | ⬜ | ✅ | QA-005 (#174) |
| `status` | running/not-running, `--json`, `--watch`, `--addr` | ⬜ | 🔶 | QA-006 (#175) |
| `connect` | flag parsing, error handling, graceful shutdown | ⬜ | 🔶 | QA-007 (#176) |
| `discover` | `--json`, `--filter`, `--auto`, `--port` | ⬜ | ⬜ | QA-008 (#177) |
| `host setup` | Windows registry/firewall/UPnP/sleep; Linux xrdp/UFW | 👤 | 👤 | QA-009 (#178) |
| `host check` | read-only, `--json` | ⬜ | 🔶 | QA-010 (#179) |
| *(cross-cutting)* | config precedence (flags > env > YAML > defaults) | ⬜ | 🔶 | QA-011 (#181) |
| *(cross-cutting)* | error handling (missing key, bad target/ports, timeouts) | ⬜ | ⬜ | QA-012 (#182) |
| *(e2e)* | multi-device real mesh, bidirectional forwarding | 👤 | 👤 | QA-013 (#183) |

## Scope note

QA-004 (#173) establishes the shared BATS infrastructure — the binary-resolution
helper, assertion helpers, the CLI-parsing section, and the CI `smoke` job — and
covers **CLI parsing** end-to-end: that every command registers, every help page
is reachable, and unknown input fails loudly. Per-command *behaviour* (files
written, sockets probed, connections dialled) is deferred to QA-005..QA-013,
each of which appends its section to the same `smoke.bats` file and updates the
row(s) above.
