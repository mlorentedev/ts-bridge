---
id: "QA-607-headscale-e2e"
type: spec
status: draft # draft | implementing | verifying | archived
created: "2026-11-05"
issue: "mlorentedev/ts-bridge#607"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal, e2e, headscale, host, qa]
template_version: "1.0"
owner: manu
---

# QA-607-headscale-e2e

## Why

<!-- from issue #607: QA: e2e data path over a local Headscale tailnet (RDP, SSH, host serve) -->

Nothing tests the client-to-host data path end to end. The QA-606 harness converges `host` on real Ubuntu and Windows guests, but no byte crosses a tailnet there, and the last live run of the data path was #183 on 1.18. Since then `host serve` (HOST-008), `connect --socks5` and the SSH bootstrap landed with fakes only. A local Headscale as the control plane lets the harness prove, before a release, that a client reaches a host's RDP and SSH through the mesh, and that `host serve` publishes only what it declares. It also automates the part of #183 (QA-013) that never needed physical hardware.

## What

A second local scenario family beside QA-606, run with `make e2e-mesh-linux`, `make e2e-mesh-windows` and `make e2e-mesh`. Not run in CI, like QA-606. It reuses `scripts/e2e/run.sh` (session lifecycle, `report()` and the `PASS` / `FAIL` / `XFAIL #N` / `XPASS #N` protocol) and changes no product code.

### Topology

```text
macOS host (Apple Silicon)
  Headscale container (colima), pinned image, label tsb-e2e=headscale
    server_url http://<host-ip>:8080, embedded DERP on the same port, no TLS
  ts-bridge (darwin/arm64) = the client
    connect --target <guest tailnet ip>:<port>        (RDP, SSH)
    connect --socks5 127.0.0.1:<port>                 (SSH through an ssh ProxyCommand)
    discover against the Headscale API                (#183 section 4)
  scripts/e2e/probe.py = X.224 and SSH-banner probe on the local listener

Lima guest, Ubuntu 24.04       ts-bridge host serve (systemd)        -> xrdp :3389, sshd :22
QEMU guest, Windows 11 Arm64   ts-bridge host serve (Scheduled Task) -> TermService :3389
```

One guest at a time, as in QA-606. Every connection into a guest goes over the tailnet: client on the host, to Headscale's DERP on the host, to the guest's `host serve`, to `127.0.0.1:<port>` inside the guest. A guest only ever needs outbound TCP to the host, which user-mode NAT already gives.

The host is a Mac, so the container runs in colima's VM. That is still local: nothing here touches `.github/**`, and no mesh target is wired into CI.

### The three open questions

| # | Question | Answer | Status |
|---|----------|--------|--------|
| 1 | Does Headscale's embedded DERP work over plain `http://` with tsnet? | The control connection does. The DERP connection does not with a stock client; it does with `TS_DEBUG_USE_DERP_HTTP=1` in the environment of every tsnet process. TLS with a local CA is the rejected alternative. | Source verified; the end-to-end run is experiment E1 |
| 2 | Does `dotf vm` need a `hostfwd` for the Windows guest? | No. | Verified from the dotfiles source |
| 3 | Where do the guests reach the control plane? | At the host's default-route IPv4 address, not at `host.lima.internal` or `10.0.2.2`. | Name choice verified; guest reachability is experiment E2 |

**Question 1, the evidence**

- The control plane over http works. `tailscale.com@v1.104.4/control/ts2021/client.go:126-155`: an `http` URL with a private or loopback host and an explicit port leaves the HTTPS port empty, so tsnet speaks Noise over plain HTTP and never tries 443. ts-bridge accepts such a URL: `internal/controlurl` validates `http://hs.lan` (`TestValidate`, case "plain http"), and ADR-005 sends `ControlURL` to tsnet unchanged.
- Headscale advertises its embedded DERP from `server_url`. In Headscale v0.29.7, the latest release (2026-10-30), `GenerateRegion` in `hscontrol/derp/server/derp_server.go` sets the node's `HostName` and `DERPPort` from `server_url` and leaves `InsecureForTests` false. The config accepts an `http://` `server_url`, and `DERPHandler` serves the `Upgrade: DERP` request over plain HTTP (`servePlain`). `derp.server.stun_listen_addr` is required once the embedded server is enabled.
- A stock tsnet client then uses TLS for that node. `useHTTPS()` in `derp/derphttp/derphttp_client.go:261-269` returns true unless the client was built with an `http` URL or `TS_DEBUG_USE_DERP_HTTP` is set, and `urlString` (`:280-296`) and `dialNode` (`:797-801`) build `https://<HostName>:<DERPPort>/derp`. Against a plain-HTTP listener the TLS handshake fails. `tailcfg.DERPNode` has no field that turns TLS off: `InsecureForTests` only skips certificate verification.
- The knob exists in the pinned version. `TS_DEBUG_USE_DERP_HTTP` is registered in `derp/derphttp/derphttp_client.go:252` and `wgengine/magicsock/debugknobs.go:42`, outside the `ts_omit_debug` tag (the stub file is for `ios` and `js` only). With it set, the client dials `http://<HostName>:<DERPPort>/derp`. `envknob.RegisterBool` (`envknob/envknob.go:165-176`) reads the variable once, when the package-level `var` initialises, so it must be in the process environment at start: an inline `env` for the Linux unit, a wrapper in the Windows task action.
- A throwaway-certificate option does not exist in the release. `HEADSCALE_DEBUG_INSECURE_TLS_LISTEN_ADDR` (a self-signed TLS listener advertised with `InsecureForTests`) is on Headscale `main` only; it matched nothing in the v0.29.7 file. Using it would pin an unreleased image.
- Rejected: TLS with a local CA. It needs the CA in the macOS, Ubuntu (`update-ca-certificates`) and Windows (`certutil`) trust stores, certificate SANs for an IP, and the control URL moved to `https`. It tests more of production, at the cost of three trust-store writes per run. It is the fallback if a tailscale upgrade removes the knob (E3).
- If the product ever needs plain-HTTP DERP outside this harness, that is an architecture question for an ADR. This spec does not settle it: the variable is harness-only and no ts-bridge code reads it.

**Question 2, the evidence**

- `dotf vm windows` starts QEMU with `-netdev user,id=net0` and no `hostfwd` (`dotfiles/cli/internal/vm/qemu.go:80`), and reaches the guest only through the QEMU guest agent; its build check asserts that no sshd is installed (`vm/build.go`, check boot). The Linux guest is a Lima `vz` guest in plain mode.
- Nothing in this plan connects from the host to a guest address. The client reaches the guest through the tailnet, and the baseline probe ("the port answers inside the guest before any mesh") runs inside the guest through `exec`. So there is no dependency on a dotfiles change. A `hostfwd` would live in `qemu.go`; it is named here only so that nobody adds it for this work.
- The real Windows hurdle is different: `host serve` must outlive the `dotf vm windows exec` call that starts it. The plan uses the documented Scheduled Task (SYSTEM, at startup), which also exercises the runbook recipe. E4 confirms the task sees the environment variable.

**Question 3, the evidence**

- The DERP map carries one `HostName` for all clients (the `GenerateRegion` evidence above). `host.lima.internal` (Lima) and `10.0.2.2` (QEMU) are two different names, and the macOS client resolves neither, so neither can be `server_url`.
- The host's default-route IPv4 address is the one address all three parties can use. On this machine the default interface is `en1` (`route -n get default`), not `en0`, so the harness discovers it per run and renders it into the Headscale config. With no default route the preflight stops and names the cause.
- Verified today with a throwaway container: colima publishes a container port on `*:<port>` (an `ssh` process listening on all interfaces), and `curl http://<en1 address>:<port>` from the host answered. Not verified: that each guest can reach the host's own LAN address from inside its NAT (a hairpin). That is E2.

### Cases

Each is one result line in the QA-606 protocol, named `mesh ...`.

- **Linux guest (`scripts/e2e/mesh_linux.sh`):**
  - sshd and xrdp present, a protected `state_dir` and `auth_key_file`, a tagged single-use key delivered on stdin;
  - the documented systemd unit starts `host serve`, which prints `READY ... enrolled=true`;
  - from the host, X.224 Connection Confirm through `connect` to 3389;
  - from the host, the SSH banner through `connect`, and a key-authenticated `ssh ... true` through `connect --socks5`;
  - a port that is listening in the guest but not declared refuses a connection through the mesh (HOST-008 AC3);
  - restart of the unit with the key file removed: `enrolled=false`, and the control plane still lists exactly one node with the same id (HOST-008 AC2);
  - a key without a tag: `ERROR reason=node_not_tagged`, no `READY`, exit 1 (HOST-008 AC7).
- **Windows guest (`scripts/e2e/mesh_windows.ps1`):** the documented Scheduled Task starts `host serve`; X.224 Connection Confirm from TermService through `connect`; an undeclared port refuses.
- **Client side, both runs:** `discover` against the Headscale API lists the registered nodes (#183 section 4).
- **SSH is published as an `rdp` service.** `host serve` publishes only `kind: rdp` today: an `ssh` service is refused and tracked by #521 (HOST-007), which is open. Port validation accepts 1-65535 (`internal/host/file.go:208`), so the harness declares `kind: rdp, port: 22` as a stand-in for the client-side paths (`connect`, `--socks5`) and labels those cases `stand-in`. A case that declares `kind: ssh` is added as `XFAIL #521`, so that it reports `XPASS` and fails the run when #521 lands. Whether the stand-in is accepted is E5.

### What this automates of #183 (QA-013)

| #183 item | Outcome |
|-----------|---------|
| 4. Bridge works with self-hosted Headscale (open) | Automated: the `mesh` cases on both guests |
| 4. Headscale API discovery works (open) | Automated: the `discover` case |
| 1. Basic bridge (4 boxes, ticked) | Repeatable for data flow and close; `/health/*` stays a `connect`-level case outside this spec |
| 2. Resilience (3 boxes, ticked) | Not added. A control-plane restart with `/health/ready` flipping is a later candidate |
| 3. Latency and throughput (ticked) | Stays manual: a DERP on the same machine says nothing about a real mesh |
| 3. and acceptance: no memory leak under sustained load, multi-hour soak (open) | Stays manual and open |
| Acceptance: works on 2+ physical devices (ticked) | Stays manual; VMs do not replace it |

This spec ticks nothing on #183. The owner does, with the log path and commit as the evidence, once the first full run is green.

### HOST-008 acceptance this covers (#522)

AC1 and AC2 (enrol, `enrolled=true` then `false`, one node, same identity), AC3 (undeclared port refused) and AC7 (untagged node refused) run against a real control plane. AC6 (`host check --json` reports `published`, behind `health_addr`) is a stretch task in the Linux scenario. AC4 (enrolment through `bootstrap_ssh` with the control hostname unresolvable) and the live parts of AC1 that need mlorentedev/kubelab#2862 stay out.

## Out of scope

- CI, and anything under `.github/**`, `harness/**`, `.agent/**`, `site/**`, `go.mod`, `go.sum`, `mise.toml` or `AGENTS.md`.
- Any change to `dotf` (the VM tooling lives in the dotfiles repo). None is needed; see question 2.
- `bootstrap_ssh` against a local sshd. It needs a restricted SSH identity (kubelab#2862) and a control hostname the guest cannot resolve, which means DNS manipulation in a guest. Whether plain-HTTP Noise upgrades pass through the loopback proxy of `internal/bootstrapssh` is also untested. A later spec can take it once this substrate exists.
- Real Tailscale SaaS, physical devices, the multi-hour soak, throughput and latency.
- macOS hosts (#459), x64 Windows, and the `kind: ssh` service itself (#521).
- Product changes. If a case finds a bug, it is filed and the case is marked `XFAIL #N`.

## Risks / open questions

Experiments are resolved by the task named; none blocks freezing `tasks.md`.

- **E1, DERP over plain HTTP end to end (PR 2, task 1).** The source says it works; nothing has run it. With `TS_DEBUG_ALWAYS_USE_DERP=1` the client and the guest node exchange bytes. UDP is disabled by that variable, so netcheck's HTTPS latency probe goes to the plain port and fails, and the home region comes from magicsock's fallback to the single region. Evidence required: the bytes flow, and the relay is observable in the tsnet log or Headscale's log.
- **E2, guest hairpin to the host's LAN address (PR 1, task 5).** Both guests must reach `http://<host-ip>:8080/health`. If either cannot, the fallback is a hosts entry in each guest and the client moved into a guest, which is a redesign and goes back to the architect. It becomes a permanent preflight in each mesh scenario.
- **E3, the knob is a debug knob (PR 1, task 4).** A tailscale upgrade can remove it. The preflight checks that the pinned module still registers it and fails with the TLS-with-local-CA plan named, instead of every case timing out.
- **E4, Windows task environment (PR 4, task 2).** A Scheduled Task does not necessarily see a freshly set machine variable. The plan sets the variable in the task action, not in the machine environment; if the guest still shows no DERP over HTTP, the fallback is a detached `Start-Process`.
- **E5, `kind: rdp` on port 22 (PR 3, task 1).** Expected to pass validation. If `host serve` refuses it, the SSH cases fall back to `XFAIL #521` only, and the client-side SSH paths wait for #521.
- **E6, stdin to `dotf vm windows exec` (PR 4, task 1).** The Linux key goes on stdin. If the Windows exec does not forward it, the key travels as a 0600 file in a `mktemp` directory through `dotf vm windows cp`, removed by the trap. The guest is deleted at the end and the key is single-use with a short expiry.
- **Secrets in the log.** The log is a durable artifact. Pre-auth and API keys are minted per run, delivered by stdin or a 0600 file, never in argv, never echoed, and the harness runs with tracing off. AC9 scans the logs for key material.
- **Time and memory.** The mesh scenarios add the Headscale container (colima) to the QA-606 peak. Guests still run one at a time.
- **Colima is a VM.** The harness needs `docker` pointing at a running engine. An engine that publishes on all interfaces works; one that binds loopback only fails E2 and the preflight says so.

## Acceptance criteria

- [ ] AC1: `scripts/e2e/run.sh headscale-up` starts the pinned Headscale with its embedded DERP at the host's default-route address, twice in a row without error, and `headscale-down` removes the container, its state and the key material, also twice in a row. A machine with no default route stops in preflight and names the cause.
- [ ] AC2: the harness refuses to run a mesh scenario unless the pinned `tailscale.com` module still registers `TS_DEBUG_USE_DERP_HTTP`, and unless each guest reaches the control plane's health endpoint; the message names the failing check.
- [ ] AC3: `make e2e-mesh-linux` ends with `PASS mesh rdp: X.224 confirm through connect` for xrdp on the Linux guest, with the client on the host and the traffic over the mesh.
- [ ] AC4: the same run ends with `PASS mesh ssh: banner through connect` and `PASS mesh ssh: key login through connect --socks5`, plus `XFAIL #521 mesh ssh: kind ssh is published`.
- [ ] AC5: `make e2e-mesh-windows` ends with `PASS mesh rdp: X.224 confirm through connect` for TermService on the Windows guest, started by the documented Scheduled Task.
- [ ] AC6: a port that is listening in the guest but not declared in `host.services` refuses a connection through the mesh, on both guests.
- [ ] AC7: after a restart with the key file removed, the Linux `host serve` prints `enrolled=false` and the control plane lists one node with an unchanged id; a node enrolled with an untagged key prints `ERROR reason=node_not_tagged`, no `READY`, and exits 1.
- [ ] AC8: `ts-bridge discover` against the local Headscale API, with a per-run API key, lists the registered guest nodes.
- [ ] AC9: no mesh log under `scripts/e2e/out/` contains key material; a run leaves no Headscale container and no `tsb-e2e-*` session whether it passes, fails or is interrupted; nothing under `.github/` references the mesh targets.
- [ ] AC10: `docs/runbooks/e2e-vm.md` documents the mesh targets, the preconditions, the environment variable and why it is harness-only, the mapping to #183 and HOST-008, and what stays manual.

## References

- Issue: mlorentedev/ts-bridge#607. Parent ticket: #183 (QA-013). Related: #606 (QA-606), #522 (HOST-008), #521 (HOST-007), #459, #451.
- Specs: `specs/QA-606-e2e-vm-harness/` (harness, protocol and counting rule), `specs/HOST-008-host-serve-mode/`.
- ADR-005 (Headscale compatibility), ADR-014 (SOCKS5), ADR-017 and Amendment A (host file, `host serve`).
- Runbooks: `docs/runbooks/host-serve.md`, `docs/runbooks/e2e-vm.md`, `docs/runbooks/guide-control-bootstrap.md`.
- Lessons: lesson-014, lesson-054.
- Sources checked: tailscale.com v1.104.4 from the module cache (`control/ts2021/client.go`, `derp/derphttp/derphttp_client.go`, `wgengine/magicsock/debugknobs.go`, `envknob/envknob.go`, `tailcfg/derpmap.go`); Headscale v0.29.7 and `main` (`hscontrol/derp/server/derp_server.go`, `hscontrol/types/config.go`); dotfiles (`cli/internal/vm/qemu.go`, `linux.yaml`, `build.go`).
- Dependency: none on the dotfiles repo. `dotf` >= 0.79, as QA-606.
