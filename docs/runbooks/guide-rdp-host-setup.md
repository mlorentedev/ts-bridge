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
| Linux: `host check` exits 2, `Firewall configuration` is `manual` and says "the iptables INPUT policy is ACCEPT" | An iptables host (no ufw) whose INPUT policy is ACCEPT: every port is open on every interface, so the `tailscale0` rule restricts nothing (#691). `host setup` still writes that rule, but never changes the policy | Enable a host firewall (ufw), or, as root, keep loopback, replies and SSH open first (`iptables -A INPUT -i lo -j ACCEPT`, `iptables -A INPUT -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT`, `iptables -A INPUT -p tcp --dport <your SSH port> -j ACCEPT`), then run `iptables -P INPUT DROP` and persist the set with your distribution's iptables service. Without the first two, local DNS and tailscaled's own control connection stop working. A host filtered by firewalld or a native nftables table reports that firewall by name instead since #739 (rows below). `host check` returns 0 again once the policy is closed |
| Linux: `host check` exits 2, `Firewall configuration` is `manual` and says "the ip6tables INPUT policy is ACCEPT" | An iptables host (no ufw) whose IPv6 INPUT policy is ACCEPT: every port is open on every IPv6 address, so the `tailscale0` rule restricts nothing there (#733). `host setup` never changes the policy | As root, keep loopback, replies, ICMPv6 and SSH open first (`ip6tables -A INPUT -i lo -j ACCEPT`, `ip6tables -A INPUT -m conntrack --ctstate ESTABLISHED,RELATED -j ACCEPT`, `ip6tables -A INPUT -p ipv6-icmp -j ACCEPT`, `ip6tables -A INPUT -p tcp --dport <your SSH port> -j ACCEPT`), then run `ip6tables -P INPUT DROP` and persist the set with your distribution's ip6tables service. Without ICMPv6 the host loses IPv6 neighbor discovery. If IPv6 is disabled, or a firewall ts-bridge does not detect filters the host, the report is a false alarm; firewalld and native nftables are reported by name since #739 |
| Linux: `host check` exits 2 on a ufw host, `Firewall configuration` is `manual` and says "ufw's incoming default is allow" or "ufw's incoming default is allow, or IPV6=no" | An active ufw whose incoming default is `allow` (`ufw status verbose` shows `allow (incoming)`), which leaves both INPUT policies at ACCEPT, or a ufw with `IPV6=no`, which leaves the IPv6 policy as it was, ACCEPT after a boot (#735). The `tailscale0` rule restricts nothing there. `host setup` never changes the default | As root, run `ufw allow <your SSH port>/tcp` first, then `ufw default deny incoming`. For IPv6, run `ufw disable`, set `IPV6=yes` in `/etc/default/ufw`, run `ufw allow <your SSH port>/tcp` (with `IPV6=no` ufw stored only the IPv4 copy of each allow; this adds the IPv6 one, and only does so cleanly while ufw is disabled), then `ufw enable`. Do not run `iptables -P` on a ufw host: ufw rewrites the policy on its next reload. `host check` returns 0 again once both policies are closed |
| Linux: `Firewall configuration` says "firewalld is running, which ts-bridge does not read" (`check`, `unverified`) or "does not write to" (`setup`, `manual`) | firewalld filters inbound traffic, in its own nftables table, so the iptables rules and policies ts-bridge reads do not decide the port, and a rule it appended to iptables would admit nothing (#739). `setup` writes nothing | As root, give `tailscale0` a zone of its own and allow the port there: `firewall-cmd --permanent --new-zone=tailnet` (once), `firewall-cmd --permanent --zone=tailnet --change-interface=tailscale0` (it also moves the interface out of another zone, where `--add-interface` fails with `ZONE_CONFLICT`), `firewall-cmd --permanent --zone=tailnet --add-port=<port>/tcp`, then `firewall-cmd --reload`. `host check` stays `unverified` on this host; confirm the port with `firewall-cmd --zone=tailnet --list-all` |
| Linux: `Firewall configuration` says "the nftables table <family> <name> filters inbound traffic" | A native nftables ruleset with a chain hooked on input, outside iptables-nft's own `ip`/`ip6` tables (`filter`, `nat`, `mangle`, `raw`, `security`), filters inbound traffic that iptables does not read (#739). Debian's stock `/etc/nftables.conf` (`table inet filter`, policy accept, no rules) and LXD's or Incus's tables count too | As root, run the `nft insert rule <family> <name> <chain> iifname "tailscale0" tcp dport <port> accept` the fix names, and add it to your distribution's ruleset (`/etc/nftables.conf` on Debian and Ubuntu) so it survives a reboot. If the table filters nothing, delete it (`nft delete table <family> <name>`) and disable the service that loads it; `host check` then reads iptables again |

## References

- [Tailscale: Access remote desktops using Windows RDP](https://tailscale.com/docs/solutions/access-remote-desktops-using-windows-rdp)
- [Tailscale: Secure a Windows RDP server](https://tailscale.com/kb/1095/secure-rdp-windows)
- [Tailscale: Firewall ports](https://tailscale.com/kb/1082/firewall-ports)
