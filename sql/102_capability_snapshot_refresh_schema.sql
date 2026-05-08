-- ============================================================
-- 102. 能力快照实时刷新增强（历史增量迁移）
-- 说明：
-- - 该脚本面向旧库增量升级，不是全新建库脚本。
-- - 全新建库请直接执行 sql/000_full_project_schema.sql。
-- - 目标是让 Scheduler 只消费当前活跃 Worker 在本次启动中写入的最新能力快照。
-- - 若基础 schema 已包含这些字段，则此迁移在新环境可跳过。
-- ============================================================

SET @worker_codec_capability_exists = (
  SELECT COUNT(*)
  FROM information_schema.tables
  WHERE table_schema = DATABASE()
    AND table_name = 't_worker_codec_capability'
);

SET @ddl_worker_codec_capability = IF(
  @worker_codec_capability_exists = 0,
  'SELECT ''skip sql/102_capability_snapshot_refresh_schema.sql because t_worker_codec_capability is missing; import sql/000_full_project_schema.sql first'' AS message',
  'ALTER TABLE `t_worker_codec_capability`
     ADD COLUMN IF NOT EXISTS `worker_instance_id` BIGINT UNSIGNED DEFAULT NULL COMMENT ''关联的 Worker 实例记录ID'' AFTER `node_id`,
     ADD COLUMN IF NOT EXISTS `gpu_device_id` BIGINT UNSIGNED DEFAULT NULL COMMENT ''关联的稳定 GPU 设备记录ID'' AFTER `worker_instance_id`,
     ADD COLUMN IF NOT EXISTS `gpu_uuid` VARCHAR(128) DEFAULT NULL COMMENT ''GPU 稳定硬件标识'' AFTER `gpu_index`,
     ADD COLUMN IF NOT EXISTS `startup_instance_id` VARCHAR(128) DEFAULT NULL COMMENT ''本次启动实例标识'' AFTER `gpu_uuid`,
     ADD COLUMN IF NOT EXISTS `probe_generation` BIGINT UNSIGNED NOT NULL DEFAULT 1 COMMENT ''同一启动周期内的探测代次'' AFTER `startup_instance_id`,
     ADD COLUMN IF NOT EXISTS `machine_fingerprint` VARCHAR(512) DEFAULT NULL COMMENT ''机器指纹'' AFTER `probe_generation`,
     ADD COLUMN IF NOT EXISTS `is_latest` TINYINT NOT NULL DEFAULT 1 COMMENT ''是否为当前最新快照，0=否，1=是'' AFTER `machine_fingerprint`,
     ADD COLUMN IF NOT EXISTS `capability_payload_json` JSON DEFAULT NULL COMMENT ''原始能力探测载荷摘要'' AFTER `is_latest`'
);

PREPARE stmt_worker_codec_capability FROM @ddl_worker_codec_capability;
EXECUTE stmt_worker_codec_capability;
DEALLOCATE PREPARE stmt_worker_codec_capability;

SET @idx_worker_instance_latest_exists = (
  SELECT COUNT(*)
  FROM information_schema.statistics
  WHERE table_schema = DATABASE()
    AND table_name = 't_worker_codec_capability'
    AND index_name = 'idx_worker_instance_latest'
);
SET @ddl_idx_worker_instance_latest = IF(
  @worker_codec_capability_exists = 0 OR @idx_worker_instance_latest_exists > 0,
  'SELECT ''skip idx_worker_instance_latest'' AS message',
  'CREATE INDEX `idx_worker_instance_latest` ON `t_worker_codec_capability` (`worker_instance_id`, `is_latest`, `collected_at`)'
);
PREPARE stmt_idx_worker_instance_latest FROM @ddl_idx_worker_instance_latest;
EXECUTE stmt_idx_worker_instance_latest;
DEALLOCATE PREPARE stmt_idx_worker_instance_latest;

SET @idx_startup_instance_latest_exists = (
  SELECT COUNT(*)
  FROM information_schema.statistics
  WHERE table_schema = DATABASE()
    AND table_name = 't_worker_codec_capability'
    AND index_name = 'idx_startup_instance_latest'
);
SET @ddl_idx_startup_instance_latest = IF(
  @worker_codec_capability_exists = 0 OR @idx_startup_instance_latest_exists > 0,
  'SELECT ''skip idx_startup_instance_latest'' AS message',
  'CREATE INDEX `idx_startup_instance_latest` ON `t_worker_codec_capability` (`startup_instance_id`, `is_latest`, `collected_at`)'
);
PREPARE stmt_idx_startup_instance_latest FROM @ddl_idx_startup_instance_latest;
EXECUTE stmt_idx_startup_instance_latest;
DEALLOCATE PREPARE stmt_idx_startup_instance_latest;

SET @idx_gpu_uuid_latest_exists = (
  SELECT COUNT(*)
  FROM information_schema.statistics
  WHERE table_schema = DATABASE()
    AND table_name = 't_worker_codec_capability'
    AND index_name = 'idx_gpu_uuid_latest'
);
SET @ddl_idx_gpu_uuid_latest = IF(
  @worker_codec_capability_exists = 0 OR @idx_gpu_uuid_latest_exists > 0,
  'SELECT ''skip idx_gpu_uuid_latest'' AS message',
  'CREATE INDEX `idx_gpu_uuid_latest` ON `t_worker_codec_capability` (`gpu_uuid`, `is_latest`, `collected_at`)'
);
PREPARE stmt_idx_gpu_uuid_latest FROM @ddl_idx_gpu_uuid_latest;
EXECUTE stmt_idx_gpu_uuid_latest;
DEALLOCATE PREPARE stmt_idx_gpu_uuid_latest;
