-- 109. B2 断点续跑：任务步骤表 + 源文件/规格指纹
--
-- 背景：
--   worker 侧 PROBE / PLAN / SEGMENT / UPLOAD 几个阶段的产出（探测结果、分片发现结果、
--   已上传分片号游标）目前只活在内存里：进程重启、换节点、或同一任务被重新调度时，
--   只能整任务从头重跑（重新下载、重新转码、重新上传）。
--   B2 要求把"步骤"本身变成可持久化的进度锚点：
--     · 每个步骤一行（t_transcode_job_step），state 标明未开始/进行中/已完成/失败；
--     · 行上记该步骤的**输入指纹** input_hash（源文件 + 输出规格的 sha256）；
--     · 续跑条件 = 任务行上的 input_hash 与当前输入一致，且存在已完成的前缀步骤 ⇒
--       从第一个"未完成"步骤开始，已完成的步骤直接跳过。
--   指纹不一致（换源、改规格）时**不允许续跑**，必须整任务重跑：否则会把上一版输入的
--   中间产物当成本版结果，产出错内容。
--
-- 字段语义：
--   step      步骤名，取值 PROBE / PLAN / SEGMENT / UPLOAD / PUBLISH / CALLBACK（大写）
--   state     0 未开始，1 进行中，2 已完成，3 失败（失败可用 attempt 计数决定是否再试）
--   input_hash  该步骤输入的 sha256（十六进制小写，64 字符）；空串表示"尚未计算"
--   detail    该步骤的产出锚点（JSON）：例如 UPLOAD 步骤记录已上传分片号集合的游标
--   attempt   已尝试次数，用于退避与"超过上限即判失败"
--
-- - 该脚本面向旧库增量升级，不是全新库脚本。
-- - 全新库请直接执行 sql/000_full_project_schema.sql。
SET @transcode_job_exists_109 = (
  SELECT COUNT(*)
  FROM information_schema.tables
  WHERE table_schema = DATABASE()
    AND table_name = 't_transcode_job'
);

SET @ddl_job_step_109 = IF(
  @transcode_job_exists_109 = 0,
  'SELECT ''skip sql/109_transcode_job_step_schema.sql because t_transcode_job is missing; import sql/000_full_project_schema.sql first'' AS message',
  'CREATE TABLE IF NOT EXISTS `t_transcode_job_step` (
     `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT ''自增主键'',
     `job_id` BIGINT UNSIGNED NOT NULL COMMENT ''任务 ID，对应 t_transcode_job.id'',
     `step` VARCHAR(16) NOT NULL COMMENT ''步骤名：PROBE/PLAN/SEGMENT/UPLOAD/PUBLISH/CALLBACK'',
     `state` TINYINT NOT NULL DEFAULT 0 COMMENT ''0 未开始 1 进行中 2 已完成 3 失败'',
     `input_hash` CHAR(64) NOT NULL DEFAULT '''' COMMENT ''该步骤输入的 sha256（源文件+输出规格），空串=未计算'',
     `detail` JSON NULL COMMENT ''步骤产出锚点：如已上传分片号游标、清单路径等'',
     `attempt` INT NOT NULL DEFAULT 0 COMMENT ''已尝试次数，用于退避与重试上限判定'',
     `started_at` DATETIME(3) NULL COMMENT ''本次尝试开始时间'',
     `finished_at` DATETIME(3) NULL COMMENT ''本次尝试结束时间（成功或失败）'',
     `created_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) COMMENT ''创建时间'',
     `updated_at` DATETIME(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3) COMMENT ''更新时间'',
     PRIMARY KEY (`id`),
     UNIQUE KEY `uk_job_step` (`job_id`, `step`),
     KEY `idx_state_updated` (`state`, `updated_at`)
   ) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT=''转码任务步骤表：按步骤记录进度与输入指纹，支持断点续跑'''
);

PREPARE stmt_job_step_109 FROM @ddl_job_step_109;
EXECUTE stmt_job_step_109;
DEALLOCATE PREPARE stmt_job_step_109;

SET @ddl_input_hash_109 = IF(
  @transcode_job_exists_109 = 0,
  'SELECT ''skip input_hash column because t_transcode_job is missing'' AS message',
  'ALTER TABLE `t_transcode_job`
     ADD COLUMN IF NOT EXISTS `input_hash` CHAR(64) NOT NULL DEFAULT '''' COMMENT ''B2: 源文件+输出规格的 sha256；变化即不可续跑'' AFTER `profile_id`'
);

PREPARE stmt_input_hash_109 FROM @ddl_input_hash_109;
EXECUTE stmt_input_hash_109;
DEALLOCATE PREPARE stmt_input_hash_109;
