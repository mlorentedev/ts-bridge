---
id: "ts-bridge-profile-connections"
type: runbook
status: draft
tags: [runbook, profiles, windows, git-bash, ssh]
created: "2026-10-15"
owner: manu
---

# Daily Foreground Profile Connections

Requires a build containing [ADR-016](../adr/adr-016-self-contained-profile-connections.md)
and #441. Older installed binaries do not support this runtime import. The
[feature verification record](../../specs/CFG-005-self-contained-connections/verification.md)
records successful Windows/Git Bash candidate acceptance on 2026-10-20.
This runbook remains draft while #442 awaits final review and delivery.

## One-Time Configuration

Start with an existing profile and managed credential. Do not enter a new
Headscale key or an application token just to configure runtime settings.

Put supported, non-secret settings in a local YAML file:

```yaml
version: 1
bootstrap_ssh: work-bastion
bootstrap_socks_addr: 127.0.0.1:1056
hostname: work-bridge
state_dir: C:\Users\user\AppData\Local\ts-bridge\state\work
local_addr: 127.0.0.1:34443
health_addr: 127.0.0.1:9081
```

Use the actual SSH alias, paths and free loopback addresses for this profile.
Keep different profiles' hostnames, state directories and listening addresses
distinct. The bootstrap listener must not overlap the bridge or health listener.
The existing custom control URL is required for Headscale SSH bootstrap.

Import once, with the intended binary:

```powershell
& 'C:\Tools\ts-bridge\ts-bridge.exe' init `
  --profile work --config "$HOME\.ts-bridge\work.yml" --force
```

`target` and `control_url` may also be supplied in the input. Omitted settings,
the existing target and the managed credential association are preserved.
Empty values also preserve existing settings; this import cannot clear a runtime
field. An explicit clearing operation is follow-up work tracked under #399.
Repeating this command with the same input is idempotent. `--force` authorizes
updating this profile, not replacing its credential or changing another profile.
The command does not generate a `.env` sidecar in profile mode.

Only this profile schema is imported: unknown settings, unsupported versions,
additional YAML documents, auth keys and credential paths/references are errors,
not silently discarded. Runtime settings stay local; `tsb://` descriptors do not
export them.

## Daily Use

In the intended Windows/Git Bash terminal:

```bash
ts-bridge connect --profile work
```

No launcher or daily `--config` argument is needed. If the executable is not on
PATH, invoke its existing absolute path instead; the feature does not modify
PATH or install a binary. Check which binary and SSH client the shell selects:

```bash
command -v ts-bridge
command -v ssh
```

The process stays in the foreground. Complete any SSH host-key or passphrase
prompt in that terminal; verify unfamiliar host fingerprints through a trusted
channel. Press Ctrl+C in the owning terminal to close that connection.

Precedence remains **flags > environment > YAML > profile > defaults**. Existing
process variables and a CWD `.env` can still override profile settings. Use a
terminal/directory without unrelated overrides when verifying profile-only use;
do not clear another profile's environment globally.

## SSH Authentication Diagnostics

Use the same shell and OpenSSH client selected by `connect`. Git for Windows
OpenSSH and Windows OpenSSH can use different configuration and agents.

```bash
ssh -G work-bastion
ssh -v work-bastion
ssh-add -l
```

`ssh -G` inspects effective configuration without connecting. `ssh -v` connects
interactively and helps distinguish identity, permission, host-key and network
failures. Check the alias's `IdentityFile` or keys available to that client's
agent. Use a matching `ssh-add` when diagnosing an agent.

A missing agent, an empty agent and an unavailable identity are different
conditions. An identity file can authenticate interactively without an agent.
ts-bridge does not start agents, enable Windows services, load keys, capture
passphrases or bypass host-key verification. Never start a fresh agent on every
connection merely to satisfy this command.

Startup errors identify the selected SSH executable. If it cannot be found,
install OpenSSH through the workstation's normal tooling or correct that shell's
PATH. If it starts but fails, inspect its terminal message and retry the selected
client interactively.

## Duplicate Ownership and Listener Conflicts

A second `connect` or `browser` selecting the same `--profile` name fails before
initializing logging or another runtime/SSH child. Close the owning profile's
foreground terminal normally before retrying. Different profile names have
independent ownership locks; they still need non-conflicting runtime settings.
Invocations without `--profile`, configured only through flags, environment or
YAML, do not acquire a named-profile lock; ordinary listener conflicts still apply.

Persistent files under the profile store's `locks` directory are normal. Their
presence does not indicate a running process: the operating system owns the lock
and releases it on process exit. Do not delete lock files to bypass ownership.

An occupied listener is an error, not evidence that the desired bridge is ready
or that the process belongs to this profile. Do not kill a process by name or
reuse an unidentified listener. Abrupt parent termination can still leave an SSH
child holding its port (#376); identify its owner and resolve that specific
connection, or configure distinct addresses.

## Verify the Destination, Not Just Startup

`READY`, the bridge's health endpoint and `status` output do not prove that the
application is reachable. For a Gitea HTTPS target, probe its unauthenticated
health endpoint through the actual local address printed by the bridge:

```powershell
curl.exe --noproxy "*" --fail --show-error --connect-timeout 5 --max-time 15 `
  --connect-to "forge.example.internal:443:127.0.0.1:34443" `
  "https://forge.example.internal/api/healthz"
```

Require an HTTP success and Gitea JSON with `"status":"pass"`. Keep the canonical
service hostname in the URL: `--connect-to` changes only the TCP destination,
preserving HTTPS certificate verification, SNI and Host. Do not use `-k`, an
application token, hosts/DNS edits or a global proxy change.

For live acceptance, also attempt a duplicate from a second terminal and verify
that it fails before startup. Compare unrelated profile settings and any
previously running unrelated processes before and after configuration/connection.
Record those observations separately: a successful curl alone does not prove
ownership or isolation.
