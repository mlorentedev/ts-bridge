# Shell script tests

BATS-core tests for the launchers under `scripts/`. CI runs these on Linux
runners via the `bats` job in `.github/workflows/ci.yml`.

## Running locally

```sh
# Install bats-core (Ubuntu/Debian)
sudo apt install bats

# Or via npm
npm install -g bats

# Run the suite
bats scripts/tests/
```

On macOS:

```sh
brew install bats-core
bats scripts/tests/
```

## What is tested

| File | Coverage |
|---|---|
| `run.bats` | Exit-1 on missing `.env` / required vars; happy path with mock binary; arg parsing (`--instance`, `-i`); auto vs manual mode banners |
| `dev.bats` | Exit-1 with proper error when no `.env`/`.env.example`; bootstrap path; build+run with mock `go` |

## Mock strategy

`test_helper.bash` stages a fake `PROJECT_ROOT` under `$BATS_TEST_TMPDIR/project`,
copies the scripts under test, and installs a mock `ts-bridge` binary that
prints its env vars and exits 0. `dev.bats` additionally puts a mock `go` on
`PATH` that creates the binary at the requested `-o` path on `build`.

This avoids spinning up a real Tailscale node during CI and keeps tests
deterministic.

## What is NOT tested

- `bootstrap.sh` — setup helper, lower-risk surface.
- `host/setup.ps1` — Windows-only; out of scope for bash tests.
- Cross-platform behavior on Windows — `run.ps1` is exercised manually by
  the operator; static parity audit lives at
  `~/Projects/knowledge/10_projects/ts-bridge/40-runbooks/guide-launcher-parity.md`.
