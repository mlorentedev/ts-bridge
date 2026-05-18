#!/bin/bash
# Bridge Launcher (Linux/macOS Client)
# Reads configuration from .env file in project root.
# Auto mode is enabled by default for minimal friction.
#
# Usage: ./run.sh [--reset] [--instance NAME]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
ENV_FILE="$PROJECT_ROOT/.env"
STATE_DIR="$PROJECT_ROOT/ts-state"

# Parse args (default: preserve state)
RESET_STATE=false
INSTANCE_NAME=""
while [[ $# -gt 0 ]]; do
    case $1 in
        -r|--reset) RESET_STATE=true; shift ;;
        -i|--instance)
            if [[ -z "${2:-}" ]]; then
                echo "  ERROR: --instance requires a value"
                exit 1
            fi
            INSTANCE_NAME="$2"
            shift 2
            ;;
        *) shift ;;
    esac
done

echo ""
echo "  TAILSCALE BRIDGE (Client)"
echo "  ---------------------------------------"

# Check .env
if [[ ! -f "$ENV_FILE" ]]; then
    echo "  ERROR: .env not found"
    echo "  Run: cp .env.example .env"
    exit 1
fi

# Load .env
set -a
# shellcheck source=/dev/null
source "$ENV_FILE"
set +a

# Validate
if [[ -z "${TS_AUTHKEY:-}" ]]; then
    echo "  ERROR: TS_AUTHKEY not set in .env"
    exit 1
fi

if [[ -z "${TS_TARGET:-}" ]]; then
    echo "  ERROR: TS_TARGET not set in .env"
    exit 1
fi

# Optional instance alias override
if [[ -n "$INSTANCE_NAME" ]]; then
    export TS_INSTANCE_NAME="$INSTANCE_NAME"
fi

# Auto mode default: enabled unless explicitly disabled.
# Case-insensitive truthy parsing keeps parity with run.ps1's
# .ToLowerInvariant() behavior — e.g. TS_AUTO_INSTANCE=True must
# be honored identically across Windows and Linux/macOS.
lc() { printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]'; }

AUTO_INSTANCE=true
if [[ -n "${TS_AUTO_INSTANCE:-}" ]]; then
    case "$(lc "$TS_AUTO_INSTANCE")" in
        1|true|yes|on) AUTO_INSTANCE=true ;;
        *) AUTO_INSTANCE=false ;;
    esac
fi
case "$(lc "${TS_MANUAL_MODE:-}")" in
    1|true|yes|on) AUTO_INSTANCE=false ;;
esac

if [[ "$AUTO_INSTANCE" == "true" ]]; then
    if [[ "$RESET_STATE" == "true" ]]; then
        echo "  Reset ignored in auto mode (state is ephemeral)"
    fi
    echo "  Auto instance mode enabled"
    if [[ -n "${TS_INSTANCE_NAME:-}" ]]; then
        echo "  Instance: $TS_INSTANCE_NAME"
    fi
else
    # Handle persistent state directory
    if [[ "$RESET_STATE" == "true" ]] && [[ -d "$STATE_DIR" ]]; then
        rm -rf "$STATE_DIR"
        echo "  State reset (new IP will be allocated)"
    fi

    if [[ -d "$STATE_DIR" ]]; then
        echo "  Reusing existing state"
    else
        mkdir -p "$STATE_DIR"
        echo "  Created new state directory"
    fi
fi

echo "  Target: $TS_TARGET"
echo "  ---------------------------------------"
echo ""

# Launch bridge
cd "$PROJECT_ROOT"

# Determine binary name
BINARY="./ts-bridge"
if [[ "$OSTYPE" == "msys" || "$OSTYPE" == "cygwin" ]]; then
    BINARY="./ts-bridge.exe"
fi

if [[ -f "$BINARY" ]]; then
    "$BINARY"
elif command -v go &> /dev/null && [[ -f "main.go" ]]; then
    echo "  Binary not found, falling back to 'go run'..."
    go run main.go
else
    echo "  ERROR: ts-bridge binary not found and 'go' is not available/main.go missing."
    exit 1
fi
