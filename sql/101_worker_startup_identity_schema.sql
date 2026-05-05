-- ============================================================
-- 101. Worker 启动身份与机器指纹增强（第一阶段增量迁移）
-- 说明：
-- - 若基础 schema 已包含这些字段，则此迁移在新环境可跳过。
-- - 若历史环境尚未包含退出态字段，可继续执行本脚本补齐生命周期信息。
-- ============================================================

ALTER TABLE `t_worker_instance`
  ADD COLUMN `logical_worker_id` VARCHAR(128) DEFAULT NULL COMMENT '逻辑 Worker 标识' AFTER `worker_id`,
  ADD COLUMN `physical_worker_id` VARCHAR(128) DEFAULT NULL COMMENT '物理 Worker 标识（本次实例级）' AFTER `logical_worker_id`,
  ADD COLUMN `machine_fingerprint` VARCHAR(512) DEFAULT NULL COMMENT '机器指纹' AFTER `physical_worker_id`,
  ADD COLUMN `startup_instance_id` VARCHAR(128) DEFAULT NULL COMMENT '本次启动实例标识' AFTER `machine_fingerprint`,
  ADD COLUMN `boot_id` VARCHAR(128) DEFAULT NULL COMMENT '系统启动或进程启动批次标识' AFTER `startup_instance_id`,
  ADD COLUMN `exited_at` DATETIME DEFAULT NULL COMMENT '退出时间' AFTER `start_at`,
  ADD COLUMN `exit_reason` VARCHAR(512) DEFAULT NULL COMMENT '退出原因' AFTER `exited_at`;
