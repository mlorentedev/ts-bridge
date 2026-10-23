---
tags: [spec, verification, templates]
created: "2026-10-22"
---

# Verification - BUG-483-iptables-portless-allow

## Evidence

- [x] AC1 -> `TestIptablesCovers` (`port-less blanket`, `interface`, `source`, `tcp`, `negated loopback interface`, `negated loopback source`, `tcp flags`, `comment`); `TestSetupForLinux/iptables, port-less tailnet allow` (`ok`, no `run`); `TestCheckForLinux/iptables, port-less tailnet allow` (`ok`, `firewall_ok`)
- [x] AC2 -> `TestIptablesCovers` (`port-less loopback interface`, `loopback source`, `loopback destination`, `ipv6 loopback source`, `udp`, `icmp`, `negated tcp`, `state`, `limit`, `recent`, `drop`, and the existing `no destination port` conntrack case); `TestSetupForLinux/iptables, only loopback and established allowed` (one `-A`, `changed`); `TestCheckForLinux/iptables, only loopback and established allowed` (`changed`)
- [x] AC3 -> every pre-existing `TestIptablesCovers` case is unchanged and passes

## Test status

- The new setup and check cases fail against `master`'s `iptablesCovers` (`port-less tailnet allow`, both) and pass with the change.
- `go test -race -count=1 ./...` -> all packages ok
- `GOOS=linux|windows|darwin go vet ./...` -> clean; `golangci-lint run` -> 0 issues
- End to end, `ubuntu:24.04` with `NET_ADMIN` (iptables 1.8.10 nf_tables), `GOOS=linux` binary, no ufw, INPUT holding a `ts-input` jump and `-i lo -j ACCEPT`:
  - plus `-i tailscale0 -j ACCEPT`: two `host setup` runs both report "Firewall allows TCP 3389 (iptables)", and `iptables -S INPUT` holds no `--dport 3389` rule;
  - with the tailnet allow removed: the first run reports "Firewall rule added", the second "Firewall allows", and `-S INPUT` holds exactly one `--dport 3389` rule.
