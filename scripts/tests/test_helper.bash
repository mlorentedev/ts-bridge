#!/usr/bin/env bash
# Shared setup/teardown for BATS tests.
# Stages a fake PROJECT_ROOT under $BATS_TEST_TMPDIR with the script under
# test, a mock binary that records its invocation, and a working .env file.

# shellcheck disable=SC2034  # Variables are consumed by individual .bats files.

setup_fake_project_root() {
    REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
    FAKE_ROOT="$BATS_TEST_TMPDIR/project"
    mkdir -p "$FAKE_ROOT/scripts/client"

    # Copy the launchers we want to exercise.
    cp "$REPO_ROOT/scripts/dev.sh" "$FAKE_ROOT/scripts/dev.sh"
    cp "$REPO_ROOT/.env.example" "$FAKE_ROOT/.env.example"

    # Mock ts-bridge binary: writes its invocation marker so tests can confirm
    # the launcher reached the exec step instead of error-exiting earlier.
    cat > "$FAKE_ROOT/ts-bridge" <<'EOF'
#!/usr/bin/env bash
echo "MOCK_TSBRIDGE_RAN"
echo "TS_AUTHKEY=${TS_AUTHKEY:-unset}"
echo "TS_TARGET=${TS_TARGET:-unset}"
echo "TS_INSTANCE_NAME=${TS_INSTANCE_NAME:-unset}"
exit 0
EOF
    chmod +x "$FAKE_ROOT/ts-bridge"
}

write_env() {
    cat > "$FAKE_ROOT/.env"
}

run_script() {
    cd "$FAKE_ROOT" || return 1
    run bash "$@"
}
