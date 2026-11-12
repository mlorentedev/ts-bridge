---
tags: [spec, verification, templates]
created: "2026-11-11"
id: "HOST-011-control-liveness"
type: spec-verification
status: active
owner: manu
---

# Verification - HOST-011-control-liveness

## Evidence

- [x] AC1 -> commit `d308e1a` / `TestWatchControlOnline` ("the first online sample after a down is up"), `TestHealth_FoldsControlOnline` (with a bootstrap source down too), and `make e2e-mesh-linux`: `mesh outage: /health/ready reads control_plane_unreachable` (503 after 20 s, budget 40 s) and `mesh outage: ready again after the control plane returns` (200 after 19 s, budget 30 s)
- [x] AC2 -> `TestWatchControlOnline` rows "two offline samples do not flap", "the third offline sample is down", "an online sample resets the count", "a failed status read counts as offline", "a node with no self entry counts as offline". Mutant: `offlineSamplesDown = 1` fails the suite.
- [x] AC3 -> `TestServeHealth_ControlSession` and `TestHealth_FoldsSources` unchanged and passing

## Test status

### 2026-11-11: measurement that settles the signal

Local QA-607 mesh (`E2E_KEEP=1 make e2e-mesh-linux` at `1d9f92f`, Headscale v0.29.7 in colima, Ubuntu 24.04 Lima guest running `host serve` with `health_addr: 127.0.0.1:9090`). A temporary, uncommitted build logged every IPN bus notification (with `NotifyInitialHealthState`) and polled `LocalClient.Status` every 10 s. The guest polled `/health/ready` every 5 s.

| Time (UTC) | Event | Bus state | Health warnings | `Self.Online` | `/health/ready` |
|---|---|---|---|---|---|
| 07:42:53 to 07:43:13 | baseline | `Running` | none | true | 200 `status: ok` |
| 07:43:30 | `docker stop tsb-e2e-headscale` | | | | |
| 07:43:33 | first poll after the stop | `Running` (no notification) | none | **false** | 200 |
| 07:43:30 to 07:47:43 | outage, 4 min | `Running` (no notification) | none | false | **200 throughout** |
| 07:47:59 | `docker start tsb-e2e-headscale` | | | | |
| 07:48:13 | first poll after recovery | `Running` | none | **true** | 200 |

Findings: the bus sends no notification during the outage, so the current watcher cannot see it and `/health/ready` stays 200 (the #565 premise holds). Health warnings stay empty. `Self.Online` drops within 3 s of the stop and recovers 14 s after the restart, so it is the signal.

### 2026-11-11: implementation at `d308e1a`

- `go test -race ./...`, `golangci-lint run` (0 issues), `gosec ./internal/serve/ ./cmd/cli/`, `shellcheck -x scripts/e2e/run.sh scripts/e2e/mesh_health.sh`: clean.
- `make e2e-mesh-linux`: 5 pass, 0 fail. Before this change the same outage left `/health/ready` at 200 for four minutes (the table above), so the outage cases are red without the poller.
- No regressions: yes.

## Decisions made during implementation

- `Self.Online` over health warnings and over netmap age: it is the one value the measurement saw change, and it reads with one LocalAPI call.
- A second readiness source rather than merging into `control`: the bus watcher and the poller stay independent, and the fold already gives one stable reason.

## Promotion candidates

Answer each line `yes: <path>`, naming the file you promoted, or `no: <reason>`. `dotf spec archive` refuses a line left unanswered, a `no` without a reason, and a `yes` whose file does not exist; a `00_meta/` path is looked up in the vault.

- [x] Lesson for the repo's `docs/lessons/`? yes: docs/lessons/lesson-058-2026-11-11.md
- [x] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? no: a second readiness source inside the existing fold, no new contract
- [x] New pattern candidate for `00_meta/patterns/`? Only if this recurs in >1 project. no: seen in one project

## Pull request

PR #719 carries `Closes #565`. The spec review's Minor (the PR was unnamed here, and `tasks.md` keeps its PR box unticked) is answered by this section; `tasks.md` stays as reviewed.

## Round 2 (spec review at 32b10fb, PASS) and what changed after round 1

Round 1 (`review-round1.md`) passed 7ce4b45. Commits after it, answering round-1 F1-F3 and the PR reviewers:

- 23221b8: each `Self.Online` read is bounded by `controlReadTimeout` (5 s) and a timeout counts as offline (`TestWatchControlOnline_BoundsEachRead`, red before the fix); the poller starts whatever the IPN bus does (`TestWatchControl_PollsSelfOnline`); `mesh_health.sh` polls every 1 s and rejects a late answer. Mesh e2e on 23221b8: 5 pass, 0 fail, 503 after 24 s, 200 after 9 s.
- 67da1c1: `scripts/tests/test-e2e-probe.sh` runs `mesh_health.sh` against fake endpoints in the `hygiene` job.
- b26ec4b (after round 2, test only): `TestWatchServeControl_PollsThroughTheEntryPoint` drives `watchServeControl` with a real `*local.Client` on an absent socket; replacing the `watchControl` call with nothing turns it red (checked).

`features.json` f1-f3 evidence still names d308e1a; their commands pass at the head, and the contract set stays as reviewed.

Round-2 dispositions:

| Finding | Class | Disposition |
|---|---|---|
| F1 outer hop `watchServeControl` unpinned | REAL | Applied in b26ec4b |
| F2 `mesh_outage_case` has no shim case | REAL | Ticketed #730 |
| F3 this file described d308e1a | REAL | Applied: this section |
| F4 40 s budget near the worst case | THEORETICAL | Declined: measured 20 s and 24 s; the bound is the AC, and a slow guest shows as a FAIL with the elapsed time, not a silent pass |
| F5 health address duplicated in two e2e files | SPECULATIVE | Ticketed with F2 in #730 |
| F6 `BusWatcher` exported for a seam; `controlPollInterval` blocks `t.Parallel` | THEORETICAL | Declined: source-compatible, and `cmd/cli` runs no parallel tests; revisit if it does |

## Archive checklist

- [x] `proposal.md` frontmatter set to `status: archived`
- [x] Folder moved: `specs/HOST-011-control-liveness/` -> `specs/archive/HOST-011-control-liveness/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [x] Promotions above executed (if any)
