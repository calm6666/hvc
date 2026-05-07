#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

VERSION="${1:-$(git describe --tags --always --dirty 2>/dev/null || echo 'dev')}"
IMAGE_REGISTRY="${IMAGE_REGISTRY:-registry.example.com/hvc}"
IMAGE_TAG="${IMAGE_REGISTRY}/hvc-server:${VERSION}"

echo "=== Releasing HVC Server v${VERSION} ==="

cd "$PROJECT_ROOT"

echo "Running tests..."
go test ./... -count=1 -timeout 60s

echo "Running go vet..."
go vet ./...

echo "Building Docker image..."
docker build -t "$IMAGE_TAG" -f deployments/docker/Dockerfile .

echo "Pushing image..."
docker push "$IMAGE_TAG"

echo "=== Release complete: ${IMAGE_TAG} ==="
