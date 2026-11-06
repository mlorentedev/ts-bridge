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

- [ ] AC1: `scripts/e2e/run.sh headscale-up` and `headscale-down` twice each, exit 0, no container left -> commit `<hash>`, `bash scripts/tests/test-e2e-headscale.sh`
- [ ] AC2: `bash scripts/tests/test-e2e-headscale.sh` (premise and guest-reach failure messages); E2 and E3 outcomes below -> commit `<hash>`
- [ ] AC3: `make e2e-mesh-linux` -> `PASS mesh rdp: X.224 confirm through connect`, log `scripts/e2e/out/mesh-linux-<timestamp>.log`
- [ ] AC4: same log -> `PASS mesh ssh: banner through connect`, `PASS mesh ssh: key login through connect --socks5`, `XFAIL #521 mesh ssh: kind ssh is published`
- [ ] AC5: `make e2e-mesh-windows` -> `PASS mesh rdp: X.224 confirm through connect`, log `scripts/e2e/out/mesh-windows-<timestamp>.log`
- [ ] AC6: both logs -> `PASS mesh refuses an undeclared port`
- [ ] AC7: Linux log -> `PASS mesh restart without the key: enrolled=false, one node, same id` and `PASS mesh untagged key: node_not_tagged`
- [ ] AC8: Linux log -> `PASS mesh discover lists the registered nodes`
- [ ] AC9: `bash scripts/tests/test-e2e-report.sh`; `make e2e-clean` twice; `docker ps -aq --filter label=tsb-e2e=headscale` empty; `git grep -nE 'e2e-mesh|scripts/e2e' .github` empty
- [ ] AC10: `docs/runbooks/e2e-vm.md` section, commit `<hash>`

## Test status

- Fixture tests: `bash scripts/tests/test-e2e-headscale.sh`, `bash scripts/tests/test-e2e-probe.sh`, `bash scripts/tests/test-e2e-report.sh` -> `<exit codes>`
- `bash scripts/check-shellcheck.sh` and `bash scripts/check-features-shape.sh` -> `<exit codes>`
- Data-path runs: `make e2e-mesh-linux` and `make e2e-mesh-windows` on the candidate commit -> `<pass, fail, xfail, xpass counts and log paths>`
- Not run in CI by design: GitHub runners cannot nest a VM, and `.github/**` is not touched.

## Experiments

Outcomes are recorded here by the task that resolves each, with the command and its output.

- E1 (PR 2, task 1): DERP over plain HTTP carries bytes between the host client and the Linux guest -> `<result, how the relay was observed>`
- E2 (PR 1, task 6): each guest reaches `http://<host-ip>:8080/health` -> `<result per guest>`
- E3 (PR 1, task 4): the pinned module registers `TS_DEBUG_USE_DERP_HTTP` -> `<module version, file>`
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
