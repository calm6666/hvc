-- 106. 对外 public gRPC 运行配置拆分（历史增量迁移）
-- 说明：
-- - 该脚本面向旧库增量升级，不是全新建库脚本。
-- - 全新建库请直接执行 sql/000_full_project_schema.sql。

SET @runtime_config_exists_106 = (
  SELECT COUNT(*)
  FROM information_schema.tables
  WHERE table_schema = DATABASE()
    AND table_name = 't_runtime_config'
);

SET @ddl_runtime_config_106 = IF(
  @runtime_config_exists_106 = 0,
  'SELECT ''skip sql/106_public_grpc_runtime_schema.sql because t_runtime_config is missing; import sql/000_full_project_schema.sql first'' AS message',
  'ALTER TABLE `t_runtime_config`
     ADD COLUMN IF NOT EXISTS `public_grpc_enabled` TINYINT NOT NULL DEFAULT 0 COMMENT ''是否启用对外 public gRPC 服务，0=否，1=是'' AFTER `callback_mq_topic`,
     ADD COLUMN IF NOT EXISTS `public_grpc_host` VARCHAR(255) NOT NULL DEFAULT ''0.0.0.0'' COMMENT ''对外 public gRPC 监听主机'' AFTER `public_grpc_enabled`,
     ADD COLUMN IF NOT EXISTS `public_grpc_port` INT NOT NULL DEFAULT 9090 COMMENT ''对外 public gRPC 监听端口'' AFTER `public_grpc_host`'
);

PREPARE stmt_runtime_config_106 FROM @ddl_runtime_config_106;
EXECUTE stmt_runtime_config_106;
DEALLOCATE PREPARE stmt_runtime_config_106;

SET @sql_runtime_config_106_update = IF(
  @runtime_config_exists_106 = 0,
  'SELECT ''skip migrate public_grpc fields'' AS message',
  'UPDATE `t_runtime_config`
     SET `public_grpc_enabled` = `rpc_callback_receiver_enabled`,
         `public_grpc_host` = CASE
           WHEN `rpc_callback_receiver_host` IS NULL OR `rpc_callback_receiver_host` = '''' THEN ''0.0.0.0''
           ELSE `rpc_callback_receiver_host`
         END,
         `public_grpc_port` = CASE
           WHEN `rpc_callback_receiver_port` IS NULL OR `rpc_callback_receiver_port` <= 0 THEN 9090
           ELSE `rpc_callback_receiver_port`
         END'
);

PREPARE stmt_runtime_config_106_update FROM @sql_runtime_config_106_update;
EXECUTE stmt_runtime_config_106_update;
DEALLOCATE PREPARE stmt_runtime_config_106_update;
