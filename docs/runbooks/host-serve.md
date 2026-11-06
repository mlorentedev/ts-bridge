---
id: "ts-bridge-host-serve"
type: runbook
status: active
created: "2026-11-02"
tags: [operations, host, headscale, windows, linux, systemd]
---

# Host Serve: a Persistent Node Behind a Filtered Control Plane

`ts-bridge host serve` joins the tailnet as a persistent node and publishes
exactly the services the host file declares, each forwarded to
`127.0.0.1:<port>` on the host. Use it on a host whose network sinkholes or
TLS-inspects a self-hosted control plane, where native Tailscale cannot enrol
(lesson-014). Elsewhere, native Tailscale on the host stays the simpler option.

Serve runs in the foreground, like `connect`. ts-bridge installs no service and
supervises nothing (ADR-017 Amendment A): the OS starts and restarts it, and the
recipes below register it.

## Requirements

- A tagged, single-use auth key from the control plane. Serve refuses a node
  that comes up without tags (`ERROR reason=node_not_tagged`), so a host never
  becomes a user's node.
- For `bootstrap_ssh`: a restricted SSH identity on the endpoint, allowed to
  forward only to the control plane (mlorentedev/kubelab#2862), with its key and
  `known_hosts` readable by the account serve runs as. See
  [SSH Control Bootstrap](guide-control-bootstrap.md).
- Windows and Linux. macOS refuses `host.mesh` until #459.
- SSH and RDP must already be provisioned and listening locally. Serve can publish
  both on Windows and Linux, but does not install or configure either service.
  `host setup` and `host check` retain their stricter OS-step contract and reject
  SSH declarations until #521 adds those convergence steps. `host init` still
  generates an RDP-only file.

## 1. Write the host file

```yaml
# Windows: C:\ProgramData\ts-bridge\host.yaml   Linux: /etc/ts-bridge/host.yaml
version: 1
host:
  control_url: https://headscale.example.com
  services:
    - kind: rdp
      port: 3389
    - kind: ssh
      port: 22
  mesh:
    hostname: lab-pc-01
    state_dir: 'C:\ProgramData\ts-bridge\serve'      # /var/lib/ts-bridge/serve on Linux
    auth_key_file: 'C:\ProgramData\ts-bridge\authkey' # /etc/ts-bridge/authkey on Linux
    bootstrap_ssh: tsb-host@jump.example.com           # optional
    health_addr: 127.0.0.1:9099                        # optional, loopback only
```

Serve requires `control_url`, `host.mesh` and `state_dir`; `auth_key_file` only
for the first enrolment. `hostname` defaults to the OS hostname in lower case.
`bootstrap_socks_addr` defaults to `127.0.0.1:1055`; set another loopback port if
a bootstrap-enabled `connect` runs on the same host.

## 2. Protect the state directory and the key

Serve refuses, before it starts, a `state_dir` or an existing `auth_key_file` that
another account may access. The key file is checked before it is read on the first
enrolment, and on every later start while it still exists, since a leftover key is a
live credential; once deleted, it is no longer checked:

| | `state_dir` | `auth_key_file` |
|---|---|---|
| Windows | Owned by SYSTEM or Administrators; no other SID in the DACL (inherit-only ACEs included); no link or junction on the path | Owned by SYSTEM or Administrators; no other SID in the DACL |
| Linux | Directory, mode `0700`, owned by root; every existing ancestor a root-owned directory no group or other may write, or a root-owned symlink whose target passes the same check; an ancestor whose POSIX ACL gives a named user or group other than root write is refused too, naming the ancestor and the principal | Regular file owned by root, no group or other bits (`0600` or `0400`); every existing ancestor passes the same check as for `state_dir` |

So a Linux `state_dir` or `auth_key_file` under `/tmp` or `/var/tmp` is refused: the sticky world-writable
ancestor would let another user rename the key store away or swap the key file. The refusal names the
ancestor. Any named user or group entry other than root's that carries write is refused, even one the ACL mask currently hides (`setfacl -n`): the next `chmod g+w` would make it effective. An ACL that cannot be read, or that is malformed, is refused. On macOS only the mode bits are checked: an extended ACL is readable only through libc's `acl_get_file`, which a pure-Go build does not reach, so an ancestor that carries one (`ls -le`) is not seen; check it yourself. An absent `state_dir` is created protected, after its ancestors pass. An existing one is checked, never
repaired. Protect the key file yourself:

```powershell
# Windows, elevated
icacls C:\ProgramData\ts-bridge\authkey /inheritance:r /grant:r "SYSTEM:F" "Administrators:F"
```

```sh
# Linux
sudo install -m 0600 -o root -g root authkey /etc/ts-bridge/authkey
```

## 3. Register serve with the OS

### Windows: Scheduled Task as SYSTEM

```powershell
# Elevated PowerShell
$action   = New-ScheduledTaskAction -Execute 'C:\Program Files\ts-bridge\ts-bridge.exe' `
              -Argument 'host serve --config C:\ProgramData\ts-bridge\host.yaml'
$trigger  = New-ScheduledTaskTrigger -AtStartup
$settings = New-ScheduledTaskSettingsSet -RestartCount 999 -RestartInterval (New-TimeSpan -Minutes 1) `
              -ExecutionTimeLimit ([TimeSpan]::Zero) -StartWhenAvailable
Register-ScheduledTask -TaskName 'ts-bridge host serve' -Action $action -Trigger $trigger `
  -Settings $settings -User 'SYSTEM' -RunLevel Highest -Force
Start-ScheduledTask -TaskName 'ts-bridge host serve'
```

`-Force` makes a second run replace the task with the same definition.

### Linux: systemd unit

```ini
# /etc/systemd/system/ts-bridge-serve.service
[Unit]
Description=ts-bridge host serve
Wants=network-online.target
After=network-online.target

[Service]
ExecStart=/usr/local/bin/ts-bridge host serve --config /etc/ts-bridge/host.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now ts-bridge-serve.service
```

## 4. Confirm, then delete the key

The first start prints `READY node=lab-pc-01 services=rdp:3389,ssh:22 enrolled=true`.
`enrolled=true` means this start used the auth key, so the provisioning script
should delete the key file now; serve never deletes it. Later starts reuse the
identity in `state_dir` and print `enrolled=false`.

With `health_addr` declared:

```sh
curl -s http://127.0.0.1:9099/health/ready
```

The response lists each declared service and its `published` diagnostic. For an
RDP-only file, `host check --json --config <path>` can read the same diagnostic.
It still rejects a combined SSH/RDP file until #521 supplies the SSH OS steps;
read serve's health endpoint directly for that file instead. Do not remove SSH
from the file and count the RDP-only result as combined-service acceptance.

`published` is a diagnostic: it trusts a responder that names the file's node,
which proves the contract, not the process. The tailnet ACL governs who can
reach the published ports.

## Troubleshooting

| Signal | Meaning | Action |
|---|---|---|
| `ERROR reason=ssh_bootstrap_failed` at start | The SSH endpoint is unreachable or refused the identity | Serve exits 1 and the OS restarts it. Run `ssh -v` as the serve account against the endpoint |
| `ERROR reason=ssh_bootstrap_failed` after `READY` | The SSH child died | Serve retries with backoff (1-30 s) and stays up; `/health/ready` reads not ready meanwhile |
| `/health/ready` `reason=control_plane_unreachable` | The node lost its control session | tsnet reconnects on its own; check the control plane and the bootstrap |
| `ERROR reason=node_not_tagged` | The auth key carries no tag | Issue a tagged key, empty `state_dir`, start again |
| `ERROR reason=publish_failed` | A declared port could not listen on the node | Look for a duplicate port or another listener on the node |
| `state_dir ...` or `auth_key_file ...` refusal | Permissions too open, wrong owner, or a link | Fix as in step 2; serve never repairs them |

## References

- ADR-017 Amendment A (`docs/adr/adr-017-host-desired-state.md`), HOST-008 (#522)
- [SSH Control Bootstrap](guide-control-bootstrap.md), CLI-363
