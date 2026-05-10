-- 112. 系统运行日志表（历史增量迁移）
-- 说明：
-- - 该脚本面向旧库增量升级，不是全新建库脚本。
-- - 全新建库请直接执行 sql/000_full_project_schema.sql。

CREATE TABLE IF NOT EXISTS `t_system_runtime_log` (
  `log_id` BIGINT UNSIGNED NOT NULL COMMENT '运行日志主键ID',
  `service_name` VARCHAR(128) NOT NULL COMMENT '服务名称',
  `log_level` VARCHAR(16) NOT NULL COMMENT '日志级别，例如 info、error',
  `action_name` VARCHAR(128) NOT NULL COMMENT '日志动作名称',
  `fields_json` JSON NOT NULL COMMENT '结构化日志字段 JSON',
  `logged_at` DATETIME NOT NULL COMMENT '日志产生时间',
  PRIMARY KEY (`log_id`),
  KEY `idx_level_logged_at` (`log_level`, `logged_at`),
  KEY `idx_action_logged_at` (`action_name`, `logged_at`),
  KEY `idx_service_logged_at` (`service_name`, `logged_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='系统运行日志表，用于持久化结构化业务日志并支持后台分页查询';
