#!/usr/bin/env bash
# ============================================================
# HVC 视频转码服务 - 发布脚本
# ============================================================
#
# 功能：
#   - 编译多平台二进制文件
#   - 构建 Docker 镜像
#   - 推送镜像到仓库
#   - 生成发布包（含配置文件和部署脚本）
#   - 创建 Git Tag
#
# 使用方法：
#   ./scripts/release/release.sh v1.0.0              # 发布 v1.0.0
#   ./scripts/release/release.sh v1.0.0 --skip-push   # 不推送镜像
#   ./scripts/release/release.sh v1.0.0 --skip-tag    # 不创建 Git Tag
#
# 前置条件：
#   - Docker 已安装并登录
#   - 如果需要创建 Git Tag，则工作目录应保持干净
# ============================================================

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
VERSION="${1:-}"
SKIP_PUSH=false
SKIP_TAG=false

# ---- 解析参数 ----
for arg in "$@"; do
  case ${arg} in
    --skip-push) SKIP_PUSH=true ;;
    --skip-tag)  SKIP_TAG=true ;;
  esac
done

# ---- 校验版本号 ----
if [ -z "${VERSION}" ]; then
  echo "错误：请指定版本号"
  echo "用法: $0 <version> [--skip-push] [--skip-tag]"
  echo "示例: $0 v1.0.0"
  exit 1
fi

# 版本号必须以 v 开头
if [[ ! "${VERSION}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+ ]]; then
  echo "错误：版本号格式不正确，应为 v1.0.0 格式"
  exit 1
fi

# ---- 检查 Git 工作目录 ----
if [ "${SKIP_TAG}" = false ]; then
  if [ -n "$(git status --porcelain)" ]; then
    echo "错误：Git 工作目录不干净，请先提交或暂存更改"
    exit 1
  fi
fi

GIT_COMMIT=$(git rev-parse --short HEAD)
BUILD_TIME=$(date -u '+%Y-%m-%dT%H:%M:%SZ')

echo "=========================================="
echo "  HVC 发布"
echo "=========================================="
echo "  版本号:     ${VERSION}"
echo "  Git Commit: ${GIT_COMMIT}"
echo "  构建时间:   ${BUILD_TIME}"
echo "  跳过推送:   ${SKIP_PUSH}"
echo "  跳过 Tag:   ${SKIP_TAG}"
echo "=========================================="

# ---- 编译多平台二进制 ----
echo "[1/5] 编译多平台二进制文件..."

LDFLAGS="-s -w \
  -X main.version=${VERSION} \
  -X main.commit=${GIT_COMMIT} \
  -X main.buildTime=${BUILD_TIME}"

OUTPUT_DIR="${PROJECT_ROOT}/release/${VERSION}"
mkdir -p "${OUTPUT_DIR}"

# 编译目标平台列表
PLATFORMS=(
  "linux/amd64"
  "linux/arm64"
  "darwin/amd64"
  "darwin/arm64"
  "windows/amd64"
)

for PLATFORM in "${PLATFORMS[@]}"; do
  IFS='/' read -r GOOS GOARCH <<< "${PLATFORM}"
  BINARY_NAME="hvc-server"
  if [ "${GOOS}" = "windows" ]; then
    BINARY_NAME="hvc-server.exe"
  fi
  mkdir -p "${OUTPUT_DIR}/${GOOS}_${GOARCH}"

  echo "  编译: ${GOOS}/${GOARCH}"
  CGO_ENABLED=0 GOOS="${GOOS}" GOARCH="${GOARCH}" \
    go build \
    -ldflags "${LDFLAGS}" \
    -o "${OUTPUT_DIR}/${GOOS}_${GOARCH}/${BINARY_NAME}" \
    ./cmd
done

echo "  多平台编译完成"

# ---- 构建 Docker 镜像 ----
echo "[2/5] 构建 Docker 镜像..."
DOCKER_REGISTRY="${DOCKER_REGISTRY:-}"
IMAGE_NAME="${DOCKER_REGISTRY}hvc-server:${VERSION}"

docker build \
  -t "${IMAGE_NAME}" \
  -t "${DOCKER_REGISTRY}hvc-server:latest" \
  -f "${PROJECT_ROOT}/deployments/docker/Dockerfile" \
  "${PROJECT_ROOT}"

echo "  Docker 镜像构建完成: ${IMAGE_NAME}"

# ---- 推送 Docker 镜像 ----
if [ "${SKIP_PUSH}" = false ] && [ -n "${DOCKER_REGISTRY}" ]; then
  echo "[3/5] 推送 Docker 镜像..."
  docker push "${IMAGE_NAME}"
  docker push "${DOCKER_REGISTRY}hvc-server:latest"
  echo "  Docker 镜像推送完成"
else
  echo "[3/5] 跳过 Docker 镜像推送"
fi

# ---- 生成发布包 ----
echo "[4/5] 生成发布包..."
RELEASE_DIR="${PROJECT_ROOT}/release/${VERSION}/package"
mkdir -p "${RELEASE_DIR}"

# 复制配置文件和部署脚本
cp -r "${PROJECT_ROOT}/configs" "${RELEASE_DIR}/configs"
cp -r "${PROJECT_ROOT}/deployments" "${RELEASE_DIR}/deployments"
cp -r "${PROJECT_ROOT}/scripts" "${RELEASE_DIR}/scripts"
cp -r "${PROJECT_ROOT}/sql" "${RELEASE_DIR}/sql"
cp "${PROJECT_ROOT}/DEPLOYMENT_GUIDE.md" "${RELEASE_DIR}/DEPLOYMENT_GUIDE.md"

# 打包
cd "${PROJECT_ROOT}/release/${VERSION}"
tar -czf "hvc-server-${VERSION}-linux-amd64.tar.gz" "linux_amd64/" "package/"
tar -czf "hvc-server-${VERSION}-linux-arm64.tar.gz" "linux_arm64/" "package/"
tar -czf "hvc-server-${VERSION}-darwin-amd64.tar.gz" "darwin_amd64/" "package/"
tar -czf "hvc-server-${VERSION}-darwin-arm64.tar.gz" "darwin_arm64/" "package/"
zip -r "hvc-server-${VERSION}-windows-amd64.zip" "windows_amd64/" "package/" >/dev/null

echo "  发布包生成完成"

# ---- 创建 Git Tag ----
if [ "${SKIP_TAG}" = false ]; then
  echo "[5/5] 创建 Git Tag..."
  git tag -a "${VERSION}" -m "Release ${VERSION}"
  echo "  Git Tag 创建完成: ${VERSION}"
else
  echo "[5/5] 跳过 Git Tag 创建"
fi

echo "=========================================="
echo "  发布完成！"
echo "=========================================="
echo "  版本: ${VERSION}"
echo "  镜像: ${IMAGE_NAME}"
echo "  发布包目录: ${PROJECT_ROOT}/release/${VERSION}/"
echo "=========================================="
