数据库迁移脚本说明

- `migrate.sh` 会连接 MySQL，自动创建数据库，并按模式执行 SQL
- 所有 SQL 通过 `utf8mb4` 字符集执行，避免中文注释和字符串乱码
- 服务端不会在启动时自动建表，数据库必须先通过该脚本或手工导入完成初始化
- 默认模式是 `full`，仅执行 `sql/000_full_project_schema.sql`，适用于全新建库
- `incremental` 模式只执行 `100_*.sql` 及之后的历史增量脚本，适用于存量库升级

示例：

- `./scripts/migrate/migrate.sh`
- `./scripts/migrate/migrate.sh -m full`
- `./scripts/migrate/migrate.sh -m incremental`
- `./scripts/migrate/migrate.sh -h 127.0.0.1 -P 3306 -u root -p 123456 -d hvc`
