-- ============================================================
-- 完整初始化脚本
-- 说明：
-- 1. 本脚本按“当前代码实际依赖”的最终结构整理，适用于全新初始化数据库。
-- 2. 已内联管理员鉴权、转码、水印、缩略图、运行配置、Worker 能力、Outbox、Live 等全部当前主链路表。
-- 3. 已避免历史迁移脚本之间的重复列/旧结构问题。
-- 4. 若是存量库升级，请不要直接在生产库执行本脚本覆盖旧表，应改为按差异迁移。
-- 5. 本脚本不再强制 CREATE DATABASE / USE，请在目标数据库上下文中执行，例如 hvc。
-- ============================================================

SET NAMES utf8mb4;
SET FOREIGN_KEY_CHECKS = 0;

-- ============================================================
-- 1. 管理后台鉴权与审计
-- ============================================================

CREATE TABLE IF NOT EXISTS `t_admin_user` (
  `admin_user_id` BIGINT UNSIGNED NOT NULL COMMENT '管理员用户主键ID，由应用侧雪花算法生成',
  `username` VARCHAR(64) NOT NULL COMMENT '管理员登录用户名，系统内唯一',
  `password_hash` VARCHAR(255) NOT NULL COMMENT '管理员密码哈希值，禁止存储明文密码',
  `password_salt` VARCHAR(64) DEFAULT NULL COMMENT '密码盐值，为空时使用无盐哈希（兼容旧数据）',
  `display_name` VARCHAR(128) NOT NULL COMMENT '管理员展示名称，用于后台审计与界面展示',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '管理员状态，1=启用，2=禁用，3=锁定',
  `last_login_at` DATETIME DEFAULT NULL COMMENT '最近一次成功登录时间',
  `last_login_ip` VARCHAR(64) DEFAULT NULL COMMENT '最近一次成功登录来源IP',
  `created_at` DATETIME NOT NULL COMMENT '记录创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '记录更新时间',
  PRIMARY KEY (`admin_user_id`),
  UNIQUE KEY `uk_username` (`username`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='管理员用户表，用于保存后台登录账户与状态';

CREATE TABLE IF NOT EXISTS `t_admin_session` (
  `session_id` BIGINT UNSIGNED NOT NULL COMMENT '管理员会话主键ID，由应用侧雪花算法生成',
  `admin_user_id` BIGINT UNSIGNED NOT NULL COMMENT '所属管理员用户ID，对应 t_admin_user.admin_user_id',
  `session_token` VARCHAR(255) NOT NULL COMMENT '后台会话令牌，用于 Cookie 或 Header 鉴权',
  `session_status` TINYINT NOT NULL DEFAULT 1 COMMENT '会话状态，1=生效中，2=已退出，3=已过期，4=已吊销',
  `login_ip` VARCHAR(64) DEFAULT NULL COMMENT '本次登录来源IP',
  `user_agent` VARCHAR(512) DEFAULT NULL COMMENT '本次登录请求的 User-Agent',
  `expire_at` DATETIME NOT NULL COMMENT '会话过期时间，超过后应拒绝继续访问后台接口',
  `last_seen_at` DATETIME DEFAULT NULL COMMENT '最近一次访问后台接口时间',
  `created_at` DATETIME NOT NULL COMMENT '记录创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '记录更新时间',
  PRIMARY KEY (`session_id`),
  UNIQUE KEY `uk_session_token` (`session_token`),
  KEY `idx_admin_user_id` (`admin_user_id`),
  KEY `idx_session_status_expire_at` (`session_status`, `expire_at`),
  CONSTRAINT `fk_admin_session_admin_user_id` FOREIGN KEY (`admin_user_id`) REFERENCES `t_admin_user` (`admin_user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='管理员会话表，用于支持 Cookie 与 Header 双通道后台鉴权';

CREATE TABLE IF NOT EXISTS `t_admin_audit_log` (
  `audit_log_id` BIGINT UNSIGNED NOT NULL COMMENT '管理员审计日志主键ID，由应用侧雪花算法生成',
  `admin_user_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '操作管理员用户ID；未登录操作可为空',
  `username` VARCHAR(64) DEFAULT NULL COMMENT '操作时的管理员用户名快照',
  `action_name` VARCHAR(128) NOT NULL COMMENT '后台操作名称，例如 admin.login、config.publish、job.retry',
  `target_type` VARCHAR(64) DEFAULT NULL COMMENT '操作对象类型，例如 transcode_job、runtime_config、live_channel',
  `target_id` VARCHAR(128) DEFAULT NULL COMMENT '操作对象主键或业务标识',
  `request_id` VARCHAR(64) DEFAULT NULL COMMENT '关联请求ID，便于按业务请求串联审计与幂等链路',
  `request_ip` VARCHAR(64) DEFAULT NULL COMMENT '请求来源IP',
  `request_user_agent` VARCHAR(512) DEFAULT NULL COMMENT '请求 User-Agent',
  `result_code` INT NOT NULL DEFAULT 0 COMMENT '操作结果码，0 表示成功，其它值表示失败',
  `result_message` VARCHAR(1024) DEFAULT NULL COMMENT '操作结果描述或失败原因',
  `created_at` DATETIME NOT NULL COMMENT '记录创建时间',
  PRIMARY KEY (`audit_log_id`),
  KEY `idx_admin_user_id_created_at` (`admin_user_id`, `created_at`),
  KEY `idx_action_name_created_at` (`action_name`, `created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='管理员审计日志表，用于记录后台登录、配置变更和任务管理等关键操作';

CREATE TABLE IF NOT EXISTS `t_admin_role` (
  `role_id` BIGINT UNSIGNED NOT NULL COMMENT '角色主键ID',
  `role_key` VARCHAR(64) NOT NULL COMMENT '角色唯一键，例如 super_admin、operator、viewer',
  `role_name` VARCHAR(128) NOT NULL COMMENT '角色名称',
  `role_desc` VARCHAR(512) DEFAULT NULL COMMENT '角色说明',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '角色状态，1=启用，2=禁用',
  `created_at` DATETIME NOT NULL COMMENT '记录创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '记录更新时间',
  PRIMARY KEY (`role_id`),
  UNIQUE KEY `uk_role_key` (`role_key`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='管理员角色表';

CREATE TABLE IF NOT EXISTS `t_admin_permission` (
  `perm_id` BIGINT UNSIGNED NOT NULL COMMENT '权限点主键ID',
  `perm_key` VARCHAR(128) NOT NULL COMMENT '权限唯一键，例如 config.version.read',
  `perm_name` VARCHAR(128) NOT NULL COMMENT '权限名称',
  `perm_desc` VARCHAR(512) DEFAULT NULL COMMENT '权限说明',
  `module` VARCHAR(64) DEFAULT NULL COMMENT '所属模块',
  `created_at` DATETIME NOT NULL COMMENT '记录创建时间',
  PRIMARY KEY (`perm_id`),
  UNIQUE KEY `uk_perm_key` (`perm_key`),
  KEY `idx_module` (`module`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='管理员权限点表';

CREATE TABLE IF NOT EXISTS `t_admin_user_role` (
  `id` BIGINT UNSIGNED NOT NULL COMMENT '用户角色绑定主键ID',
  `user_id` BIGINT UNSIGNED NOT NULL COMMENT '管理员用户ID',
  `role_id` BIGINT UNSIGNED NOT NULL COMMENT '角色ID',
  `created_at` DATETIME NOT NULL COMMENT '记录创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_role` (`user_id`, `role_id`),
  KEY `idx_role_id` (`role_id`),
  CONSTRAINT `fk_admin_user_role_user_id` FOREIGN KEY (`user_id`) REFERENCES `t_admin_user` (`admin_user_id`),
  CONSTRAINT `fk_admin_user_role_role_id` FOREIGN KEY (`role_id`) REFERENCES `t_admin_role` (`role_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='管理员用户与角色绑定表';

CREATE TABLE IF NOT EXISTS `t_admin_role_permission` (
  `id` BIGINT UNSIGNED NOT NULL COMMENT '角色权限绑定主键ID',
  `role_id` BIGINT UNSIGNED NOT NULL COMMENT '角色ID',
  `perm_id` BIGINT UNSIGNED NOT NULL COMMENT '权限点ID',
  `created_at` DATETIME NOT NULL COMMENT '记录创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_role_perm` (`role_id`, `perm_id`),
  KEY `idx_perm_id` (`perm_id`),
  CONSTRAINT `fk_admin_role_permission_role_id` FOREIGN KEY (`role_id`) REFERENCES `t_admin_role` (`role_id`),
  CONSTRAINT `fk_admin_role_permission_perm_id` FOREIGN KEY (`perm_id`) REFERENCES `t_admin_permission` (`perm_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='角色与权限点绑定表';

-- ============================================================
-- 2. 点播转码任务主链路
-- ============================================================

CREATE TABLE IF NOT EXISTS `t_transcode_job` (
  `job_id` BIGINT UNSIGNED NOT NULL COMMENT '转码任务主键ID，由应用侧雪花算法生成，用于数据库内部唯一标识一条转码任务记录',
  `request_id` VARCHAR(64) NOT NULL COMMENT '外部请求唯一标识，用于实现跨 HTTP、gRPC、MQ 三种入口的幂等控制',
  `biz_key` VARCHAR(128) DEFAULT NULL COMMENT '业务侧资源主键，例如视频ID、节目ID、课程ID等，便于业务系统回查',
  `mode` TINYINT NOT NULL COMMENT '任务模式，1 表示点播转码，2 表示直播转码；当前主要处理点播转码',
  `status` TINYINT NOT NULL COMMENT '任务当前状态，1=已创建，2=排队中，3=已分配，4=执行中，5=上传中，6=已完成，7=失败，8=已取消',
  `priority` INT NOT NULL DEFAULT 0 COMMENT '任务优先级，数值越大表示优先级越高，调度器可优先调度更高优先级任务',
  `source_url` VARCHAR(2048) NOT NULL COMMENT '源视频地址，当前主要支持 HTTP/HTTPS，可扩展到对象存储签名地址等来源',
  `source_protocol` TINYINT NOT NULL DEFAULT 2 COMMENT '源地址协议类型，1=HTTP，2=HTTPS，3=其他扩展协议',
  `profile_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '转码模板 Profile 主键ID',
  `job_config_version` BIGINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '任务锁定的配置版本号，保证任务执行期间的配置快照可追溯',
  `segment_duration_sec` INT NOT NULL DEFAULT 4 COMMENT '目标分片时长，单位为秒',
  `segment_template` VARCHAR(255) DEFAULT NULL COMMENT '该任务首次执行时锁定的分片命名模板快照，用于保证重试和动态清单生成一致',
  `support_dash` TINYINT NOT NULL DEFAULT 1 COMMENT '是否支持 DASH 协议，0 表示不支持，1 表示支持',
  `support_hls` TINYINT NOT NULL DEFAULT 1 COMMENT '是否支持 HLS 协议，0 表示不支持，1 表示支持',
  `enable_watermark` TINYINT NOT NULL DEFAULT 0 COMMENT '是否启用水印，0=否，1=是',
  `watermark_image_url` VARCHAR(2048) DEFAULT NULL COMMENT '水印图片地址，可为 HTTP/HTTPS 或对象存储可访问地址',
  `watermark_anchor` TINYINT NOT NULL DEFAULT 1 COMMENT '水印锚点，1=左上，2=右上，3=左下，4=右下',
  `watermark_x_ratio` DECIMAL(6,5) NOT NULL DEFAULT 0.05000 COMMENT '水印横向偏移比例，相对输出宽度',
  `watermark_y_ratio` DECIMAL(6,5) NOT NULL DEFAULT 0.05000 COMMENT '水印纵向偏移比例，相对输出高度',
  `watermark_width_ratio` DECIMAL(6,5) NOT NULL DEFAULT 0.12000 COMMENT '水印宽度比例，相对输出宽度',
  `watermark_opacity` DECIMAL(6,5) NOT NULL DEFAULT 1.00000 COMMENT '水印透明度，范围 0 到 1',
  `enable_thumbnail_sprite` TINYINT NOT NULL DEFAULT 0 COMMENT '是否启用雪碧图缩略图，0=否，1=是',
  `thumb_rows` INT NOT NULL DEFAULT 10 COMMENT '雪碧图行数',
  `thumb_cols` INT NOT NULL DEFAULT 10 COMMENT '雪碧图列数',
  `thumb_interval_sec` INT NOT NULL DEFAULT 10 COMMENT '缩略图抽帧间隔秒数',
  `thumb_width` INT NOT NULL DEFAULT 320 COMMENT '单个缩略图宽度',
  `thumb_height` INT NOT NULL DEFAULT 180 COMMENT '单个缩略图高度',
  `thumb_image_format` VARCHAR(16) NOT NULL DEFAULT 'jpeg' COMMENT '雪碧图格式，支持 jpeg/png',
  `thumb_storage_prefix` VARCHAR(256) NOT NULL DEFAULT 'thumbnails/' COMMENT '雪碧图存储相对前缀',
  `enable_thumbnail_binary_index` TINYINT NOT NULL DEFAULT 0 COMMENT '是否启用缩略图二进制索引，0=否，1=是',
  `thumb_binary_storage_prefix` VARCHAR(256) NOT NULL DEFAULT 'thumbnails/' COMMENT '缩略图二进制索引存储相对前缀',
  `thumb_binary_max_size_bytes` BIGINT UNSIGNED NOT NULL DEFAULT 10485760 COMMENT '单个缩略图二进制索引包最大字节数',
  `output_storage_id` BIGINT UNSIGNED DEFAULT 0 COMMENT '输出对象存储配置ID，第一阶段允许保留占位值，后续接入真实 S3 配置表',
  `output_base_prefix` VARCHAR(256) DEFAULT '' COMMENT '输出对象存储根前缀，用于统一组织该任务的分片与缩略图等对象路径',
  `assigned_node_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '最近一次调度命中的节点ID；真正执行权以租约和执行代次为准',
  `assigned_worker_id` VARCHAR(64) DEFAULT NULL COMMENT '最近一次执行该任务的 Worker 实例标识，用于快速定位',
  `executor_worker_instance_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '当前或最近一次持有执行权的 Worker 实例记录ID',
  `selected_execution_hwaccel` VARCHAR(32) DEFAULT NULL COMMENT '调度阶段最终选中的执行模式，例如 nvidia/intel_qsv/amd_amf/vaapi/apple_videotoolbox/software',
  `selected_gpu_index` INT NOT NULL DEFAULT 0 COMMENT '调度阶段选中的 GPU 设备索引，仅用于诊断展示',
  `selected_gpu_device_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '调度阶段选中的稳定 GPU 设备记录ID，用于多 GPU 节点避免因索引漂移导致误调度',
  `lease_owner` VARCHAR(64) DEFAULT NULL COMMENT '任务租约持有者标识，防止多个调度器或多个 Worker 重复消费同一任务',
  `lease_generation` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '任务租约代次，每次重新接管任务时递增，用于拒绝陈旧执行实例回写',
  `attempt_no` INT NOT NULL DEFAULT 0 COMMENT '任务执行尝试次数，每次重新开始执行时递增',
  `lease_expire_at` DATETIME DEFAULT NULL COMMENT '任务租约过期时间，超过该时间可被其他调度实例重新接管',
  `last_worker_heartbeat_at` DATETIME DEFAULT NULL COMMENT '当前执行实例最后一次心跳时间，用于更快判断执行权是否失活',
  `progress_permille` INT NOT NULL DEFAULT 0 COMMENT '任务总体进度千分比，范围通常为 0 到 1000',
  `last_retry_mode` VARCHAR(16) NOT NULL DEFAULT 'resume' COMMENT '最近一次管理员重试模式，resume=尽量沿用已有进度，restart=从头开始',
  `error_code` VARCHAR(64) DEFAULT NULL COMMENT '任务失败错误码，便于程序判断失败类型和自动化恢复策略',
  `error_message` VARCHAR(1024) DEFAULT NULL COMMENT '任务失败错误详情，便于排查问题与后台展示',
  `created_at` DATETIME NOT NULL COMMENT '任务创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '任务最近更新时间',
  PRIMARY KEY (`job_id`),
  UNIQUE KEY `uk_request_id` (`request_id`),
  KEY `idx_status_priority_created` (`status`, `priority`, `created_at`),
  KEY `idx_assigned_worker_lease_status` (`assigned_worker_id`, `lease_owner`, `status`, `lease_expire_at`),
  KEY `idx_executor_worker_instance_id` (`executor_worker_instance_id`),
  KEY `idx_selected_gpu_device_id` (`selected_gpu_device_id`),
  KEY `idx_lease_expire_at` (`lease_expire_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='转码任务主表，保存任务主状态、输入参数摘要、调度状态与执行状态';

CREATE TABLE IF NOT EXISTS `t_transcode_job_request_override` (
  `id` BIGINT UNSIGNED NOT NULL COMMENT '覆盖参数记录主键ID',
  `job_id` BIGINT UNSIGNED NOT NULL COMMENT '所属转码任务ID，对应 t_transcode_job.job_id',
  `override_profile_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '请求级覆盖后的 Profile ID，用于覆盖默认模板配置',
  `override_segment_duration_sec` INT DEFAULT NULL COMMENT '请求级覆盖后的目标分片时长，单位为秒',
  `override_preferred_hwaccel` VARCHAR(32) DEFAULT NULL COMMENT '请求级指定的硬件加速偏好，例如 nvidia、intel、amd',
  `override_enable_watermark` TINYINT DEFAULT NULL COMMENT '请求级覆盖后的是否启用水印标记',
  `override_watermark_image_url` VARCHAR(2048) DEFAULT NULL COMMENT '请求级覆盖后的水印图片地址',
  `override_watermark_anchor` TINYINT DEFAULT NULL COMMENT '请求级覆盖后的水印锚点',
  `override_watermark_x_ratio` DECIMAL(6,5) DEFAULT NULL COMMENT '请求级覆盖后的水印横向偏移比例',
  `override_watermark_y_ratio` DECIMAL(6,5) DEFAULT NULL COMMENT '请求级覆盖后的水印纵向偏移比例',
  `override_watermark_width_ratio` DECIMAL(6,5) DEFAULT NULL COMMENT '请求级覆盖后的水印宽度比例',
  `override_watermark_opacity` DECIMAL(6,5) DEFAULT NULL COMMENT '请求级覆盖后的水印透明度',
  `override_enable_thumbnail_sprite` TINYINT DEFAULT NULL COMMENT '请求级覆盖后的缩略图开关',
  `override_thumb_rows` INT DEFAULT NULL COMMENT '请求级覆盖后的雪碧图行数',
  `override_thumb_cols` INT DEFAULT NULL COMMENT '请求级覆盖后的雪碧图列数',
  `override_thumb_interval_sec` INT DEFAULT NULL COMMENT '请求级覆盖后的缩略图抽帧间隔秒数',
  `override_thumb_width` INT DEFAULT NULL COMMENT '请求级覆盖后的缩略图宽度',
  `override_thumb_height` INT DEFAULT NULL COMMENT '请求级覆盖后的缩略图高度',
  `override_thumb_image_format` VARCHAR(16) DEFAULT NULL COMMENT '请求级覆盖后的雪碧图格式',
  `override_thumb_storage_prefix` VARCHAR(256) DEFAULT NULL COMMENT '请求级覆盖后的雪碧图存储前缀',
  `override_enable_thumbnail_binary_index` TINYINT DEFAULT NULL COMMENT '请求级覆盖后的缩略图二进制索引开关',
  `override_thumb_binary_storage_prefix` VARCHAR(256) DEFAULT NULL COMMENT '请求级覆盖后的缩略图二进制索引存储前缀',
  `override_thumb_binary_max_size_bytes` BIGINT UNSIGNED DEFAULT NULL COMMENT '请求级覆盖后的单个缩略图二进制索引包最大字节数',
  `override_bucket_prefix` VARCHAR(256) DEFAULT NULL COMMENT '请求级覆盖后的对象存储根前缀',
  `override_segment_prefix` VARCHAR(256) DEFAULT NULL COMMENT '请求级覆盖后的分片输出前缀',
  `override_callback_url` VARCHAR(2048) DEFAULT NULL COMMENT '请求级覆盖后的单任务回调目标，支持 HTTP/gRPC/MQ',
  `created_at` DATETIME NOT NULL COMMENT '记录创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_job_id` (`job_id`),
  CONSTRAINT `fk_job_override_job_id` FOREIGN KEY (`job_id`) REFERENCES `t_transcode_job` (`job_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='单任务覆盖参数表，用于记录外部请求对默认配置的个性化覆盖项';

CREATE TABLE IF NOT EXISTS `t_transcode_profile` (
  `profile_id` BIGINT UNSIGNED NOT NULL COMMENT '转码模板主键ID，由应用侧雪花算法生成',
  `profile_name` VARCHAR(128) NOT NULL COMMENT '模板名称，便于后台展示与业务选择',
  `biz_code` VARCHAR(64) DEFAULT NULL COMMENT '业务编码，用于按业务侧约定快速识别模板',
  `container_format` VARCHAR(32) NOT NULL DEFAULT 'fmp4' COMMENT '默认封装格式，例如 fmp4、ts',
  `segment_duration_sec` INT NOT NULL DEFAULT 4 COMMENT '默认目标分片时长，单位秒',
  `video_codec` VARCHAR(32) NOT NULL DEFAULT 'h264' COMMENT '默认视频编码名称',
  `audio_codec` VARCHAR(32) NOT NULL DEFAULT 'aac' COMMENT '默认音频编码名称',
  `enabled` TINYINT NOT NULL DEFAULT 1 COMMENT '模板是否启用，0=禁用，1=启用',
  `created_at` DATETIME NOT NULL COMMENT '记录创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '记录更新时间',
  PRIMARY KEY (`profile_id`),
  UNIQUE KEY `uk_profile_name` (`profile_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='转码模板主表，用于定义点播和直播可复用的转码输出策略';

CREATE TABLE IF NOT EXISTS `t_transcode_profile_rendition` (
  `profile_rendition_id` BIGINT UNSIGNED NOT NULL COMMENT '模板清晰度记录主键ID，由应用侧雪花算法生成',
  `profile_id` BIGINT UNSIGNED NOT NULL COMMENT '所属转码模板ID，对应 t_transcode_profile.profile_id',
  `rendition_name` VARCHAR(64) NOT NULL COMMENT '清晰度名称，例如 source、1080p、720p',
  `enabled` TINYINT NOT NULL DEFAULT 1 COMMENT '该档位是否启用，0=禁用，1=启用',
  `out_width` INT DEFAULT NULL COMMENT '输出宽度，单位像素',
  `out_height` INT DEFAULT NULL COMMENT '输出高度，单位像素',
  `video_bitrate_kbps` INT DEFAULT NULL COMMENT '目标视频码率，单位 kbps',
  `audio_bitrate_kbps` INT DEFAULT NULL COMMENT '目标音频码率，单位 kbps',
  `fps` DECIMAL(6,2) DEFAULT NULL COMMENT '目标帧率',
  `created_at` DATETIME NOT NULL COMMENT '记录创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '记录更新时间',
  PRIMARY KEY (`profile_rendition_id`),
  KEY `idx_profile_id` (`profile_id`),
  CONSTRAINT `fk_profile_rendition_profile_id` FOREIGN KEY (`profile_id`) REFERENCES `t_transcode_profile` (`profile_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='转码模板清晰度档位表，用于定义每个模板下的输出档位参数';

CREATE TABLE IF NOT EXISTS `t_storage_config` (
  `storage_id` BIGINT UNSIGNED NOT NULL COMMENT '对象存储配置主键ID，由应用侧雪花算法生成',
  `storage_name` VARCHAR(128) NOT NULL COMMENT '对象存储配置名称',
  `provider_type` VARCHAR(32) NOT NULL COMMENT '存储提供商类型，例如 s3、minio、oss、cos',
  `endpoint` VARCHAR(512) NOT NULL COMMENT '对象存储接入地址',
  `bucket_name` VARCHAR(128) NOT NULL COMMENT '默认桶名称',
  `region_name` VARCHAR(64) DEFAULT NULL COMMENT '区域名称',
  `access_key_id` VARCHAR(255) DEFAULT NULL COMMENT '访问密钥ID',
  `secret_access_key` VARCHAR(255) DEFAULT NULL COMMENT '访问密钥 Secret，后续应改为密文存储',
  `base_prefix` VARCHAR(255) DEFAULT NULL COMMENT '默认对象前缀',
  `enabled` TINYINT NOT NULL DEFAULT 1 COMMENT '配置是否启用，0=禁用，1=启用',
  `priority` INT NOT NULL DEFAULT 0 COMMENT '配置优先级，值越大优先级越高',
  `created_at` DATETIME NOT NULL COMMENT '记录创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '记录更新时间',
  PRIMARY KEY (`storage_id`),
  UNIQUE KEY `uk_storage_name` (`storage_name`),
  KEY `idx_enabled_priority_updated` (`enabled`, `priority`, `updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='对象存储配置表，用于统一管理点播和直播的产物上传目标';

CREATE TABLE IF NOT EXISTS `t_transcode_rendition` (
  `rendition_id` BIGINT UNSIGNED NOT NULL COMMENT '清晰度子任务主键ID',
  `job_id` BIGINT UNSIGNED NOT NULL COMMENT '所属转码任务ID，对应主任务表',
  `rendition_name` VARCHAR(64) NOT NULL COMMENT '清晰度名称，例如 1080p、720p、540p，用于业务展示与播放侧标识',
  `rendition_key` VARCHAR(32) NOT NULL COMMENT '任务内清晰度稳定短 key，用于分片命名和重试复用',
  `status` TINYINT NOT NULL COMMENT '清晰度子任务状态，1=待执行，2=执行中，3=上传中，4=已完成，5=失败',
  `out_width` INT NOT NULL COMMENT '该清晰度输出视频宽度，单位为像素',
  `out_height` INT NOT NULL COMMENT '该清晰度输出视频高度，单位为像素',
  `video_codec` VARCHAR(32) NOT NULL COMMENT '输出视频编码名称，例如 h264、hevc',
  `audio_codec` VARCHAR(32) NOT NULL COMMENT '输出音频编码名称，例如 aac',
  `video_bitrate_kbps` INT NOT NULL COMMENT '目标视频码率，单位为 kbps',
  `audio_bitrate_kbps` INT NOT NULL COMMENT '目标音频码率，单位为 kbps',
  `segment_count_video` INT NOT NULL DEFAULT 0 COMMENT '当前清晰度已生成的视频分片数量',
  `segment_count_audio` INT NOT NULL DEFAULT 0 COMMENT '当前清晰度已生成的音频分片数量',
  `progress_permille` INT NOT NULL DEFAULT 0 COMMENT '当前清晰度执行进度千分比',
  `error_code` VARCHAR(64) DEFAULT NULL COMMENT '当前清晰度失败错误码',
  `error_message` VARCHAR(1024) DEFAULT NULL COMMENT '当前清晰度失败错误详情',
  `created_at` DATETIME NOT NULL COMMENT '记录创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '记录更新时间',
  PRIMARY KEY (`rendition_id`),
  KEY `idx_job_id` (`job_id`),
  UNIQUE KEY `uk_job_rendition_name` (`job_id`, `rendition_name`),
  CONSTRAINT `fk_rendition_job_id` FOREIGN KEY (`job_id`) REFERENCES `t_transcode_job` (`job_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='任务清晰度子任务表，用于记录每个输出档位的执行状态与转码参数';

CREATE TABLE IF NOT EXISTS `t_transcode_segment` (
  `segment_id` BIGINT UNSIGNED NOT NULL COMMENT '分片记录主键ID',
  `job_id` BIGINT UNSIGNED NOT NULL COMMENT '所属转码任务ID',
  `rendition_id` BIGINT UNSIGNED NOT NULL COMMENT '所属清晰度子任务ID',
  `rendition_name` VARCHAR(64) NOT NULL COMMENT '分片所属清晰度名称，做动态清单和排障时可避免额外 join',
  `rendition_key` VARCHAR(32) NOT NULL COMMENT '分片所属清晰度稳定短 key，便于按模板重建 DASH media URL',
  `media_type` TINYINT NOT NULL COMMENT '媒体轨道类型，1=视频，2=音频',
  `is_init_segment` TINYINT NOT NULL DEFAULT 0 COMMENT '是否为初始化分片，0=否，1=是',
  `sequence_no` INT NOT NULL COMMENT '分片顺序号；初始化分片可固定为0，媒体分片按时间顺序递增',
  `duration_ms` INT NOT NULL COMMENT '分片时长，单位为毫秒',
  `width` INT NOT NULL DEFAULT 0 COMMENT '该分片所属视频输出宽度，单位像素',
  `height` INT NOT NULL DEFAULT 0 COMMENT '该分片所属视频输出高度，单位像素',
  `video_bitrate_kbps` INT NOT NULL DEFAULT 0 COMMENT '该分片所属视频码率，单位 kbps',
  `audio_bitrate_kbps` INT NOT NULL DEFAULT 0 COMMENT '该分片所属音频码率，单位 kbps',
  `video_codec` VARCHAR(32) DEFAULT NULL COMMENT '该分片所属视频编码名称，例如 h264、hevc',
  `audio_codec` VARCHAR(32) DEFAULT NULL COMMENT '该分片所属音频编码名称，例如 aac',
  `support_dash` TINYINT NOT NULL DEFAULT 1 COMMENT '该分片是否可用于 DASH 播放',
  `support_hls` TINYINT NOT NULL DEFAULT 1 COMMENT '该分片是否可用于 HLS 播放',
  `codec_name` VARCHAR(32) DEFAULT NULL COMMENT '该分片对应的编码名称，例如 h264、aac',
  `object_key` VARCHAR(1024) NOT NULL COMMENT '对象存储 Key，用于定位该分片在对象存储中的实际路径',
  `object_size_bytes` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '对象大小，单位为字节',
  `object_etag` VARCHAR(128) DEFAULT NULL COMMENT '对象存储返回的 ETag，用于校验对象版本或完整性',
  `sha256` CHAR(64) DEFAULT NULL COMMENT '分片内容的 SHA256 摘要值，用于完整性校验与去重判断',
  `start_pts_ms` BIGINT DEFAULT NULL COMMENT '分片起始 PTS（毫秒）',
  `end_pts_ms` BIGINT DEFAULT NULL COMMENT '分片结束 PTS（毫秒）',
  `upload_status` TINYINT NOT NULL COMMENT '上传状态，1=待上传，2=上传中，3=上传成功，4=上传失败',
  `upload_retry_count` INT NOT NULL DEFAULT 0 COMMENT '上传失败后的累计重试次数',
  `upload_error_message` VARCHAR(1024) DEFAULT NULL COMMENT '上传失败错误信息',
  `created_at` DATETIME NOT NULL COMMENT '记录创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '记录更新时间',
  PRIMARY KEY (`segment_id`),
  UNIQUE KEY `uk_rendition_media_seq_init` (`rendition_id`, `media_type`, `sequence_no`, `is_init_segment`),
  KEY `idx_job_id` (`job_id`),
  KEY `idx_job_media` (`job_id`, `media_type`, `is_init_segment`, `sequence_no`),
  KEY `idx_upload_status` (`upload_status`, `updated_at`),
  CONSTRAINT `fk_segment_job_id` FOREIGN KEY (`job_id`) REFERENCES `t_transcode_job` (`job_id`),
  CONSTRAINT `fk_segment_rendition_id` FOREIGN KEY (`rendition_id`) REFERENCES `t_transcode_rendition` (`rendition_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='点播分片元数据表，仅保存 init/media 分片对象元数据，不保存任何 MPD、M3U8 或 playlist 文件';

CREATE TABLE IF NOT EXISTS `t_transcode_thumbnail_sprite` (
  `sprite_id` BIGINT UNSIGNED NOT NULL COMMENT '雪碧图记录主键ID',
  `job_id` BIGINT UNSIGNED NOT NULL COMMENT '所属转码任务ID',
  `sprite_no` INT NOT NULL COMMENT '雪碧图顺序号，从 1 开始递增',
  `rows_count` INT NOT NULL DEFAULT 10 COMMENT '雪碧图行数',
  `cols_count` INT NOT NULL DEFAULT 10 COMMENT '雪碧图列数',
  `thumb_count` INT NOT NULL DEFAULT 0 COMMENT '当前雪碧图中包含的缩略图数量',
  `thumb_width` INT NOT NULL COMMENT '单个缩略图宽度，单位为像素',
  `thumb_height` INT NOT NULL COMMENT '单个缩略图高度，单位为像素',
  `image_format` VARCHAR(16) NOT NULL COMMENT '雪碧图图片格式，例如 jpeg、png、webp',
  `object_key` VARCHAR(1024) NOT NULL COMMENT '雪碧图在对象存储中的对象路径',
  `object_size_bytes` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '雪碧图文件大小，单位为字节',
  `upload_status` TINYINT NOT NULL COMMENT '上传状态，1=待上传，2=上传中，3=上传成功，4=上传失败',
  `upload_error_message` VARCHAR(1024) DEFAULT NULL COMMENT '上传失败错误详情',
  `created_at` DATETIME NOT NULL COMMENT '记录创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '记录更新时间',
  PRIMARY KEY (`sprite_id`),
  UNIQUE KEY `uk_job_sprite_no` (`job_id`, `sprite_no`),
  CONSTRAINT `fk_sprite_job_id` FOREIGN KEY (`job_id`) REFERENCES `t_transcode_job` (`job_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='缩略图雪碧图文件表，保存任务级雪碧图对象元数据与上传结果';

CREATE TABLE IF NOT EXISTS `t_transcode_thumbnail_bin` (
  `bin_id` BIGINT UNSIGNED NOT NULL COMMENT '缩略图二进制索引包记录主键ID',
  `job_id` BIGINT UNSIGNED NOT NULL COMMENT '所属转码任务ID',
  `bin_no` INT NOT NULL COMMENT '二进制索引包顺序号，从1开始',
  `item_count` INT NOT NULL DEFAULT 0 COMMENT '该二进制索引包中缩略图项数量',
  `max_size_bytes` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '该二进制索引包最大字节数限制',
  `actual_size_bytes` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '该二进制索引包实际字节数',
  `object_key` VARCHAR(1024) NOT NULL COMMENT '二进制索引包对象路径',
  `created_at` DATETIME NOT NULL COMMENT '记录创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '记录更新时间',
  PRIMARY KEY (`bin_id`),
  UNIQUE KEY `uk_job_bin_no` (`job_id`, `bin_no`),
  KEY `idx_job_id` (`job_id`),
  CONSTRAINT `fk_thumbnail_bin_job_id` FOREIGN KEY (`job_id`) REFERENCES `t_transcode_job` (`job_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='缩略图二进制索引包表，保存任务级逗号分隔base64缩略图分包元数据';

CREATE TABLE IF NOT EXISTS `t_transcode_thumbnail_item` (
  `item_id` BIGINT UNSIGNED NOT NULL COMMENT '缩略图索引项主键ID',
  `job_id` BIGINT UNSIGNED NOT NULL COMMENT '所属转码任务ID',
  `item_index` INT NOT NULL COMMENT '缩略图项顺序号，从0开始',
  `capture_time_ms` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '截图时间戳（毫秒）',
  `sprite_no` INT NOT NULL DEFAULT 0 COMMENT '所属雪碧图编号，从1开始；未启用雪碧图时可为0',
  `sprite_row_index` INT NOT NULL DEFAULT 0 COMMENT '雪碧图内行下标，从0开始',
  `sprite_col_index` INT NOT NULL DEFAULT 0 COMMENT '雪碧图内列下标，从0开始',
  `bin_no` INT NOT NULL COMMENT '所属二进制索引包编号，从1开始',
  `bin_item_index` INT NOT NULL DEFAULT 0 COMMENT '在对应二进制索引包中的条目序号，从0开始',
  `base64_length` INT NOT NULL DEFAULT 0 COMMENT 'base64缩略图长度（字符数）',
  `thumb_sha256` CHAR(64) NOT NULL COMMENT '缩略图原始PNG数据SHA256',
  `created_at` DATETIME NOT NULL COMMENT '记录创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '记录更新时间',
  PRIMARY KEY (`item_id`),
  UNIQUE KEY `uk_job_item_index` (`job_id`, `item_index`),
  KEY `idx_job_bin_no` (`job_id`, `bin_no`),
  CONSTRAINT `fk_thumbnail_item_job_id` FOREIGN KEY (`job_id`) REFERENCES `t_transcode_job` (`job_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='缩略图索引项表，保存任务级缩略图时间点与雪碧图/bin条目定位信息';

-- ============================================================
-- 3. 回调 / Outbox / 失败重试
-- ============================================================

CREATE TABLE IF NOT EXISTS `t_callback_config` (
  `callback_config_id` BIGINT UNSIGNED NOT NULL COMMENT '回调配置主键ID',
  `callback_name` VARCHAR(128) NOT NULL COMMENT '回调配置名称，便于后台识别该配置的业务用途',
  `callback_type` TINYINT NOT NULL COMMENT '回调类型，1=HTTP，2=RPC，3=MQ',
  `target_url` VARCHAR(2048) DEFAULT NULL COMMENT 'HTTP 回调目标地址；若为非 HTTP 类型可为空',
  `rpc_endpoint` VARCHAR(512) DEFAULT NULL COMMENT 'RPC 回调目标 endpoint；仅当 callback_type 为 RPC 时使用',
  `rpc_service_name` VARCHAR(128) DEFAULT NULL COMMENT 'RPC 回调服务名称；仅当 callback_type 为 RPC 时使用',
  `mq_exchange` VARCHAR(128) DEFAULT NULL COMMENT 'MQ 回调交换机名称；仅当 callback_type 为 MQ 时使用',
  `mq_routing_key` VARCHAR(128) DEFAULT NULL COMMENT 'MQ 回调路由键；仅当 callback_type 为 MQ 时使用',
  `timeout_ms` INT NOT NULL DEFAULT 3000 COMMENT '回调超时时间，单位毫秒',
  `retry_times` INT NOT NULL DEFAULT 3 COMMENT '回调失败后的最大重试次数',
  `enabled` TINYINT NOT NULL DEFAULT 1 COMMENT '配置是否启用，0=禁用，1=启用',
  `priority` INT NOT NULL DEFAULT 0 COMMENT '配置优先级，值越大优先级越高',
  `registry_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'RPC 回调关联的 etcd 注册配置ID，0表示未绑定',
  `created_at` DATETIME NOT NULL COMMENT '记录创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '记录更新时间',
  PRIMARY KEY (`callback_config_id`),
  KEY `idx_type_enabled_priority_updated` (`callback_type`, `enabled`, `priority`, `updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='完成通知回调配置表，用于统一管理 HTTP、RPC、MQ 三种回调配置';

CREATE TABLE IF NOT EXISTS `t_registry_etcd_config` (
  `registry_id` BIGINT UNSIGNED NOT NULL COMMENT 'etcd 注册配置主键ID',
  `registry_name` VARCHAR(128) NOT NULL COMMENT 'etcd 注册配置名称',
  `endpoints` VARCHAR(1024) NOT NULL COMMENT 'etcd endpoints，多个地址使用逗号分隔',
  `service_namespace` VARCHAR(255) NOT NULL DEFAULT '/vod/transcoding' COMMENT '服务注册命名空间或前缀',
  `lease_ttl_sec` INT NOT NULL DEFAULT 15 COMMENT '服务租约 TTL，单位秒',
  `dial_timeout_ms` INT NOT NULL DEFAULT 3000 COMMENT '连接 etcd 超时时间，单位毫秒',
  `enabled` TINYINT NOT NULL DEFAULT 1 COMMENT '配置是否启用，0=禁用，1=启用',
  `priority` INT NOT NULL DEFAULT 0 COMMENT '配置优先级，值越大优先级越高',
  `created_at` DATETIME NOT NULL COMMENT '记录创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '记录更新时间',
  PRIMARY KEY (`registry_id`),
  UNIQUE KEY `uk_registry_name` (`registry_name`),
  KEY `idx_enabled_priority_updated` (`enabled`, `priority`, `updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='RPC 接收端 etcd 注册配置表';

CREATE TABLE IF NOT EXISTS `t_event_outbox` (
  `event_id` BIGINT UNSIGNED NOT NULL COMMENT '事件记录主键ID',
  `job_id` BIGINT UNSIGNED NOT NULL COMMENT '关联的转码任务ID',
  `request_id` VARCHAR(128) DEFAULT NULL COMMENT '关联请求ID，便于按外部请求链路排查',
  `event_type` VARCHAR(64) NOT NULL COMMENT '事件类型，例如 job.created、job.completed、job.failed',
  `payload_json` JSON NOT NULL COMMENT '事件负载 JSON 内容，用于后续异步投递给外部系统',
  `delivery_status` TINYINT NOT NULL DEFAULT 1 COMMENT '投递状态，1=待投递，2=投递中，3=投递成功，4=投递失败',
  `retry_count` INT NOT NULL DEFAULT 0 COMMENT '已重试次数',
  `max_retry_count` INT NOT NULL DEFAULT 3 COMMENT '最大重试次数，超过后转入最终失败',
  `next_retry_at` DATETIME DEFAULT NULL COMMENT '下次允许重试的时间点',
  `last_error_message` VARCHAR(1024) DEFAULT NULL COMMENT '最近一次投递失败错误详情，便于补偿和审计',
  `created_at` DATETIME NOT NULL COMMENT '记录创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '记录更新时间',
  PRIMARY KEY (`event_id`),
  KEY `idx_job_id` (`job_id`),
  KEY `idx_request_id` (`request_id`),
  KEY `idx_delivery_status` (`delivery_status`, `next_retry_at`, `retry_count`, `max_retry_count`),
  CONSTRAINT `fk_outbox_job_id` FOREIGN KEY (`job_id`) REFERENCES `t_transcode_job` (`job_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='事件外发表，用于把任务状态变化可靠投递到外部系统';

CREATE TABLE IF NOT EXISTS `t_delivery_failure_queue` (
  `failure_id` BIGINT UNSIGNED NOT NULL COMMENT '投递失败记录主键ID，由应用侧雪花算法生成',
  `event_id` BIGINT UNSIGNED NOT NULL COMMENT '关联的 outbox 事件ID，对应 t_event_outbox.event_id',
  `failure_stage` VARCHAR(64) NOT NULL COMMENT '失败阶段，例如 http.callback、rpc.callback、mq.publish',
  `failure_code` VARCHAR(64) DEFAULT NULL COMMENT '失败错误码',
  `failure_message` VARCHAR(1024) DEFAULT NULL COMMENT '失败错误详情',
  `callback_config_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '失败时命中的回调配置ID，0表示兼容回退链路',
  `callback_target` VARCHAR(2048) DEFAULT NULL COMMENT '失败时尝试的具体回调目标地址或路由键',
  `retry_count` INT NOT NULL DEFAULT 0 COMMENT '当前失败链路累计重试次数',
  `next_retry_at` DATETIME DEFAULT NULL COMMENT '下次允许重试时间',
  `resolved` TINYINT NOT NULL DEFAULT 0 COMMENT '是否已恢复，0=未恢复，1=已恢复',
  `created_at` DATETIME NOT NULL COMMENT '记录创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '记录更新时间',
  PRIMARY KEY (`failure_id`),
  KEY `idx_event_id` (`event_id`),
  KEY `idx_resolved_next_retry_at` (`resolved`, `next_retry_at`),
  KEY `idx_callback_config_id` (`callback_config_id`),
  CONSTRAINT `fk_delivery_failure_event_id` FOREIGN KEY (`event_id`) REFERENCES `t_event_outbox` (`event_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='外部投递失败队列表，用于保存 HTTP、RPC、MQ 投递失败事件以便补偿和排查';

-- ============================================================
-- 4. 运行配置
-- ============================================================

CREATE TABLE IF NOT EXISTS `t_runtime_config` (
  `enable_http_server` TINYINT NOT NULL DEFAULT 1 COMMENT '是否启用 HTTP 服务和 WebSocket 监控入口，0=否，1=是',
  `enable_callback` TINYINT NOT NULL DEFAULT 1 COMMENT '是否启用回调投递模块，0=否，1=是',
  `config_version` BIGINT UNSIGNED NOT NULL COMMENT '运行配置版本号，由应用侧递增控制',
  `default_profile_id` BIGINT UNSIGNED NOT NULL COMMENT '默认转码模板ID',
  `max_global_transcode_sessions` INT NOT NULL DEFAULT 10 COMMENT '全局最大转码并发数',
  `job_lease_ttl_sec` INT NOT NULL DEFAULT 60 COMMENT '任务租约过期时间，单位秒',
  `worker_heartbeat_timeout_sec` INT NOT NULL DEFAULT 20 COMMENT 'Worker 心跳超时时间，单位秒',
  `allow_request_override_profile` TINYINT NOT NULL DEFAULT 1 COMMENT '是否允许请求覆盖 profile_id，0=否，1=是',
  `allow_request_override_segment_duration` TINYINT NOT NULL DEFAULT 1 COMMENT '是否允许请求覆盖分片时长，0=否，1=是',
  `allow_request_override_hwaccel` TINYINT NOT NULL DEFAULT 1 COMMENT '是否允许请求覆盖硬件加速偏好，0=否，1=是',
  `published` TINYINT NOT NULL DEFAULT 0 COMMENT '该版本是否已发布，0=草稿，1=已发布',
  `scheduler_loop_interval_ms` INT NOT NULL DEFAULT 5000 COMMENT '调度器轮询间隔，单位毫秒',
  `worker_assigned_status` INT NOT NULL DEFAULT 3 COMMENT 'Worker 领取态状态码',
  `worker_probe_fail_progress_permille` INT NOT NULL DEFAULT 0 COMMENT '探测失败时回写进度千分比',
  `worker_upload_fail_progress_permille` INT NOT NULL DEFAULT 800 COMMENT '上传失败时回写进度千分比',
  `worker_success_progress_permille` INT NOT NULL DEFAULT 1000 COMMENT '任务成功时回写进度千分比',
  `worker_loop_interval_ms` INT NOT NULL DEFAULT 5000 COMMENT 'Worker 轮询间隔，单位毫秒',
  `rpc_loop_interval_ms` INT NOT NULL DEFAULT 15000 COMMENT 'RPC 角色健康检查间隔，单位毫秒',
  `mq_loop_interval_ms` INT NOT NULL DEFAULT 15000 COMMENT 'MQ 角色健康检查间隔，单位毫秒',
  `require_hardware_encode` TINYINT NOT NULL DEFAULT 1 COMMENT '是否强制所有输出视频使用硬件编码，0=否，1=是',
  `allow_software_decode_fallback` TINYINT NOT NULL DEFAULT 1 COMMENT '当源视频不支持硬件解码时是否允许软件解码回退，0=否，1=是',
  `soft_decode_cpu_limit_percent` INT NOT NULL DEFAULT 50 COMMENT '软解路径允许占用的 CPU 上限百分比，默认不得超过50',
  `node_cpu_safety_limit_percent` INT NOT NULL DEFAULT 85 COMMENT '节点 CPU 总保护阈值百分比，超过后必须主动收缩并发',
  `node_memory_safety_limit_percent` INT NOT NULL DEFAULT 85 COMMENT '节点内存总保护阈值百分比，超过后必须主动收缩并发',
  `node_gpu_memory_safety_limit_percent` INT NOT NULL DEFAULT 90 COMMENT '节点显存总保护阈值百分比，超过后必须主动收缩并发',
  `single_job_upload_concurrency_limit` INT NOT NULL DEFAULT 4 COMMENT '单任务分片上传并发上限',
  `dynamic_concurrency_control_enabled` TINYINT NOT NULL DEFAULT 1 COMMENT '是否启用基于实时资源的动态并发调控，0=否，1=是',
  `require_hardware_watermark` TINYINT NOT NULL DEFAULT 1 COMMENT '是否强制水印与图像处理优先走硬件滤镜链路，0=否，1=是',
  `worker_probe_fail_message` VARCHAR(255) NOT NULL COMMENT '探测失败默认消息',
  `worker_upload_fail_message` VARCHAR(255) NOT NULL COMMENT '上传失败默认消息',
  `worker_success_message` VARCHAR(255) NOT NULL COMMENT '任务成功默认消息',
  `callback_http_url` VARCHAR(2048) DEFAULT NULL COMMENT 'HTTP 回调地址（兼容旧单配置回退）',
  `callback_rpc_endpoint` VARCHAR(512) DEFAULT NULL COMMENT 'RPC 回调接入点（兼容旧单配置回退）',
  `callback_mq_topic` VARCHAR(255) DEFAULT NULL COMMENT 'MQ 回调主题或 routing key（兼容旧单配置回退）',
  `rpc_callback_receiver_enabled` TINYINT NOT NULL DEFAULT 1 COMMENT '是否启用 RPC 回调接收端监听，0=否，1=是',
  `rpc_callback_receiver_host` VARCHAR(255) NOT NULL DEFAULT '0.0.0.0' COMMENT 'RPC 回调接收端监听主机',
  `rpc_callback_receiver_port` INT NOT NULL DEFAULT 9090 COMMENT 'RPC 回调接收端监听端口',
  `rpc_callback_receiver_registry_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT 'RPC 回调接收端绑定的 etcd 注册配置ID，0表示未绑定',
  `enable_mq_consumer` TINYINT NOT NULL DEFAULT 0 COMMENT '是否启用 MQ 创建任务消费者，0=否，1=是',
  `mq_queue_name` VARCHAR(255) DEFAULT NULL COMMENT 'MQ 消费队列名称',
  `mq_host` VARCHAR(255) DEFAULT NULL COMMENT 'RabbitMQ 主机地址',
  `mq_port` INT NOT NULL DEFAULT 5672 COMMENT 'RabbitMQ 端口',
  `mq_username` VARCHAR(128) DEFAULT NULL COMMENT 'RabbitMQ 用户名',
  `mq_password` VARCHAR(255) DEFAULT NULL COMMENT 'RabbitMQ 密码',
  `mq_vhost` VARCHAR(64) NOT NULL DEFAULT '/' COMMENT 'RabbitMQ vhost',
  `mq_consumer_tag` VARCHAR(128) NOT NULL DEFAULT 'vod-mq-consumer' COMMENT 'RabbitMQ consumer tag',
  `mq_prefetch_count` INT NOT NULL DEFAULT 10 COMMENT 'RabbitMQ prefetch 数量',
  `storage_type` VARCHAR(32) NOT NULL DEFAULT 'local' COMMENT '运行期生效的存储类型，local 或 s3',
  `storage_endpoint` VARCHAR(255) DEFAULT NULL COMMENT '运行期生效的对象存储 endpoint',
  `storage_bucket` VARCHAR(128) DEFAULT NULL COMMENT '运行期生效的对象存储 bucket',
  `storage_access_key_id` VARCHAR(255) DEFAULT NULL COMMENT '运行期生效的对象存储 access key',
  `storage_secret_access_key` VARCHAR(255) DEFAULT NULL COMMENT '运行期生效的对象存储 secret key',
  `storage_use_ssl` TINYINT NOT NULL DEFAULT 0 COMMENT '运行期对象存储是否启用 SSL，0=否，1=是',
  `storage_play_domain` VARCHAR(255) DEFAULT NULL COMMENT '运行期直播播放域名',
  `storage_flv_domain` VARCHAR(255) DEFAULT NULL COMMENT '运行期 HTTP-FLV 播放域名',
  `storage_local_base_path` VARCHAR(1024) DEFAULT NULL COMMENT '运行期本地存储根目录',
  `default_storage_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '运行期默认存储配置ID，0表示直接使用内嵌存储参数',
  `scheduler_worker_id` VARCHAR(64) NOT NULL COMMENT '调度器分配给 Worker 的实例标识',
  `worker_output_path` VARCHAR(1024) DEFAULT NULL COMMENT 'Worker 本地临时输出目录或产物路径',
  `worker_object_prefix` VARCHAR(255) NOT NULL COMMENT '对象存储输出根前缀，仅用于分片与缩略图等对象路径组织',
  `change_summary` VARCHAR(1024) DEFAULT NULL COMMENT '本次配置变更摘要',
  `config_source` VARCHAR(64) DEFAULT NULL COMMENT '配置来源标识',
  `source_revision` VARCHAR(128) DEFAULT NULL COMMENT '来源修订号',
  `published_by` VARCHAR(128) DEFAULT NULL COMMENT '发布人',
  `published_at` DATETIME DEFAULT NULL COMMENT '发布时间',
  `effective_config_hash` VARCHAR(128) DEFAULT NULL COMMENT '有效配置摘要哈希',
  `created_at` DATETIME NOT NULL COMMENT '记录创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '记录更新时间',
  `public_grpc_enabled` TINYINT NOT NULL DEFAULT 0 COMMENT '是否启用对外 public gRPC 服务，0=否，1=是',
  `public_grpc_host` VARCHAR(255) NOT NULL DEFAULT '0.0.0.0' COMMENT '对外 public gRPC 监听主机',
  `public_grpc_port` INT NOT NULL DEFAULT 9090 COMMENT '对外 public gRPC 监听端口',
  PRIMARY KEY (`config_version`),
  KEY `idx_published_updated_at` (`published`, `updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='运行配置版本表，用于保存草稿配置与已发布配置';

-- ============================================================
-- 5. Worker / Cluster / 编解码能力
-- ============================================================

CREATE TABLE IF NOT EXISTS `t_cluster_node` (
  `node_id` BIGINT UNSIGNED NOT NULL COMMENT '节点ID',
  `node_name` VARCHAR(64) NOT NULL COMMENT '节点名称（唯一）',
  `host_ip` VARCHAR(64) NOT NULL COMMENT '节点IP',
  `grpc_host` VARCHAR(128) DEFAULT NULL COMMENT '节点RPC地址（含端口）',
  `http_host` VARCHAR(128) DEFAULT NULL COMMENT '节点HTTP地址（含端口）',
  `enabled` TINYINT NOT NULL DEFAULT 1 COMMENT '是否启用：0=否；1=是',
  `quarantined` TINYINT NOT NULL DEFAULT 0 COMMENT '是否隔离：0=否；1=是',
  `quarantine_reason` VARCHAR(256) DEFAULT NULL COMMENT '隔离原因',
  `last_state_change_at` DATETIME DEFAULT NULL COMMENT '最近一次节点状态变化时间',
  `capacity_generation` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '节点能力或容量变更代次，用于调度缓存失效',
  `support_nvenc` TINYINT NOT NULL DEFAULT 0 COMMENT '是否支持 NVIDIA NVENC',
  `support_qsv` TINYINT NOT NULL DEFAULT 0 COMMENT '是否支持 Intel QSV',
  `support_amf` TINYINT NOT NULL DEFAULT 0 COMMENT '是否支持 AMD AMF',
  `support_vaapi` TINYINT NOT NULL DEFAULT 0 COMMENT '是否支持 VAAPI',
  `support_videotoolbox` TINYINT NOT NULL DEFAULT 0 COMMENT '是否支持 Apple VideoToolbox',
  `cpu_cores` INT NOT NULL DEFAULT 0 COMMENT 'CPU 核心数',
  `memory_total_mb` INT NOT NULL DEFAULT 0 COMMENT '内存总量(MB)',
  `disk_total_gb` INT NOT NULL DEFAULT 0 COMMENT '磁盘总量(GB)',
  `net_up_mbps` INT NOT NULL DEFAULT 0 COMMENT '上行带宽(Mbps)',
  `net_down_mbps` INT NOT NULL DEFAULT 0 COMMENT '下行带宽(Mbps)',
  `max_transcode_sessions` INT NOT NULL DEFAULT 0 COMMENT '该节点最大同时转码会话数，达到上限后不得继续派发新任务',
  `max_upload_concurrency` INT NOT NULL DEFAULT 0 COMMENT '该节点最大上传并发，必须与后台动态限流联动',
  `node_tags` VARCHAR(256) DEFAULT NULL COMMENT '节点标签（逗号分隔）',
  `last_heartbeat_at` DATETIME DEFAULT NULL COMMENT '最后心跳时间',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '更新时间',
  PRIMARY KEY (`node_id`),
  UNIQUE KEY `uk_node_name` (`node_name`),
  KEY `idx_enabled_heartbeat` (`enabled`, `last_heartbeat_at`),
  KEY `idx_quarantined` (`quarantined`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='集群节点表';

CREATE TABLE IF NOT EXISTS `t_node_gpu_device` (
  `gpu_device_id` BIGINT UNSIGNED NOT NULL COMMENT '稳定 GPU 设备记录ID',
  `node_id` BIGINT UNSIGNED NOT NULL COMMENT '所属节点ID',
  `gpu_index` INT NOT NULL DEFAULT 0 COMMENT '当前进程观测到的 GPU 索引',
  `gpu_uuid` VARCHAR(128) NOT NULL COMMENT 'GPU 稳定硬件标识',
  `vendor` VARCHAR(32) DEFAULT NULL COMMENT 'GPU 厂商，例如 nvidia、intel、amd、apple',
  `model` VARCHAR(128) DEFAULT NULL COMMENT 'GPU 型号',
  `driver_version` VARCHAR(128) DEFAULT NULL COMMENT 'GPU 驱动版本',
  `memory_total_mb` INT NOT NULL DEFAULT 0 COMMENT '显存总量(MB)',
  `max_transcode_sessions` INT NOT NULL DEFAULT 0 COMMENT '该 GPU 允许的最大转码会话数，超过后必须拒绝继续绑定新任务',
  `healthy` TINYINT NOT NULL DEFAULT 1 COMMENT '健康状态，0=故障，1=健康',
  `schedulable` TINYINT NOT NULL DEFAULT 1 COMMENT '是否允许调度，0=否，1=是',
  `last_seen_at` DATETIME DEFAULT NULL COMMENT '最近一次被 Worker 上报时间',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '更新时间',
  PRIMARY KEY (`gpu_device_id`),
  UNIQUE KEY `uk_gpu_uuid` (`gpu_uuid`),
  UNIQUE KEY `uk_node_gpu_index` (`node_id`, `gpu_index`),
  KEY `idx_node_schedulable` (`node_id`, `schedulable`, `healthy`),
  CONSTRAINT `fk_gpu_device_node_id` FOREIGN KEY (`node_id`) REFERENCES `t_cluster_node` (`node_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='节点 GPU 设备表，用于为多 GPU 单机调度提供稳定硬件身份';

CREATE TABLE IF NOT EXISTS `t_worker_instance` (
  `id` BIGINT UNSIGNED NOT NULL COMMENT '记录ID',
  `node_id` BIGINT UNSIGNED NOT NULL COMMENT '节点ID',
  `worker_id` VARCHAR(64) NOT NULL COMMENT 'Worker 实例ID（唯一）',
  `logical_worker_id` VARCHAR(128) DEFAULT NULL COMMENT '逻辑 Worker 标识',
  `physical_worker_id` VARCHAR(128) DEFAULT NULL COMMENT '物理 Worker 标识（本次实例级）',
  `machine_fingerprint` VARCHAR(512) DEFAULT NULL COMMENT '机器指纹',
  `startup_instance_id` VARCHAR(128) DEFAULT NULL COMMENT '本次启动实例标识',
  `boot_id` VARCHAR(128) DEFAULT NULL COMMENT '系统启动或进程启动批次标识',
  `pid` INT DEFAULT NULL COMMENT '进程 PID（可选）',
  `version` VARCHAR(64) DEFAULT NULL COMMENT 'Worker 版本号',
  `status` TINYINT NOT NULL COMMENT '状态：1=在线；2=离线；3=隔离；4=已退出',
  `start_at` DATETIME DEFAULT NULL COMMENT '启动时间',
  `exited_at` DATETIME DEFAULT NULL COMMENT '退出时间',
  `exit_reason` VARCHAR(512) DEFAULT NULL COMMENT '退出原因',
  `last_heartbeat_at` DATETIME DEFAULT NULL COMMENT '最后心跳时间',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_worker` (`worker_id`),
  KEY `idx_node_status` (`node_id`, `status`),
  KEY `idx_startup_instance_id` (`startup_instance_id`),
  KEY `idx_heartbeat` (`last_heartbeat_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Worker 实例表';

CREATE TABLE IF NOT EXISTS `t_worker_codec_capability` (
  `id` BIGINT UNSIGNED NOT NULL COMMENT '记录ID',
  `node_id` BIGINT UNSIGNED NOT NULL COMMENT '节点ID',
  `worker_instance_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '关联的 Worker 实例记录ID',
  `gpu_device_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '关联的稳定 GPU 设备记录ID',
  `gpu_index` INT NOT NULL DEFAULT 0 COMMENT 'GPU 索引（无 GPU 可固定为 0）',
  `gpu_uuid` VARCHAR(128) DEFAULT NULL COMMENT 'GPU 稳定硬件标识，用于跨重启识别同一张卡',
  `startup_instance_id` VARCHAR(128) DEFAULT NULL COMMENT '本次启动实例标识',
  `probe_generation` BIGINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '同一启动周期内的探测代次',
  `machine_fingerprint` VARCHAR(512) DEFAULT NULL COMMENT '机器指纹',
  `codec_name` VARCHAR(32) NOT NULL COMMENT '编码名称，如 h264/hevc/av1/vp9',
  `cap_type` TINYINT NOT NULL COMMENT '能力类型：1=硬件解码；2=硬件编码',
  `hw_type` TINYINT NOT NULL COMMENT '硬件类型：1=NVENC/NVDEC；2=QSV；3=AMF；4=VAAPI；5=VideoToolbox',
  `max_sessions` INT NOT NULL DEFAULT 0 COMMENT '该能力最大会话数（如未知可为0）',
  `enabled` TINYINT NOT NULL DEFAULT 1 COMMENT '是否启用：0=否；1=是',
  `is_latest` TINYINT NOT NULL DEFAULT 1 COMMENT '是否为当前最新快照，0=否，1=是',
  `capability_payload_json` JSON DEFAULT NULL COMMENT '原始能力探测载荷摘要',
  `collected_at` DATETIME NOT NULL COMMENT '采集时间',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_node_startup_probe_codec` (`node_id`, `startup_instance_id`, `gpu_index`, `codec_name`, `cap_type`, `hw_type`, `probe_generation`),
  KEY `idx_node_type` (`node_id`, `cap_type`),
  KEY `idx_codec_type` (`codec_name`, `cap_type`),
  KEY `idx_worker_instance_latest` (`worker_instance_id`, `is_latest`, `collected_at`),
  KEY `idx_startup_instance_latest` (`startup_instance_id`, `is_latest`, `collected_at`),
  KEY `idx_gpu_uuid_latest` (`gpu_uuid`, `is_latest`, `collected_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='节点硬件编解码能力快照表，仅允许当前在线 Worker 的最新启动代次快照参与调度';

CREATE TABLE IF NOT EXISTS `t_transcode_job_execution` (
  `execution_id` BIGINT UNSIGNED NOT NULL COMMENT '任务执行实例ID',
  `job_id` BIGINT UNSIGNED NOT NULL COMMENT '所属转码任务ID',
  `attempt_no` INT NOT NULL DEFAULT 0 COMMENT '执行尝试次数，从0开始递增',
  `lease_generation` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '本次执行对应的租约代次，用于 fencing',
  `node_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '执行节点ID',
  `worker_instance_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '执行 Worker 实例记录ID',
  `gpu_device_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '执行绑定的稳定 GPU 设备记录ID',
  `selected_gpu_index` INT NOT NULL DEFAULT 0 COMMENT '执行时观测到的 GPU 索引，仅用于诊断展示',
  `selected_execution_hwaccel` VARCHAR(32) DEFAULT NULL COMMENT '执行时采用的硬件加速模式',
  `status` TINYINT NOT NULL COMMENT '状态：1=已租约；2=执行中；3=上传中；4=成功；5=失败；6=已放弃',
  `lease_owner` VARCHAR(64) DEFAULT NULL COMMENT '租约持有者标识',
  `lease_expire_at` DATETIME DEFAULT NULL COMMENT '租约过期时间',
  `last_heartbeat_at` DATETIME DEFAULT NULL COMMENT '该执行实例最近一次心跳时间',
  `failure_reason` VARCHAR(1024) DEFAULT NULL COMMENT '失败或放弃原因',
  `recoverable_flag` TINYINT NOT NULL DEFAULT 1 COMMENT '是否可恢复，0=否，1=是',
  `started_at` DATETIME DEFAULT NULL COMMENT '开始执行时间',
  `finished_at` DATETIME DEFAULT NULL COMMENT '结束执行时间',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '更新时间',
  PRIMARY KEY (`execution_id`),
  UNIQUE KEY `uk_job_attempt_no` (`job_id`, `attempt_no`),
  UNIQUE KEY `uk_job_lease_generation` (`job_id`, `lease_generation`),
  KEY `idx_status_lease_expire_at` (`status`, `lease_expire_at`),
  KEY `idx_worker_instance_status` (`worker_instance_id`, `status`),
  KEY `idx_node_status` (`node_id`, `status`),
  CONSTRAINT `fk_job_execution_job_id` FOREIGN KEY (`job_id`) REFERENCES `t_transcode_job` (`job_id`),
  CONSTRAINT `fk_job_execution_node_id` FOREIGN KEY (`node_id`) REFERENCES `t_cluster_node` (`node_id`),
  CONSTRAINT `fk_job_execution_worker_instance_id` FOREIGN KEY (`worker_instance_id`) REFERENCES `t_worker_instance` (`id`),
  CONSTRAINT `fk_job_execution_gpu_device_id` FOREIGN KEY (`gpu_device_id`) REFERENCES `t_node_gpu_device` (`gpu_device_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='任务执行实例表，用于支持租约代次、故障接管与陈旧执行实例 fencing';

-- ============================================================
-- 6. 直播主链路
-- ============================================================

CREATE TABLE IF NOT EXISTS `t_live_channel` (
  `channel_id` BIGINT UNSIGNED NOT NULL COMMENT '直播频道主键ID',
  `channel_key` VARCHAR(128) NOT NULL COMMENT '直播频道业务唯一标识，例如房间号、频道编码、栏目编码',
  `channel_name` VARCHAR(256) NOT NULL COMMENT '直播频道名称，便于后台展示与业务识别',
  `profile_id` BIGINT UNSIGNED NOT NULL COMMENT '绑定的直播转码模板ID',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '频道状态，1=已创建，2=运行中，3=已停止',
  `play_domain` VARCHAR(256) DEFAULT NULL COMMENT '播放域名，用于拼装 HLS/HTTP-FLV 播放地址',
  `push_domain` VARCHAR(256) DEFAULT NULL COMMENT '推流域名，用于拼装主播推流地址',
  `enable_source_rendition` TINYINT NOT NULL DEFAULT 1 COMMENT '是否启用原画档输出，0=否，1=是',
  `enable_watermark` TINYINT NOT NULL DEFAULT 0 COMMENT '是否启用直播水印，0=否，1=是',
  `created_at` DATETIME NOT NULL COMMENT '记录创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '记录更新时间',
  PRIMARY KEY (`channel_id`),
  UNIQUE KEY `uk_channel_key` (`channel_key`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='直播频道主表，用于保存频道基础信息、状态和播放推流域名配置';

CREATE TABLE IF NOT EXISTS `t_live_profile_rendition` (
  `id` BIGINT UNSIGNED NOT NULL COMMENT '直播模板清晰度记录主键ID',
  `profile_id` BIGINT UNSIGNED NOT NULL COMMENT '直播模板ID',
  `rendition_name` VARCHAR(64) NOT NULL COMMENT '清晰度名称，例如 source、1080p、720p、480p',
  `is_source` TINYINT NOT NULL DEFAULT 0 COMMENT '是否为原画档，0=否，1=是',
  `enabled` TINYINT NOT NULL DEFAULT 1 COMMENT '该清晰度档位是否启用，0=禁用，1=启用',
  `out_width` INT DEFAULT NULL COMMENT '输出宽度，原画档可为空或与输入一致',
  `out_height` INT DEFAULT NULL COMMENT '输出高度，原画档可为空或与输入一致',
  `video_codec` VARCHAR(32) NOT NULL COMMENT '输出视频编码名称，例如 h264、hevc',
  `video_bitrate_kbps` INT DEFAULT NULL COMMENT '目标视频码率，单位 kbps',
  `created_at` DATETIME NOT NULL COMMENT '记录创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '记录更新时间',
  PRIMARY KEY (`id`),
  KEY `idx_profile_id` (`profile_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='直播模板清晰度配置表，用于定义原画档和各转码档位的输出能力';

CREATE TABLE IF NOT EXISTS `t_live_session` (
  `session_id` BIGINT UNSIGNED NOT NULL COMMENT '直播会话主键ID',
  `channel_id` BIGINT UNSIGNED NOT NULL COMMENT '所属直播频道ID，对应 t_live_channel.channel_id',
  `session_key` VARCHAR(128) NOT NULL COMMENT '直播会话唯一标识，用于区分不同场次或不同推流周期',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '会话状态，1=待启动，2=直播中，3=已结束，4=异常中断',
  `ingest_url` VARCHAR(1024) DEFAULT NULL COMMENT '本次会话实际推流接入地址，便于联调和排查推流问题',
  `playback_hls_url` VARCHAR(1024) DEFAULT NULL COMMENT '本次会话对应的 HLS 播放地址，用于播放网关或后台展示',
  `push_protocol` VARCHAR(32) DEFAULT NULL COMMENT '推流协议，例如 rtmp、srt',
  `assigned_node_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '当前承载该直播会话的节点ID',
  `assigned_worker_id` VARCHAR(128) NOT NULL DEFAULT '' COMMENT '当前承载该直播会话的 Worker 标识',
  `resume_count` INT NOT NULL DEFAULT 0 COMMENT '中断恢复次数',
  `started_at` DATETIME DEFAULT NULL COMMENT '会话启动时间',
  `ended_at` DATETIME DEFAULT NULL COMMENT '会话结束时间',
  `created_at` DATETIME NOT NULL COMMENT '记录创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '记录更新时间',
  PRIMARY KEY (`session_id`),
  UNIQUE KEY `uk_session_key` (`session_key`),
  KEY `idx_channel_status` (`channel_id`, `status`),
  CONSTRAINT `fk_live_session_channel_id` FOREIGN KEY (`channel_id`) REFERENCES `t_live_channel` (`channel_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='直播会话表，用于记录频道每次启动后的推流会话、状态与播放地址快照';

CREATE TABLE IF NOT EXISTS `t_live_session_event` (
  `event_id` BIGINT UNSIGNED NOT NULL COMMENT '直播会话事件主键ID',
  `session_id` BIGINT UNSIGNED NOT NULL COMMENT '所属直播会话ID，对应 t_live_session.session_id',
  `channel_id` BIGINT UNSIGNED NOT NULL COMMENT '所属直播频道ID，便于按频道维度快速检索事件',
  `event_type` VARCHAR(64) NOT NULL COMMENT '事件类型，例如 live.started、live.stopped、stream.interrupted',
  `event_payload_json` JSON NOT NULL COMMENT '事件负载 JSON 内容，用于记录会话状态变化时的上下文数据',
  `created_at` DATETIME NOT NULL COMMENT '记录创建时间',
  PRIMARY KEY (`event_id`),
  KEY `idx_session_event` (`session_id`, `created_at`),
  KEY `idx_channel_event` (`channel_id`, `created_at`),
  CONSTRAINT `fk_live_session_event_session_id` FOREIGN KEY (`session_id`) REFERENCES `t_live_session` (`session_id`),
  CONSTRAINT `fk_live_session_event_channel_id` FOREIGN KEY (`channel_id`) REFERENCES `t_live_channel` (`channel_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='直播会话事件表，用于记录启停播、异常中断和恢复等关键事件轨迹';

CREATE TABLE IF NOT EXISTS `t_live_playback_token` (
  `token_id` BIGINT UNSIGNED NOT NULL COMMENT '播放令牌记录主键ID',
  `channel_id` BIGINT UNSIGNED NOT NULL COMMENT '所属直播频道ID，对应 t_live_channel.channel_id',
  `user_token` VARCHAR(256) NOT NULL COMMENT '播放侧用户令牌原文或业务透传令牌，用于鉴权和审计',
  `viewer_id` VARCHAR(128) DEFAULT NULL COMMENT '观众业务唯一标识，便于统计单用户观看信息与风控分析',
  `allow_play` TINYINT NOT NULL DEFAULT 1 COMMENT '是否允许播放，0=拒绝播放，1=允许播放',
  `expire_at` DATETIME NOT NULL COMMENT '令牌过期时间，超过该时间后播放网关应拒绝继续使用该令牌',
  `issued_at` DATETIME NOT NULL COMMENT '令牌签发时间',
  `created_at` DATETIME NOT NULL COMMENT '记录创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '记录更新时间',
  PRIMARY KEY (`token_id`),
  UNIQUE KEY `uk_channel_user_token` (`channel_id`, `user_token`),
  KEY `idx_expire_at` (`expire_at`),
  CONSTRAINT `fk_live_token_channel_id` FOREIGN KEY (`channel_id`) REFERENCES `t_live_channel` (`channel_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='直播播放令牌表，用于保存播放鉴权令牌、观众标识和过期时间';

CREATE TABLE IF NOT EXISTS `t_live_publish_session` (
  `publish_session_id` BIGINT UNSIGNED NOT NULL COMMENT '直播推流会话主键ID，由应用侧雪花算法生成',
  `channel_id` BIGINT UNSIGNED NOT NULL COMMENT '所属直播频道ID',
  `session_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '关联直播会话ID，对应 t_live_session.session_id',
  `stream_key` VARCHAR(255) NOT NULL COMMENT '推流鉴权 key 或流标识',
  `publish_ip` VARCHAR(64) DEFAULT NULL COMMENT '推流来源IP',
  `publish_status` TINYINT NOT NULL DEFAULT 1 COMMENT '推流状态，1=待接入，2=推流中，3=已断开，4=被拒绝',
  `connected_at` DATETIME DEFAULT NULL COMMENT '推流连接建立时间',
  `disconnected_at` DATETIME DEFAULT NULL COMMENT '推流断开时间',
  `created_at` DATETIME NOT NULL COMMENT '记录创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '记录更新时间',
  PRIMARY KEY (`publish_session_id`),
  KEY `idx_channel_id_publish_status` (`channel_id`, `publish_status`),
  CONSTRAINT `fk_live_publish_session_channel_id` FOREIGN KEY (`channel_id`) REFERENCES `t_live_channel` (`channel_id`),
  CONSTRAINT `fk_live_publish_session_session_id` FOREIGN KEY (`session_id`) REFERENCES `t_live_session` (`session_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='直播推流会话表，用于记录主播侧推流接入和断开过程';

CREATE TABLE IF NOT EXISTS `t_live_publish_auth_log` (
  `auth_log_id` BIGINT UNSIGNED NOT NULL COMMENT '直播推流鉴权日志主键ID，由应用侧雪花算法生成',
  `channel_id` BIGINT UNSIGNED NOT NULL COMMENT '所属直播频道ID',
  `stream_key` VARCHAR(255) NOT NULL COMMENT '鉴权时提交的 stream key',
  `request_ip` VARCHAR(64) DEFAULT NULL COMMENT '鉴权请求来源IP',
  `auth_result` TINYINT NOT NULL COMMENT '鉴权结果，1=通过，2=拒绝',
  `auth_message` VARCHAR(512) DEFAULT NULL COMMENT '鉴权结果描述',
  `created_at` DATETIME NOT NULL COMMENT '记录创建时间',
  PRIMARY KEY (`auth_log_id`),
  KEY `idx_channel_id_created_at` (`channel_id`, `created_at`),
  CONSTRAINT `fk_live_publish_auth_log_channel_id` FOREIGN KEY (`channel_id`) REFERENCES `t_live_channel` (`channel_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='直播推流鉴权日志表，用于记录直播推流接入鉴权过程';

-- ============================================================
-- 7. bootstrap 配置源绑定表
-- ============================================================

CREATE TABLE IF NOT EXISTS `t_config_center_binding` (
  `binding_id` BIGINT UNSIGNED NOT NULL COMMENT '绑定ID',
  `binding_name` VARCHAR(128) NOT NULL COMMENT '绑定名称',
  `provider_type` VARCHAR(64) NOT NULL COMMENT 'bootstrap 配置源类型，如 nacos/apollo/etcd/consul',
  `endpoint` VARCHAR(1024) DEFAULT NULL COMMENT 'bootstrap 配置源连接地址',
  `namespace` VARCHAR(255) DEFAULT NULL COMMENT '配置命名空间或租户空间',
  `auth_mode` VARCHAR(32) NOT NULL DEFAULT 'none' COMMENT '鉴权模式，如 none/token/rbac',
  `access_key` VARCHAR(256) DEFAULT NULL COMMENT '访问密钥',
  `secret_key` VARCHAR(256) DEFAULT NULL COMMENT '密钥',
  `token` VARCHAR(512) DEFAULT NULL COMMENT '鉴权令牌',
  `enabled` TINYINT NOT NULL DEFAULT 1 COMMENT '是否启用',
  `priority` INT NOT NULL DEFAULT 0 COMMENT '优先级，数值越大越优先',
  `last_sync_status` VARCHAR(64) DEFAULT NULL COMMENT '最近同步状态',
  `last_sync_message` VARCHAR(1024) DEFAULT NULL COMMENT '最近同步消息',
  `last_sync_at` DATETIME DEFAULT NULL COMMENT '最近同步时间',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '更新时间',
  PRIMARY KEY (`binding_id`),
  UNIQUE KEY `uk_binding_name` (`binding_name`),
  KEY `idx_enabled_priority` (`enabled`, `priority`, `updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='bootstrap 配置源绑定表';

CREATE TABLE IF NOT EXISTS `t_effective_runtime_config_snapshot` (
  `snapshot_id` BIGINT UNSIGNED NOT NULL COMMENT '有效配置快照主键ID',
  `config_version` BIGINT UNSIGNED NOT NULL COMMENT '关联的运行配置版本号',
  `config_source` VARCHAR(64) NOT NULL COMMENT '有效配置来源，例如 admin_db、bootstrap_default、bootstrap_fallback',
  `source_revision` VARCHAR(128) DEFAULT NULL COMMENT '来源修订号',
  `merged_payload_json` JSON NOT NULL COMMENT '最终合并后的有效配置完整载荷',
  `config_hash` VARCHAR(128) NOT NULL COMMENT '有效配置摘要哈希',
  `created_by` VARCHAR(128) DEFAULT NULL COMMENT '创建人或触发来源',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  PRIMARY KEY (`snapshot_id`),
  KEY `idx_config_version` (`config_version`),
  KEY `idx_config_hash` (`config_hash`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='有效运行配置快照表，用于持久化最终配置结果；运行期热路径应优先从 Redis 缓存读取';

-- ============================================================
-- 8. 可选初始化种子数据
-- 说明：
-- - admin 用户 / 默认 profile 属于可选初始化数据，便于首次登录和快速联调。
-- - t_runtime_config 不再通过 SQL 种子写入；应用首次启动时会自动生成首版已发布 runtime config。
-- ============================================================

START TRANSACTION;

SET @now = NOW();

INSERT INTO `t_admin_user` (
  `admin_user_id`,
  `username`,
  `password_hash`,
  `display_name`,
  `status`,
  `last_login_at`,
  `last_login_ip`,
  `created_at`,
  `updated_at`
) VALUES (
  1000000000000000001,
  'admin',
  SHA2('admin123', 256),
  '系统管理员',
  1,
  NULL,
  NULL,
  @now,
  @now
)
ON DUPLICATE KEY UPDATE
  `password_hash` = VALUES(`password_hash`),
  `display_name` = VALUES(`display_name`),
  `status` = VALUES(`status`),
  `updated_at` = VALUES(`updated_at`);

INSERT INTO `t_transcode_profile` (
  `profile_id`,
  `profile_name`,
  `biz_code`,
  `container_format`,
  `segment_duration_sec`,
  `video_codec`,
  `audio_codec`,
  `enabled`,
  `created_at`,
  `updated_at`
) VALUES (
  1,
  'default',
  'default',
  'fmp4',
  4,
  'h264',
  'aac',
  1,
  @now,
  @now
)
ON DUPLICATE KEY UPDATE
  `biz_code` = VALUES(`biz_code`),
  `container_format` = VALUES(`container_format`),
  `segment_duration_sec` = VALUES(`segment_duration_sec`),
  `video_codec` = VALUES(`video_codec`),
  `audio_codec` = VALUES(`audio_codec`),
  `enabled` = VALUES(`enabled`),
  `updated_at` = VALUES(`updated_at`);

INSERT INTO `t_transcode_profile_rendition` (
  `profile_rendition_id`,
  `profile_id`,
  `rendition_name`,
  `enabled`,
  `out_width`,
  `out_height`,
  `video_bitrate_kbps`,
  `audio_bitrate_kbps`,
  `fps`,
  `created_at`,
  `updated_at`
) VALUES (
  1000000000000000101,
  1,
  'source',
  1,
  NULL,
  NULL,
  NULL,
  128,
  NULL,
  @now,
  @now
)
ON DUPLICATE KEY UPDATE
  `enabled` = VALUES(`enabled`),
  `audio_bitrate_kbps` = VALUES(`audio_bitrate_kbps`),
  `updated_at` = VALUES(`updated_at`);

COMMIT;

SET FOREIGN_KEY_CHECKS = 1;
