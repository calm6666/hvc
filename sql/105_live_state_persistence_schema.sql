ALTER TABLE `t_live_channel`
  ADD COLUMN `assigned_node_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'current assigned live node id' AFTER `enable_watermark`,
  ADD COLUMN `assigned_worker_id` VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'current assigned live worker id' AFTER `assigned_node_id`;

ALTER TABLE `t_live_session`
  ADD COLUMN `push_protocol` VARCHAR(32) DEFAULT NULL COMMENT 'publish protocol such as rtmp or srt' AFTER `playback_hls_url`,
  ADD COLUMN `assigned_node_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'current assigned session node id' AFTER `push_protocol`,
  ADD COLUMN `assigned_worker_id` VARCHAR(128) NOT NULL DEFAULT '' COMMENT 'current assigned session worker id' AFTER `assigned_node_id`,
  ADD COLUMN `resume_count` INT NOT NULL DEFAULT 0 COMMENT 'interrupt resume counter' AFTER `assigned_worker_id`;
