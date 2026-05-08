-- 108. 分片模板快照与 rendition_key（历史增量迁移）
-- 说明：
-- - 该脚本面向旧库增量升级，不是全新建库脚本。
-- - 全新建库请直接执行 sql/000_full_project_schema.sql。

SET @transcode_job_exists_108 = (
  SELECT COUNT(*)
  FROM information_schema.tables
  WHERE table_schema = DATABASE()
    AND table_name = 't_transcode_job'
);
SET @ddl_transcode_job_108 = IF(
  @transcode_job_exists_108 = 0,
  'SELECT ''skip sql/108_segment_template_rendition_key_schema.sql transcode_job because base table is missing'' AS message',
  'ALTER TABLE `t_transcode_job`
     ADD COLUMN IF NOT EXISTS `segment_template` VARCHAR(255) DEFAULT NULL COMMENT ''该任务首次执行时锁定的分片命名模板快照，用于保证重试和动态清单生成一致'' AFTER `segment_duration_sec`'
);
PREPARE stmt_transcode_job_108 FROM @ddl_transcode_job_108;
EXECUTE stmt_transcode_job_108;
DEALLOCATE PREPARE stmt_transcode_job_108;

SET @transcode_rendition_exists_108 = (
  SELECT COUNT(*)
  FROM information_schema.tables
  WHERE table_schema = DATABASE()
    AND table_name = 't_transcode_rendition'
);
SET @ddl_transcode_rendition_108 = IF(
  @transcode_rendition_exists_108 = 0,
  'SELECT ''skip sql/108_segment_template_rendition_key_schema.sql transcode_rendition because base table is missing'' AS message',
  'ALTER TABLE `t_transcode_rendition`
     ADD COLUMN IF NOT EXISTS `rendition_key` VARCHAR(32) NOT NULL DEFAULT '''' COMMENT ''任务内清晰度稳定短 key，用于分片命名和重试复用'' AFTER `rendition_name`'
);
PREPARE stmt_transcode_rendition_108 FROM @ddl_transcode_rendition_108;
EXECUTE stmt_transcode_rendition_108;
DEALLOCATE PREPARE stmt_transcode_rendition_108;

SET @uk_job_rendition_name_exists_108 = (
  SELECT COUNT(*)
  FROM information_schema.statistics
  WHERE table_schema = DATABASE()
    AND table_name = 't_transcode_rendition'
    AND index_name = 'uk_job_rendition_name'
);
SET @ddl_uk_job_rendition_name_108 = IF(
  @transcode_rendition_exists_108 = 0 OR @uk_job_rendition_name_exists_108 > 0,
  'SELECT ''skip uk_job_rendition_name'' AS message',
  'ALTER TABLE `t_transcode_rendition` ADD UNIQUE KEY `uk_job_rendition_name` (`job_id`, `rendition_name`)'
);
PREPARE stmt_uk_job_rendition_name_108 FROM @ddl_uk_job_rendition_name_108;
EXECUTE stmt_uk_job_rendition_name_108;
DEALLOCATE PREPARE stmt_uk_job_rendition_name_108;

SET @transcode_segment_exists_108 = (
  SELECT COUNT(*)
  FROM information_schema.tables
  WHERE table_schema = DATABASE()
    AND table_name = 't_transcode_segment'
);
SET @ddl_transcode_segment_108 = IF(
  @transcode_segment_exists_108 = 0,
  'SELECT ''skip sql/108_segment_template_rendition_key_schema.sql transcode_segment because base table is missing'' AS message',
  'ALTER TABLE `t_transcode_segment`
     ADD COLUMN IF NOT EXISTS `rendition_name` VARCHAR(64) NOT NULL DEFAULT '''' COMMENT ''分片所属清晰度名称，做动态清单和排障时可避免额外 join'' AFTER `rendition_id`,
     ADD COLUMN IF NOT EXISTS `rendition_key` VARCHAR(32) NOT NULL DEFAULT '''' COMMENT ''分片所属清晰度稳定短 key，便于按模板重建 DASH media URL'' AFTER `rendition_name`,
     ADD COLUMN IF NOT EXISTS `width` INT NOT NULL DEFAULT 0 COMMENT ''该分片所属视频输出宽度，单位像素'' AFTER `duration_ms`,
     ADD COLUMN IF NOT EXISTS `height` INT NOT NULL DEFAULT 0 COMMENT ''该分片所属视频输出高度，单位像素'' AFTER `width`,
     ADD COLUMN IF NOT EXISTS `video_bitrate_kbps` INT NOT NULL DEFAULT 0 COMMENT ''该分片所属视频码率，单位 kbps'' AFTER `height`,
     ADD COLUMN IF NOT EXISTS `audio_bitrate_kbps` INT NOT NULL DEFAULT 0 COMMENT ''该分片所属音频码率，单位 kbps'' AFTER `video_bitrate_kbps`,
     ADD COLUMN IF NOT EXISTS `video_codec` VARCHAR(32) DEFAULT NULL COMMENT ''该分片所属视频编码名称，例如 h264、hevc'' AFTER `audio_bitrate_kbps`,
     ADD COLUMN IF NOT EXISTS `audio_codec` VARCHAR(32) DEFAULT NULL COMMENT ''该分片所属音频编码名称，例如 aac'' AFTER `video_codec`'
);
PREPARE stmt_transcode_segment_108 FROM @ddl_transcode_segment_108;
EXECUTE stmt_transcode_segment_108;
DEALLOCATE PREPARE stmt_transcode_segment_108;
