package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
	nodeMode                string
	nodeID                  uint64
	workerID                string
	jobRepository           *mysql.JobRepository
	renditionRepository     *mysql.TranscodeRenditionRepository
	segmentRepository       *mysql.SegmentRepository
	progressStore           *rediscache.ProgressStore
	outboxRepository        *mysql.OutboxRepository
	workerInstanceRepo      *mysql.WorkerInstanceRepository
	clusterNodeRepository   *mysql.ClusterNodeRepository
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
	controlMu               sync.RWMutex
	offlineRequested        bool
	offlineReason           string
	exitRequested           bool
	exitReason              string
	runCancel               context.CancelFunc
	lastNodeSnapshotAt      time.Time
	lastNodeSnapshotTags    string
	lastNodeSnapshotCPU     int
	lastNodeSnapshotMemory  int
	lastNodeSnapshotTrans   int
	lastNodeSnapshotUpload  int
	lastNodeHeartbeatAt     time.Time
	lastWorkerHeartbeatAt   time.Time
	jobRuntimePersistGate   *cluster.JobRuntimePersistGate
}

const nodeSnapshotPersistInterval = 30 * time.Second

// NewModule 创建执行模块。
func NewModule(cfg config.DynamicRuntimeConfig, effectiveConfig *configcenter.EffectiveConfig, nodeMode string, nodeID uint64, workerID string, jobRepository *mysql.JobRepository, renditionRepository *mysql.TranscodeRenditionRepository, segmentRepository *mysql.SegmentRepository, progressStore *rediscache.ProgressStore, outboxRepository *mysql.OutboxRepository, hotpathBus *hotpath.MemoryBus, stateCache *cluster.StateCache, workerInstanceRepo *mysql.WorkerInstanceRepository, clusterNodeRepository *mysql.ClusterNodeRepository, gpuDeviceRepository *mysql.GPUDeviceRepository, gpuCapabilityRepository *mysql.WorkerCodecCapabilityRepository, jobExecutionRepository *mysql.JobExecutionRepository) *Module {
	uploader, _ := storage.Open(cfg.Storage)
	startupInstanceID := workerID + "-startup"
	machineFingerprint := "node-" + workerID
	return &Module{
		cfg:                     cfg,
		effectiveConfig:         effectiveConfig,
		nodeMode:                nodeMode,
		nodeID:                  nodeID,
		workerID:                workerID,
		jobRepository:           jobRepository,
		renditionRepository:     renditionRepository,
		segmentRepository:       segmentRepository,
		progressStore:           progressStore,
		outboxRepository:        outboxRepository,
		workerInstanceRepo:      workerInstanceRepo,
		clusterNodeRepository:   clusterNodeRepository,
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
		jobRuntimePersistGate:  cluster.NewJobRuntimePersistGate(),
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
	runCtx, cancel := context.WithCancel(ctx)
	m.setRunCancel(cancel)
	defer func() {
		m.clearRunCancel(cancel)
		m.markWorkerExited(context.Background(), m.shutdownReason(ctx.Err()))
	}()

	jobSem := make(chan struct{}, 1024)
	uploadSem := make(chan struct{}, 1024)
	for {
		cfg := m.currentConfig()
		m.refreshRuntimeState(cfg)
		select {
		case <-runCtx.Done():
			return nil
		case <-time.After(loopInterval(cfg.Worker.LoopInterval, 2*time.Second)):
			if !cfg.Mode.EnableWorker {
				continue
			}
			m.reportHeartbeatOnce(runCtx)
			m.reportMetricsOnce(runCtx)
			m.dispatchJobs(runCtx, cfg, jobSem)
			m.dispatchUploads(runCtx, cfg, uploadSem)
		}
	}
}

// dispatchJobs 异步分发转码任务。
//
// 使用信号量控制最大并发转码数，避免资源过载。
// 每个任务在独立 goroutine 中执行，不阻塞主循环。
func (m *Module) dispatchJobs(ctx context.Context, cfg config.DynamicRuntimeConfig, sem chan struct{}) {
	if !m.acceptingNewAssignments() {
		return
	}
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

	/*
	 * B2 断点续跑的第一道判据：先确认"这一版输入"与上一次执行的是不是同一份。
	 *   · 任务行指纹为空   ⇒ 首次执行，记下当前指纹（之后各步骤行都带同一个指纹）；
	 *   · 与当前指纹一致   ⇒ 允许按步骤续跑（下面各阶段的 Running/Done 标记才有意义）；
	 *   · 不一致（换源/改规格）⇒ 清指纹 + 清步骤行，整任务重跑。不清就会把上一版输入的中间产物
	 *     当成本版结果（例如两版清晰度的分片混进同一份清单）。
	 * 指纹只含源 URL 与输出规格，不含源文件大小/ETag：那两项在网络抖动时可能取不到，
	 * 放进来会把"这次没取到"误判成"输入变了"而白白全量重跑（见 computeJobInputHash 的说明）。
	 */
	inputHash := computeJobInputHash(job)
	resumeAllowed := false

	switch {
	case job.InputHash == "":
		if err := m.jobRepository.SetInputHash(ctx, job.JobID, inputHash); err != nil {
			logx.Error("worker.job.set_input_hash_failed", err, logx.Fields{
				"job_id": job.JobID,
			})
		}

	case job.InputHash == inputHash:
		resumeAllowed = true
		logx.Info("worker.job.input_unchanged", logx.Fields{
			"job_id":    job.JobID,
			"input_hash": inputHash,
			"steps":     len(m.jobRepository.ListJobSteps(ctx, job.JobID)),
		})

	default:
		logx.Info("worker.job.input_changed", logx.Fields{
			"job_id":   job.JobID,
			"old_hash": job.InputHash,
			"new_hash": inputHash,
		})
		if err := m.jobRepository.DeleteJobSteps(ctx, job.JobID); err != nil {
			logx.Error("worker.job.clear_steps_failed", err, logx.Fields{
				"job_id": job.JobID,
			})
		}
		if err := m.jobRepository.ResetInputHash(ctx, job.JobID); err != nil {
			logx.Error("worker.job.reset_input_hash_failed", err, logx.Fields{
				"job_id": job.JobID,
			})
		}
		if err := m.jobRepository.SetInputHash(ctx, job.JobID, inputHash); err != nil {
			logx.Error("worker.job.set_input_hash_failed", err, logx.Fields{
				"job_id": job.JobID,
			})
		}
	}

	/* PROBE：标记开始/结束。注意——ffprobe 结果目前还没有落库（步骤行的 detail 列就是为它准备的），
	 * 所以续跑时这一步仍会真的重跑一次；等结果进 detail 之后才能跳过。 */
	_ = m.jobRepository.UpsertJobStep(ctx, job.JobID, mysql.JobStepProbe, mysql.JobStepRunning, inputHash, nil)

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

	_ = m.jobRepository.UpsertJobStep(ctx, job.JobID, mysql.JobStepProbe, mysql.JobStepDone, inputHash, nil)
	logx.Info("worker.job.resume_decision", logx.Fields{
		"job_id":         job.JobID,
		"input_hash":     inputHash,
		"resume_allowed": resumeAllowed,
		"probe_done":     m.jobRepository.IsJobStepDone(ctx, job.JobID, mysql.JobStepProbe),
		"plan_done":      m.jobRepository.IsJobStepDone(ctx, job.JobID, mysql.JobStepPlan),
		"segment_done":   m.jobRepository.IsJobStepDone(ctx, job.JobID, mysql.JobStepSegment),
		"upload_done":    m.jobRepository.IsJobStepDone(ctx, job.JobID, mysql.JobStepUpload),
	})

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

	_ = m.jobRepository.UpsertJobStep(ctx, job.JobID, mysql.JobStepPlan, mysql.JobStepDone, inputHash, nil)
	/* UPLOAD：先把这一步标成进行中（真正的完成标记在 uploadSegments 全部分片传完后写），
	 * 这样续跑时"上传没做完"这件事在步骤表里是可见的，而不是只看分片行的状态去猜。 */
	_ = m.jobRepository.UpsertJobStep(ctx, job.JobID, mysql.JobStepUpload, mysql.JobStepRunning, inputHash, nil)

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
		snapshot.UpdatedAt = time.Now()
		m.progressStore.Save(ctx, snapshot)
		m.hotpathBus.SaveProgress(ctx, progress)
		if m.jobRepository != nil && m.jobRuntimePersistGate.ShouldPersistProgress(snapshot, snapshot.UpdatedAt) {
			if err := m.jobRepository.UpdateProgressAt(ctx, job.JobID, snapshot.ProgressPermille, snapshot.Stage, snapshot.UpdatedAt); err != nil {
				logx.Error("worker.job.update_progress_failed", err, logx.Fields{
					"job_id": job.JobID,
				})
			} else {
				m.jobRuntimePersistGate.MarkProgressPersisted(snapshot, snapshot.UpdatedAt)
			}
		}
		if m.jobExecutionRepository != nil && m.jobRuntimePersistGate.ShouldPersistExecutionHeartbeat(snapshot.UpdatedAt, job.JobID, job.LeaseGeneration, cfg.Scheduler.WorkerHeartbeatTimeout) {
			if err := m.jobExecutionRepository.TouchHeartbeatAt(ctx, job.JobID, job.LeaseGeneration, snapshot.UpdatedAt); err == nil {
				m.jobRuntimePersistGate.MarkExecutionHeartbeatPersisted(job.JobID, job.LeaseGeneration, snapshot.UpdatedAt)
			}
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

	_ = m.jobRepository.UpsertJobStep(ctx, job.JobID, mysql.JobStepSegment, mysql.JobStepRunning, inputHash, nil)
	discoverResult := segmenter.Discover(job, pipeline)
	/* 分片发现的结果本身就是这一步的产出（分片行随后逐条落库，续跑时以那些行为准），
	 * 所以标记完成放在发现函数返回之后。 */
	_ = m.jobRepository.UpsertJobStep(ctx, job.JobID, mysql.JobStepSegment, mysql.JobStepDone, inputHash, nil)
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
			// B3：分片内容摘要随分片行一起落库（生产侧算一次），发布校验与清单物化都用它。
			SHA256:           computeFileSHA256(seg.ObjectKey),
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
	// A1：这里只把完成回调要用的 payload **落库暂存**，不立刻发回调。
	//
	// 分片是之后由全局待传队列异步上传的（别的节点也可能在传这个任务的分片），此刻发回调会让
	// 下游拉到残缺清单。待传数归零后由 publishCompletion 取出 payload、逐片校验、推进
	// PUBLISHED 并写 outbox（回调）事件。
	payload := m.buildCompletedPayload(ctx, job, probeResult, discoverResult)
	payloadJSON, _ := json.Marshal(payload)
	if err := m.jobRepository.SavePendingCompletion(ctx, job.JobID, string(payloadJSON)); err != nil {
		// 暂存失败就退回旧行为（立刻写回调事件）：宁可回调早于分片，也不能让任务永远不回调。
		logx.Error("worker.job.save_pending_completion_failed", err, logx.Fields{
			"job_id": job.JobID,
		})
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
		if saveErr := m.outboxRepository.Save(ctx, event); saveErr != nil {
			logx.Error("worker.job.outbox_save_failed", saveErr, logx.Fields{
				"job_id": job.JobID,
			})
		}
	} else {
		// 小文件/极快上传：分片可能此刻已经全部传完，立即尝试发布一次。
		m.publishCompletion(ctx, job.JobID)
	}
	logx.Info("worker.job.completed", logx.Fields{
		"job_id":     job.JobID,
		"request_id": job.RequestID,
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

	// A1：本批分片上传完成后立即检查"本任务待传数是否归零"，归零就发布（写回调 outbox 事件）。
	//
	// 就地触发是主路径（比对账更快，回调更及时）；跨节点上传与本进程重启由 ReconcilePublish 兜底。
	// 同一任务可能有多片落在这一批里，按任务去重，避免对同一个任务重复查待传数。
	publishedJobs := make(map[uint64]struct{}, len(tasks))
	for _, task := range tasks {
		if task.JobID == 0 {
			continue
		}
		if _, done := publishedJobs[task.JobID]; done {
			continue
		}
		publishedJobs[task.JobID] = struct{}{}
		/* B2：本任务的分片全部传完 ⇒ UPLOAD 步骤完成（inputHash 传空串表示"不覆盖已记的指纹"）。
		 * 续跑时这一步的完成状态与分片行一起构成"上传不用重做"的依据。 */
		_ = m.jobRepository.UpsertJobStep(ctx, task.JobID, mysql.JobStepUpload, mysql.JobStepDone, "", nil)
		m.publishCompletion(ctx, task.JobID)
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
	cfg := m.currentConfig()
	heartbeatAt := time.Now()
	heartbeat := model.WorkerHeartbeat{
		NodeID:             m.nodeID,
		WorkerID:           m.workerID,
		StartupInstanceID:  m.startupInstanceID,
		MachineFingerprint: m.machineFingerprint,
		Timestamp:          heartbeatAt,
	}
	if m.stateCache != nil {
		m.stateCache.SaveHeartbeat(ctx, heartbeat)
	}
	if m.clusterNodeRepository != nil && m.shouldPersistDBHeartbeat(heartbeatAt, m.lastNodeHeartbeatAt, cfg.Scheduler.WorkerHeartbeatTimeout) {
		if err := m.clusterNodeRepository.TouchHeartbeat(ctx, m.nodeID, heartbeatAt); err != nil {
			logx.Error("worker.node.touch_heartbeat_failed", err, logx.Fields{
				"node_id": m.nodeID,
			})
		} else {
			m.lastNodeHeartbeatAt = heartbeatAt
		}
	}
	if m.workerInstanceRepo != nil && m.shouldPersistDBHeartbeat(heartbeatAt, m.lastWorkerHeartbeatAt, cfg.Scheduler.WorkerHeartbeatTimeout) {
		if err := m.workerInstanceRepo.TouchHeartbeat(ctx, m.workerID, heartbeatAt); err != nil {
			logx.Error("worker.instance.touch_heartbeat_failed", err, logx.Fields{
				"worker_id": m.workerID,
			})
		} else {
			m.lastWorkerHeartbeatAt = heartbeatAt
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
		UploadQueueDepth:        m.segmentRepository.CountPendingUpload(ctx, cfg.Worker.UploadMaxRetryCount),
		ActiveTranscodeSessions: m.hotpathBus.ActiveProgressCount(ctx),
		GPUCapabilities:         gpuCapabilities,
		Timestamp:               time.Now(),
	}
	reporter.ReportMetrics(ctx, m.stateCache, metrics)
	if m.clusterNodeRepository != nil {
		m.persistNodeSnapshotIfNeeded(ctx, cfg, gpuCapabilities)
	}
}

func (m *Module) persistNodeSnapshotIfNeeded(ctx context.Context, cfg config.DynamicRuntimeConfig, gpuCapabilities []model.GPUCapability) {
	if m.clusterNodeRepository == nil {
		return
	}

	cpuCores := runtime.NumCPU()
	memoryTotalMB := hoststats.TotalMemoryMB()
	maxTranscodeSessions := cfg.Scheduler.MaxNodeTranscodeSessions
	maxUploadConcurrency := cfg.Scheduler.MaxNodeUploadConcurrency
	tags := buildNodeTags(m.nodeMode, gpuCapabilities)
	now := time.Now()

	if !m.shouldPersistNodeSnapshot(now, cpuCores, memoryTotalMB, maxTranscodeSessions, maxUploadConcurrency, tags) {
		return
	}
	if err := m.clusterNodeRepository.SaveSnapshot(
		ctx,
		m.nodeID,
		cpuCores,
		memoryTotalMB,
		maxTranscodeSessions,
		maxUploadConcurrency,
		tags,
	); err != nil {
		logx.Error("worker.node.save_snapshot_failed", err, logx.Fields{
			"node_id": m.nodeID,
		})
		return
	}
	m.lastNodeSnapshotAt = now
	m.lastNodeSnapshotCPU = cpuCores
	m.lastNodeSnapshotMemory = memoryTotalMB
	m.lastNodeSnapshotTrans = maxTranscodeSessions
	m.lastNodeSnapshotUpload = maxUploadConcurrency
	m.lastNodeSnapshotTags = tags
}

func (m *Module) shouldPersistNodeSnapshot(now time.Time, cpuCores, memoryTotalMB, maxTranscodeSessions, maxUploadConcurrency int, tags string) bool {
	if m.lastNodeSnapshotAt.IsZero() {
		return true
	}
	if m.lastNodeSnapshotCPU != cpuCores ||
		m.lastNodeSnapshotMemory != memoryTotalMB ||
		m.lastNodeSnapshotTrans != maxTranscodeSessions ||
		m.lastNodeSnapshotUpload != maxUploadConcurrency ||
		m.lastNodeSnapshotTags != tags {
		return true
	}
	return now.Sub(m.lastNodeSnapshotAt) >= nodeSnapshotPersistInterval
}

// shouldPersistDBHeartbeat 判断本轮是否需要把心跳从 Redis 热路径同步落到数据库。
//
// 设计原则：
// 1. Redis 继续高频写，调度器和监控优先读缓存；
// 2. MySQL 只保留“可审计、可离线判定”的最近时间戳，不承担每轮心跳写放大；
// 3. 持久化间隔始终明显小于 worker_heartbeat_timeout，避免后台误判离线。
func (m *Module) shouldPersistDBHeartbeat(now time.Time, lastPersistAt time.Time, timeout time.Duration) bool {
	if lastPersistAt.IsZero() {
		return true
	}
	return now.Sub(lastPersistAt) >= cluster.ResolveHeartbeatPersistInterval(timeout)
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

	/* B3 清单物化：把每片的 sha256 一并放进回调载荷。
	 * 摘要取自分片行（切片产出时算好落库，见 segment_digest.go），不在这里重算文件：
	 * 输出目录在任务收尾时会被清理，重算既慢又可能读不到文件。 */
	segmentDigests := make(map[string]string)

	if m.segmentRepository != nil {
		for _, record := range m.segmentRepository.ListByJobID(ctx, job.JobID) {
			if record.SHA256 != "" {
				segmentDigests[record.ObjectKey] = record.SHA256
			}
		}
	}

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
				// 北向清单接口已经收敛为无扩展名路由，
				// 回调载荷里必须与真实服务路由保持一致，避免外部系统拿到旧 URL 后直接 404。
				ManifestDashURL:       fmt.Sprintf("/v1/manifest/dash/%d", job.JobID),
				ManifestHLSURL:        fmt.Sprintf("/v1/manifest/hls/%d", job.JobID),
				ManifestHLSVariantURL: fmt.Sprintf("/v1/manifest/hls/%d/%s", job.JobID, seg.RenditionName),
			}
			renditionMap[key] = rend
		}
		if seg.IsInit {
			rend.InitSegmentObjectKey = seg.ObjectKey
		} else {
			rend.SegmentCount++
			/* 每片摘要随清单一起物化出去（init 段不带：它的内容由 init_segment_object_key 指认）。 */
			rend.Segments = append(rend.Segments, model.CompletedSegmentDigest{
				ObjectKey: seg.ObjectKey,
				SHA256:    segmentDigests[seg.ObjectKey],
				SizeBytes: uint64(seg.FileSize),
			})
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

// SetOffline 将当前 Worker 切换到“运维下线”状态。
//
// 下线后的语义是：
// 1. 不再领取新的 assigned 任务；
// 2. 已在跑的任务和上传协程继续推进；
// 3. worker_instance 状态收口为 offline，便于后台明确看到该实例已被人工摘除。
func (m *Module) SetOffline(ctx context.Context, workerID string, reason string) error {
	if strings.TrimSpace(workerID) != strings.TrimSpace(m.workerID) {
		return fmt.Errorf("worker %s not hosted by current process", workerID)
	}
	m.controlMu.Lock()
	m.offlineRequested = true
	m.offlineReason = strings.TrimSpace(reason)
	m.controlMu.Unlock()
	if m.workerInstanceRepo != nil {
		if err := m.workerInstanceRepo.MarkOffline(ctx, m.workerID, reason); err != nil {
			return err
		}
	}
	return nil
}

// RequestExit 请求当前 Worker 执行模块退出。
//
// 这里不是直接退出整个服务进程，而是：
// 1. 将 worker_instance 状态标记为 exited；
// 2. 停止新的调度领取；
// 3. 取消 Worker 自己的运行上下文，让执行/上传链路尽快收口。
func (m *Module) RequestExit(ctx context.Context, workerID string, reason string) error {
	if strings.TrimSpace(workerID) != strings.TrimSpace(m.workerID) {
		return fmt.Errorf("worker %s not hosted by current process", workerID)
	}

	cancel := func() context.CancelFunc {
		m.controlMu.Lock()
		defer m.controlMu.Unlock()
		m.offlineRequested = true
		m.exitRequested = true
		m.offlineReason = strings.TrimSpace(reason)
		m.exitReason = strings.TrimSpace(reason)
		return m.runCancel
	}()

	if m.workerInstanceRepo != nil {
		if err := m.workerInstanceRepo.MarkExited(ctx, m.workerID, reason); err != nil {
			return err
		}
	}
	if cancel != nil {
		cancel()
	}
	return nil
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

func (m *Module) acceptingNewAssignments() bool {
	m.controlMu.RLock()
	defer m.controlMu.RUnlock()
	return !m.offlineRequested && !m.exitRequested
}

func (m *Module) shutdownReason(parentErr error) string {
	m.controlMu.RLock()
	defer m.controlMu.RUnlock()
	if m.exitRequested {
		if strings.TrimSpace(m.exitReason) != "" {
			return m.exitReason
		}
		return "remote_exit_requested"
	}
	if parentErr != nil {
		return "context_canceled"
	}
	return "worker_stopped"
}

func (m *Module) setRunCancel(cancel context.CancelFunc) {
	m.controlMu.Lock()
	defer m.controlMu.Unlock()
	m.runCancel = cancel
}

func (m *Module) clearRunCancel(cancel context.CancelFunc) {
	m.controlMu.Lock()
	defer m.controlMu.Unlock()
	if fmt.Sprintf("%p", m.runCancel) == fmt.Sprintf("%p", cancel) {
		m.runCancel = nil
	}
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

func (m *Module) markWorkerExited(ctx context.Context, reason string) {
	if m.workerInstanceRepo == nil || m.workerID == "" {
		return
	}
	if err := m.workerInstanceRepo.MarkExited(ctx, m.workerID, reason); err != nil {
		logx.Error("worker.instance.mark_exited_failed", err, logx.Fields{
			"worker_id": m.workerID,
			"reason":    reason,
		})
	}
}

func buildNodeTags(nodeMode string, capabilities []model.GPUCapability) string {
	tags := make([]string, 0, 4)
	switch strings.TrimSpace(nodeMode) {
	case config.NodeModeStandalone:
		tags = append(tags, "mode:"+config.NodeModeStandalone)
	case config.NodeModeClusterAllInOne:
		tags = append(tags, "mode:"+config.NodeModeClusterAllInOne)
	case config.NodeModeClusterControl:
		tags = append(tags, "mode:"+config.NodeModeClusterControl)
	case config.NodeModeClusterWorker:
		tags = append(tags, "mode:"+config.NodeModeClusterWorker)
	}
	if len(capabilities) == 0 {
		tags = append(tags, "compute:cpu")
	} else {
		tags = append(tags, "compute:gpu")
	}
	return strings.Join(tags, ",")
}
