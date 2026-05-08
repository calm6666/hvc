-- 109. outbox 表结构对齐（历史增量迁移）
-- 说明：
-- - 该脚本面向旧库增量升级，不是全新建库脚本。
-- - 全新建库请直接执行 sql/000_full_project_schema.sql。

SET @event_outbox_exists_109 = (
  SELECT COUNT(*)
  FROM information_schema.tables
  WHERE table_schema = DATABASE()
    AND table_name = 't_event_outbox'
);
SET @ddl_event_outbox_109 = IF(
  @event_outbox_exists_109 = 0,
  'SELECT ''skip sql/109_outbox_schema_alignment.sql t_event_outbox because base table is missing'' AS message',
  'ALTER TABLE `t_event_outbox`
     ADD COLUMN IF NOT EXISTS `request_id` VARCHAR(128) DEFAULT NULL COMMENT ''关联请求ID，便于按外部请求链路排查'' AFTER `job_id`,
     ADD COLUMN IF NOT EXISTS `max_retry_count` INT NOT NULL DEFAULT 3 COMMENT ''最大重试次数，超过后转入最终失败'' AFTER `retry_count`,
     ADD COLUMN IF NOT EXISTS `last_error_message` VARCHAR(1024) DEFAULT NULL COMMENT ''最近一次投递失败错误详情，便于补偿和审计'' AFTER `next_retry_at`'
);
PREPARE stmt_event_outbox_109 FROM @ddl_event_outbox_109;
EXECUTE stmt_event_outbox_109;
DEALLOCATE PREPARE stmt_event_outbox_109;

SET @idx_request_id_exists_109 = (
  SELECT COUNT(*)
  FROM information_schema.statistics
  WHERE table_schema = DATABASE()
    AND table_name = 't_event_outbox'
    AND index_name = 'idx_request_id'
);
SET @ddl_idx_request_id_109 = IF(
  @event_outbox_exists_109 = 0 OR @idx_request_id_exists_109 > 0,
  'SELECT ''skip idx_request_id'' AS message',
  'ALTER TABLE `t_event_outbox` ADD KEY `idx_request_id` (`request_id`)'
);
PREPARE stmt_idx_request_id_109 FROM @ddl_idx_request_id_109;
EXECUTE stmt_idx_request_id_109;
DEALLOCATE PREPARE stmt_idx_request_id_109;
