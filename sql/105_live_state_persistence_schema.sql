-- 105. 直播状态持久化增强（历史增量迁移）
-- 说明：
-- - 该脚本面向旧库增量升级，不是全新建库脚本。
-- - 全新建库请直接执行 sql/000_full_project_schema.sql。

SET @live_channel_exists = (
  SELECT COUNT(*)
  FROM information_schema.tables
  WHERE table_schema = DATABASE()
    AND table_name = 't_live_channel'
);
SET @ddl_live_channel = IF(
  @live_channel_exists = 0,
  'SELECT ''skip sql/105_live_state_persistence_schema.sql live_channel because base table is missing'' AS message',
  'ALTER TABLE `t_live_channel`
     ADD COLUMN IF NOT EXISTS `assigned_node_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT ''current assigned live node id'' AFTER `enable_watermark`,
     ADD COLUMN IF NOT EXISTS `assigned_worker_id` VARCHAR(128) NOT NULL DEFAULT '''' COMMENT ''current assigned live worker id'' AFTER `assigned_node_id`'
);
PREPARE stmt_live_channel FROM @ddl_live_channel;
EXECUTE stmt_live_channel;
DEALLOCATE PREPARE stmt_live_channel;

SET @live_session_exists = (
  SELECT COUNT(*)
  FROM information_schema.tables
  WHERE table_schema = DATABASE()
    AND table_name = 't_live_session'
);
SET @ddl_live_session = IF(
  @live_session_exists = 0,
  'SELECT ''skip sql/105_live_state_persistence_schema.sql live_session because base table is missing'' AS message',
  'ALTER TABLE `t_live_session`
     ADD COLUMN IF NOT EXISTS `push_protocol` VARCHAR(32) DEFAULT NULL COMMENT ''publish protocol such as rtmp or srt'' AFTER `playback_hls_url`,
     ADD COLUMN IF NOT EXISTS `assigned_node_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT ''current assigned session node id'' AFTER `push_protocol`,
     ADD COLUMN IF NOT EXISTS `assigned_worker_id` VARCHAR(128) NOT NULL DEFAULT '''' COMMENT ''current assigned session worker id'' AFTER `assigned_node_id`,
     ADD COLUMN IF NOT EXISTS `resume_count` INT NOT NULL DEFAULT 0 COMMENT ''interrupt resume counter'' AFTER `assigned_worker_id`'
);
PREPARE stmt_live_session FROM @ddl_live_session;
EXECUTE stmt_live_session;
DEALLOCATE PREPARE stmt_live_session;
