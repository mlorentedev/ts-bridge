---
tags: [spec, tasks]
created: "2026-11-05"
id: "QA-607-headscale-e2e"
type: spec-tasks
status: active
owner: manu
---

# Tasks - QA-607-headscale-e2e

> TDD order. One task = one focused commit. `[P]` marks a task with no dependency on another unchecked task; `[AC<n>]` ties it to `proposal.md`.
>
> **Cut.** Five PRs, each at most 300 executable lines (non-blank, non-comment lines of shell, Python and PowerShell; fixture tests count too, so the cap is not met by moving code into tests). QA-606 excluded its scenario scripts from the cap as test code; this plan does not rely on that. A PR that grows past 300 is split before review, not argued about.
>
> | PR | Delivers | Depends on | Budget |
> |----|----------|------------|--------|
> | 1 | Headscale substrate, preflights | none | 250 |
> | 2 | Linux enrolment and RDP, the probe | PR 1 | 300 |
> | 3 | Linux SSH, refusals, persistence | PR 2 | 250 |
> | 4 | Windows data path | PR 2 (not PR 3) | 250 |
> | 5 | `discover`, log hygiene, runbook | PR 2; merge last | 200 plus docs |
>
> PR 3 and PR 4 can proceed in parallel once PR 2 has merged. No PR depends on a change that has not merged. No dotfiles change is a dependency (proposal, question 2).
>
> **Common to every PR.** Branch from `master`; the PR body carries `Part of #607` (the last one carries `Closes #607` only if the owner agrees that the remaining #183 boxes stay manual) and a `## Knowledge` section; shellcheck passes (`bash scripts/check-shellcheck.sh`); `make e2e-clean` leaves nothing; the data-path run is pasted in the PR as the summary lines. Case names are fixed here so that `features.json` can grep them.

## Setup

- [ ] Branch created from master: `docs/issue-607-headscale-e2e-spec` (this spec); one branch per implementation PR, `test/issue-607-<slug>`
- [ ] `proposal.md` is complete and acceptance criteria are testable
- [ ] Experiments E1-E6 are named in `proposal.md` with the task that resolves each; no other open question

## Implementation

### PR 1: Headscale substrate (AC1, AC2, AC9)

Files: `scripts/e2e/headscale.sh` (functions, sourced by `run.sh`), `scripts/e2e/headscale/config.yaml.tmpl`, `scripts/e2e/headscale/policy.hujson`, `scripts/e2e/run.sh` (subcommands, `clean`), `scripts/tests/test-e2e-headscale.sh`.

1. [x] [AC1] Write `scripts/tests/test-e2e-headscale.sh` with `route` and `ipconfig` shims on `PATH`. Cases: `hs_host_ip` returns the address of the default-route interface; with no default route it exits non-zero and the message contains `no default route`. Run `bash scripts/tests/test-e2e-headscale.sh`. Expected: FAIL, `hs_host_ip: command not found`.
2. [x] [AC1] Implement `hs_host_ip` in `scripts/e2e/headscale.sh`. Re-run. Expected: exit 0.
3. [x] [AC1] Extend the test: `hs_render_config <ip>` writes a config whose `server_url` is `http://<ip>:8080`, with `derp.server.enabled: true`, `stun_listen_addr` set, `derp.urls: []`, `magic_dns: false`, no `tls_` key; and a policy that has `tagOwners` for `tag:e2e-host` and `tag:e2e-client` and one ACL from client to host on 22 and 3389. Expected: FAIL. Implement the two templates and the function. Expected: exit 0. The image tag is pinned to the version in the proposal, by digest once PR 1 has pulled it.
4. [x] [AC2] Extend the test: `hs_premise_ok` exits 0 when the module directory it is given registers `TS_DEBUG_USE_DERP_HTTP` in `derp/derphttp/derphttp_client.go`, and exits non-zero naming the TLS-with-local-CA fallback when it does not (a fixture directory without the string). The real directory comes from `go list -m -f '{{.Dir}}' tailscale.com`. Expected: FAIL, then exit 0 after implementing it. This resolves E3.
5. [x] [AC1] [AC2] Write the failing cases for `hs_up` and `hs_down` with a `docker` shim that records its arguments: a second `hs_up` makes no second container, `hs_down` twice exits 0, the container carries the label `tsb-e2e=headscale`, the ports are published on all interfaces, nothing is bind-mounted from outside `scripts/e2e/out/headscale/`. Implement them, with `hs_create_user` (one user, `e2e`, which owns the keys and is the owner in `tagOwners`), `hs_mint_key <tag>` (single-use, one hour, tagged), `hs_mint_api_key` and `hs_nodes_json` as thin wrappers over `docker exec ... headscale`, taking the flag names from the pinned image's `--help` and not from memory. Expected: exit 0.
6. [x] [AC2] Add `hs_guest_reaches <os> <session>`, which runs one `curl` (Linux) or `Invoke-WebRequest` (Windows) of `http://<host-ip>:8080/health` through `dotf vm <os> exec`, with a shim test for the failure message (`guest <os> cannot reach the control plane at <url>`). Then run it once for real: start Headscale, boot each guest, record the result in `verification.md`. This resolves E2. If a guest cannot reach it, stop and hand the topology back to the architect.
   Do the E1 check by hand in the same sitting, so that PR 1 does not merge on an unverified premise: on the Linux guest, copy the candidate binary with `dotf vm linux cp`, write a host file and a tagged key, run `host serve` in the foreground through `dotf vm linux exec --elevated` with both variables set, run `ts-bridge connect` from the host with the same variables, and send an X.224 Connection Request with a Python one-liner. Record the commands and the output in `verification.md`. If no bytes flow, stop: the answer to question 1 changes. The automated case stays in PR 2 task 1.
7. [x] [AC1] [AC9] Wire `headscale-up`, `headscale-down` and the existing `clean` subcommand in `scripts/e2e/run.sh`: `clean` also runs `hs_down`, and the teardown trap of a mesh run calls it (the trap and `hs_premise_ok` in the preflight move to PR 2 task 1: there is no mesh run until then). Run `scripts/e2e/run.sh headscale-up` twice and `headscale-down` twice. Expected: all four exit 0, `docker ps -aq --filter label=tsb-e2e=headscale` empty afterwards.

### PR 2: Linux enrolment and RDP (AC3, AC9)

Files: `scripts/e2e/run.sh` (mesh mode), `scripts/e2e/mesh_linux.sh`, `scripts/e2e/probe.py`, `scripts/tests/test-e2e-probe.sh`, `Makefile` (targets), `scripts/tests/test-e2e-report.sh` (unchanged cases still pass).

1. [x] [AC3] Resolve E1 first, with the smallest scenario that can fail: in `run.sh`, a `mesh linux` mode that builds the client for `darwin/arm64` and the guest binary for `linux/arm64`, runs preflight (`hs_premise_ok`, `hs_host_ip`, `docker`, `python3`), starts Headscale, boots the session, runs `hs_guest_reaches`, and calls `mesh_linux.sh` with the key on stdin. The log is `scripts/e2e/out/mesh-linux-<timestamp>.log`, and `mesh-windows-...` for Windows. The scenario installs the documented unit with `Environment=TS_DEBUG_USE_DERP_HTTP=1` and `TS_DEBUG_ALWAYS_USE_DERP=1`, starts it, and waits for `READY` in the journal. The first case, `mesh enrol: READY enrolled=true`, must pass. Then run the host-side `connect` with the same two variables against the node and record, in `verification.md`, how the relay was observed. Expected today: the case reports `FAIL` until the unit and the control URL are right, then `PASS`. If bytes do not flow, stop: the answer to question 1 changes. Also write `scripts/tests/test-e2e-mesh-preflight.sh`: it runs `run.sh mesh linux` with `hs_premise_ok` failing (a fixture module directory) and, separately, with `hs_guest_reaches` failing (a `dotf` shim), and asserts a non-zero exit, the check's own message, and that no scenario ran. Without it, AC2's preflight could be shim-tested in isolation and never called.
2. [x] [P] [AC3] Write `scripts/tests/test-e2e-probe.sh`: a fake server on an ephemeral port answers an X.224 Connection Request with a Connection Confirm (TPKT version 3, X.224 type `0xD0`); another answers garbage; another closes at once; another never answers. `probe.py x224 <addr>` must exit 0, 1, 1 and 1 within a two-second deadline; `probe.py ssh <addr>` reads a banner that starts with `SSH-2.0-`. Expected: FAIL, `probe.py: No such file`. Implement `scripts/e2e/probe.py` with the standard library only. Expected: exit 0. The probe is Python, not a compiled helper, so that CI builds nothing new.
3. [x] [AC3] In `mesh_linux.sh` and `run.sh`, add the case `mesh rdp: X.224 confirm through connect`: the host starts `ts-bridge connect --control-url http://<host-ip>:8080 --auth-key-file <0600 file> --state-dir <mktemp> --target <guest ip>:3389 --local-addr 127.0.0.1:<port>` in the background with the same two variables, waits for its `READY` line, runs `probe.py x224`, and stops the client. The guest baseline case `mesh rdp: xrdp answers inside the guest` runs first, so a failure points at the mesh and not at xrdp. Run `make e2e-mesh-linux`. Expected: `PASS mesh rdp: X.224 confirm through connect`.
4. [x] [AC3] [AC9] The tagged key is written into the guest from stdin with mode 0600 root and never appears in argv, the log or `set -x` output; `run.sh` disables tracing around it. Add `Makefile` targets `e2e-mesh-linux`, `e2e-mesh-windows` (calls the Windows mode, which is not implemented until PR 4 and refuses with that message) and `e2e-mesh`. Run `make e2e-mesh-linux` twice. Expected: both exit 0, `make e2e-clean` prints `no session left`, no Headscale container. Then interrupt a third run with SIGINT once Headscale is up, and before any `e2e-clean` assert no `tsb-e2e=headscale` container and no `tsb-e2e-*` session: the teardown trap, not the cleanup, must leave nothing.

### PR 3: Linux SSH, refusals, persistence (AC4, AC6, AC7)

Files: `scripts/e2e/mesh_linux.sh`, `scripts/e2e/run.sh` (client helpers), `scripts/e2e/probe.py` (only if a case needs more).

1. [ ] [AC4] Resolve E5 first: a host file with `kind: rdp, port: 22` next to the RDP service. Add the case `mesh ssh: stand-in rdp service on 22 is accepted`. If `host serve` refuses it, stop and record that the SSH cases wait for #521. Expected: `PASS`.
2. [ ] [AC4] Case `mesh ssh: banner through connect` with `probe.py ssh`. Expected: `PASS`.
3. [ ] [AC4] Case `mesh ssh: key login through connect --socks5`: a throwaway ed25519 key pair in a `mktemp` directory, its public half appended to the guest user's `authorized_keys`; the host runs `ts-bridge connect --socks5 127.0.0.1:<port>` (no `--target`, ADR-014), then `ssh -o BatchMode=yes -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null -o ProxyCommand='nc -X 5 -x 127.0.0.1:<port> %h %p' <user>@<guest ip> true`. Expected: exit 0, `PASS`. The host key checking is off only because the guest is a throwaway.
4. [ ] [P] [AC4] Case `mesh ssh: kind ssh is published` marked `XFAIL #521`: a host file with `kind: ssh, port: 22`, `host serve` started, banner through `connect`. The case reports `XFAIL #521` only when `host serve` exits non-zero with the loader message naming the unsupported `ssh` kind. Any other failure is `FAIL`, so an unrelated key or relay error cannot hide behind the marker. Expected today: `XFAIL #521`. When #521 lands the run reports `XPASS #521` and fails until the marker is removed.
5. [ ] [AC6] Case `mesh refuses an undeclared port`: a listener on guest `127.0.0.1:3390` that is not in `host.services`; `connect --target <guest ip>:3390` through the mesh does not complete a TCP session, and `probe.py x224` against it exits non-zero within its deadline. The baseline, `3390 answers inside the guest`, runs first. Expected: `PASS`.
6. [ ] [AC7] Case `mesh restart without the key: enrolled=false, one node, same id`: record the node id from `hs_nodes_json`, remove the key file, `systemctl restart` the unit, wait for `READY`, and assert `enrolled=false`, one node with that name, same id. Expected: `PASS`.
7. [ ] [AC7] Case `mesh untagged key: node_not_tagged`: mint a key without a tag, empty a second `state_dir`, run `host serve` in the foreground with a timeout, and assert `ERROR reason=node_not_tagged` on stderr, no `READY`, exit 1, and no second node in the control plane after removal. Expected: `PASS`.
8. [ ] [P] [AC6] Stretch, drop if the PR passes 250 lines: `host check --json` with `health_addr` reports `published` for the declared service and not for the undeclared one (HOST-008 AC6). Case `mesh check reports published`.

### PR 4: Windows data path (AC5, AC6)

Files: `scripts/e2e/mesh_windows.ps1`, `scripts/e2e/run.sh` (Windows mode).

1. [ ] [AC5] Resolve E6 first: pipe a throwaway string to `dotf vm windows exec ... -- powershell -NoProfile -Command '[Console]::In.ReadToEnd().Length'` and check the length. If it is 0, use the 0600 file through `dotf vm windows cp` and remove it in the trap. Record the result in `verification.md`. Then write the key file with `icacls /inheritance:r /grant:r "SYSTEM:F" "Administrators:F"`, as the runbook does.
2. [ ] [AC5] Resolve E4: register the documented Scheduled Task (`SYSTEM`, at startup, `-Force`) with an action that sets `TS_DEBUG_USE_DERP_HTTP` and `TS_DEBUG_ALWAYS_USE_DERP` before `ts-bridge.exe host serve`, start it, and poll the serve log for `READY`. Case `mesh enrol: READY enrolled=true`. Expected: `PASS`. If the task does not see the variables, switch to a detached `Start-Process` and record why.
3. [ ] [AC5] Case `mesh rdp: X.224 confirm through connect` for TermService: the same host-side client as PR 2 task 3, `probe.py x224`, after the baseline `mesh rdp: TermService answers inside the guest`. Run `make e2e-mesh-windows`. Expected: `PASS`.
4. [ ] [AC6] Case `mesh refuses an undeclared port` on Windows: a listener on `127.0.0.1:3390` inside the guest, not declared. Expected: `PASS`.
5. [ ] [AC9] Run `make e2e-mesh-windows` twice and `make e2e-clean`. Expected: both runs exit 0, no session, no container.

### PR 5: `discover`, log hygiene, runbook (AC8, AC9, AC10)

Files: `scripts/e2e/run.sh`, `scripts/tests/test-e2e-report.sh`, `docs/runbooks/e2e-vm.md`, `docs/runbooks/guide-development-workflow.md` (one line, if needed).

1. [ ] [AC9] Write the failing fixture: `scan_log <file>` fails when the file contains an `hskey-` token, the API key value or the auth key value (the harness passes the values it minted), and passes on a clean log. Expected: FAIL, `scan_log: command not found`. Implement it and call it at the end of every mesh run, after `report()`, so a leak fails the run. Expected: exit 0.
2. [ ] [AC8] Case `mesh discover lists the registered nodes`: `hs_mint_api_key` writes a 0600 file; the host runs `TS_CONTROL_URL=http://<host-ip>:8080 ts-bridge discover --api-key-file <file> --json` and the case asserts the guest node by name. It runs in each mesh scenario against the node that scenario enrolled. Expected: `PASS`.
3. [ ] [AC9] Write the guard as a test: `scripts/tests/test-e2e-report.sh` gains a case that `git grep` finds no reference to `e2e-mesh` or `scripts/e2e` under `.github/`. Expected: exit 0.
4. [ ] [AC10] Extend `docs/runbooks/e2e-vm.md`: the three targets, the preconditions (docker engine publishing on all interfaces, a default route, Python 3), the two environment variables and why they are harness-only and what removes the need, the #183 and HOST-008 mappings from `proposal.md`, what stays manual, and the troubleshooting rows for E2 and E3. Replace the line "Not covered: the tailnet data path" with a pointer. Run `bash scripts/check-lessons.sh` and the repo's markdown checks. Expected: exit 0.
5. [ ] [AC1] [AC3] [AC5] [AC9] Add `e2e-mesh` to the `e2e` aggregate target only after one full green run of both, with the log paths pasted in `verification.md`.

## Closing

- [ ] Every acceptance criterion from `proposal.md` is covered by a case or a fixture test
- [ ] Every acceptance criterion has a matching entry in `features.json` with a non-vacuous verification command
- [ ] shellcheck passes (`bash scripts/check-shellcheck.sh`) and `bash scripts/check-features-shape.sh` exits 0
- [ ] `verification.md` filled in with the run summaries and the E1-E6 outcomes
- [ ] The owner has been told which #183 boxes the first full run supports, and which stay manual
- [ ] Independent adversarial review (`review.md`) before archive, by a reviewer who is not the implementer
- [ ] Each PR opened referencing this spec folder

## Machine-readable features

Sibling `features.json` follows the shape `scripts/check-features-shape.sh` enforces: an array of objects with exactly `id`, `behavior`, `verification`, `state` and `evidence`, all strings. The agent never writes `passing`; only the harness does, after a verification exits 0.
