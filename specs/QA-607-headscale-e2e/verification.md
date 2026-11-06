---
tags: [spec, verification]
created: "2026-11-05"
id: "QA-607-headscale-e2e"
type: spec-verification
status: active
owner: manu
---

# Verification - QA-607-headscale-e2e

Nothing is implemented yet: this file records how each criterion will be proved, and is filled in by the PR that proves it. A criterion is checked only with the log path and commit of a run, produced in the session that checks it.

## Evidence

- [x] AC1: `scripts/e2e/run.sh headscale-up` and `headscale-down` twice each, exit 0, no container left -> commit `b574736`, `bash scripts/tests/test-e2e-headscale.sh` exit 0; live: `headscale-up` twice, `headscale-down` twice, all rc=0, `docker ps -aq --filter label=tsb-e2e=headscale` empty, `make e2e-clean` printed `e2e: no session left, no headscale container`
- [ ] AC2 (open; preflight pieces done here): `bash scripts/tests/test-e2e-headscale.sh` covers the premise and guest-reach failure messages; E2 and E3 outcomes below -> commit `b574736`. Calling both from a mesh run, and the test that proves it, is PR 2 task 1; AC2 closes there
- [ ] AC3: `make e2e-mesh-linux` -> `PASS mesh rdp: X.224 confirm through connect`, log `scripts/e2e/out/mesh-linux-<timestamp>.log`
- [ ] AC4: same log -> `PASS mesh ssh: banner through connect`, `PASS mesh ssh: key login through connect --socks5`, `XFAIL #521 mesh ssh: kind ssh is published`
- [ ] AC5: `make e2e-mesh-windows` -> `PASS mesh rdp: X.224 confirm through connect`, log `scripts/e2e/out/mesh-windows-<timestamp>.log`
- [ ] AC6: both logs -> `PASS mesh refuses an undeclared port`
- [ ] AC7: Linux log -> `PASS mesh restart without the key: enrolled=false, one node, same id` and `PASS mesh untagged key: node_not_tagged`
- [ ] AC8: Linux log -> `PASS mesh discover lists the registered nodes`
- [ ] AC9: `bash scripts/tests/test-e2e-report.sh`; `make e2e-clean` twice; `docker ps -aq --filter label=tsb-e2e=headscale` empty; `git grep -nE 'e2e-mesh|scripts/e2e' .github` empty
- [ ] AC10: `docs/runbooks/e2e-vm.md` section, commit `<hash>`

## Test status

- Fixture tests: `bash scripts/tests/test-e2e-headscale.sh` and `bash scripts/tests/test-e2e-report.sh` -> exit 0 and exit 0 at `b574736` (`test-e2e-probe.sh` arrives with PR 2)
- `bash scripts/check-shellcheck.sh` -> exit 0 at `b574736` (22 scripts); `bash scripts/check-features-shape.sh` -> exit 0 (37 files)
- Data-path runs: `make e2e-mesh-linux` and `make e2e-mesh-windows` on the candidate commit -> `<pass, fail, xfail, xpass counts and log paths>`
- Not run in CI by design: GitHub runners cannot nest a VM, and `.github/**` is not touched.

## Experiments

Outcomes are recorded here by the task that resolves each, with the command and its output.

- E1 (PR 1, task 6 by hand; PR 2, task 1 automated): DERP over plain HTTP carries bytes between the host client and the Linux guest -> PASS. Headscale v0.29.7 (digest in `scripts/e2e/headscale.sh`), host IP 10.0.0.185 (en1), client `ts-bridge connect` darwin/arm64 on the host, `host serve` linux/arm64 in the Lima guest `tsb-e2e-linux` with xrdp on 3389. Both processes had `TS_DEBUG_USE_DERP_HTTP=1 TS_DEBUG_ALWAYS_USE_DERP=1`. Guest: `READY node=tsb-e2e-linux services=rdp:3389 enrolled=true`; client: `READY local=127.0.0.1:34389 target=100.64.0.1:3389`. A 11-byte X.224 Connection Request (`0300000b06e00000000000`) through the local listener returned the Connection Confirm `0300000b06d00000123400`; `host serve` logged `connection opened client=100.64.0.2:39797` and `connection closed ... bytes_tx=11 bytes_rx=11`.
  Relay observation: the tsnet and ts-bridge logs print no DERP lines even with `-v`, and Headscale logs no per-connection DERP line at info level. What shows it: `ss -unp` in the guest lists no UDP socket for the process (`ALWAYS_USE_DERP` disables UDP), and `ss -tnp` lists two established TCP connections to `10.0.0.185:8080` (control long-poll and DERP); `lsof` on the host shows the same two for the client. Headscale logged the DERP node it advertises: `HostName:10.0.0.185 DERPPort:8080 InsecureForTests:false`.
  Negative control: the same client run without either variable (state reused; the guest kept both) got `ConnectionResetError` on the same probe. It does not isolate `TS_DEBUG_USE_DERP_HTTP` from `TS_DEBUG_ALWAYS_USE_DERP`.
- E2 (PR 1, task 6): each guest reaches `http://<host-ip>:8080/health` -> Linux PASS. `hs_guest_reaches linux tsb-e2e-linux` exited 0; `curl http://10.0.0.185:8080/health` inside the guest returned `{"status":"pass"}` (guest default route via 192.168.5.2, NAT). Windows: not run (the guest is booked by another session); it stays for PR 4.
- E3 (PR 1, task 4): the pinned module registers `TS_DEBUG_USE_DERP_HTTP` -> PASS. `go list -m -f '{{.Dir}}' tailscale.com` is `tailscale.com@v1.104.4`; `derp/derphttp/derphttp_client.go` contains `RegisterBool("TS_DEBUG_USE_DERP_HTTP")`; `hs_premise_ok` exits 0 on it and non-zero, naming TLS with a local CA, on a fixture without it.
- E4 (PR 4, task 2): the Scheduled Task sees the variables -> `<result>`
- E5 (PR 3, task 1): `host serve` accepts `kind: rdp, port: 22` -> `<result>`
- E6 (PR 4, task 1): `dotf vm windows exec` forwards stdin -> `<result>`

## #183 (QA-013)

After the first full green run, the owner decides which boxes to tick, with the log path as the evidence. Supported by this work: section 4 (both boxes). Not supported, stays manual: the memory-leak soak (section 3 and the acceptance criterion), the 2+ physical devices criterion, latency and throughput.

## Decisions made during implementation

-

## Promotion candidates

Answer each line `yes: <path>`, naming the file you promoted, or `no: <reason>`.

- [ ] Lesson for the repo's `docs/lessons/`? <yes: path / no: reason>. Candidate: a tailscale debug knob is the only way to run DERP over HTTP against a released Headscale, and the harness guards its existence.
- [ ] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? <yes: path / no: reason>. Candidate: none, unless plain-HTTP DERP is ever wanted outside the harness; that is the architect's question.
- [ ] New pattern candidate for `00_meta/patterns/`? <yes: path / no: reason>

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/QA-607-headscale-e2e/` -> `specs/archive/QA-607-headscale-e2e/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
