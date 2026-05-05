package worker

import (
	"context"
	"time"

	"hvc/internal/config"
	hotpath "hvc/internal/cluster/hotpath"
	rediscache "hvc/internal/infra/cache/redis"
	"hvc/internal/infra/db/mysql"
	"hvc/internal/infra/storage/s3"
	"hvc/internal/model"
	"hvc/internal/worker/executor"
	"hvc/internal/worker/probe"
	uploadworker "hvc/internal/worker/uploader"
	"hvc/pkg/idgen"
	"hvc/pkg/logx"
)

// Module 表示执行模块。
type Module struct {
	cfg               config.RuntimeConfig
	jobRepository     *mysql.JobRepository
	segmentRepository *mysql.SegmentRepository
	progressStore     *rediscache.ProgressStore
	outboxRepository  *mysql.OutboxRepository
	hotpathBus        *hotpath.MemoryBus
	runner            *executor.Runner
	uploader          *storage.Client
	retryPolicy       RetryPolicy
}

// NewModule 创建执行模块。
func NewModule(cfg config.RuntimeConfig, jobRepository *mysql.JobRepository, segmentRepository *mysql.SegmentRepository, progressStore *rediscache.ProgressStore, outboxRepository *mysql.OutboxRepository, hotpathBus *hotpath.MemoryBus) *Module {
	uploader, _ := storage.Open(cfg.Storage)
	return &Module{
		cfg:               cfg,
		jobRepository:     jobRepository,
		segmentRepository: segmentRepository,
		progressStore:     progressStore,
		outboxRepository:  outboxRepository,
		hotpathBus:        hotpathBus,
		runner:            executor.NewRunner(),
		uploader:          uploader,
		retryPolicy: RetryPolicy{
			BaseDelay: cfg.Worker.UploadRetryBaseDelay,
			MaxDelay:  cfg.Worker.UploadRetryMaxDelay,
			MaxRetry:  cfg.Worker.UploadMaxRetryCount,
		},
	}
}

// CanUseSoftwareDecode 判断是否允许软解。
func (m *Module) CanUseSoftwareDecode(cpuPercent int) bool {
	if !m.cfg.Scheduler.AllowSoftwareDecodeFallback {
		return false
	}
	return cpuPercent < m.cfg.Scheduler.SoftDecodeCPULimitPercent
}

// CanAcceptNewTask 判断是否允许接收新任务。
func (m *Module) CanAcceptNewTask(cpuPercent, memPercent, gpuMemPercent, uploadQueueDepth, activeSessions int) bool {
	if cpuPercent >= m.cfg.Scheduler.NodeCPUSafetyLimitPercent {
		return false
	}
	if memPercent >= m.cfg.Scheduler.NodeMemorySafetyLimitPercent {
		return false
	}
	if gpuMemPercent >= m.cfg.Scheduler.NodeGPUSafetyLimitPercent {
		return false
	}
	if activeSessions >= m.cfg.Scheduler.MaxNodeTranscodeSessions {
		return false
	}
	if uploadQueueDepth >= m.cfg.Scheduler.MaxNodeUploadConcurrency {
		return false
	}
	return true
}

// Start 启动执行模块。
func (m *Module) Start(ctx context.Context) error {
	ticker := time.NewTicker(m.cfg.Worker.LoopInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			m.runOnce(ctx)
			m.uploadOnce(ctx)
		}
	}
}

func (m *Module) runOnce(ctx context.Context) {
	jobs := m.jobRepository.ListAssigned(ctx, m.cfg.Server.NodeID, m.cfg.Server.WorkerID)
	for _, job := range jobs {
		probeResult := probe.Inspect(job.SourceURL)
		logx.Info("worker.job.probe", logx.Fields{
			"job_id":             job.JobID,
			"video_codec":        probeResult.VideoCodec,
			"audio_codec":        probeResult.AudioCodec,
			"duration_ms":        probeResult.DurationMS,
			"width":              probeResult.Width,
			"height":             probeResult.Height,
		})
		logx.Info("worker.job.start", logx.Fields{
			"job_id":      job.JobID,
			"request_id":  job.RequestID,
			"worker_id":   m.cfg.Server.WorkerID,
			"node_id":     m.cfg.Server.NodeID,
		})
		progressStream := m.runner.Run(ctx, job)
		for progress := range progressStream {
			status := model.JobStatusRunning
			if progress.Stage == model.StageUploading {
				status = model.JobStatusUploading
			}
			snapshot := executor.ToSnapshot(progress, status)
			m.progressStore.Save(ctx, snapshot)
			m.hotpathBus.SaveProgress(ctx, progress)
			_ = m.jobRepository.UpdateProgress(ctx, job.JobID, snapshot.ProgressPermille, snapshot.Stage)
			logx.Info("worker.job.progress", logx.Fields{
				"job_id":             job.JobID,
				"request_id":         job.RequestID,
				"progress_permille":  snapshot.ProgressPermille,
				"stage":              snapshot.Stage,
				"fps":                snapshot.CurrentFPS,
				"bitrate_kbps":       snapshot.CurrentBitrateKbps,
				"speed":              snapshot.CurrentSpeed,
			})
			if snapshot.ProgressPermille >= 900 {
				segment, _ := m.segmentRepository.Save(ctx, model.Segment{
					JobID:         job.JobID,
					RenditionID:   1,
					MediaType:     1,
					IsInitSegment: false,
					SequenceNo:    1,
					DurationMS:    job.SegmentDurationSec * 1000,
					SupportDash:   job.SupportDash,
					SupportHLS:    job.SupportHLS,
					CodecName:     probeResult.VideoCodec,
					ObjectKey:     job.OutputBasePrefix + "/segment-0001.m4s",
					UploadStatus:  model.SegmentUploadPending,
					CreatedAt:     time.Now(),
					UpdatedAt:     time.Now(),
				})
				logx.Info("worker.segment.created", logx.Fields{
					"job_id":      job.JobID,
					"segment_id":  segment.SegmentID,
					"object_key":  segment.ObjectKey,
				})
			}
		}
		_ = m.jobRepository.MarkCompleted(ctx, job.JobID)
		m.progressStore.Save(ctx, model.ProgressSnapshot{
			JobID:            job.JobID,
			Status:           model.JobStatusCompleted,
			Stage:            model.StageCompleted,
			ProgressPermille: 1000,
		})
		event := model.OutboxEvent{
			EventID:       idgen.Next(),
			EventType:     "transcode.completed",
			JobID:         job.JobID,
			RequestID:     job.RequestID,
			PayloadJSON:   "{}",
			Status:        model.OutboxStatusPending,
			MaxRetryCount: m.cfg.Worker.UploadMaxRetryCount,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}
		_ = m.outboxRepository.Save(ctx, event)
		logx.Info("worker.job.completed", logx.Fields{
			"job_id":      job.JobID,
			"request_id":  job.RequestID,
			"event_id":    event.EventID,
		})
	}
}

func (m *Module) uploadOnce(ctx context.Context) {
	segments := m.segmentRepository.ListPendingUpload(ctx, m.cfg.Worker.SingleJobUploadConcurrency)
	if len(segments) == 0 {
		return
	}
	tasks := make([]model.UploadTask, 0, len(segments))
	for _, segment := range segments {
		_ = m.segmentRepository.MarkUploading(ctx, segment.SegmentID)
		logx.Info("worker.upload.start", logx.Fields{
			"segment_id":   segment.SegmentID,
			"job_id":       segment.JobID,
			"object_key":   segment.ObjectKey,
			"retry_count":  segment.UploadRetryCount,
		})
		tasks = append(tasks, model.UploadTask{
			SegmentID:   segment.SegmentID,
			JobID:       segment.JobID,
			RenditionID: segment.RenditionID,
			ObjectKey:   m.uploader.BuildObjectKey(segment),
			LocalPath:   segment.ObjectKey,
			RetryCount:  segment.UploadRetryCount,
			CreatedAt:   time.Now(),
		})
	}
	pool := uploadworker.NewWorkerPool(m.cfg.Worker.SingleJobUploadConcurrency, m.uploader)
	results := pool.Run(ctx, tasks)
	for _, result := range results {
		if result.Success {
			_ = m.segmentRepository.MarkUploaded(ctx, result.SegmentID, result.ObjectETag, result.ObjectSizeBytes)
			logx.Info("worker.upload.success", logx.Fields{
				"segment_id":        result.SegmentID,
				"object_etag":       result.ObjectETag,
				"object_size_bytes": result.ObjectSizeBytes,
			})
			continue
		}
		_ = m.segmentRepository.MarkUploadFailed(ctx, result.SegmentID, result.ErrorMessage)
		logx.Error("worker.upload.failed", nil, logx.Fields{
			"segment_id":    result.SegmentID,
			"error_message": result.ErrorMessage,
		})
		}
}
