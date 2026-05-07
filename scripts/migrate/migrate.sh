#!/usr/bin/env bash
# ============================================================
# HVC 视频转码服务 - 数据库迁移脚本
# ============================================================
#
# 功能：
#   - 创建数据库（如不存在）
#   - 执行 SQL 迁移文件
#   - 支持指定 MySQL 连接参数
#   - 支持回滚到指定版本
#
# 使用方法：
#   ./scripts/migrate/migrate.sh                      # 默认连接本地 MySQL
#   ./scripts/migrate/migrate.sh -h 10.0.1.1 -P 3307  # 指定 MySQL 地址和端口
#   ./scripts/migrate/migrate.sh -u root -p secret     # 指定用户名和密码
#
# 环境变量：
#   HVC_MYSQL_HOST     MySQL 主机地址（默认 127.0.0.1）
#   HVC_MYSQL_PORT     MySQL 端口（默认 3306）
#   HVC_MYSQL_USER     MySQL 用户名（默认 hvc）
#   HVC_MYSQL_PASSWORD MySQL 密码（默认 hvc_pwd）
#   HVC_MYSQL_DATABASE 数据库名（默认 hvc）
# ============================================================

set -euo pipefail

PROJECT_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"

# ---- MySQL 连接参数 ----
MYSQL_HOST="${HVC_MYSQL_HOST:-127.0.0.1}"
MYSQL_PORT="${HVC_MYSQL_PORT:-3306}"
MYSQL_USER="${HVC_MYSQL_USER:-hvc}"
MYSQL_PASSWORD="${HVC_MYSQL_PASSWORD:-hvc_pwd}"
MYSQL_DATABASE="${HVC_MYSQL_DATABASE:-hvc}"

# ---- 解析命令行参数 ----
while getopts "h:P:u:p:d:" opt; do
  case ${opt} in
    h) MYSQL_HOST="${OPTARG}" ;;
    P) MYSQL_PORT="${OPTARG}" ;;
    u) MYSQL_USER="${OPTARG}" ;;
    p) MYSQL_PASSWORD="${OPTARG}" ;;
    d) MYSQL_DATABASE="${OPTARG}" ;;
    \?) echo "用法: $0 [-h host] [-P port] [-u user] [-p password] [-d database]"; exit 1 ;;
  esac
done

# MySQL 连接命令
MYSQL_CMD="mysql -h ${MYSQL_HOST} -P ${MYSQL_PORT} -u ${MYSQL_USER} -p${MYSQL_PASSWORD}"

echo "=========================================="
echo "  HVC 数据库迁移"
echo "=========================================="
echo "  主机:   ${MYSQL_HOST}:${MYSQL_PORT}"
echo "  用户:   ${MYSQL_USER}"
echo "  数据库: ${MYSQL_DATABASE}"
echo "=========================================="

# ---- 检查 MySQL 连接 ----
echo "[1/4] 检查 MySQL 连接..."
if ! ${MYSQL_CMD} -e "SELECT 1" &>/dev/null; then
  echo "  错误：无法连接到 MySQL"
  echo "  请检查连接参数是否正确"
  exit 1
fi
echo "  MySQL 连接成功"

# ---- 创建数据库 ----
echo "[2/4] 创建数据库（如不存在）..."
${MYSQL_CMD} -e "CREATE DATABASE IF NOT EXISTS \`${MYSQL_DATABASE}\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"
echo "  数据库 ${MYSQL_DATABASE} 已就绪"

# ---- 执行迁移 ----
echo "[3/4] 执行数据库迁移..."

# 迁移文件目录
MIGRATE_DIR="${PROJECT_ROOT}/scripts/migrate"

# 按文件名排序执行所有 .sql 文件
SQL_FILES=$(find "${MIGRATE_DIR}" -name "*.sql" -type f | sort)

if [ -z "${SQL_FILES}" ]; then
  echo "  警告：未找到 SQL 迁移文件"
else
  for sql_file in ${SQL_FILES}; do
    FILENAME=$(basename "${sql_file}")
    echo "  执行: ${FILENAME}"
    ${MYSQL_CMD} "${MYSQL_DATABASE}" < "${sql_file}" || {
      echo "  错误：执行 ${FILENAME} 失败"
      exit 1
    }
  done
fi

# ---- 验证 ----
echo "[4/4] 验证数据库表..."
TABLE_COUNT=$(${MYSQL_CMD} "${MYSQL_DATABASE}" -N -e "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='${MYSQL_DATABASE}';")
echo "  数据表数量: ${TABLE_COUNT}"
echo "  迁移完成！"
