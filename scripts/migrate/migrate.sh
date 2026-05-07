#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

SQL_DIR="$PROJECT_ROOT/sql"

echo "=== HVC Database Migration ==="

DB_HOST="${DB_HOST:-127.0.0.1}"
DB_PORT="${DB_PORT:-3306}"
DB_USER="${DB_USER:-root}"
DB_PASS="${DB_PASS:-root}"
DB_NAME="${DB_NAME:-hvc}"

run_sql() {
    local file=$1
    echo "Applying: $(basename "$file")"
    mysql -h "$DB_HOST" -P "$DB_PORT" -u "$DB_USER" -p"$DB_PASS" < "$file"
}

if [ -f "$SQL_DIR/000_full_project_schema.sql" ]; then
    run_sql "$SQL_DIR/000_full_project_schema.sql"
else
    echo "Error: Schema file not found at $SQL_DIR/000_full_project_schema.sql"
    exit 1
fi

echo "=== Migration complete ==="
