-- ============================================================
-- 100. bootstrap 配置源绑定与有效运行配置快照（历史增量迁移）
-- 说明：
-- - 该脚本面向“旧库增量升级”，不是全新建库脚本。
-- - 全新建库请直接执行 sql/000_full_project_schema.sql。
-- - 本脚本不覆盖旧表结构，只以增量方式补齐新能力。
-- - 第一阶段采用“bootstrap 配置源抽象优先”策略，先落地外部基础配置源绑定信息，
--   具体 provider 接入留到后续实现。
-- ============================================================

CREATE TABLE IF NOT EXISTS `t_config_center_binding` (
  `binding_id` BIGINT UNSIGNED NOT NULL COMMENT '配置中心绑定主键ID',
  `binding_name` VARCHAR(128) NOT NULL COMMENT '配置中心绑定名称',
  `provider_type` VARCHAR(64) NOT NULL COMMENT '配置中心提供方类型，例如 db_adapter、nacos、apollo、consul、自研http',
  `endpoint` VARCHAR(1024) DEFAULT NULL COMMENT '配置中心接入地址',
  `namespace` VARCHAR(255) DEFAULT NULL COMMENT '配置命名空间或租户空间',
  `auth_mode` VARCHAR(64) NOT NULL DEFAULT 'none' COMMENT '鉴权模式，例如 none、token、access_key、basic_auth',
  `access_key` VARCHAR(255) DEFAULT NULL COMMENT '访问凭证标识',
  `secret_key` VARCHAR(255) DEFAULT NULL COMMENT '访问凭证密钥，后续建议改为密文存储',
  `token` VARCHAR(1024) DEFAULT NULL COMMENT '配置中心访问令牌',
  `enabled` TINYINT NOT NULL DEFAULT 1 COMMENT '是否启用，0=否，1=是',
  `priority` INT NOT NULL DEFAULT 0 COMMENT '优先级，值越大越优先',
  `last_sync_status` VARCHAR(64) DEFAULT NULL COMMENT '最近一次同步状态，例如 ok、failed、disabled',
  `last_sync_message` VARCHAR(1024) DEFAULT NULL COMMENT '最近一次同步结果说明',
  `last_sync_at` DATETIME DEFAULT NULL COMMENT '最近一次同步时间',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '更新时间',
  PRIMARY KEY (`binding_id`),
  UNIQUE KEY `uk_binding_name` (`binding_name`),
  KEY `idx_enabled_priority` (`enabled`, `priority`, `updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='bootstrap 配置源绑定表';

CREATE TABLE IF NOT EXISTS `t_effective_runtime_config_snapshot` (
  `snapshot_id` BIGINT UNSIGNED NOT NULL COMMENT '有效配置快照主键ID',
  `config_version` BIGINT UNSIGNED NOT NULL COMMENT '关联的运行配置版本号',
  `config_source` VARCHAR(64) NOT NULL COMMENT '有效配置来源，例如 admin_db、bootstrap_default、bootstrap_fallback',
  `source_revision` VARCHAR(128) DEFAULT NULL COMMENT '来源修订号',
  `merged_payload_json` JSON NOT NULL COMMENT '最终合并后的有效配置完整载荷',
  `config_hash` VARCHAR(128) NOT NULL COMMENT '有效配置摘要哈希',
  `created_by` VARCHAR(128) DEFAULT NULL COMMENT '创建人或触发来源',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  PRIMARY KEY (`snapshot_id`),
  KEY `idx_config_version` (`config_version`),
  KEY `idx_config_hash` (`config_hash`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='有效运行配置快照表，用于持久化最终配置结果；运行期热路径应优先从 Redis 缓存读取';

SET @runtime_config_exists = (
  SELECT COUNT(*)
  FROM information_schema.tables
  WHERE table_schema = DATABASE()
    AND table_name = 't_runtime_config'
);

SET @ddl_runtime_config = IF(
  @runtime_config_exists = 0,
  'SELECT ''skip sql/100_config_center_schema.sql ALTER TABLE t_runtime_config because base table is missing; import sql/000_full_project_schema.sql first'' AS message',
  'ALTER TABLE `t_runtime_config`
     ADD COLUMN IF NOT EXISTS `config_source` VARCHAR(64) DEFAULT NULL COMMENT ''配置来源标识'' AFTER `change_summary`,
     ADD COLUMN IF NOT EXISTS `source_revision` VARCHAR(128) DEFAULT NULL COMMENT ''来源修订号'' AFTER `config_source`,
     ADD COLUMN IF NOT EXISTS `published_by` VARCHAR(128) DEFAULT NULL COMMENT ''发布人'' AFTER `source_revision`,
     ADD COLUMN IF NOT EXISTS `published_at` DATETIME DEFAULT NULL COMMENT ''发布时间'' AFTER `published_by`,
     ADD COLUMN IF NOT EXISTS `effective_config_hash` VARCHAR(128) DEFAULT NULL COMMENT ''有效配置摘要哈希'' AFTER `published_at`'
);

PREPARE stmt_runtime_config FROM @ddl_runtime_config;
EXECUTE stmt_runtime_config;
DEALLOCATE PREPARE stmt_runtime_config;
