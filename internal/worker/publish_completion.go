package worker

import (
	"context"
	"errors"
	"time"

	"hvc/internal/model"
	"hvc/pkg/idgen"
	"hvc/pkg/logx"
)

var (
	// errPublishSegmentsIncomplete 表示本任务仍有分片未上传完成（含上传失败待重试）。
	errPublishSegmentsIncomplete = errors.New("publish: segments not fully uploaded")
	// errPublishSegmentEvidenceMissing 表示分片缺少对象回执（ObjectKey/ETag/大小），
	// 无法证明"该片可 GET 且与上传时一致"，因此不允许发布。
	errPublishSegmentEvidenceMissing = errors.New("publish: segment object evidence missing")
	// errPublishSegmentDigestMissing 表示分片没有内容摘要 SHA256，无法满足 A1 判据里的
	// "sha256 通过"这一条，因此不允许发布。
	errPublishSegmentDigestMissing = errors.New("publish: segment sha256 digest missing")
)

// publishCompletion 执行 A1 的"发布"步骤：本任务待传分片数归零后，把转码结束时**暂存**的
// 完成 payload 取出来，校验内容完整，推进状态到 PUBLISHED，并写入完成回调的 outbox 事件。
//
// 为什么必须分成两步：worker 在转码结束时只能先把 payload 落库暂存 —— 分片是之后由**全局
// 待传队列**异步上传的，别的节点也可能在传这个任务的分片。若那一刻就直接发回调，下游按回调
// 去拉清单会拿到残缺内容（见 sql/107_completion_payload_schema.sql 的背景说明）。
//
// 幂等与竞争：TakePendingCompletion 是"取 + 清"的比较交换，多个节点/协程同时看到待传数归零时
// 只有一个能取到 payload，其余拿到空串直接返回 ⇒ 回调只会发一次。
//
// 返回 true 表示本次调用完成了发布（状态已推进、outbox 事件已写）。
func (m *Module) publishCompletion(ctx context.Context, jobID uint64) bool {
	if m == nil || m.jobRepository == nil || m.segmentRepository == nil {
		return false
	}

	maxRetry := m.cfg.Worker.UploadMaxRetryCount

	// 待传数 > 0（含"上传失败仍可重试"的分片）⇒ 还没到发布的时机，保持 COMPLETED，
	// 等下一次分片上传完成时再试。这里查的是**本任务**的待传数，不是全局队列深度。
	if pending := m.segmentRepository.CountPendingUploadByJob(ctx, jobID, maxRetry); pending > 0 {
		return false
	}

	// 逐片校验：每片都必须是"已上传"，且记录了对象 ETag 与对象大小。
	//
	// 为什么这两项可以当"可 GET 且内容正确"的凭据：MarkUploaded 只在对象存储 PUT 成功返回
	// 之后才会被调用，ETag/大小就是对象存储对**这一片**的回执；逐片回执齐备即等价于
	// "清单引用的每个分片都能取到，且与上传时一致"。
	if err := m.verifyJobSegments(ctx, jobID); err != nil {
		logx.Error("worker.job.publish_verify_failed", err, logx.Fields{
			"job_id": jobID,
		})
		return false
	}

	// 取 + 清（比较交换）：只有第一个执行到的调用者能拿到 payload。
	payloadJSON, ok := m.jobRepository.TakePendingCompletion(ctx, jobID)
	if !ok {
		// 别的节点已经发布，或这个任务从未暂存过 payload（老任务/直传任务）⇒ 不重复发回调。
		return false
	}

	job, found := m.jobRepository.FindByJobID(ctx, jobID)
	if !found {
		logx.Error("worker.job.publish_job_missing", nil, logx.Fields{
			"job_id": jobID,
		})
		return false
	}

	if err := m.jobRepository.MarkPublished(ctx, jobID); err != nil {
		logx.Error("worker.job.mark_published_failed", err, logx.Fields{
			"job_id": jobID,
		})
		return false
	}

	if m.progressStore != nil {
		m.progressStore.Save(ctx, model.ProgressSnapshot{
			JobID:            jobID,
			Status:           model.JobStatusPublished,
			Stage:            model.StagePublished,
			ProgressPermille: 1000,
		})
	}

	// 回调事件此刻才写：事件一旦投递成功，下游拉清单必然拿到完整内容。
	if m.outboxRepository != nil {
		event := model.OutboxEvent{
			EventID:       idgen.Next(),
			EventType:     "transcode.completed",
			JobID:         jobID,
			RequestID:     job.RequestID,
			PayloadJSON:   payloadJSON,
			Status:        model.OutboxStatusPending,
			MaxRetryCount: maxRetry,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		if err := m.outboxRepository.Save(ctx, event); err != nil {
			logx.Error("worker.job.publish_outbox_save_failed", err, logx.Fields{
				"job_id":   jobID,
				"event_id": event.EventID,
			})
			return false
		}
		logx.Info("worker.job.published", logx.Fields{
			"job_id":     jobID,
			"request_id": job.RequestID,
			"event_id":   event.EventID,
		})
		return true
	}

	logx.Info("worker.job.published", logx.Fields{
		"job_id":     jobID,
		"request_id": job.RequestID,
	})
	return true
}

// ReconcilePublish 对账"已完成但尚未发布"的任务，兜住漏掉的唤醒。
//
// 为什么需要它：待传队列是**全局**的，某个任务的最后一片可能由**别的节点**上传，那次上传不会
// 经过本进程的 publishCompletion；本进程也可能在两次上传之间重启。这两类情况都不会有任何本地
// 事件来触发发布，任务就会永远停在 COMPLETED。对账器的职责只是"发现该发布的没发布"，
// 判据与 publishCompletion 完全一致（待传归零 + 逐片回执齐备 + 比较交换领取），不引入新规则。
func (m *Module) ReconcilePublish(ctx context.Context) {
	if m == nil || m.jobRepository == nil || m.segmentRepository == nil {
		return
	}

	const publishSweepLimit = 200

	for _, job := range m.jobRepository.ListByStatus(ctx, model.JobStatusCompleted, publishSweepLimit) {
		m.publishCompletion(ctx, job.JobID)
	}
}

// verifyJobSegments 校验任务的分片是否全部上传完成且带齐内容凭据。
//
// 语义：
//   - 分片表为空 ⇒ 视为"没有需要校验的内容"（单文件直传等场景），直接通过；
//   - 任一分片不是 Uploaded，或缺少 ObjectKey/ObjectETag/ObjectSizeBytes，或没有内容摘要
//     SHA256 ⇒ 返回错误，发布必须失败并保持 COMPLETED，等下一轮重试。
//
// 为什么把 SHA256 也列为必需：A1 的判据要求"清单引用的每个分片都能 GET 到且 sha256 通过"。
//   对象存储的 ETag/大小只能证明"传上去了、大小对"，证明不了内容；摘要由生产侧在分片产出时
//   算好（segment_digest.go），清单物化时一并发出 ⇒ 下游与门禁可以据此复算校验。
func (m *Module) verifyJobSegments(ctx context.Context, jobID uint64) error {
	segments := m.segmentRepository.ListByJobID(ctx, jobID)
	if len(segments) == 0 {
		return nil
	}

	for _, seg := range segments {
		if seg.UploadStatus != model.SegmentUploaded {
			return errPublishSegmentsIncomplete
		}
		if seg.ObjectKey == "" || seg.ObjectETag == "" || seg.ObjectSizeBytes == 0 {
			return errPublishSegmentEvidenceMissing
		}
		if seg.SHA256 == "" {
			return errPublishSegmentDigestMissing
		}
	}

	return nil
}
