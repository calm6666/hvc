ALTER TABLE `t_runtime_config`
  ADD COLUMN `enable_http_server` TINYINT NOT NULL DEFAULT 1 COMMENT '是否启用 HTTP 服务和 WebSocket 监控入口，0=否，1=是' AFTER `config_version`,
  ADD COLUMN `enable_callback` TINYINT NOT NULL DEFAULT 1 COMMENT '是否启用回调投递模块，0=否，1=是' AFTER `enable_http_server`;

UPDATE `t_runtime_config`
SET `enable_http_server` = 1
WHERE `enable_http_server` IS NULL;

UPDATE `t_runtime_config`
SET `enable_callback` = 1
WHERE `enable_callback` IS NULL;
