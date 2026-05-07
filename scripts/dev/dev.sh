#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

echo "=== Starting HVC Dev Environment ==="

cd "$PROJECT_ROOT"

if [ ! -f "configs/config.yaml" ]; then
    echo "Error: configs/config.yaml not found"
    exit 1
fi

echo "Running go mod tidy..."
go mod tidy

echo "Starting HVC server in dev mode..."
go run ./cmd/server --config configs/config.yaml
