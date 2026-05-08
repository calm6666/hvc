-- ============================================================
-- 103. 单任务 callback_url 持久化支持（历史增量迁移）
-- 说明：
-- - 该脚本面向旧库增量升级，不是全新建库脚本。
-- - 全新建库请直接执行 sql/000_full_project_schema.sql。
-- ============================================================

SET @job_request_override_exists = (
  SELECT COUNT(*)
  FROM information_schema.tables
  WHERE table_schema = DATABASE()
    AND table_name = 't_transcode_job_request_override'
);

SET @ddl_job_request_override = IF(
  @job_request_override_exists = 0,
  'SELECT ''skip sql/103_request_callback_url_schema.sql because t_transcode_job_request_override is missing; import sql/000_full_project_schema.sql first'' AS message',
  'ALTER TABLE `t_transcode_job_request_override`
     ADD COLUMN IF NOT EXISTS `override_callback_url` VARCHAR(2048) DEFAULT NULL COMMENT ''请求级覆盖后的单任务回调目标，支持 HTTP/gRPC/MQ'' AFTER `override_segment_prefix`'
);

PREPARE stmt_job_request_override FROM @ddl_job_request_override;
EXECUTE stmt_job_request_override;
DEALLOCATE PREPARE stmt_job_request_override;
