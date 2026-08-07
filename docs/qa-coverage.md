# QA Coverage Matrix

Tracks what the automated smoke suites cover versus what still requires a human
with real hardware. This is a **living** document: each QA ticket
(QA-004..QA-013) updates the row(s) it lands.

- **POSIX / Linux / macOS** → [`scripts/tests/smoke.bats`](../scripts/tests/smoke.bats) (BATS),
  run by the `smoke` job on every PR.
- **Cross-platform (incl. Windows)** → Go tests under [`cmd/cli/`](../cmd/cli/),
  run by the `test` and `test-windows` jobs on every PR.

There is no PowerShell suite. `smoke.ps1` was retired in #271: it ran in no CI
job, so it drifted from the CLI it claimed to test — three of its nineteen cases
asserted things that were never true. Cross-platform CLI coverage is written in
Go, where it is visible to `go test -cover` and to the mutation harness (QA-014);
a smoke suite invoked as a subprocess is invisible to both.

## Legend

| Mark | Meaning |
|------|---------|
| ✅ | Automated (green in CI) |
| 🔶 | Partial — parsing/help only, behaviour deferred |
| ⬜ | Not yet automated |
| 👤 | Manual only (needs real mesh / admin privileges) |

## Coverage by command

| Command | Surface | BATS | Ticket |
|---------|---------|------|--------|
| `version` | output, `--short`, `--version` flag | ✅ | QA-004 (#173) |
| *(root)* | `--help`, `-h`, `help`, no-args, `-v`, unknown command, unknown flag | ✅ | QA-004 (#173) |
| `connect` | `--help` reachable | 🔶 | QA-004 (#173) |
| `init` | `--help` reachable | 🔶 | QA-004 (#173) |
| `status` | `--help` reachable | 🔶 | QA-004 (#173) |
| `discover` | `--help` reachable | 🔶 | QA-004 (#173) |
| `import` | `--help` reachable | 🔶 | QA-004 (#173) |
| `host` | `--help`, subcommand `--help` reachable | 🔶 | QA-004 (#173) |
| `init` | all flags, formats, overwrite protection, auth-key-not-in-yaml | ✅ | QA-005 (#174) |
| `status` | not-running, `--json` (down), `--addr`, `--watch`/`--interval` flags | 🔶 | QA-006 (#175) |
| `connect` | flag parsing + config-validation errors (target/auth/auth-key-file/bad flag), auth-key warning | 🔶 | QA-007 (#176) |
| `discover` | required auth/tailnet errors, `--port` validation, `--json` no-auth path, flag surface | 🔶 | QA-008 (#177) |
| `host setup` | `--help` flags + all platforms, non-root elevation guard, `--port` parse error | 🔶 | QA-009 (#178) |
| `host check` | `--help`, read-only status block (exit 0, no admin), `--json` fields, stdout is exactly one JSON object | ✅ | QA-010 (#179) |
| *(cross-cutting)* | config precedence (flags > env > YAML > defaults) | ⬜ | QA-011 (#181) |
| *(cross-cutting)* | error handling (missing key, bad target/ports, timeouts) | ⬜ | QA-012 (#182) |
| *(e2e)* | multi-device real mesh, bidirectional forwarding | 👤 | QA-013 (#183) |

> The Go-side column is deliberately absent: some existing `cmd/cli` tests
> exercise hand-built replicas of the command tree rather than the real one, so
> a per-row Go mark would overstate coverage. It lands once the command tree is
> constructible from tests.

## Scope note

QA-004 (#173) establishes the shared BATS infrastructure — the binary-resolution
helper, assertion helpers, the CLI-parsing section, and the CI `smoke` job — and
covers **CLI parsing** end-to-end: that every command registers, every help page
is reachable, and unknown input fails loudly. Per-command *behaviour* (files
written, sockets probed, connections dialled) is deferred to QA-005..QA-013,
each of which appends its section to the same `smoke.bats` file and updates the
row(s) above.

### Deliberately not smoke-tested

Some behaviour is intentionally left to unit tests / e2e rather than the smoke
suite, to keep the suite fast, hermetic, and non-hanging:

- **`status` RUNNING state + metrics JSON** — needs a live health server; covered
  by `cmd/cli/status_test.go` and the QA-013 e2e run.
- **`status --watch` loop** — a signal-terminated infinite loop. Executing it
  risks hanging CI if a stop signal is not delivered (observed on native
  Windows), so only its flags are smoke-checked; `runWatchLoop` has unit
  coverage via an injected signal channel.
- **`connect` bridge start + graceful shutdown** — once config validates,
  `connect` starts tsnet and runs until signalled (same hang risk as
  `--watch`). Smoke tests cover only the pre-start validation/error paths;
  starting the bridge and draining it is covered by `main_integration_test.go`
  (mock `Dialer`) and the QA-013 e2e run.
- **`discover` live fetch, interactive selection, `--filter`/`--auto`** — these
  are reached only after a successful tailnet API query (needs a real auth key,
  tailnet, and network). Smoke tests cover the pre-fetch validation/error paths
  and the flag surface; the live behaviour is covered by the QA-013 e2e run.
- **`host setup` real execution (Windows registry/firewall/UPnP/sleep; Linux
  xrdp/UFW/iptables)** — mutates the machine and needs admin/root. The smoke
  suite exercises only `--help`, flag parsing, and the non-root elevation guard,
  which returns before any side effect; idempotency, per-step results, and
  partial-failure handling stay manual / QA-013 e2e. `internal/host` carries the
  unit coverage for the platform-specific logic.
(`host check --json` strict parseability used to be listed here, deferred on
bug #254. That bug is fixed — host logs now go to stderr — and the suite asserts
stdout holds exactly one JSON object, so the exclusion no longer applies.)
