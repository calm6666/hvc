#!/usr/bin/env bash
# ============================================================
# HVC 视频转码服务 - 构建脚本
# ============================================================
#
# 功能：
#   - 编译 Go 二进制文件
#   - 支持多平台交叉编译（linux/amd64, linux/arm64, darwin/amd64, darwin/arm64, windows/amd64）
#   - 注入版本信息（Git Commit、构建时间、版本号）
#   - 输出目录：./build/
#
# 使用方法：
#   ./scripts/build/build.sh                    # 默认构建当前平台
#   ./scripts/build/build.sh linux amd64        # 交叉编译 Linux amd64
#   ./scripts/build/build.sh linux arm64 v1.0.0 # 指定版本号
# ============================================================

set -euo pipefail

# ---- 默认参数 ----
# 目标操作系统
TARGET_OS="${1:-$(go env GOOS)}"
# 目标架构
TARGET_ARCH="${2:-$(go env GOARCH)}"
# 版本号（优先使用参数，否则从 Git Tag 获取，最后使用 dev）
VERSION="${3:-$(git describe --tags --always 2>/dev/null || echo 'dev')}"
# Git Commit Hash
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo 'unknown')
# 构建时间
BUILD_TIME=$(date -u '+%Y-%m-%dT%H:%M:%SZ')

# ---- 项目路径 ----
PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
# 二进制名称
BINARY_NAME="hvc-server"
# 输出目录
OUTPUT_DIR="${PROJECT_ROOT}/build"

echo "=========================================="
echo "  HVC 构建信息"
echo "=========================================="
echo "  目标系统:   ${TARGET_OS}"
echo "  目标架构:   ${TARGET_ARCH}"
echo "  版本号:     ${VERSION}"
echo "  Git Commit: ${GIT_COMMIT}"
echo "  构建时间:   ${BUILD_TIME}"
echo "  输出目录:   ${OUTPUT_DIR}"
echo "=========================================="

# ---- 创建输出目录 ----
mkdir -p "${OUTPUT_DIR}"

# ---- 编译 ----
echo "[1/3] 编译 ${BINARY_NAME}..."

# 构建标志说明：
#   -s -w           去除调试信息和符号表，减小二进制体积
#   -X main.version 注入版本号
#   -X main.commit  注入 Git Commit
#   -X main.buildTime 注入构建时间
LDFLAGS="-s -w \
  -X main.version=${VERSION} \
  -X main.commit=${GIT_COMMIT} \
  -X main.buildTime=${BUILD_TIME}"

CGO_ENABLED=0 GOOS="${TARGET_OS}" GOARCH="${TARGET_ARCH}" \
  go build \
  -ldflags "${LDFLAGS}" \
  -o "${OUTPUT_DIR}/${BINARY_NAME}" \
  ./cmd/server

echo "[2/3] 编译完成: ${OUTPUT_DIR}/${BINARY_NAME}"

# ---- 验证 ----
echo "[3/3] 验证二进制文件..."
if [ -f "${OUTPUT_DIR}/${BINARY_NAME}" ]; then
  FILE_SIZE=$(du -h "${OUTPUT_DIR}/${BINARY_NAME}" | cut -f1)
  echo "  文件大小: ${FILE_SIZE}"
  echo "  构建成功！"
else
  echo "  错误：二进制文件未生成"
  exit 1
fi
