---
id: "HOST-005-host-config"
type: spec
status: draft # draft | implementing | verifying | archived
created: "2026-10-30"
issue: "mlorentedev/ts-bridge#458"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal, host, config, cross-platform]
template_version: "1.0"
---

# HOST-005-host-config

## Why

<!-- from issue #458: HOST-005: declared host state in an explicit host configuration file -->

ADR-017 makes `host setup` converge a declared state and `host check` diff it, but today the declared state exists only implicitly in code. The host has no file that says what it should look like, and `host init` writes `TS_HOST_*` lines into a `.env` in the working directory, which is the path #399 retires (G3) and which `host init` rewrites with no confirmation (G5). Meanwhile `host setup --config x` is accepted and silently ignored, because `--config` is a root flag "reserved for future use". The per-OS convergence tickets (#454, #455, #459) and the descriptor fix (#453) all need one declared state to read, so it lands before them (ADR-017 plan, step 2).

## What

Three PRs.

**PR A1: the loader.** `internal/host` gains `LoadFile(path, goos, warn)`: the schema, version check, unknown-key warnings and validation below, including the per-OS capability table. Nothing calls it yet; no command changes. Split from PR A on 2026-10-30 (owner) because the loader alone is ~240 lines.

**PR A2: the declared state, read by `setup` and `check`.** Everything below that is not the loader: resolution, `--config` on the commands, C9, the descriptor.

The file and its rules (A1), and how the commands use it (A2):

- `host setup` and `host check` take `--config <path>`, a YAML file declaring the host's desired state (ADR-017 §1):

  ```yaml
  version: 1
  host:
    services:
      - kind: rdp        # rdp | ssh | vnc
        port: 3389       # optional; defaults per kind (rdp 3389, ssh 22, vnc 5900)
    firewall:
      rule: Tailscale-RDP-Ingress
    power:
      prevent_sleep: true
    control_url: https://headscale.example   # optional override of the control plane
  ```

- The file is optional and only read when `--config` names it. A named file that is missing or unreadable is an error (exit 1), never a fallback to defaults.
- Resolution order is `flags > env (TS_HOST_*) > file > per-OS defaults`. A `.env` in the working directory still loads into the env layer, as today. `--port` / `TS_HOST_RDP_PORT` set the `rdp` service's port; `--no-sleep` / `TS_HOST_NO_SLEEP` are the inverse of `power.prevent_sleep`.
- Per-OS default when no file declares `services`: Windows `rdp`, Linux `rdp`, macOS `ssh` (port defaults per kind).
- Validation, before any step runs:
  - `version` absent is version 1; any other value than 1 is an error naming the supported version.
  - Unknown keys at any level are ignored with one warning each on stderr, as the client YAML does. Keys reached through an alias or a merge key are checked as the decoder resolves them. A second YAML document is an error, not silently dropped. A known key with no value (`control_url:`, `services:`) is an error too, so a placeholder line cannot pass for an absent key (review of #519).
  - `kind` outside `rdp|ssh|vnc`, a port outside 1-65535, a duplicate `kind`, an empty `services` list, an invalid `firewall.rule`, or an empty, non-`http(s)`, host-less or credential-carrying `control_url` is an error naming the entry. A `control_url` with user info is rejected because the host needs no credential and PR B writes the file `0644` (review of #519).
  - **Against the target OS** (owner decision 2026-10-30): an entry the OS has no step for *in this version* is an error naming the entry and the OS, never skipped (ADR-017 §1). The capability table is what the steps implement today, and each per-OS ticket widens it: Windows `{rdp}` + `power` + named `firewall.rule`; Linux `{rdp}`; macOS `{}`. The error cites the ticket that adds the missing step where one exists (`ssh` on Linux and Windows: #521; `ssh`/`vnc` on macOS: #459), and ADR-017 §5 otherwise (`vnc` on Linux/Windows, `rdp` on macOS). `power` and `firewall` error outside Windows (only the Windows firewall step uses a rule name). Review of #519: #454 and #455 add no `ssh` step, so they are not cited.
  - Validation applies to the file only. Flags and env keep today's behaviour on every OS (for example, `--no-sleep` on Linux stays accepted and has no effect).
- A declared `control_url` is honoured now (owner decision 2026-10-30): it becomes the `cp=` of the `tsb://` descriptor printed by `setup` and `check` (text and `--json`). Absent means today's behaviour; detection is #453.
- Host commands never read the per-user profile or credential store (ADR-017 C9).
- Coupled fix. The defect: `host setup --firewall-rule` is declared with the non-empty default `"Tailscale-RDP-Ingress"`, so the flag always wins over `TS_HOST_FIREWALL_RULE` today and would win over `firewall.rule`. The fix: the flag gets an empty default (its help text still names the default rule), so it applies only when passed.

**PR B: `host init` writes the file.**

- `host init` requires `--config <path>` (owner decision 2026-10-30: explicit, no default path, per ADR-017's rejected "default machine-wide config path"). Without it, the command fails before prompting, naming the flag.
- It writes the YAML above instead of `.env`, with only the keys the target OS accepts, so the file it writes always loads back under the same validation. `power` and `firewall` are written on Windows only. On macOS it prints "not applicable" and exits 0, as `host setup` does, until #459.
- An existing file is not overwritten without interactive confirmation or `--force`; non-interactive without `--force` refuses. An existing path that cannot be read (permission, a directory) is refused even with `--force` (#408 second site).
- The file is written `0644`: it holds no secret, and `host check` runs unelevated and must be able to read a file root wrote. `0644` is necessary, not sufficient: the parent directories must be traversable too, so the docs recommend a path such as `/etc/ts-bridge/host.yaml` rather than one under `/root` (review of #519).
- Docs: `cli-reference.md`, `configuration.md`, `README.md` host section, and the G3/G5 rows of `docs/design/host-client-parity.md`.

## Out of scope

- Steps for new kinds or read-first behaviour: Linux and Windows `ssh` (#521), Windows state-first firewall (#455), macOS Remote Login / Screen Sharing (#459). They widen the capability table here.
- Control-plane detection, per-service descriptors and `svc=` (#453). PR A2 only passes a declared `control_url` through.
- A default or machine-wide config path, `TS_HOST_CONFIG`, or `--profile` on host commands (ADR-017 rejected alternatives).
- Removing `.env` support from `setup`/`check`, or migrating an existing `.env` into the YAML file.
- JSON contract changes beyond the descriptor's `cp=` value.

## Risks / open questions

- **`host init` without `--config` breaks.** Bare `ts-bridge host init` wrote `./.env`; it now fails with a message naming `--config`. Decided by the owner on 2026-10-30; the error says what to run, and the CHANGELOG entry flags it.
- **macOS default is declared but unused.** With no file, macOS declares `ssh`, but no darwin step reads it until #459, and `check` keeps reporting no service. The default is not a user declaration, so §1's "never skipped" holds for the file; #459 consumes it.
- **Strict capability table.** A user who declares `ssh` on Linux gets an error, not partial convergence. That is the point of §1; the message names the ticket that adds the step.
- **`.env` and the file disagree.** A stale `.env` in the working directory overrides the file (env beats file). This is the documented precedence; `host check --verbose` logs the resolved values.
- **The ~300 LOC cap.** The loader alone is ~240 production lines, so PR A was split into A1 (loader) and A2 (resolution, commands, C9, descriptor; ~120 estimated). PR B is ~150 net (the `.env` writer is removed). If A2 runs over, the descriptor `control_url` pass-through moves to its own PR with AC6.

## Acceptance criteria

- [ ] AC1: `--config` on `host setup` / `host check` loads the schema above (ADR-017 §1). `version` is checked, unknown keys warn and are ignored, and invalid entries fail with exit 1 before any step runs, naming the entry (loader in PR A1, commands in PR A2).
- [ ] AC2: Resolution order is `flags > env (TS_HOST_*) > file > per-OS defaults`, with `.env` in the working directory still loading into the env layer. `--firewall-rule` no longer overrides env or file when not passed (PR A2).
- [ ] AC3: Host commands never read the per-user profile or credential store (ADR-017 C9). A test proves it with elevation simulated and the home directories pointed at a store whose values, if read, would change the result (PR A2, `host init` added in PR B).
- [ ] AC4: A file entry the target OS has no step for in this version (`kind`, or `power` outside Windows) is a configuration error naming the entry and the OS (loader in PR A1, wired in PR A2).
- [ ] AC5: Without a file, the declared state is the per-OS default: Windows `rdp`, Linux `rdp`, macOS `ssh`, each on its kind's default port (PR A2).
- [ ] AC6: A declared `control_url` is the descriptor's control plane in `setup` and `check` output; an empty one is a configuration error (validation in PR A1, descriptor in PR A2).
- [ ] AC7: `host init` requires `--config`, writes the YAML (never `.env`) with only the keys the target OS accepts, and the written file loads back unchanged. On macOS, until #459, it prints "not applicable", exits 0 and writes nothing (PR B).
- [ ] AC8: `host init` refuses to overwrite an existing file without confirmation or `--force`, and refuses to write when an existing path cannot be read, even with `--force` (#408 second site) (PR B).

## References

- Bitácora board: `mlorentedev/ts-bridge#458` (epic #450)
- Related ADR: `docs/adr/adr-017-host-desired-state.md` (§1 declared state, §5 OS boundaries, C9, implementation plan step 2)
- Gap inventory: `docs/design/host-client-parity.md` (G3, G5)
- Prior spec: `specs/archive/HOST-004-host-step-model/` (step/outcome model this builds on)
- Lesson: `docs/lessons/lesson-040-2026-10-20.md` (OS as an input)
- Related issues: #408 (unreadable file must not be rewritten), #399 (`.env` retirement), #453, #454, #455, #459, #521
