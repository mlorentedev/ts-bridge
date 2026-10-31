---
id: "CI-514-hygiene-wiring-guard"
type: spec
status: implementing # draft | implementing | verifying | archived
created: "2026-10-31"
issue: "mlorentedev/ts-bridge#514"   # repo#NNN — GitHub issue / Project item that tracks this spec
tags: [spec, proposal]
template_version: "1.0"
---

# CI-514-hygiene-wiring-guard

## Why

<!-- from issue #514: CI-359: conflict-marker wiring check accepts continue-on-error on the guarded steps -->

The conflict-marker gate (#491) blocks a merge only because `Repo hygiene` runs its guard and
fixture suite as plain steps of the required `hygiene` job. A single `continue-on-error: true` on
either step, or on the job, leaves every line in place but stops a failure from failing the job.
The only check of that wiring is CI-491's `features.json` f4, a spec verification command that no
CI step runs, and the CI-491 round-2 review showed that f4 passes that mutation.

## What

`scripts/tests/test-hygiene-wiring.sh` checks the structure of
`.github/workflows/repo-hygiene.yml` and fails, naming the reason, when:

- `continue-on-error` appears anywhere in the workflow (any value, on any step or on the job);
- the `hygiene` job has an `if:` or a `needs:`;
- the conflict-marker guard step (`run: bash scripts/check-conflict-markers.sh`) or its suite
  step (`run: bash scripts/tests/test-conflict-markers.sh`) is missing as a step's own `run:`,
  or carries an `if:`.

The check runs first over committed fixtures (one per way the wiring can break, plus harmless
spellings) and then over the real workflow. `repo-hygiene.yml` runs it as its own step.

## Out of scope

- Editing `.github/workflows/repo-hygiene.yml`. The owner adds the step, and its YAML is in the
  PR body. Until then the check runs only locally.
- Editing the archived CI-491 `features.json` f4. It is a signed record, and the gap is that no
  CI step asserts the wiring, not the wording of f4.
- The wiring of other guards and other workflows. The `if:` rule covers only the guarded steps
  and the job, because the AI-trailer step legitimately carries `if: github.event_name == ...`.

## Risks / open questions

- **Threat model (as in CI-348).** This is a regression lint for workflows that maintainers
  write, not a defence against a workflow written to evade it. Folded into the suite step, the
  check could be disabled together with the suite by one `continue-on-error`. As a separate
  step, disabling the gate takes edits to two steps, one of them named for this check. A
  `continue-on-error` on this check's own step together with one on a guarded step is the
  accepted residual risk.
- **Structure, not spelling (lesson 036).** Comments are stripped, and each step line is keyed
  by its parent, so a `run:` under `env:` does not count. `continue-on-error` is matched as a
  key, never by its value. A CRLF file reads like an LF one.
- **Shells.** The check runs under bash only, like its sibling `test-site-pr-workflow.sh`: it
  checks a YAML file on the CI runner, not shell portability.
- No blocking open questions remain.

## Acceptance criteria

- [ ] The check fails when either guarded step, the `hygiene` job, or any other step sets
      `continue-on-error`, whatever its value.
- [ ] The check fails when a guarded step or the job carries `if:`, when the job carries
      `needs:`, and when a guarded `run:` is missing or nested under another key.
- [ ] The check accepts the current `repo-hygiene.yml`, a comment that names
      `continue-on-error`, and the AI-trailer step's `if:`. A CRLF copy of a red case stays red.
- [ ] Negative probes on the real workflow (`continue-on-error: true` added to each guarded
      step in turn) exit non-zero, and the results are recorded in `verification.md`.

## References

- Bitácora board: mlorentedev/ts-bridge#514
- Origin: CI-491 round-2 review (`specs/archive/CI-491-conflict-marker-gate/review.md`)
- Sibling: `scripts/tests/test-site-pr-workflow.sh` (CI-344); lesson 036
