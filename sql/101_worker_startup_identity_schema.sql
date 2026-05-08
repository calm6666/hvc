-- ============================================================
-- 101. Worker 启动身份与机器指纹增强（历史增量迁移）
-- 说明：
-- - 该脚本面向旧库增量升级，不是全新建库脚本。
-- - 全新建库请直接执行 sql/000_full_project_schema.sql。
-- - 若基础 schema 已包含这些字段，则此迁移在新环境可跳过。
-- - 若历史环境尚未包含退出态字段，可继续执行本脚本补齐生命周期信息。
-- ============================================================

SET @worker_instance_exists = (
  SELECT COUNT(*)
  FROM information_schema.tables
  WHERE table_schema = DATABASE()
    AND table_name = 't_worker_instance'
);

SET @ddl_worker_instance = IF(
  @worker_instance_exists = 0,
  'SELECT ''skip sql/101_worker_startup_identity_schema.sql because t_worker_instance is missing; import sql/000_full_project_schema.sql first'' AS message',
  'ALTER TABLE `t_worker_instance`
     ADD COLUMN IF NOT EXISTS `logical_worker_id` VARCHAR(128) DEFAULT NULL COMMENT ''逻辑 Worker 标识'' AFTER `worker_id`,
     ADD COLUMN IF NOT EXISTS `physical_worker_id` VARCHAR(128) DEFAULT NULL COMMENT ''物理 Worker 标识（本次实例级）'' AFTER `logical_worker_id`,
     ADD COLUMN IF NOT EXISTS `machine_fingerprint` VARCHAR(512) DEFAULT NULL COMMENT ''机器指纹'' AFTER `physical_worker_id`,
     ADD COLUMN IF NOT EXISTS `startup_instance_id` VARCHAR(128) DEFAULT NULL COMMENT ''本次启动实例标识'' AFTER `machine_fingerprint`,
     ADD COLUMN IF NOT EXISTS `boot_id` VARCHAR(128) DEFAULT NULL COMMENT ''系统启动或进程启动批次标识'' AFTER `startup_instance_id`,
     ADD COLUMN IF NOT EXISTS `exited_at` DATETIME DEFAULT NULL COMMENT ''退出时间'' AFTER `start_at`,
     ADD COLUMN IF NOT EXISTS `exit_reason` VARCHAR(512) DEFAULT NULL COMMENT ''退出原因'' AFTER `exited_at`'
);

PREPARE stmt_worker_instance FROM @ddl_worker_instance;
EXECUTE stmt_worker_instance;
DEALLOCATE PREPARE stmt_worker_instance;
