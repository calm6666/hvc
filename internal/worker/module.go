package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"hvc/internal/cluster"
	"hvc/internal/cluster/hotpath"
	"hvc/internal/config"
	"hvc/internal/configcenter"
	rediscache "hvc/internal/infra/cache/redis"
	"hvc/internal/infra/db/mysql"
	ffprobe "hvc/internal/infra/ffmpeg/probe"
	"hvc/internal/infra/gpu"
	"hvc/internal/infra/hoststats"
	"hvc/internal/infra/storage"
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
	effectiveConfig         *configcenter.EffectiveConfig
	nodeID                  uint64
	workerID                string
	jobRepository           *mysql.JobRepository
	renditionRepository     *mysql.TranscodeRenditionRepository
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
	uploaderConfig          config.StorageConfig
	uploaderMu              sync.RWMutex
	retryPolicy             RetryPolicy
	startupInstanceID       string
	machineFingerprint      string
	workerInstanceID        uint64
	currentProbeGeneration  uint64
	runningJobs             map[uint64]struct{}
	runningGPUSessions      map[int]int
	runningJobsMu           sync.Mutex
	hostStats               *hoststats.Collector
}

// NewModule 创建执行模块。
func NewModule(cfg config.DynamicRuntimeConfig, effectiveConfig *configcenter.EffectiveConfig, nodeID uint64, workerID string, jobRepository *mysql.JobRepository, renditionRepository *mysql.TranscodeRenditionRepository, segmentRepository *mysql.SegmentRepository, progressStore *rediscache.ProgressStore, outboxRepository *mysql.OutboxRepository, hotpathBus *hotpath.MemoryBus, stateCache *cluster.StateCache, workerInstanceRepo *mysql.WorkerInstanceRepository, gpuDeviceRepository *mysql.GPUDeviceRepository, gpuCapabilityRepository *mysql.WorkerCodecCapabilityRepository, jobExecutionRepository *mysql.JobExecutionRepository) *Module {
	uploader, _ := storage.Open(cfg.Storage)
	startupInstanceID := workerID + "-startup"
	machineFingerprint := "node-" + workerID
	return &Module{
		cfg:                     cfg,
		effectiveConfig:         effectiveConfig,
		nodeID:                  nodeID,
		workerID:                workerID,
		jobRepository:           jobRepository,
		renditionRepository:     renditionRepository,
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
		uploaderConfig:          cfg.Storage,
		retryPolicy: RetryPolicy{
			BaseDelay: cfg.Worker.UploadRetryBaseDelay,
			MaxDelay:  cfg.Worker.UploadRetryMaxDelay,
			MaxRetry:  cfg.Worker.UploadMaxRetryCount,
		},
		startupInstanceID:      startupInstanceID,
		machineFingerprint:     machineFingerprint,
		currentProbeGeneration: 1,
		runningJobs:            make(map[uint64]struct{}),
		runningGPUSessions:     make(map[int]int),
		hostStats:              hoststats.NewCollector(),
	}
}

// CanUseSoftwareDecode 判断是否允许软解。
func (m *Module) CanUseSoftwareDecode(cpuPercent int) bool {
	cfg := m.currentConfig()
	if !cfg.Scheduler.AllowSoftwareDecodeFallback {
		return false
	}
	return cpuPercent < cfg.Scheduler.SoftDecodeCPULimitPercent
}

// CanAcceptNewTask 判断是否允许接收新任务。
func (m *Module) CanAcceptNewTask(cpuPercent, memPercent, gpuMemPercent, uploadQueueDepth, activeSessions int) bool {
	cfg := m.currentConfig()
	if cpuPercent >= cfg.Scheduler.NodeCPUSafetyLimitPercent {
		return false
	}
	if memPercent >= cfg.Scheduler.NodeMemorySafetyLimitPercent {
		return false
	}
	if gpuMemPercent >= cfg.Scheduler.NodeGPUSafetyLimitPercent {
		return false
	}
	if activeSessions >= cfg.Scheduler.MaxNodeTranscodeSessions {
		return false
	}
	if uploadQueueDepth >= cfg.Scheduler.MaxNodeUploadConcurrency {
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

	jobSem := make(chan struct{}, 1024)
	uploadSem := make(chan struct{}, 1024)
	for {
		cfg := m.currentConfig()
		m.refreshRuntimeState(cfg)
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(loopInterval(cfg.Worker.LoopInterval, 2*time.Second)):
			if !cfg.Mode.EnableWorker {
				continue
			}
			m.reportHeartbeatOnce(ctx)
			m.reportMetricsOnce(ctx)
			m.dispatchJobs(ctx, cfg, jobSem)
			m.dispatchUploads(ctx, cfg, uploadSem)
		}
	}
}

// dispatchJobs 异步分发转码任务。
//
// 使用信号量控制最大并发转码数，避免资源过载。
// 每个任务在独立 goroutine 中执行，不阻塞主循环。
func (m *Module) dispatchJobs(ctx context.Context, cfg config.DynamicRuntimeConfig, sem chan struct{}) {
	jobs := m.jobRepository.ListAssigned(ctx, m.nodeID, m.workerID)
	for _, job := range jobs {
		m.runningJobsMu.Lock()
		_, running := m.runningJobs[job.JobID]
		if running {
			m.runningJobsMu.Unlock()
			continue
		}
		if cfg.Scheduler.MaxNodeTranscodeSessions > 0 && len(sem) >= cfg.Scheduler.MaxNodeTranscodeSessions {
			m.runningJobsMu.Unlock()
			return
		}
		select {
		case sem <- struct{}{}:
			m.runningJobs[job.JobID] = struct{}{}
			m.incrementGPUSessionLocked(job.SelectedGPUIndex)
			m.runningJobsMu.Unlock()
			go func(j model.TranscodeJob) {
				defer func() {
					<-sem
					m.runningJobsMu.Lock()
					delete(m.runningJobs, j.JobID)
					m.decrementGPUSessionLocked(j.SelectedGPUIndex)
					m.runningJobsMu.Unlock()
				}()
				m.executeJob(ctx, j)
			}(job)
		default:
			m.runningJobsMu.Unlock()
		}
	}
}

// dispatchUploads 异步分发上传任务。
//
// 使用信号量控制最大并发上传数。
// 每批上传在独立 goroutine 中执行，不阻塞主循环。
func (m *Module) dispatchUploads(ctx context.Context, cfg config.DynamicRuntimeConfig, sem chan struct{}) {
	segments := m.segmentRepository.ListPendingUpload(ctx, cfg.Worker.SingleJobUploadConcurrency, cfg.Worker.UploadMaxRetryCount)
	if len(segments) == 0 {
		return
	}
	if cfg.Scheduler.MaxNodeUploadConcurrency > 0 && len(sem) >= cfg.Scheduler.MaxNodeUploadConcurrency {
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
	cfg := m.currentConfig()
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
	naming.JobID = job.JobID
	if template, err := m.jobRepository.EnsureSegmentTemplateSnapshot(ctx, job.JobID, cfg.Worker.SegmentTemplate); err != nil {
		logx.Error("worker.job.lock_segment_template_failed", err, logx.Fields{
			"job_id": job.JobID,
		})
		m.failJob(ctx, job, "SEGMENT_TEMPLATE_LOCK_FAILED", err.Error())
		return
	} else {
		naming.SegmentTemplate = template
		job.SegmentTemplate = template
	}
	if job.OutputBasePrefix != "" {
		naming.ObjectKeyPrefix = job.OutputBasePrefix
	} else if cfg.Storage.BasePrefix != "" {
		naming.ObjectKeyPrefix = cfg.Storage.BasePrefix
	}
	pipeline := planner.BuildPlan(job, probeResult, executionHW, naming)
	if err := m.attachStableRenditionKeys(ctx, job, pipeline.Renditions); err != nil {
		logx.Error("worker.job.ensure_renditions_failed", err, logx.Fields{
			"job_id": job.JobID,
		})
		m.failJob(ctx, job, "RENDITION_SNAPSHOT_FAILED", err.Error())
		return
	}
	logx.Info("worker.job.pipeline", logx.Fields{
		"job_id":          job.JobID,
		"execution_hw":    executionHW,
		"output_dir":      pipeline.OutputDir,
		"rendition_count": len(pipeline.Renditions),
		"hw_decode":       pipeline.HardwareDecode,
		"hw_encode":       pipeline.HardwareEncode,
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
		segmentType := "media"
		if seg.IsInit {
			segmentType = "init"
		}
		_, saveErr := m.segmentRepository.Save(ctx, model.Segment{
			JobID:            job.JobID,
			RenditionID:      resolveRenditionID(pipeline.Renditions, seg.RenditionName),
			RenditionName:    seg.RenditionName,
			RenditionKey:     seg.RenditionKey,
			SegmentType:      segmentType,
			MediaType:        seg.MediaType,
			IsInitSegment:    seg.IsInit,
			SequenceNo:       seg.SequenceNo,
			DurationMS:       job.SegmentDurationSec * 1000,
			Width:            seg.Width,
			Height:           seg.Height,
			VideoBitrateKbps: seg.VideoBitrateKbps,
			AudioBitrateKbps: seg.AudioBitrateKbps,
			VideoCodec:       seg.VideoCodec,
			AudioCodec:       resolveAudioCodec(pipeline.Renditions, seg.RenditionName),
			SupportDash:      job.SupportDash,
			SupportHLS:       job.SupportHLS,
			CodecName:        probeResult.VideoCodec,
			ObjectKey:        seg.ObjectKey,
			UploadStatus:     model.SegmentUploadPending,
			CreatedAt:        time.Now(),
			UpdatedAt:        time.Now(),
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
	payload := m.buildCompletedPayload(ctx, job, probeResult, discoverResult)
	payloadJSON, _ := json.Marshal(payload)
	event := model.OutboxEvent{
		EventID:       idgen.Next(),
		EventType:     "transcode.completed",
		JobID:         job.JobID,
		RequestID:     job.RequestID,
		PayloadJSON:   string(payloadJSON),
		Status:        model.OutboxStatusPending,
		MaxRetryCount: cfg.Worker.UploadMaxRetryCount,
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
	cfg := m.currentConfig()
	if job.AttemptNo < cfg.Worker.UploadMaxRetryCount {
		if err := m.jobRepository.ResetToQueued(ctx, job.JobID); err != nil {
			logx.Error("worker.fail_job.reset_to_queued_failed", err, logx.Fields{
				"job_id": job.JobID,
			})
		} else {
			logx.Info("worker.fail_job.retried", logx.Fields{
				"job_id":        job.JobID,
				"attempt_no":    job.AttemptNo,
				"max_retry":     cfg.Worker.UploadMaxRetryCount,
				"error_code":    errorCode,
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

	failedPayload := model.TranscodeFailedPayload{
		JobID:        job.JobID,
		RequestID:    job.RequestID,
		BizKey:       job.BizKey,
		SourceURL:    job.SourceURL,
		Status:       model.JobStatusFailed,
		StatusName:   "failed",
		ErrorCode:    errorCode,
		ErrorMessage: errorMessage,
		FailedStage:  strings.ToLower(strings.TrimSuffix(errorCode, "_FAILED")),
		RetryCount:   job.AttemptNo,
	}
	failedPayloadJSON, _ := json.Marshal(failedPayload)
	failedEvent := model.OutboxEvent{
		EventID:       idgen.Next(),
		EventType:     "transcode.failed",
		JobID:         job.JobID,
		RequestID:     job.RequestID,
		PayloadJSON:   string(failedPayloadJSON),
		Status:        model.OutboxStatusPending,
		MaxRetryCount: cfg.Worker.UploadMaxRetryCount,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	if err := m.outboxRepository.Save(ctx, failedEvent); err != nil {
		logx.Error("worker.fail_job.outbox_save_failed", err, logx.Fields{
			"job_id": job.JobID,
		})
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
	cfg := m.currentConfig()
	uploader := m.currentUploader()
	if uploader == nil {
		logx.Error("worker.upload.uploader_not_ready", nil, nil)
		return
	}
	tasks := make([]model.UploadTask, 0, len(segments))
	for _, segment := range segments {
		if err := m.segmentRepository.MarkUploading(ctx, segment.SegmentID); err != nil {
			logx.Error("worker.upload.mark_uploading_failed", err, logx.Fields{
				"segment_id": segment.SegmentID,
				"job_id":     segment.JobID,
			})
			continue
		}
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
			ObjectKey:   uploader.BuildObjectKey(segment),
			LocalPath:   segment.ObjectKey,
			RetryCount:  segment.UploadRetryCount,
			CreatedAt:   time.Now(),
		})
	}
	if len(tasks) == 0 {
		return
	}
	pool := uploadworker.NewWorkerPool(cfg.Worker.SingleJobUploadConcurrency, uploader)
	pool.SetRetryPolicy(m.retryPolicy.MaxRetry, m.retryPolicy.BaseDelay, m.retryPolicy.MaxDelay)
	results := pool.Run(ctx, tasks)
	for _, result := range results {
		if result.Success {
			if err := m.segmentRepository.MarkUploaded(ctx, result.SegmentID, result.ObjectETag, result.ObjectSizeBytes); err != nil {
				logx.Error("worker.upload.mark_uploaded_failed", err, logx.Fields{
					"segment_id": result.SegmentID,
				})
				continue
			}
			logx.Info("worker.upload.success", logx.Fields{
				"segment_id":        result.SegmentID,
				"object_etag":       result.ObjectETag,
				"object_size_bytes": result.ObjectSizeBytes,
			})
			continue
		}
		if err := m.segmentRepository.MarkUploadFailed(ctx, result.SegmentID, result.ErrorMessage); err != nil {
			logx.Error("worker.upload.mark_failed_failed", err, logx.Fields{
				"segment_id": result.SegmentID,
			})
			continue
		}
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
	cfg := m.currentConfig()
	snapshot := hoststats.Snapshot{}
	if m.hostStats != nil {
		snapshot = m.hostStats.Collect()
	}
	gpuCapabilities := gpu.ToGPUCapabilities(gpu.Probe())
	gpuCapabilities = m.attachRuntimeGPUStats(gpuCapabilities, snapshot.GPUDevices)
	gpuCapabilities = m.persistGPUCapabilities(ctx, gpuCapabilities)
	metrics := model.NodeMetrics{
		NodeID:                  m.nodeID,
		CPUUsagePercent:         snapshot.CPUUsagePercent,
		MemoryUsagePercent:      snapshot.MemoryUsagePercent,
		GPUMemoryUsagePercent:   snapshot.GPUMemoryUsagePercent,
		UploadQueueDepth:        len(m.segmentRepository.ListPendingUpload(ctx, cfg.Scheduler.MaxNodeUploadConcurrency, cfg.Worker.UploadMaxRetryCount)),
		ActiveTranscodeSessions: m.hotpathBus.ActiveProgressCount(ctx),
		GPUCapabilities:         gpuCapabilities,
		Timestamp:               time.Now(),
	}
	reporter.ReportMetrics(ctx, m.stateCache, metrics)
}

func (m *Module) attachRuntimeGPUStats(capabilities []model.GPUCapability, runtime []hoststats.GPUDeviceSnapshot) []model.GPUCapability {
	if len(capabilities) == 0 {
		return capabilities
	}

	runtimeByUUID := make(map[string]hoststats.GPUDeviceSnapshot, len(runtime))
	runtimeByIndex := make(map[int]hoststats.GPUDeviceSnapshot, len(runtime))
	for _, item := range runtime {
		if item.GPUUUID != "" {
			runtimeByUUID[item.GPUUUID] = item
		}
		runtimeByIndex[item.GPUIndex] = item
	}

	sessionCounts := m.currentGPUSessionCounts()
	for idx := range capabilities {
		if item, ok := runtimeByUUID[capabilities[idx].GPUUUID]; ok {
			applyRuntimeGPUStat(&capabilities[idx], item)
		} else if item, ok := runtimeByIndex[capabilities[idx].GPUIndex]; ok {
			applyRuntimeGPUStat(&capabilities[idx], item)
		}
		capabilities[idx].ActiveSessions = sessionCounts[capabilities[idx].GPUIndex]
	}
	return capabilities
}

func applyRuntimeGPUStat(capability *model.GPUCapability, stat hoststats.GPUDeviceSnapshot) {
	if capability == nil {
		return
	}
	if capability.MemoryTotalMB <= 0 && stat.MemoryTotalMB > 0 {
		capability.MemoryTotalMB = stat.MemoryTotalMB
	}
	capability.GPUMemoryUsedMB = stat.MemoryUsedMB
	capability.GPUMemoryUsagePercent = stat.MemoryUsagePercent
	capability.GPUUtilizationPercent = stat.GPUUtilizationPercent
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
		if err := m.gpuCapabilityRepository.ReplaceLatestSnapshot(ctx, m.nodeID, m.workerInstanceID, m.startupInstanceID, m.machineFingerprint, m.currentProbeGeneration, items); err != nil {
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

func (m *Module) attachStableRenditionKeys(ctx context.Context, job model.TranscodeJob, renditions []planner.RenditionSpec) error {
	if len(renditions) == 0 || m.renditionRepository == nil {
		return nil
	}

	items := make([]model.TranscodeRendition, 0, len(renditions))
	seenNames := make(map[string]struct{}, len(renditions))
	for _, rend := range renditions {
		if _, exists := seenNames[rend.Name]; exists {
			return fmt.Errorf("duplicate rendition name: %s", rend.Name)
		}
		seenNames[rend.Name] = struct{}{}
		items = append(items, model.TranscodeRendition{
			JobID:            job.JobID,
			RenditionName:    rend.Name,
			RenditionKey:     model.BuildStableRenditionKey(job.JobID, rend.Name, rend.Width, rend.Height, rend.VideoBitrateKbps, rend.AudioBitrateKbps, rend.VideoCodec, rend.AudioCodec),
			Status:           model.JobStatusRunning,
			OutWidth:         rend.Width,
			OutHeight:        rend.Height,
			VideoCodec:       rend.VideoCodec,
			AudioCodec:       rend.AudioCodec,
			VideoBitrateKbps: rend.VideoBitrateKbps,
			AudioBitrateKbps: rend.AudioBitrateKbps,
		})
	}

	stored, err := m.renditionRepository.EnsureForJob(ctx, items)
	if err != nil {
		return err
	}

	storedByName := make(map[string]model.TranscodeRendition, len(stored))
	for _, item := range stored {
		storedByName[item.RenditionName] = item
	}

	for idx := range renditions {
		if item, ok := storedByName[renditions[idx].Name]; ok {
			renditions[idx].RenditionID = item.RenditionID
			renditions[idx].RenditionKey = item.RenditionKey
		}
		if renditions[idx].RenditionKey == "" {
			renditions[idx].RenditionKey = model.BuildStableRenditionKey(
				job.JobID,
				renditions[idx].Name,
				renditions[idx].Width,
				renditions[idx].Height,
				renditions[idx].VideoBitrateKbps,
				renditions[idx].AudioBitrateKbps,
				renditions[idx].VideoCodec,
				renditions[idx].AudioCodec,
			)
		}
	}
	return nil
}

func resolveRenditionID(renditions []planner.RenditionSpec, renditionName string) uint64 {
	for _, rend := range renditions {
		if rend.Name == renditionName {
			return rend.RenditionID
		}
	}
	return 0
}

func resolveAudioCodec(renditions []planner.RenditionSpec, renditionName string) string {
	for _, rend := range renditions {
		if rend.Name == renditionName {
			return rend.AudioCodec
		}
	}
	return ""
}

// buildCompletedPayload 构建转码完成回调载荷。
//
// 从任务信息、探测结果和分片发现结果中提取完整信息，
// 生成包含源视频信息、各清晰度输出详情和播放地址的回调载荷。
func (m *Module) buildCompletedPayload(ctx context.Context, job model.TranscodeJob, probeResult ffprobe.Result, discoverResult segmenter.Result) model.TranscodeCompletedPayload {
	cfg := m.currentConfig()
	renditionMap := make(map[string]*model.CompletedRendition)
	totalSegments := 0
	var totalSizeBytes uint64

	for _, seg := range discoverResult.Segments {
		totalSegments++
		totalSizeBytes += uint64(seg.FileSize)

		key := seg.RenditionName
		rend, ok := renditionMap[key]
		if !ok {
			rend = &model.CompletedRendition{
				RenditionName:         seg.RenditionName,
				RenditionKey:          seg.RenditionKey,
				QualityLabel:          seg.QualityLabel,
				Width:                 seg.Width,
				Height:                seg.Height,
				VideoCodec:            seg.VideoCodec,
				VideoBitrateKbps:      seg.VideoBitrateKbps,
				AudioBitrateKbps:      seg.AudioBitrateKbps,
				ManifestDashURL:       fmt.Sprintf("/v1/manifest/dash/%d.mpd", job.JobID),
				ManifestHLSURL:        fmt.Sprintf("/v1/manifest/hls/%d.m3u8", job.JobID),
				ManifestHLSVariantURL: fmt.Sprintf("/v1/manifest/hls/%d/%s.m3u8", job.JobID, seg.RenditionName),
			}
			renditionMap[key] = rend
		}
		if seg.IsInit {
			rend.InitSegmentObjectKey = seg.ObjectKey
		} else {
			rend.SegmentCount++
		}
	}

	renditions := make([]model.CompletedRendition, 0, len(renditionMap))
	for _, rend := range renditionMap {
		renditions = append(renditions, *rend)
	}

	storageType := "s3"
	if cfg.Storage.StorageType == "local" {
		storageType = "local"
	}
	segmentTemplate := job.SegmentTemplate
	if segmentTemplate == "" {
		segmentTemplate = cfg.Worker.SegmentTemplate
	}

	return model.TranscodeCompletedPayload{
		JobID:           job.JobID,
		RequestID:       job.RequestID,
		BizKey:          job.BizKey,
		SourceURL:       job.SourceURL,
		Status:          model.JobStatusCompleted,
		StatusName:      "completed",
		DurationMS:      int64(probeResult.DurationMS),
		SegmentDuration: job.SegmentDurationSec,
		SupportDash:     job.SupportDash,
		SupportHLS:      job.SupportHLS,
		SegmentTemplate: segmentTemplate,
		StorageType:     storageType,
		StorageBucket:   cfg.Storage.Bucket,
		PlayDomain:      cfg.Storage.PlayDomain,
		SourceInfo: model.SourceInfo{
			Width:            probeResult.Width,
			Height:           probeResult.Height,
			VideoCodec:       probeResult.VideoCodec,
			VideoBitrateKbps: probeResult.VideoBitrateKbps,
			AudioCodec:       probeResult.AudioCodec,
			AudioBitrateKbps: probeResult.AudioBitrateKbps,
			FPS:              probeResult.FPS,
			DurationMS:       int64(probeResult.DurationMS),
		},
		Renditions:        renditions,
		TotalSegmentCount: totalSegments,
		TotalSizeBytes:    totalSizeBytes,
	}
}

func (m *Module) currentConfig() config.DynamicRuntimeConfig {
	if m.effectiveConfig == nil {
		return m.cfg
	}
	return m.effectiveConfig.Snapshot()
}

func (m *Module) refreshRuntimeState(cfg config.DynamicRuntimeConfig) {
	m.uploaderMu.Lock()
	defer m.uploaderMu.Unlock()
	if m.uploader != nil && m.uploaderConfig == cfg.Storage {
		m.retryPolicy = RetryPolicy{
			BaseDelay: cfg.Worker.UploadRetryBaseDelay,
			MaxDelay:  cfg.Worker.UploadRetryMaxDelay,
			MaxRetry:  cfg.Worker.UploadMaxRetryCount,
		}
		return
	}
	uploader, err := storage.Open(cfg.Storage)
	if err != nil {
		logx.Error("worker.storage.reopen_failed", err, logx.Fields{
			"storage_endpoint": cfg.Storage.Endpoint,
			"storage_bucket":   cfg.Storage.Bucket,
		})
		return
	}
	m.uploader = uploader
	m.uploaderConfig = cfg.Storage
	m.retryPolicy = RetryPolicy{
		BaseDelay: cfg.Worker.UploadRetryBaseDelay,
		MaxDelay:  cfg.Worker.UploadRetryMaxDelay,
		MaxRetry:  cfg.Worker.UploadMaxRetryCount,
	}
}

func (m *Module) currentUploader() *storage.Client {
	m.uploaderMu.RLock()
	defer m.uploaderMu.RUnlock()
	return m.uploader
}

func (m *Module) currentGPUSessionCounts() map[int]int {
	m.runningJobsMu.Lock()
	defer m.runningJobsMu.Unlock()
	result := make(map[int]int, len(m.runningGPUSessions))
	for gpuIndex, count := range m.runningGPUSessions {
		result[gpuIndex] = count
	}
	return result
}

func (m *Module) incrementGPUSessionLocked(gpuIndex int) {
	if gpuIndex < 0 {
		return
	}
	m.runningGPUSessions[gpuIndex]++
}

func (m *Module) decrementGPUSessionLocked(gpuIndex int) {
	if gpuIndex < 0 {
		return
	}
	if count := m.runningGPUSessions[gpuIndex] - 1; count > 0 {
		m.runningGPUSessions[gpuIndex] = count
		return
	}
	delete(m.runningGPUSessions, gpuIndex)
}

func loopInterval(interval time.Duration, fallback time.Duration) time.Duration {
	if interval > 0 {
		return interval
	}
	return fallback
}
