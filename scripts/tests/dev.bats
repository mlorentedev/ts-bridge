#!/usr/bin/env bats
# Tests for scripts/dev.sh — local development launcher.
# Mocks the `go` binary so we don't trigger a real Go build.

load test_helper

setup() {
    setup_fake_project_root

    # Mock `go` on PATH so dev.sh's `go build` succeeds without doing work.
    MOCK_BIN="$BATS_TEST_TMPDIR/mock-bin"
    mkdir -p "$MOCK_BIN"
    cat > "$MOCK_BIN/go" <<'EOF'
#!/usr/bin/env bash
# Mock go: ignore args, succeed silently, and if the command is `build -o ts-bridge`
# create the binary file with our mock so the subsequent ./ts-bridge invocation works.
if [[ "${1:-}" == "build" ]]; then
    # Find -o argument and emit a tiny stub binary at that path.
    for ((i = 2; i <= $#; i++)); do
        if [[ "${!i}" == "-o" ]]; then
            ((i++))
            out="${!i}"
            cp "$BATS_TEST_TMPDIR/project/ts-bridge" "$out" 2>/dev/null || true
        fi
    done
fi
exit 0
EOF
    chmod +x "$MOCK_BIN/go"
    export PATH="$MOCK_BIN:$PATH"
    export BATS_TEST_TMPDIR
}

@test "dev.sh: exits 1 when neither .env nor .env.example exists" {
    rm -f "$FAKE_ROOT/.env" "$FAKE_ROOT/.env.example"
    run_script scripts/dev.sh
    [ "$status" -eq 1 ]
    [[ "$output" == *"No .env or .env.example found"* ]]
}

@test "dev.sh: bootstraps .env from .env.example then exits 1 with instructions" {
    rm -f "$FAKE_ROOT/.env"
    [ -f "$FAKE_ROOT/.env.example" ]
    run_script scripts/dev.sh
    [ "$status" -eq 1 ]
    [ -f "$FAKE_ROOT/.env" ]
    [[ "$output" == *"Creating .env from .env.example"* ]]
    [[ "$output" == *"Please edit .env"* ]]
}

@test "dev.sh: with .env present, builds and runs (exits 0 via mock binary)" {
    write_env <<'EOF'
TS_AUTHKEY=tskey-auth-fake
TS_TARGET=100.64.0.1:3389
EOF
    run_script scripts/dev.sh
    [ "$status" -eq 0 ]
    [[ "$output" == *"Building ts-bridge"* ]]
    [[ "$output" == *"Starting ts-bridge"* ]]
}
