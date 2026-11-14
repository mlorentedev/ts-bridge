---
id: "guide-rdp-host-setup"
type: runbook
status: active
tags: [operations, rdp, windows, host, tailscale]
created: "2026-02-26"
updated: "2026-06-12"
owner: manu
---

# Guide: RDP Host Configuration for ts-bridge

> How to configure a Windows host machine to accept RDP connections over Tailscale from ts-bridge clients.

## Prerequisites

| Requirement | Details |
|-------------|---------|
| Windows Edition | **Pro, Enterprise, Education, or Server**. Home cannot host RDP. |
| Tailscale | Installed and connected to the tailnet |
| Admin rights | For initial setup only |

## Automated Setup (Recommended)

The ts-bridge CLI automates all steps. Run as Administrator:

```powershell
# Configure everything: RDP, firewall, Tailscale, power settings
ts-bridge host setup
# Verify readiness (read-only; exit 0 = ready, 2 = a step is not ok)
ts-bridge host check
```

The `setup` subcommand handles:
1. Tailscale unattended mode (`tailscale set --unattended`, skipped when already on)
2. Tailscale service auto-start
3. UPnP services (SSDP, UPnP Device Host)
4. Network profile set to Private
5. RDP enabled via registry
6. Firewall rule for RDP on TCP port
7. (Optional) Sleep disabled

For manual setup or hosts that cannot run the binary, use the PowerShell script:

```powershell
# Run as Administrator
.\scripts\host\setup.ps1
```

## Manual Steps

If you prefer to configure each component individually:

### 1. Enable Remote Desktop

```powershell
Set-ItemProperty -Path 'HKLM:\System\CurrentControlSet\Control\Terminal Server' `
    -Name "fDenyTSConnections" -Value 0
```

A Group Policy value under
`HKLM:\SOFTWARE\Policies\Microsoft\Windows NT\Terminal Services` overrides the local one, and
the next policy refresh reverts a local write. `host check` reports that case as `manual` and names
the key. Enable RDP in the policy instead: "Allow users to connect remotely by using Remote
Desktop Services".

### 2. Verify RDP Port

```powershell
(Get-ItemProperty -Path 'HKLM:\SYSTEM\CurrentControlSet\Control\Terminal Server\WinStations\RDP-Tcp').PortNumber
# Default: 3389
```

### 3. Firewall — Allow RDP from Tailscale Subnet

```powershell
# Create a rule restricted to the tailnet addresses (what `host setup` writes)
New-NetFirewallRule -DisplayName "Tailscale-RDP-Ingress" `
    -Direction Inbound -Protocol TCP -LocalPort 3389 `
    -RemoteAddress 100.64.0.0/10,fd7a:115c:a1e0::/48 -Action Allow -Profile Any
```

Restricting to the tailnet addresses ensures RDP is only accessible via Tailscale, not the local LAN.
`host setup` no longer enables the built-in `Remote Desktop` rule group: its rules allow any
address and would void that scope. If an earlier setup or an operator enabled the group, disable
it yourself (`Disable-NetFirewallRule -Group "@FirewallAPI.dll,-28752"`, the group's resource id,
which works whatever the display language; `-DisplayGroup "Remote Desktop"` matches nothing on a
non-English install). Both `host check` and
`host setup` report an enabled group as `manual` (and `firewall_ok` stays false) until you do;
`host setup` never disables it. If the group query matches no rule at all, both report
`unverified` once the named rule is in place: Windows ships the group with rules, so an empty answer means the read missed it. While the named rule is missing or drifted the step reports that instead, and the group is read again on the next run. A named rule that allows any address is reported as a legacy rule,
and `host setup` recreates it scoped.

### 4. Tailscale Configuration

```powershell
tailscale set --unattended   # Stay connected without user logged in; keeps a Headscale login server
tailscale ip -4              # Note IP for client .env
```

Disable key expiry for the host in the [admin console](https://login.tailscale.com/admin/machines).

### 5. Network Profile

Tailscale adapter must be "Private" (usually automatic):

```powershell
Get-NetConnectionProfile | Where-Object { $_.InterfaceAlias -match "Tailscale" } |
    Set-NetConnectionProfile -NetworkCategory Private
```

## Authentication

NLA (Network Level Authentication) is on by default — keep it enabled.

### What Works

| Auth Method | Works? | Notes |
|-------------|--------|-------|
| Local account + password | Yes | Simplest approach |
| Microsoft account + password | Yes | Username: `MicrosoftAccount\user@outlook.com` |
| Domain account (AD) | Yes | Requires DC reachable from host |

### What Does NOT Work

| Auth Method | Why | Fix |
|-------------|-----|-----|
| Passwordless Microsoft account | RDP can't use Hello/FIDO/Authenticator | Set traditional password at account.microsoft.com |
| Windows Hello PIN | PIN is device-local, not remote-capable | Use password |
| Blank password | Windows blocks remote login with empty passwords | Set a password |

### CredSSP Gotcha

If client and host have different Windows Update levels, CredSSP "Encryption Oracle Remediation" errors can occur. Patch both machines to the same level.

## Tailscale ACLs

Default ACLs allow all traffic. If custom ACLs are in use, ensure ts-bridge ephemeral nodes can reach RDP:

```json
{
  "action": "accept",
  "src": ["autogroup:members"],
  "dst": ["host-machine:3389"],
  "proto": "tcp"
}
```

If using tagged auth keys (e.g., `tag:bridge`), the source must match the tag.

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| "Home edition doesn't support Remote Desktop" | Windows Home | Upgrade to Pro |
| Connection refused | Firewall blocking | Create explicit rule for `100.64.0.0/10` |
| "CredSSP encryption oracle" | Patch mismatch | Update both machines |
| Connection drops after reboot | Tailscale not a service | `Get-Service Tailscale`, ensure Running |
| "Credentials did not work" | Passwordless account | Set traditional password |
| Third-party AV blocking | AV conflicts with Tailscale WFP | Add Tailscale exception |
| RDP works locally, not over Tailscale | ACLs restricting | Check ACL rules |
| Host disappears from tailnet | Key expired | Disable key expiry in admin console |
| Windows: `host setup` reports `Tailscale unattended mode` as `manual`, saying it "does not persist until the node logs in" (before #740 it reported `changed` on every run) | The node has never logged in, so Tailscale keeps no profile: `tailscale set --unattended` holds only in memory and the next service restart drops it (#740) | Run `tailscale up` (with `--login-server` on Headscale) to log the node in, then `ts-bridge host setup` again, which reports `changed` or `ok` once the node is logged in |
| Windows: `host check` or `host setup` reports `OpenSSH Server capability` as `manual` ("The OpenSSH Server is not installed") on a host that declares `kind: ssh` | The OpenSSH Server Windows capability is `NotPresent` and there is no `sshd` service. ts-bridge installs nothing, and no `sshd` step runs until the server is there (#521). A host that installed OpenSSH by another route (the Win32-OpenSSH MSI) has no capability but has the `sshd` service, and is read as installed | In an elevated PowerShell, run `Add-WindowsCapability -Online -Name OpenSSH.Server~~~~0.0.1.0` (a managed host needs Windows Update, WSUS or a Features-on-Demand source for it), then `ts-bridge host setup --config <the host file that declares ssh>` (without `--config`, setup converges the RDP default instead, ADR-017 §1), which sets `sshd` to Automatic, starts it and writes the tailnet-scoped `Tailscale-SSH-Ingress` rule |
| Windows: `OpenSSH Server capability` is `failed` (setup) or `unverified` (check), saying `Get-WindowsCapability` failed or printed a state that is not `Installed` or `NotPresent` | The read needs an elevated shell, or the capability is `Staged` or pending a restart, or this Windows has no such capability (#521) | Run `ts-bridge host check` from an elevated shell, or `Get-WindowsCapability -Online -Name 'OpenSSH.Server*'` to see the state; finish a pending install and restart |
| Windows: `Firewall rule (ssh)` is `manual` and names `OpenSSH-Server-In-TCP` | The rule the OpenSSH Server installer creates is enabled and allows the declared port from any address, which voids the tailnet scope of `Tailscale-SSH-Ingress` (#521, ADR-017 Amendment B). `host setup` never disables it, since it is Microsoft's own | In an elevated PowerShell, run `Disable-NetFirewallRule -Name OpenSSH-Server-In-TCP`; `firewall_ok` is true once it is off. Keep another way into the host open first if you are connected over SSH |
| Windows: `SSH port` is `manual` ("sshd listens on TCP 22, not the declared 2222") or `unverified` (`sshd -T` could not be read) | sshd's effective configuration, read with `sshd -T`, does not name the declared port; `sshd -T` needs an elevated shell and the host keys, which sshd generates on its first start (#521); a `sshd` service that starts sshd with arguments (`-f` another file, in `sc qc sshd`) is not probed, since `sshd -T` without them reads another configuration. ts-bridge never edits `sshd_config` | Set `Port 2222` in `%ProgramData%\ssh\sshd_config` and run `Restart-Service sshd`, or declare the port sshd listens on. For `unverified`, run `host check` elevated after sshd has started once |
| Linux: `host check` exits 2, `Firewall configuration` is `manual` and says "the iptables INPUT policy is ACCEPT" | An iptables host (no ufw) whose INPUT policy is ACCEPT: every port is open on every interface, so the `tailscale0` rule restricts nothing (#691). `host setup` still writes that rule, but never changes the policy | Enable a host firewall (ufw), or, as root, keep loopback, replies and SSH open first (`iptables -A INPUT -i lo -j ACCEPT`, `iptables -A INPUT -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT`, `iptables -A INPUT -p tcp --dport <your SSH port> -j ACCEPT`), then run `iptables -P INPUT DROP` and persist the set with your distribution's iptables service. Without the first two, local DNS and tailscaled's own control connection stop working. A host filtered by firewalld or a native nftables table reports that firewall by name instead since #739 (rows below). `host check` returns 0 again once the policy is closed |
| Linux: `host check` exits 2, `Firewall configuration` is `manual` and says "the ip6tables INPUT policy is ACCEPT" | An iptables host (no ufw) whose IPv6 INPUT policy is ACCEPT: every port is open on every IPv6 address, so the `tailscale0` rule restricts nothing there (#733). `host setup` never changes the policy | As root, keep loopback, replies, ICMPv6 and SSH open first (`ip6tables -A INPUT -i lo -j ACCEPT`, `ip6tables -A INPUT -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT`, `ip6tables -A INPUT -p ipv6-icmp -j ACCEPT`, `ip6tables -A INPUT -p tcp --dport <your SSH port> -j ACCEPT`), then run `ip6tables -P INPUT DROP` and persist the set with your distribution's ip6tables service. Without ICMPv6 the host loses IPv6 neighbor discovery. If IPv6 is disabled, or a firewall ts-bridge does not detect filters the host, the report is a false alarm; firewalld and native nftables are reported by name since #739 |
| Linux: `host check` exits 2 on a ufw host, `Firewall configuration` is `manual` and says "ufw's incoming default is allow" or "ufw's incoming default is allow, or IPV6=no" | An active ufw whose incoming default is `allow` (`ufw status verbose` shows `allow (incoming)`), which leaves both INPUT policies at ACCEPT, or a ufw with `IPV6=no`, which leaves the IPv6 policy as it was, ACCEPT after a boot (#735). The `tailscale0` rule restricts nothing there. `host setup` never changes the default | As root, run `ufw allow <your SSH port>/tcp` first, then `ufw default deny incoming`. For IPv6, run `ufw disable`, set `IPV6=yes` in `/etc/default/ufw`, run `ufw allow <your SSH port>/tcp` (with `IPV6=no` ufw stored only the IPv4 copy of each allow; this adds the IPv6 one, and only does so cleanly while ufw is disabled), then `ufw enable`. Do not run `iptables -P` on a ufw host: ufw rewrites the policy on its next reload. `host check` returns 0 again once both policies are closed |
| Linux: `Firewall configuration` says "firewalld is running, which ts-bridge does not read" (`check`, `unverified`) or "does not write to" (`setup`, `manual`) | firewalld filters inbound traffic, in its own nftables table, so the iptables rules and policies ts-bridge reads do not decide the port, and a rule it appended to iptables would admit nothing (#739). `setup` writes nothing | As root, give `tailscale0` a zone of its own and allow the port there: `firewall-cmd --permanent --new-zone=tailnet` (once), `firewall-cmd --permanent --zone=tailnet --change-interface=tailscale0` (it also moves the interface out of another zone, where `--add-interface` fails with `ZONE_CONFLICT`), `firewall-cmd --permanent --zone=tailnet --add-port=<port>/tcp`, then `firewall-cmd --reload`. `host check` stays `unverified` on this host; confirm the port with `firewall-cmd --zone=tailnet --list-all` |
| Linux: `Firewall configuration` says "the nftables table <family> <name> filters inbound traffic" | A native nftables ruleset with a chain hooked on input, outside iptables-nft's own `ip`/`ip6` tables (`filter`, `nat`, `mangle`, `raw`, `security`), filters inbound traffic that iptables does not read (#739). Debian's stock `/etc/nftables.conf` (`table inet filter`, policy accept, no rules) and LXD's or Incus's tables count too | As root, run the `nft insert rule <family> <name> <chain> iifname "tailscale0" tcp dport <port> accept` the fix names, and add it to your distribution's ruleset (`/etc/nftables.conf` on Debian and Ubuntu) so it survives a reboot. If the table filters nothing, delete it (`nft delete table <family> <name>`) and disable the service that loads it; `host check` then reads iptables again |
| Linux: `Firewall configuration` is `manual` and says "ip6tables admits TCP <port> on every interface: -A <chain> ..." with a chain other than INPUT | An IPv6 allow for the port with no interface or source scope, inside a chain INPUT jumps to (tailscaled's `ts-input`, or your own): the jump sends every IPv6 packet for the port there, so the allow opens it off the tailnet (#720). The read follows a jump only when it applies to all the port's traffic; ufw's own `ufw6-*` chains are read through ufw. `host setup` never deletes it | As root, run the `ip6tables -D <chain> ...` the fix names, or scope the rule with `-i tailscale0`; `host check` is `ok` once it is gone |
| Linux: `Firewall configuration` is `manual` and names a legacy unscoped rule `-A <chain> ...` with a chain other than INPUT | An IPv4 allow for the port, or a port-less `-j ACCEPT`, with no interface or source scope, inside a chain of your own that INPUT jumps to: the jump sends every packet for the port there, so the allow opens it off the tailnet (#762). The read follows a jump only when it applies to all the port's traffic; tailscaled's `ts-input` and, on a ufw host, ufw's own `ufw-*` chains are read elsewhere. `host setup` never deletes it | As root, run `iptables -D <chain> ...` for the rule, or scope it with `-i tailscale0`. Before you remove a port-less `-j ACCEPT`, or one whose port list holds other ports (`--dports 22,3389`), under a DROP policy, keep loopback, replies and SSH open first (`iptables -A INPUT -i lo -j ACCEPT`, `iptables -A INPUT -m conntrack --ctstate RELATED,ESTABLISHED -j ACCEPT`, `iptables -A INPUT -p tcp --dport <your SSH port> -j ACCEPT`), as the fix says. `host check` is `ok` once it is gone |
| Linux: `Firewall configuration` is `manual` and names an allow written with a negated scope, such as `-A INPUT ! -i tailscale0 -j ACCEPT` or `-A INPUT -p tcp --dport 3389 ! -s 100.64.0.0/10 -j ACCEPT` | A `!` before `-i`, `-s` or `-d` admits every client outside the set it names, the tailnet included unless that set is the tailnet, so it does not scope the allow to `tailscale0` and the port is reachable off it (#743; the same reading applies to a `-s 0.0.0.0/0` or `-d 0.0.0.0/0` allow, which names every address). A state match that admits new connections does not scope an allow either: `-A INPUT -m conntrack ! --ctstate INVALID -j ACCEPT` is reported the same way, on IPv4 and IPv6, with or without a port (#778), while one held to replies (`--ctstate RELATED,ESTABLISHED`, `! --ctstate NEW`) is not (#758). `host setup` still adds the scoped rule and never deletes yours | As root, replace the rule with a positive scope (`-i tailscale0`), or remove it with the `iptables -D ...` for the line named. Before you remove a rule that names no port or lists other ports, under a DROP policy, keep loopback, replies and SSH open first, as the fix says. `host check` is `ok` once it is gone |
| Linux: `Firewall configuration` is `manual` on a ufw host and names a legacy unscoped rule `ufw allow ...` that setup did not write | A ufw allow for the port with no interface or source scope that is not the exact `ufw allow <port>/tcp` setup wrote before #631: `ufw allow <port>`, a port list (`ufw allow 22,3389/tcp`), a range (`ufw allow 3380:3390/tcp`), or a port-less `ufw allow from any`. It opens the port off the tailnet. `host setup` never deletes it (#772) | As root, scope the rule with `on tailscale0`, or delete it. Before you change a rule that lists other ports or names none, under ufw's default deny incoming, re-add them first so SSH stays open: the fix names the other ports of a list it can read (`ufw allow 22,3389/tcp` gives `ufw allow 22/tcp`) and `ufw allow <your SSH port>/tcp` for a range or a port-less rule. A rule that names only the service port carries no caution. `host check` is `ok` once the rule is gone |
| Linux: `SSH server` is `manual` and says "ssh.service and ssh.socket are both enabled" (`host setup` and `host check`; with `sshd` for the unit name on Fedora, RHEL and Arch) | The OpenSSH service and its socket are both enabled, typically a socket-activated Ubuntu host (22.10 or later) that an earlier ts-bridge release converged with `systemctl enable --now ssh` (#777). sshd keeps answering, so this is not an outage, but the host starts sshd outside the socket's activation. ts-bridge never disables a unit, so the step stays `manual` until you choose. If neither unit was running, setup starts `ssh.socket` so sshd is not left down and the message says so; `host check` only reports that sshd is not running | Run `sudo systemctl disable ssh.service` (`sshd.service` on Fedora, RHEL and Arch). It does not touch the running sshd, so an SSH session stays up, and the socket starts sshd from the next boot. To switch immediately, from a console and not over SSH (the listener is down until the socket takes over), run `sudo systemctl disable --now ssh.service && sudo systemctl start ssh.socket`. `host check` returns to `ok` once only the socket is enabled |
| Linux: `Firewall configuration` is `unverified` and says "part of the firewall was not read: iptables -S <chain>: ..." (or `ip6tables -S <chain>`) | `iptables -S` or `ip6tables -S` failed on a chain INPUT jumps to, for a reason other than the chain not existing, so what that chain admits is unknown (#762, #763) | As root, run the same `-S <chain>` command and fix what it reports; then confirm the chain admits the port only on `tailscale0` and run `host check` again |
| Linux: `Firewall configuration` is `unverified` and says "part of the firewall was not read: a jump <chain> makes under a match ts-bridge does not read: -A <chain> ... -j <target>" | A rule jumps to an existing chain under a match ts-bridge does not interpret (`-m mark`, `-m limit`, `-m recent`, `-m addrtype`, `-m set`, ...), so the chain may take every packet for the port and what it admits is unknown (#766); `host setup` still writes its scoped rule, and `firewall_ok` is false. A jump that a match ts-bridge does read narrows (another port or protocol, `-i eth0`, loopback, a source or destination prefix, or replies only such as `--ctstate RELATED,ESTABLISHED`) is not reported, and nor is a jump to a target that is no chain (`LOG`) | Read the target chain as root (`iptables -S <target>`, or `ip6tables -S`) and confirm it admits the port only on `tailscale0` when the match holds, or remove the jump; then run `host check` again |

## References

- [Tailscale: Access remote desktops using Windows RDP](https://tailscale.com/docs/solutions/access-remote-desktops-using-windows-rdp)
- [Tailscale: Secure a Windows RDP server](https://tailscale.com/kb/1095/secure-rdp-windows)
- [Tailscale: Firewall ports](https://tailscale.com/kb/1082/firewall-ports)
