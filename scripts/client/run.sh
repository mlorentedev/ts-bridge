#!/bin/bash
# Bridge Launcher (Linux/macOS Client)
# Reads configuration from .env file in project root.
# State is preserved by default to maintain Tailscale IP allocation.
#
# Usage: ./run.sh [--reset]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
ENV_FILE="$PROJECT_ROOT/.env"
STATE_DIR="$PROJECT_ROOT/ts-state"

# Parse args (default: preserve state)
RESET_STATE=false
while [[ $# -gt 0 ]]; do
    case $1 in
        -r|--reset) RESET_STATE=true; shift ;;
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

# Handle state directory
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
