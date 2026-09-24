---
id: "guide-dual-homed-windows-host"
type: runbook
status: active
tags: [windows, networking, tailscale, rdp, multihoming]
created: "2026-09-24"
owner: manu
---

# Dual-Homed Windows Host: Internet over Wi-Fi, Management over Ethernet

Use this pattern when a Windows target needs:

- a trusted Wi-Fi or other primary uplink for Internet and Tailscale; and
- a direct Ethernet link for low-latency RDP, SSH, or recovery access.

The Ethernet interface is LAN-only. It has an address and subnet route, but no
default gateway or DNS server. This prevents Windows from sending Tailscale
control traffic through the management link or an Internet Connection Sharing
gateway.

## Safety gate

Before changing routes:

1. Connect the primary Internet interface and verify ordinary HTTPS access.
2. Keep a local console or a second remote path available.
3. Record the current configuration:

```powershell
Get-NetIPConfiguration
Get-NetRoute -AddressFamily IPv4 |
    Where-Object DestinationPrefix -EQ '0.0.0.0/0'
```

Changing the interface used by the current RDP or SSH session can disconnect
that session. Reconnect through the unchanged LAN address after the change.

## Configure the LAN-only Ethernet interface

Choose an address from the direct-link subnet. Do not set `DefaultGateway`.

```powershell
$lanInterface = 'Ethernet'
$lanAddress = '192.168.137.2'
$prefixLength = 24

Set-NetIPInterface -InterfaceAlias $lanInterface -AddressFamily IPv4 -Dhcp Disabled
Get-NetIPAddress -InterfaceAlias $lanInterface -AddressFamily IPv4 |
    Where-Object PrefixOrigin -NE 'WellKnown' |
    Remove-NetIPAddress -Confirm:$false
New-NetIPAddress `
    -InterfaceAlias $lanInterface `
    -IPAddress $lanAddress `
    -PrefixLength $prefixLength
Set-NetIPInterface `
    -InterfaceAlias $lanInterface `
    -AddressFamily IPv4 `
    -AutomaticMetric Disabled `
    -InterfaceMetric 50
Set-DnsClientServerAddress -InterfaceAlias $lanInterface -ResetServerAddresses
```

Configure the peer Ethernet interface in the same subnet, for example
`192.168.137.1/24`. Neither side needs a gateway for direct LAN traffic.

## Prefer the Internet interface

The Internet-facing interface must own the default route:

```powershell
$internetInterface = 'Wi-Fi'
Set-NetIPInterface `
    -InterfaceAlias $internetInterface `
    -AddressFamily IPv4 `
    -AutomaticMetric Disabled `
    -InterfaceMetric 10

Get-NetRoute -AddressFamily IPv4 |
    Where-Object DestinationPrefix -EQ '0.0.0.0/0' |
    Sort-Object RouteMetric |
    Format-Table InterfaceAlias, NextHop, RouteMetric
```

The output should show no Ethernet default route.

## Validate both paths

On the target:

```powershell
$ts = "$env:ProgramFiles\Tailscale\tailscale.exe"
Test-NetConnection controlplane.tailscale.com -Port 443
& $ts up --unattended
& $ts status
& $ts ip -4
```

From the directly connected peer:

```powershell
Test-NetConnection <lan-address> -Port <service-port>
mstsc /v:<lan-address>:<service-port> /admin
```

From the ts-bridge client, use a named profile so operators do not need to
remember the mesh hostname or custom service port:

```powershell
.\ts-bridge.exe import <profile> "tsb://<mesh-host>:<service-port>?cp=saas"
.\ts-bridge.exe connect --profile <profile> --auth-key-file <key-file>
```

## TLS inspection caveat

Internet access alone does not prove that Tailscale can authenticate. If
`tailscale status` reports:

```text
fetch control key ... x509: certificate signed by unknown authority
```

the selected uplink is presenting an inspection certificate that the host or
the Tailscale service does not trust. Move the host to a non-inspected network,
or ask IT to install the authorized enterprise CA in the Local Machine trust
store. Never disable TLS certificate validation.

See
[Corporate TLS Inspection Breaks Headscale TCP Passthrough](../lessons/lesson-014-corporate-tls-inspection-breaks-headscale-tcp.md).
