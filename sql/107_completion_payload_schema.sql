-- 107. 任务完成后的回调 payload 暂存列（A1：回调必须等分片全部传完）
--
-- 背景（见 docs/TRANSCODE-SERVICE-DESIGN.md 的 §18.4 / §18.6）：
--   原来 worker 在转码结束时立刻 MarkCompleted + 写 outbox（回调）事件，而分片是**之后**
--   由全局待传队列异步上传的 ⇒ 回调可能早于任意一片上传完成，下游按回调去拉清单会拿到
--   残缺内容。修法是把"完成 + 回调"推迟到**本任务待传数归零**那一刻，但完成事件需要的
--   payload 依赖转码阶段的内存对象（探测结果、分片发现结果），上传那一刻不一定还在同一个
--   进程里（待传队列是全局的，别的节点也可能在传）⇒ payload 必须**落库**暂存。
--
-- - 该脚本面向旧库增量升级，不是全新库脚本。
-- - 全新库请直接执行 sql/000_full_project_schema.sql。
SET @transcode_job_exists_107 = (
  SELECT COUNT(*)
  FROM information_schema.tables
  WHERE table_schema = DATABASE()
    AND table_name = 't_transcode_job'
);

SET @ddl_transcode_job_107 = IF(
  @transcode_job_exists_107 = 0,
  'SELECT ''skip sql/107_completion_payload_schema.sql because t_transcode_job is missing; import sql/000_full_project_schema.sql first'' AS message',
  'ALTER TABLE `t_transcode_job`
     ADD COLUMN IF NOT EXISTS `completion_payload` MEDIUMTEXT NULL COMMENT ''A1: 完成+回调的 payload 暂存；非空表示“分片还没传完，尚未发布”'' AFTER `status`'
);

PREPARE stmt_transcode_job_107 FROM @ddl_transcode_job_107;
EXECUTE stmt_transcode_job_107;
DEALLOCATE PREPARE stmt_transcode_job_107;