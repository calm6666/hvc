-- ============================================================
-- 102. 能力快照实时刷新增强（第一阶段增量迁移）
-- 说明：
-- - 目标是让 Scheduler 只消费当前活跃 Worker 在本次启动中写入的最新能力快照。
-- - 若基础 schema 已包含这些字段，则此迁移在新环境可跳过。
-- ============================================================

ALTER TABLE `t_worker_codec_capability`
  ADD COLUMN `worker_instance_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '关联的 Worker 实例记录ID' AFTER `node_id`,
  ADD COLUMN `gpu_device_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '关联的稳定 GPU 设备记录ID' AFTER `worker_instance_id`,
  ADD COLUMN `gpu_uuid` VARCHAR(128) DEFAULT NULL COMMENT 'GPU 稳定硬件标识' AFTER `gpu_index`,
  ADD COLUMN `startup_instance_id` VARCHAR(128) DEFAULT NULL COMMENT '本次启动实例标识' AFTER `gpu_uuid`,
  ADD COLUMN `probe_generation` BIGINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '同一启动周期内的探测代次' AFTER `startup_instance_id`,
  ADD COLUMN `machine_fingerprint` VARCHAR(512) DEFAULT NULL COMMENT '机器指纹' AFTER `probe_generation`,
  ADD COLUMN `is_latest` TINYINT NOT NULL DEFAULT 1 COMMENT '是否为当前最新快照，0=否，1=是' AFTER `machine_fingerprint`,
  ADD COLUMN `capability_payload_json` JSON DEFAULT NULL COMMENT '原始能力探测载荷摘要' AFTER `is_latest`;

CREATE INDEX `idx_worker_instance_latest` ON `t_worker_codec_capability` (`worker_instance_id`, `is_latest`, `collected_at`);
CREATE INDEX `idx_startup_instance_latest` ON `t_worker_codec_capability` (`startup_instance_id`, `is_latest`, `collected_at`);
CREATE INDEX `idx_gpu_uuid_latest` ON `t_worker_codec_capability` (`gpu_uuid`, `is_latest`, `collected_at`);
