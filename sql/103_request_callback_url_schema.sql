-- ============================================================
-- 103. 单任务 callback_url 持久化支持
-- ============================================================

ALTER TABLE `t_transcode_job_request_override`
  ADD COLUMN `override_callback_url` VARCHAR(2048) DEFAULT NULL COMMENT '请求级覆盖后的单任务 HTTP 回调地址' AFTER `override_segment_prefix`;
