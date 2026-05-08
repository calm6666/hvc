-- 104. 运行模式热开关（历史增量迁移）
-- 说明：
-- - 该脚本面向旧库增量升级，不是全新建库脚本。
-- - 全新建库请直接执行 sql/000_full_project_schema.sql。

SET @runtime_config_exists_104 = (
  SELECT COUNT(*)
  FROM information_schema.tables
  WHERE table_schema = DATABASE()
    AND table_name = 't_runtime_config'
);

SET @ddl_runtime_config_104 = IF(
  @runtime_config_exists_104 = 0,
  'SELECT ''skip sql/104_runtime_mode_hot_switch_schema.sql because t_runtime_config is missing; import sql/000_full_project_schema.sql first'' AS message',
  'ALTER TABLE `t_runtime_config`
     ADD COLUMN IF NOT EXISTS `enable_http_server` TINYINT NOT NULL DEFAULT 1 COMMENT ''是否启用 HTTP 服务和 WebSocket 监控入口，0=否，1=是'' AFTER `config_version`,
     ADD COLUMN IF NOT EXISTS `enable_callback` TINYINT NOT NULL DEFAULT 1 COMMENT ''是否启用回调投递模块，0=否，1=是'' AFTER `enable_http_server`'
);

PREPARE stmt_runtime_config_104 FROM @ddl_runtime_config_104;
EXECUTE stmt_runtime_config_104;
DEALLOCATE PREPARE stmt_runtime_config_104;

SET @sql_runtime_config_104_update_http = IF(
  @runtime_config_exists_104 = 0,
  'SELECT ''skip update enable_http_server'' AS message',
  'UPDATE `t_runtime_config` SET `enable_http_server` = 1 WHERE `enable_http_server` IS NULL'
);
PREPARE stmt_runtime_config_104_update_http FROM @sql_runtime_config_104_update_http;
EXECUTE stmt_runtime_config_104_update_http;
DEALLOCATE PREPARE stmt_runtime_config_104_update_http;

SET @sql_runtime_config_104_update_callback = IF(
  @runtime_config_exists_104 = 0,
  'SELECT ''skip update enable_callback'' AS message',
  'UPDATE `t_runtime_config` SET `enable_callback` = 1 WHERE `enable_callback` IS NULL'
);
PREPARE stmt_runtime_config_104_update_callback FROM @sql_runtime_config_104_update_callback;
EXECUTE stmt_runtime_config_104_update_callback;
DEALLOCATE PREPARE stmt_runtime_config_104_update_callback;
