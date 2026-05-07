#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

VERSION="${1:-dev}"
OUTPUT_DIR="$PROJECT_ROOT/dist"
BINARY_NAME="hvc-server"

echo "=== Building HVC Server v${VERSION} ==="

rm -rf "$OUTPUT_DIR"
mkdir -p "$OUTPUT_DIR"

build_target() {
    local os=$1
    local arch=$2
    local output="${BINARY_NAME}-${os}-${arch}"
    if [ "$os" = "windows" ]; then
        output="${output}.exe"
    fi
    echo "Building for ${os}/${arch}..."
    CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" go build \
        -ldflags="-s -w -X main.Version=${VERSION}" \
        -o "$OUTPUT_DIR/$output" \
        "$PROJECT_ROOT/cmd/server"
}

build_target linux amd64
build_target linux arm64
build_target windows amd64
build_target darwin amd64
build_target darwin arm64

echo "=== Build complete ==="
ls -la "$OUTPUT_DIR/"
