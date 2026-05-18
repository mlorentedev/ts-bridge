#!/usr/bin/env bats
# Tests for scripts/client/run.sh — Linux/macOS launcher.
# Mocks the ts-bridge binary so the launcher's logic is exercised end-to-end
# without spinning up a real Tailscale node.

load test_helper

setup() {
    setup_fake_project_root
}

@test "run.sh: exits 1 when .env is missing" {
    run_script scripts/client/run.sh
    [ "$status" -eq 1 ]
    [[ "$output" == *".env not found"* ]]
}

@test "run.sh: exits 1 when TS_AUTHKEY is missing" {
    write_env <<'EOF'
TS_TARGET=100.64.0.1:3389
EOF
    run_script scripts/client/run.sh
    [ "$status" -eq 1 ]
    [[ "$output" == *"TS_AUTHKEY not set"* ]]
}

@test "run.sh: exits 1 when TS_TARGET is missing" {
    write_env <<'EOF'
TS_AUTHKEY=tskey-auth-fake
EOF
    run_script scripts/client/run.sh
    [ "$status" -eq 1 ]
    [[ "$output" == *"TS_TARGET not set"* ]]
}

@test "run.sh: launches mock binary on happy path" {
    write_env <<'EOF'
TS_AUTHKEY=tskey-auth-fake
TS_TARGET=100.64.0.1:3389
EOF
    run_script scripts/client/run.sh
    [ "$status" -eq 0 ]
    [[ "$output" == *"MOCK_TSBRIDGE_RAN"* ]]
    [[ "$output" == *"TS_AUTHKEY=tskey-auth-fake"* ]]
    [[ "$output" == *"TS_TARGET=100.64.0.1:3389"* ]]
}

@test "run.sh: --instance arg sets TS_INSTANCE_NAME" {
    write_env <<'EOF'
TS_AUTHKEY=tskey-auth-fake
TS_TARGET=100.64.0.1:3389
EOF
    run_script scripts/client/run.sh --instance office-laptop
    [ "$status" -eq 0 ]
    [[ "$output" == *"TS_INSTANCE_NAME=office-laptop"* ]]
}

@test "run.sh: -i short flag works the same as --instance" {
    write_env <<'EOF'
TS_AUTHKEY=tskey-auth-fake
TS_TARGET=100.64.0.1:3389
EOF
    run_script scripts/client/run.sh -i shorty
    [ "$status" -eq 0 ]
    [[ "$output" == *"TS_INSTANCE_NAME=shorty"* ]]
}

@test "run.sh: auto-mode banner is printed by default" {
    write_env <<'EOF'
TS_AUTHKEY=tskey-auth-fake
TS_TARGET=100.64.0.1:3389
EOF
    run_script scripts/client/run.sh
    [ "$status" -eq 0 ]
    [[ "$output" == *"Auto instance mode enabled"* ]]
}

@test "run.sh: TS_MANUAL_MODE disables auto and uses state dir" {
    write_env <<'EOF'
TS_AUTHKEY=tskey-auth-fake
TS_TARGET=100.64.0.1:3389
TS_MANUAL_MODE=true
EOF
    run_script scripts/client/run.sh
    [ "$status" -eq 0 ]
    [[ "$output" != *"Auto instance mode enabled"* ]]
    [[ "$output" == *"state directory"* || "$output" == *"Reusing existing state"* ]]
}
