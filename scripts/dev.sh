#!/bin/bash
# Development script for ts-bridge
# Enables verbose mode and loads environment

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_DIR"

# Check for .env file
if [[ ! -f .env ]]; then
    if [[ -f .env.example ]]; then
        echo "Creating .env from .env.example..."
        cp .env.example .env
        echo ""
        echo "Please edit .env with your Tailscale auth key and target:"
        echo "  TS_AUTHKEY=tskey-auth-..."
        echo "  TS_TARGET=your-host:3389"
        echo ""
        exit 1
    else
        echo "Error: No .env or .env.example found"
        exit 1
    fi
fi

# Load environment
set -a
source .env
set +a

# Enable verbose by default in dev mode
export TS_VERBOSE="${TS_VERBOSE:-true}"

# Build and run
echo "Building ts-bridge..."
go build -ldflags="-X main.version=dev -X main.commit=$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')" -o ts-bridge .

echo "Starting ts-bridge (verbose mode)..."
echo ""
./ts-bridge -v "$@"
