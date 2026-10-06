-- 110. 后台菜单与角色菜单绑定（历史增量迁移）
-- 说明：
-- - 该脚本面向旧库增量升级，不是全新建库脚本。
-- - 全新建库请直接执行 sql/000_full_project_schema.sql。

CREATE TABLE IF NOT EXISTS `t_admin_menu` (
  `menu_id` BIGINT UNSIGNED NOT NULL COMMENT '菜单主键ID',
  `parent_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '父级菜单ID，0表示根节点',
  `menu_key` VARCHAR(128) NOT NULL COMMENT '菜单唯一键',
  `menu_name` VARCHAR(128) NOT NULL COMMENT '菜单名称',
  `route_path` VARCHAR(255) DEFAULT NULL COMMENT '前端路由路径',
  `component` VARCHAR(255) DEFAULT NULL COMMENT '前端组件路径',
  `icon_name` VARCHAR(128) DEFAULT NULL COMMENT '前端图标名称',
  `menu_type` VARCHAR(32) NOT NULL DEFAULT 'menu' COMMENT '菜单类型，例如 directory、menu、button',
  `permission_key` VARCHAR(128) DEFAULT NULL COMMENT '关联权限点 key，可为空',
  `sort_no` INT NOT NULL DEFAULT 0 COMMENT '同级排序值，越小越靠前',
  `hidden` TINYINT NOT NULL DEFAULT 0 COMMENT '是否隐藏，0=否，1=是',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '状态，1=启用，2=禁用',
  `created_at` DATETIME NOT NULL COMMENT '记录创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '记录更新时间',
  PRIMARY KEY (`menu_id`),
  UNIQUE KEY `uk_menu_key` (`menu_key`),
  KEY `idx_parent_sort_status` (`parent_id`, `sort_no`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='后台菜单表';

SET @admin_menu_exists_110_upgrade = (
  SELECT COUNT(*)
  FROM information_schema.tables
  WHERE table_schema = DATABASE()
    AND table_name = 't_admin_menu'
);

SET @admin_menu_has_component_110 = (
  SELECT COUNT(*)
  FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 't_admin_menu'
    AND column_name = 'component'
);

SET @admin_menu_has_component_name_110 = (
  SELECT COUNT(*)
  FROM information_schema.columns
  WHERE table_schema = DATABASE()
    AND table_name = 't_admin_menu'
    AND column_name = 'component_name'
);

SET @ddl_admin_menu_component_110 = IF(
  @admin_menu_exists_110_upgrade = 0 OR @admin_menu_has_component_110 > 0,
  'SELECT ''skip sql/110_admin_menu_schema.sql component add because table is missing or component already exists'' AS message',
  'ALTER TABLE `t_admin_menu`
     ADD COLUMN `component` VARCHAR(255) DEFAULT NULL COMMENT ''前端组件路径'' AFTER `route_path`'
);

PREPARE stmt_admin_menu_component_110 FROM @ddl_admin_menu_component_110;
EXECUTE stmt_admin_menu_component_110;
DEALLOCATE PREPARE stmt_admin_menu_component_110;

SET @sql_admin_menu_component_backfill_110 = IF(
  @admin_menu_exists_110_upgrade = 0 OR @admin_menu_has_component_name_110 = 0,
  'SELECT ''skip sql/110_admin_menu_schema.sql component backfill because legacy component_name is missing'' AS message',
  'UPDATE `t_admin_menu`
      SET `component` = `component_name`
    WHERE (`component` IS NULL OR `component` = '''')
      AND `component_name` IS NOT NULL
      AND LENGTH(`component_name`) > 0'
);

PREPARE stmt_admin_menu_component_backfill_110 FROM @sql_admin_menu_component_backfill_110;
EXECUTE stmt_admin_menu_component_backfill_110;
DEALLOCATE PREPARE stmt_admin_menu_component_backfill_110;

SET @admin_role_exists_110 = (
  SELECT COUNT(*)
  FROM information_schema.tables
  WHERE table_schema = DATABASE()
    AND table_name = 't_admin_role'
);

SET @admin_menu_exists_110 = (
  SELECT COUNT(*)
  FROM information_schema.tables
  WHERE table_schema = DATABASE()
    AND table_name = 't_admin_menu'
);

SET @ddl_role_menu_110 = IF(
  @admin_role_exists_110 = 0 OR @admin_menu_exists_110 = 0,
  'SELECT ''skip sql/110_admin_menu_schema.sql t_admin_role_menu because parent tables are missing'' AS message',
  'CREATE TABLE IF NOT EXISTS `t_admin_role_menu` (
     `id` BIGINT UNSIGNED NOT NULL COMMENT ''角色菜单绑定主键ID'',
     `role_id` BIGINT UNSIGNED NOT NULL COMMENT ''角色ID'',
     `menu_id` BIGINT UNSIGNED NOT NULL COMMENT ''菜单ID'',
     `created_at` DATETIME NOT NULL COMMENT ''记录创建时间'',
     PRIMARY KEY (`id`),
     UNIQUE KEY `uk_role_menu` (`role_id`, `menu_id`),
     KEY `idx_menu_id` (`menu_id`),
     CONSTRAINT `fk_admin_role_menu_role_id` FOREIGN KEY (`role_id`) REFERENCES `t_admin_role` (`role_id`),
     CONSTRAINT `fk_admin_role_menu_menu_id` FOREIGN KEY (`menu_id`) REFERENCES `t_admin_menu` (`menu_id`)
   ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT=''角色与菜单绑定表'''
);

PREPARE stmt_role_menu_110 FROM @ddl_role_menu_110;
EXECUTE stmt_role_menu_110;
DEALLOCATE PREPARE stmt_role_menu_110;
