#!/usr/bin/env bash
# ============================================================
# HVC 视频转码服务 - 开发环境启动脚本
# ============================================================
#
# 功能：
#   - 启动热重载开发服务器（使用 air）
#   - 自动检测并安装 air
#   - 支持自定义配置文件路径
#
# 使用方法：
#   ./scripts/dev/dev.sh                  # 默认启动
#   ./scripts/dev/dev.sh /path/to/config  # 指定配置文件
#
# 前置依赖：
#   - Go 1.22+
#   - air（热重载工具，脚本会自动安装）
#   - FFmpeg（转码核心依赖）
# ============================================================

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
CONFIG_PATH="${1:-${PROJECT_ROOT}/configs/config.yaml}"

echo "=========================================="
echo "  HVC 开发环境启动"
echo "=========================================="
echo "  项目根目录: ${PROJECT_ROOT}"
echo "  配置文件:   ${CONFIG_PATH}"
echo "=========================================="

# ---- 检查 FFmpeg ----
echo "[1/3] 检查 FFmpeg..."
if ! command -v ffmpeg &>/dev/null; then
  echo "  警告：未检测到 FFmpeg，转码功能将不可用"
  echo "  安装方法："
  echo "    Ubuntu/Debian: sudo apt install ffmpeg"
  echo "    CentOS/RHEL:   sudo yum install ffmpeg"
  echo "    macOS:         brew install ffmpeg"
  echo "    Windows:       choco install ffmpeg"
else
  FFMPEG_VERSION=$(ffmpeg -version 2>/dev/null | head -1 | awk '{print $3}')
  echo "  FFmpeg 版本: ${FFMPEG_VERSION}"
fi

# ---- 检查并安装 air ----
echo "[2/3] 检查 air 热重载工具..."
if ! command -v air &>/dev/null; then
  echo "  未检测到 air，正在安装..."
  go install github.com/air-verse/air@latest
  echo "  air 安装完成"
else
  echo "  air 已安装"
fi

# ---- 启动开发服务器 ----
echo "[3/3] 启动热重载开发服务器..."
cd "${PROJECT_ROOT}"

# 设置环境变量
export HVC_CONFIG_PATH="${CONFIG_PATH}"

# 启动 air 热重载
# 仓库根目录已提供 .air.toml，直接指定可避免 air 默认把根目录当作 main 包。
exec air -c "${PROJECT_ROOT}/.air.toml"
