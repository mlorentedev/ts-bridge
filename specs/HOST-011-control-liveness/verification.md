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

- [ ] AC1 -> pending
- [ ] AC2 -> pending
- [ ] AC3 -> pending

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

- Test suite: pending
- No regressions: pending

## Decisions made during implementation

- `Self.Online` over health warnings and over netmap age: it is the one value the measurement saw change, and it reads with one LocalAPI call.
- A second readiness source rather than merging into `control`: the bus watcher and the poller stay independent, and the fold already gives one stable reason.

## Promotion candidates

Answer each line `yes: <path>`, naming the file you promoted, or `no: <reason>`. `dotf spec archive` refuses a line left unanswered, a `no` without a reason, and a `yes` whose file does not exist; a `00_meta/` path is looked up in the vault.

- [ ] Lesson for the repo's `docs/lessons/`? <yes: path / no: reason>
- [ ] ADR-worthy decision for the repo's `docs/adr/adr-XXX.md`? <yes: path / no: reason>
- [ ] New pattern candidate for `00_meta/patterns/`? Only if this recurs in >1 project. <yes: path / no: reason>

## Archive checklist

- [ ] `proposal.md` frontmatter set to `status: archived`
- [ ] Folder moved: `specs/HOST-011-control-liveness/` -> `specs/archive/HOST-011-control-liveness/`
- [ ] Bitácora board ticket for this spec moved to Done / closed with PR link (ADR-018)
- [ ] Promotions above executed (if any)
