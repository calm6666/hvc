package worker

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"hvc/internal/cluster"
	"hvc/internal/cluster/hotpath"
	"hvc/internal/config"
	rediscache "hvc/internal/infra/cache/redis"
	"hvc/internal/infra/db/mysql"
	"hvc/internal/infra/gpu"
	"hvc/internal/infra/storage/s3"
	"hvc/internal/model"
	"hvc/internal/worker/executor"
	"hvc/internal/worker/planner"
	"hvc/internal/worker/probe"
	"hvc/internal/worker/reporter"
	"hvc/internal/worker/segmenter"
	uploadworker "hvc/internal/worker/uploader"
	"hvc/pkg/idgen"
	"hvc/pkg/logx"
)

// Module 表示执行模块。
//
// Worker 模块负责三类事情：
// 1. 拉取已分配任务并执行；
// 2. 推进分片上传；
// 3. 周期性汇总本机运行指标并写入热路径缓存，供调度器消费。
//
// 当前批次把 GPU 能力探测和 metrics 上报直接并入这个模块，目的是优先打通主链路，
// 而不是继续把“能力已建模、调度也会消费，但 Worker 永远不填数据”的断链状态保留下去。
type Module struct {
	cfg                     config.DynamicRuntimeConfig
	nodeID                  uint64
	workerID                string
	jobRepository           *mysql.JobRepository
	segmentRepository       *mysql.SegmentRepository
	progressStore           *rediscache.ProgressStore
	outboxRepository        *mysql.OutboxRepository
	workerInstanceRepo      *mysql.WorkerInstanceRepository
	gpuDeviceRepository     *mysql.GPUDeviceRepository
	gpuCapabilityRepository *mysql.WorkerCodecCapabilityRepository
	jobExecutionRepository  *mysql.JobExecutionRepository
	hotpathBus              *hotpath.MemoryBus
	stateCache              *cluster.StateCache
	runner                  *executor.Runner
	uploader                *storage.Client
	retryPolicy             RetryPolicy
	startupInstanceID       string
	machineFingerprint      string
	workerInstanceID        uint64
	currentProbeGeneration  uint64
	runningJobs             map[uint64]struct{}
}

// NewModule 创建执行模块。
func NewModule(cfg config.DynamicRuntimeConfig, nodeID uint64, workerID string, jobRepository *mysql.JobRepository, segmentRepository *mysql.SegmentRepository, progressStore *rediscache.ProgressStore, outboxRepository *mysql.OutboxRepository, hotpathBus *hotpath.MemoryBus, stateCache *cluster.StateCache, workerInstanceRepo *mysql.WorkerInstanceRepository, gpuDeviceRepository *mysql.GPUDeviceRepository, gpuCapabilityRepository *mysql.WorkerCodecCapabilityRepository, jobExecutionRepository *mysql.JobExecutionRepository) *Module {
	uploader, _ := storage.Open(cfg.Storage)
	startupInstanceID := workerID + "-startup"
	machineFingerprint := "node-" + workerID
	return &Module{
		cfg:                     cfg,
		nodeID:                  nodeID,
		workerID:                workerID,
		jobRepository:           jobRepository,
		segmentRepository:       segmentRepository,
		progressStore:           progressStore,
		outboxRepository:        outboxRepository,
		workerInstanceRepo:      workerInstanceRepo,
		gpuDeviceRepository:     gpuDeviceRepository,
		gpuCapabilityRepository: gpuCapabilityRepository,
		jobExecutionRepository:  jobExecutionRepository,
		hotpathBus:              hotpathBus,
		stateCache:              stateCache,
		runner:                  executor.NewRunner(),
		uploader:                uploader,
		retryPolicy: RetryPolicy{
			BaseDelay: cfg.Worker.UploadRetryBaseDelay,
			MaxDelay:  cfg.Worker.UploadRetryMaxDelay,
			MaxRetry:  cfg.Worker.UploadMaxRetryCount,
		},
		startupInstanceID:      startupInstanceID,
		machineFingerprint:     machineFingerprint,
		currentProbeGeneration: 1,
		runningJobs:            make(map[uint64]struct{}),
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
//
// 所有耗时操作（转码执行、分片上传、GPU 探测）均通过 goroutine 异步执行，
// 不阻塞主循环。主循环仅负责触发，不等待完成。
func (m *Module) Start(ctx context.Context) error {
	m.ensureWorkerInstance(ctx)

	jobSem := make(chan struct{}, m.cfg.Scheduler.MaxNodeTranscodeSessions)
	uploadSem := make(chan struct{}, m.cfg.Scheduler.MaxNodeUploadConcurrency)

	ticker := time.NewTicker(m.cfg.Worker.LoopInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			m.reportHeartbeatOnce(ctx)
			m.reportMetricsOnce(ctx)
			m.dispatchJobs(ctx, jobSem)
			m.dispatchUploads(ctx, uploadSem)
		}
	}
}

// dispatchJobs 异步分发转码任务。
//
// 使用信号量控制最大并发转码数，避免资源过载。
// 每个任务在独立 goroutine 中执行，不阻塞主循环。
func (m *Module) dispatchJobs(ctx context.Context, sem chan struct{}) {
	jobs := m.jobRepository.ListAssigned(ctx, m.nodeID, m.workerID)
	for _, job := range jobs {
		if _, running := m.runningJobs[job.JobID]; running {
			continue
		}
		select {
		case sem <- struct{}{}:
			m.runningJobs[job.JobID] = struct{}{}
			go func(j model.TranscodeJob) {
				defer func() {
					<-sem
					delete(m.runningJobs, j.JobID)
				}()
				m.executeJob(ctx, j)
			}(job)
		default:
		}
	}
}

// dispatchUploads 异步分发上传任务。
//
// 使用信号量控制最大并发上传数。
// 每批上传在独立 goroutine 中执行，不阻塞主循环。
func (m *Module) dispatchUploads(ctx context.Context, sem chan struct{}) {
	segments := m.segmentRepository.ListPendingUpload(ctx, m.cfg.Worker.SingleJobUploadConcurrency)
	if len(segments) == 0 {
		return
	}
	select {
	case sem <- struct{}{}:
		go func() {
			defer func() { <-sem }()
			m.uploadSegments(ctx, segments)
		}()
	default:
	}
}

func (m *Module) executeJob(ctx context.Context, job model.TranscodeJob) {
	if m.jobExecutionRepository != nil {
		if err := m.jobExecutionRepository.MarkRunning(ctx, job.JobID, job.LeaseGeneration); err != nil {
			logx.Error("worker.job.mark_running_failed", err, logx.Fields{
				"job_id": job.JobID,
			})
		}
	}

	probeResult := probe.Inspect(job.SourceURL)
	logx.Info("worker.job.probe", logx.Fields{
		"job_id":      job.JobID,
		"video_codec": probeResult.VideoCodec,
		"audio_codec": probeResult.AudioCodec,
		"duration_ms": probeResult.DurationMS,
		"width":       probeResult.Width,
		"height":      probeResult.Height,
	})

	if probeResult.DurationMS == 0 {
		logx.Error("worker.job.probe_failed", nil, logx.Fields{
			"job_id":     job.JobID,
			"source_url": job.SourceURL,
		})
		m.failJob(ctx, job, "PROBE_FAILED", "ffprobe returned zero duration")
		return
	}

	executionHW := job.SelectedExecutionHWAccel
	if executionHW == "" {
		executionHW = model.ExecutionHWSoftware
	}

	naming := planner.DefaultSegmentNamingConfig()
	if m.cfg.Worker.SegmentTemplate != "" {
		naming.SegmentTemplate = m.cfg.Worker.SegmentTemplate
	}
	if m.cfg.Storage.BasePrefix != "" {
		naming.ObjectKeyPrefix = m.cfg.Storage.BasePrefix
	}
	pipeline := planner.BuildPlan(job, probeResult, executionHW, naming)
	logx.Info("worker.job.pipeline", logx.Fields{
		"job_id":         job.JobID,
		"execution_hw":   executionHW,
		"output_dir":     pipeline.OutputDir,
		"rendition_count": len(pipeline.Renditions),
		"hw_decode":      pipeline.HardwareDecode,
		"hw_encode":      pipeline.HardwareEncode,
	})

	logx.Info("worker.job.start", logx.Fields{
		"job_id":     job.JobID,
		"request_id": job.RequestID,
		"worker_id":  m.workerID,
		"node_id":    m.nodeID,
	})

	progressStream, err := m.runner.Run(ctx, job, pipeline, probeResult.DurationMS)
	if err != nil {
		logx.Error("worker.job.run_failed", err, logx.Fields{
			"job_id": job.JobID,
		})
		m.failJob(ctx, job, "RUN_FAILED", err.Error())
		return
	}

	for progress := range progressStream {
		status := model.JobStatusRunning
		if progress.Stage == model.StageUploading {
			status = model.JobStatusUploading
			if m.jobExecutionRepository != nil {
				if err := m.jobExecutionRepository.MarkUploading(ctx, job.JobID, job.LeaseGeneration); err != nil {
					logx.Error("worker.job.mark_uploading_failed", err, logx.Fields{
						"job_id": job.JobID,
					})
				}
			}
		}
		snapshot := executor.ToSnapshot(progress, status)
		m.progressStore.Save(ctx, snapshot)
		m.hotpathBus.SaveProgress(ctx, progress)
		if err := m.jobRepository.UpdateProgress(ctx, job.JobID, snapshot.ProgressPermille, snapshot.Stage); err != nil {
			logx.Error("worker.job.update_progress_failed", err, logx.Fields{
				"job_id": job.JobID,
			})
		}
		if m.jobExecutionRepository != nil {
			_ = m.jobExecutionRepository.TouchHeartbeat(ctx, job.JobID, job.LeaseGeneration)
		}
		logx.Info("worker.job.progress", logx.Fields{
			"job_id":            job.JobID,
			"request_id":        job.RequestID,
			"progress_permille": snapshot.ProgressPermille,
			"stage":             snapshot.Stage,
			"fps":               snapshot.CurrentFPS,
			"bitrate_kbps":      snapshot.CurrentBitrateKbps,
			"speed":             snapshot.CurrentSpeed,
		})
	}

	discoverResult := segmenter.Discover(job, pipeline)
	logx.Info("worker.job.segments_discovered", logx.Fields{
		"job_id":        job.JobID,
		"segment_count": len(discoverResult.Segments),
	})

	for _, seg := range discoverResult.Segments {
		mediaType := 1
		if seg.MediaType == 2 {
			mediaType = 2
		}
		_, saveErr := m.segmentRepository.Save(ctx, model.Segment{
			JobID:         job.JobID,
			RenditionID:   uint64(seg.RepresentationID),
			MediaType:     mediaType,
			IsInitSegment: seg.IsInit,
			SequenceNo:    seg.SequenceNo,
			DurationMS:    job.SegmentDurationSec * 1000,
			SupportDash:   job.SupportDash,
			SupportHLS:    job.SupportHLS,
			CodecName:     probeResult.VideoCodec,
			ObjectKey:     seg.ObjectKey,
			UploadStatus:  model.SegmentUploadPending,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		})
		if saveErr != nil {
			logx.Error("worker.segment.save_failed", saveErr, logx.Fields{
				"job_id":     job.JobID,
				"object_key": seg.ObjectKey,
			})
		}
	}

	if err := m.jobRepository.MarkCompleted(ctx, job.JobID); err != nil {
		logx.Error("worker.job.mark_completed_failed", err, logx.Fields{
			"job_id": job.JobID,
		})
	}
	if m.jobExecutionRepository != nil {
		if err := m.jobExecutionRepository.MarkCompleted(ctx, job.JobID, job.LeaseGeneration); err != nil {
			logx.Error("worker.job.mark_execution_completed_failed", err, logx.Fields{
				"job_id": job.JobID,
			})
		}
	}
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
	if err := m.outboxRepository.Save(ctx, event); err != nil {
		logx.Error("worker.job.outbox_save_failed", err, logx.Fields{
			"job_id": job.JobID,
		})
	}
	logx.Info("worker.job.completed", logx.Fields{
		"job_id":     job.JobID,
		"request_id": job.RequestID,
		"event_id":   event.EventID,
	})

	go m.cleanupOutputDir(pipeline.OutputDir)

	manifestMPD := filepath.Join(pipeline.OutputDir, "manifest.mpd")
	if _, err := os.Stat(manifestMPD); err == nil {
		os.Remove(manifestMPD)
		logx.Info("worker.job.removed_mpd", logx.Fields{
			"job_id": job.JobID,
			"mpd":    manifestMPD,
		})
	}
}

func (m *Module) failJob(ctx context.Context, job model.TranscodeJob, errorCode string, errorMessage string) {
	if job.AttemptNo < m.cfg.Worker.UploadMaxRetryCount {
		if err := m.jobRepository.ResetToQueued(ctx, job.JobID); err != nil {
			logx.Error("worker.fail_job.reset_to_queued_failed", err, logx.Fields{
				"job_id": job.JobID,
			})
		} else {
			logx.Info("worker.fail_job.retried", logx.Fields{
				"job_id":       job.JobID,
				"attempt_no":   job.AttemptNo,
				"max_retry":    m.cfg.Worker.UploadMaxRetryCount,
				"error_code":   errorCode,
				"error_message": errorMessage,
			})
			return
		}
	}
	if err := m.jobRepository.MarkFailed(ctx, job.JobID, errorCode, errorMessage); err != nil {
		logx.Error("worker.fail_job.mark_failed_error", err, logx.Fields{
			"job_id": job.JobID,
		})
	}
	if err := m.jobRepository.UpdateProgress(ctx, job.JobID, 0, model.StageFailed); err != nil {
		logx.Error("worker.fail_job.update_progress_failed", err, logx.Fields{
			"job_id": job.JobID,
		})
	}
	m.progressStore.Save(ctx, model.ProgressSnapshot{
		JobID:  job.JobID,
		Status: model.JobStatusFailed,
		Stage:  model.StageFailed,
	})
	if m.jobExecutionRepository != nil {
		if err := m.jobExecutionRepository.MarkCompleted(ctx, job.JobID, job.LeaseGeneration); err != nil {
			logx.Error("worker.fail_job.mark_completed_failed", err, logx.Fields{
				"job_id": job.JobID,
			})
		}
	}
	logx.Error("worker.job.failed_permanently", nil, logx.Fields{
		"job_id":        job.JobID,
		"error_code":    errorCode,
		"error_message": errorMessage,
		"attempt_no":    job.AttemptNo,
	})
}

func (m *Module) cleanupOutputDir(dir string) {
	time.Sleep(30 * time.Second)
	if err := os.RemoveAll(dir); err != nil {
		logx.Error("worker.cleanup.failed", err, logx.Fields{
			"dir": dir,
		})
	}
}

func (m *Module) uploadSegments(ctx context.Context, segments []model.Segment) {
	if len(segments) == 0 {
		return
	}
	tasks := make([]model.UploadTask, 0, len(segments))
	for _, segment := range segments {
		_ = m.segmentRepository.MarkUploading(ctx, segment.SegmentID)
		logx.Info("worker.upload.start", logx.Fields{
			"segment_id":  segment.SegmentID,
			"job_id":      segment.JobID,
			"object_key":  segment.ObjectKey,
			"retry_count": segment.UploadRetryCount,
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

func (m *Module) ensureWorkerInstance(ctx context.Context) {
	if m.workerInstanceRepo == nil {
		return
	}
	if m.workerInstanceID != 0 {
		return
	}
	workerInstanceID, err := m.workerInstanceRepo.EnsureOnline(ctx, m.nodeID, m.workerID, m.workerID, m.workerID, m.machineFingerprint, m.startupInstanceID)
	if err != nil {
		logx.Error("worker.instance.ensure_online_failed", err, logx.Fields{
			"node_id":             m.nodeID,
			"worker_id":           m.workerID,
			"startup_instance_id": m.startupInstanceID,
		})
		return
	}
	m.workerInstanceID = workerInstanceID
}

func (m *Module) reportHeartbeatOnce(ctx context.Context) {
	heartbeat := model.WorkerHeartbeat{
		NodeID:             m.nodeID,
		WorkerID:           m.workerID,
		StartupInstanceID:  m.startupInstanceID,
		MachineFingerprint: m.machineFingerprint,
		Timestamp:          time.Now(),
	}
	if m.stateCache != nil {
		m.stateCache.SaveHeartbeat(ctx, heartbeat)
	}
	if m.workerInstanceRepo != nil {
		if err := m.workerInstanceRepo.TouchHeartbeat(ctx, m.workerID); err != nil {
			logx.Error("worker.instance.touch_heartbeat_failed", err, logx.Fields{
				"worker_id": m.workerID,
			})
		}
	}
}

func (m *Module) reportMetricsOnce(ctx context.Context) {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	gpuCapabilities := gpu.ToGPUCapabilities(gpu.Probe())
	gpuCapabilities = m.persistGPUCapabilities(ctx, gpuCapabilities)
	metrics := model.NodeMetrics{
		NodeID:                  m.nodeID,
		CPUUsagePercent:         0,
		MemoryUsagePercent:      approximateMemoryUsagePercent(mem),
		GPUMemoryUsagePercent:   0,
		UploadQueueDepth:        len(m.segmentRepository.ListPendingUpload(ctx, m.cfg.Scheduler.MaxNodeUploadConcurrency)),
		ActiveTranscodeSessions: m.hotpathBus.ActiveProgressCount(ctx),
		GPUCapabilities:         gpuCapabilities,
		Timestamp:               time.Now(),
	}
	reporter.ReportMetrics(ctx, m.stateCache, metrics)
}

func (m *Module) persistGPUCapabilities(ctx context.Context, capabilities []model.GPUCapability) []model.GPUCapability {
	if m.gpuDeviceRepository == nil || m.gpuCapabilityRepository == nil {
		return capabilities
	}
	items := make([]model.GPUCapability, 0, len(capabilities))
	for _, capability := range capabilities {
		deviceID, err := m.gpuDeviceRepository.SaveOrUpdateByCapability(ctx, m.nodeID, capability)
		if err != nil {
			logx.Error("worker.gpu.device.save_failed", err, logx.Fields{
				"node_id":   m.nodeID,
				"gpu_uuid":  capability.GPUUUID,
				"gpu_index": capability.GPUIndex,
			})
			items = append(items, capability)
			continue
		}
		capability.GPUDeviceID = deviceID
		items = append(items, capability)
	}
	if m.gpuCapabilityRepository != nil {
		if err := m.gpuCapabilityRepository.ReplaceLatestSnapshot(ctx, m.nodeID, m.startupInstanceID, m.currentProbeGeneration, items); err != nil {
			logx.Error("worker.gpu.capability_snapshot.save_failed", err, logx.Fields{
				"node_id":             m.nodeID,
				"startup_instance_id": m.startupInstanceID,
				"probe_generation":    m.currentProbeGeneration,
			})
		}
	}
	m.currentProbeGeneration++
	return items
}

func approximateMemoryUsagePercent(mem runtime.MemStats) int {
	if mem.Sys == 0 {
		return 0
	}
	used := int((mem.Alloc * 100) / mem.Sys)
	if used < 0 {
		return 0
	}
	if used > 100 {
		return 100
	}
	return used
}
