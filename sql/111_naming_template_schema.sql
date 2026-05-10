-- 111. 分片命名模板配置表（历史增量迁移）
-- 说明：
-- - 该脚本面向旧库增量升级，不是全新建库脚本。
-- - 全新建库请直接执行 sql/000_full_project_schema.sql。

CREATE TABLE IF NOT EXISTS `t_config_naming_template` (
  `id` BIGINT UNSIGNED NOT NULL COMMENT '命名模板记录主键ID',
  `config_version` BIGINT UNSIGNED NOT NULL COMMENT '命名模板记录版本号，仅用于模板自身历史追踪',
  `output_base_prefix_tpl` VARCHAR(255) DEFAULT NULL COMMENT '输出根前缀模板，当前保留用于后续扩展',
  `init_seg_name_tpl` VARCHAR(255) NOT NULL COMMENT '初始化分片命名模板',
  `media_seg_name_tpl` VARCHAR(255) NOT NULL COMMENT '媒体分片命名模板',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_config_version` (`config_version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='分片命名模板配置表，用于保存待发布或立即生效的全局命名模板记录';
