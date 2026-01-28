#!/bin/bash
# Bridge Launcher (Linux/macOS Client)
# Reads configuration from .env file in project root.
# State is cleared by default on each run.
#
# Usage: ./run.sh [--keep-state]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
ENV_FILE="$PROJECT_ROOT/.env"
STATE_DIR="$PROJECT_ROOT/ts-state"

# Parse args (default: reset state)
KEEP_STATE=false
while [[ $# -gt 0 ]]; do
    case $1 in
        -k|--keep-state) KEEP_STATE=true; shift ;;
        *) shift ;;
    esac
done

echo ""
echo "  TAILSCALE BRIDGE (Client)"
echo "  ─────────────────────────────────────"

# Check Go
if ! command -v go &> /dev/null; then
    echo "  ERROR: Go not found in PATH"
    echo "  Install from: https://go.dev/dl/"
    exit 1
fi

# Check .env
if [[ ! -f "$ENV_FILE" ]]; then
    echo "  ERROR: .env not found"
    echo "  Run: cp .env.example .env"
    exit 1
fi

# Load .env
set -a
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

# Clear state by default
if [[ "$KEEP_STATE" == "false" ]] && [[ -d "$STATE_DIR" ]]; then
    rm -rf "$STATE_DIR"
    echo "  State cleared"
fi

echo "  Target: $TS_TARGET"
echo "  ─────────────────────────────────────"
echo ""

# Launch bridge
cd "$PROJECT_ROOT"
go run main.go
