-- 107. 运行配置中的 MQ / Storage 字段增强（历史增量迁移）
-- 说明：
-- - 该脚本面向旧库增量升级，不是全新建库脚本。
-- - 全新建库请直接执行 sql/000_full_project_schema.sql。

SET @runtime_config_exists_107 = (
  SELECT COUNT(*)
  FROM information_schema.tables
  WHERE table_schema = DATABASE()
    AND table_name = 't_runtime_config'
);

SET @ddl_runtime_config_107 = IF(
  @runtime_config_exists_107 = 0,
  'SELECT ''skip sql/107_runtime_config_storage_mq_schema.sql because t_runtime_config is missing; import sql/000_full_project_schema.sql first'' AS message',
  'ALTER TABLE `t_runtime_config`
     ADD COLUMN IF NOT EXISTS `enable_mq_consumer` TINYINT NOT NULL DEFAULT 0 COMMENT ''是否启用 MQ 创建任务消费者，0=否，1=是'' AFTER `rpc_callback_receiver_registry_id`,
     ADD COLUMN IF NOT EXISTS `storage_type` VARCHAR(32) NOT NULL DEFAULT ''local'' COMMENT ''运行期生效的存储类型，local 或 s3'' AFTER `mq_prefetch_count`,
     ADD COLUMN IF NOT EXISTS `storage_endpoint` VARCHAR(255) DEFAULT NULL COMMENT ''运行期生效的对象存储 endpoint'' AFTER `storage_type`,
     ADD COLUMN IF NOT EXISTS `storage_bucket` VARCHAR(128) DEFAULT NULL COMMENT ''运行期生效的对象存储 bucket'' AFTER `storage_endpoint`,
     ADD COLUMN IF NOT EXISTS `storage_access_key_id` VARCHAR(255) DEFAULT NULL COMMENT ''运行期生效的对象存储 access key'' AFTER `storage_bucket`,
     ADD COLUMN IF NOT EXISTS `storage_secret_access_key` VARCHAR(255) DEFAULT NULL COMMENT ''运行期生效的对象存储 secret key'' AFTER `storage_access_key_id`,
     ADD COLUMN IF NOT EXISTS `storage_use_ssl` TINYINT NOT NULL DEFAULT 0 COMMENT ''运行期对象存储是否启用 SSL，0=否，1=是'' AFTER `storage_secret_access_key`,
     ADD COLUMN IF NOT EXISTS `storage_play_domain` VARCHAR(255) DEFAULT NULL COMMENT ''运行期直播播放域名'' AFTER `storage_use_ssl`,
     ADD COLUMN IF NOT EXISTS `storage_flv_domain` VARCHAR(255) DEFAULT NULL COMMENT ''运行期 HTTP-FLV 播放域名'' AFTER `storage_play_domain`,
     ADD COLUMN IF NOT EXISTS `storage_local_base_path` VARCHAR(1024) DEFAULT NULL COMMENT ''运行期本地存储根目录'' AFTER `storage_flv_domain`,
     ADD COLUMN IF NOT EXISTS `default_storage_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT ''运行期默认存储配置ID，0表示直接使用内嵌存储参数'' AFTER `storage_local_base_path`'
);

PREPARE stmt_runtime_config_107 FROM @ddl_runtime_config_107;
EXECUTE stmt_runtime_config_107;
DEALLOCATE PREPARE stmt_runtime_config_107;

SET @sql_runtime_config_107_update = IF(
  @runtime_config_exists_107 = 0,
  'SELECT ''skip migrate mq/storage defaults'' AS message',
  'UPDATE `t_runtime_config`
     SET `enable_mq_consumer` = CASE
           WHEN `mq_queue_name` IS NOT NULL AND `mq_queue_name` <> '''' THEN 1
           ELSE 0
         END,
         `storage_type` = CASE
           WHEN `storage_type` IS NULL OR `storage_type` = '''' THEN ''local''
           ELSE `storage_type`
         END,
         `storage_local_base_path` = CASE
           WHEN `storage_local_base_path` IS NULL OR `storage_local_base_path` = '''' THEN ''/data/hvc/media''
           ELSE `storage_local_base_path`
         END'
);

PREPARE stmt_runtime_config_107_update FROM @sql_runtime_config_107_update;
EXECUTE stmt_runtime_config_107_update;
DEALLOCATE PREPARE stmt_runtime_config_107_update;
