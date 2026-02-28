#!/bin/bash
# Bootstrap minimal .env for ts-bridge client mode.
# Usage:
#   ./scripts/client/bootstrap.sh --authkey tskey-auth-... --target 100.x.x.x:3389 [--instance office-laptop] [--port-range 33389-34388]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
ENV_FILE="$PROJECT_ROOT/.env"
ENV_EXAMPLE="$PROJECT_ROOT/.env.example"

AUTHKEY=""
TARGET=""
INSTANCE=""
PORT_RANGE="33389-34388"

usage() {
    echo "Usage: $0 --authkey <tskey> --target <host:port> [--instance <alias>] [--port-range <start-end>]"
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --authkey)
            AUTHKEY="${2:-}"
            shift 2
            ;;
        --target)
            TARGET="${2:-}"
            shift 2
            ;;
        --instance)
            INSTANCE="${2:-}"
            shift 2
            ;;
        --port-range)
            PORT_RANGE="${2:-}"
            shift 2
            ;;
        -h|--help)
            usage
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            usage
            exit 1
            ;;
    esac
done

if [[ -z "$AUTHKEY" || -z "$TARGET" ]]; then
    echo "ERROR: --authkey and --target are required."
    usage
    exit 1
fi

if [[ ! -f "$ENV_FILE" ]]; then
    if [[ ! -f "$ENV_EXAMPLE" ]]; then
        echo "ERROR: neither .env nor .env.example exist in project root."
        exit 1
    fi
    cp "$ENV_EXAMPLE" "$ENV_FILE"
fi

set_env_var() {
    local key="$1"
    local value="$2"
    if grep -qE "^[[:space:]]*${key}=" "$ENV_FILE"; then
        sed -i.bak -E "s|^[[:space:]]*${key}=.*|${key}=${value}|" "$ENV_FILE"
    else
        printf "\n%s=%s\n" "$key" "$value" >> "$ENV_FILE"
    fi
}

set_env_var "TS_AUTHKEY" "$AUTHKEY"
set_env_var "TS_TARGET" "$TARGET"
set_env_var "TS_AUTO_INSTANCE" "true"
set_env_var "TS_MANUAL_MODE" "false"
set_env_var "TS_PORT_RANGE" "$PORT_RANGE"

if [[ -n "$INSTANCE" ]]; then
    set_env_var "TS_INSTANCE_NAME" "$INSTANCE"
fi

rm -f "${ENV_FILE}.bak"

echo ""
echo "ts-bridge .env configured successfully."
echo "  File: $ENV_FILE"
echo "  Target: $TARGET"
if [[ -n "$INSTANCE" ]]; then
    echo "  Instance: $INSTANCE"
fi
echo ""
echo "Next step:"
if [[ -n "$INSTANCE" ]]; then
    echo "  ./scripts/client/run.sh --instance $INSTANCE"
else
    echo "  ./scripts/client/run.sh"
fi
