package mysql

import (
	"context"
	"encoding/json"
	"errors"
	"hvc/internal/model"
	"strings"
	"time"

	"gorm.io/gorm"
)

var ErrJobAssignConflict = errors.New("job assign conflict")

func toJobRecord(job model.TranscodeJob) JobRecord {
	return JobRecord{
		JobID:                      job.JobID,
		RequestID:                  job.RequestID,
		BizKey:                     job.BizKey,
		Status:                     job.Status,
		Priority:                   job.Priority,
		SourceURL:                  job.SourceURL,
		ProfileID:                  job.ProfileID,
		SegmentDurationSec:         job.SegmentDurationSec,
		SegmentTemplate:            job.SegmentTemplate,
		SupportDash:                job.SupportDash,
		SupportHLS:                 job.SupportHLS,
		EnableWatermark:            job.EnableWatermark,
		WatermarkImageURL:          job.WatermarkImageURL,
		WatermarkAnchor:            job.WatermarkAnchor,
		WatermarkXRatio:            job.WatermarkXRatio,
		WatermarkYRatio:            job.WatermarkYRatio,
		WatermarkWidthRatio:        job.WatermarkWidthRatio,
		WatermarkOpacity:           job.WatermarkOpacity,
		WatermarkSafeMarginRatio:   job.WatermarkSafeMarginRatio,
		EnableThumbnailSprite:      job.EnableThumbnailSprite,
		ThumbRows:                  job.ThumbRows,
		ThumbCols:                  job.ThumbCols,
		ThumbIntervalSec:           job.ThumbIntervalSec,
		ThumbWidth:                 job.ThumbWidth,
		ThumbHeight:                job.ThumbHeight,
		ThumbImageFormat:           job.ThumbImageFormat,
		ThumbStoragePrefix:         job.ThumbStoragePrefix,
		EnableThumbnailBinaryIndex: job.EnableThumbnailBinaryIndex,
		ThumbBinaryStoragePrefix:   job.ThumbBinaryStoragePrefix,
		ThumbBinaryMaxSizeBytes:    job.ThumbBinaryMaxSizeBytes,
		RenditionsJSON:             marshalRenditions(job.Renditions),
		OutputStorageID:            job.OutputStorageID,
		OutputBasePrefix:           job.OutputBasePrefix,
		AssignedNodeID:             job.AssignedNodeID,
		AssignedWorkerID:           job.AssignedWorkerID,
		ExecutorWorkerInstanceID:   job.ExecutorWorkerInstanceID,
		SelectedExecutionHWAccel:   job.SelectedExecutionHWAccel,
		SelectedGPUIndex:           job.SelectedGPUIndex,
		SelectedGPUDeviceID:        job.SelectedGPUDeviceID,
		LeaseOwner:                 job.LeaseOwner,
		LeaseGeneration:            job.LeaseGeneration,
		AttemptNo:                  job.AttemptNo,
		ProgressPermille:           job.ProgressPermille,
		ProgressStage:              job.ProgressStage,
		ErrorCode:                  job.ErrorCode,
		ErrorMessage:               job.ErrorMessage,
		CreatedAt:                  job.CreatedAt,
		UpdatedAt:                  job.UpdatedAt,
	}
}

func toJobModel(record JobRecord) model.TranscodeJob {
	return model.TranscodeJob{
		JobID:                      record.JobID,
		RequestID:                  record.RequestID,
		BizKey:                     record.BizKey,
		Status:                     record.Status,
		Priority:                   record.Priority,
		SourceURL:                  record.SourceURL,
		ProfileID:                  record.ProfileID,
		SegmentDurationSec:         record.SegmentDurationSec,
		SegmentTemplate:            record.SegmentTemplate,
		SupportDash:                record.SupportDash,
		SupportHLS:                 record.SupportHLS,
		EnableWatermark:            record.EnableWatermark,
		WatermarkImageURL:          record.WatermarkImageURL,
		WatermarkAnchor:            record.WatermarkAnchor,
		WatermarkXRatio:            record.WatermarkXRatio,
		WatermarkYRatio:            record.WatermarkYRatio,
		WatermarkWidthRatio:        record.WatermarkWidthRatio,
		WatermarkOpacity:           record.WatermarkOpacity,
		WatermarkSafeMarginRatio:   record.WatermarkSafeMarginRatio,
		EnableThumbnailSprite:      record.EnableThumbnailSprite,
		ThumbRows:                  record.ThumbRows,
		ThumbCols:                  record.ThumbCols,
		ThumbIntervalSec:           record.ThumbIntervalSec,
		ThumbWidth:                 record.ThumbWidth,
		ThumbHeight:                record.ThumbHeight,
		ThumbImageFormat:           record.ThumbImageFormat,
		ThumbStoragePrefix:         record.ThumbStoragePrefix,
		EnableThumbnailBinaryIndex: record.EnableThumbnailBinaryIndex,
		ThumbBinaryStoragePrefix:   record.ThumbBinaryStoragePrefix,
		ThumbBinaryMaxSizeBytes:    record.ThumbBinaryMaxSizeBytes,
		Renditions:                 unmarshalRenditions(record.RenditionsJSON),
		OutputStorageID:            record.OutputStorageID,
		OutputBasePrefix:           record.OutputBasePrefix,
		AssignedNodeID:             record.AssignedNodeID,
		AssignedWorkerID:           record.AssignedWorkerID,
		ExecutorWorkerInstanceID:   record.ExecutorWorkerInstanceID,
		SelectedExecutionHWAccel:   record.SelectedExecutionHWAccel,
		SelectedGPUIndex:           record.SelectedGPUIndex,
		SelectedGPUDeviceID:        record.SelectedGPUDeviceID,
		LeaseOwner:                 record.LeaseOwner,
		LeaseGeneration:            record.LeaseGeneration,
		AttemptNo:                  record.AttemptNo,
		ProgressPermille:           record.ProgressPermille,
		ProgressStage:              record.ProgressStage,
		ErrorCode:                  record.ErrorCode,
		ErrorMessage:               record.ErrorMessage,
		CreatedAt:                  record.CreatedAt,
		UpdatedAt:                  record.UpdatedAt,
	}
}

// JobRepository 表示任务仓储。
type JobRepository struct {
	db *DB
}

type JobListFilter struct {
	Page      int
	PageSize  int
	Status    int
	BizKey    string
	RequestID string
}

// ForceTakeoverResult 表示人工强制接管的收口结果。
//
// 后台集群治理动作不是“杀掉远端进程”，而是把控制面占用状态先收回来：
// 1. 任务主表重新回到 queued，供调度器重新分配；
// 2. 旧执行实例标记为 abandoned，方便后续审计与排障；
// 3. 返回影响条数，便于后台明确知道本次接管到底收口了多少任务。
type ForceTakeoverResult struct {
	MatchedJobTotal         int `json:"matched_job_total"`
	ResetJobTotal           int `json:"reset_job_total"`
	AbandonedExecutionTotal int `json:"abandoned_execution_total"`
}

// NewJobRepository 创建任务仓储。
func NewJobRepository(db *DB) *JobRepository {
	return &JobRepository{db: db}
}

// FindByRequestID 根据 request_id 查询任务。
func (r *JobRepository) FindByRequestID(ctx context.Context, requestID string) (model.TranscodeJob, bool) {
	var record JobRecord
	if err := r.db.WithContext(ctx).Where("request_id = ?", requestID).Take(&record).Error; err != nil {
		return model.TranscodeJob{}, false
	}
	return toJobModel(record), true
}

// FindByJobID 根据 job_id 查询任务。
func (r *JobRepository) FindByJobID(ctx context.Context, jobID uint64) (model.TranscodeJob, bool) {
	var record JobRecord
	if err := r.db.WithContext(ctx).Where("job_id = ?", jobID).Take(&record).Error; err != nil {
		return model.TranscodeJob{}, false
	}
	return toJobModel(record), true
}

// Save 保存任务。
func (r *JobRepository) Save(ctx context.Context, job model.TranscodeJob) error {
	record := toJobRecord(job)
	return r.db.WithContext(ctx).Create(&record).Error
}

// CreateWithOverrideAtomic 以 request_id 唯一键为准原子创建任务。
func (r *JobRepository) CreateWithOverrideAtomic(ctx context.Context, job model.TranscodeJob, override *model.TranscodeJobRequestOverride) (model.TranscodeJob, bool, error) {
	record := toJobRecord(job)
	created := false
	createdJob := job
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&record).Error; err != nil {
			if !isDuplicateEntryError(err) {
				return err
			}
			var existing JobRecord
			if findErr := tx.Where("request_id = ?", job.RequestID).Take(&existing).Error; findErr != nil {
				return findErr
			}
			createdJob = toJobModel(existing)
			return nil
		}
		created = true
		if override != nil {
			overrideRecord := toJobRequestOverrideRecord(*override)
			if err := tx.Create(&overrideRecord).Error; err != nil {
				return err
			}
		}
		return nil
	})
	return createdJob, created, err
}

// ListQueued 返回待调度任务。
func (r *JobRepository) ListQueued(ctx context.Context) []model.TranscodeJob {
	var records []JobRecord
	if err := r.db.WithContext(ctx).Where("status = ?", model.JobStatusQueued).Find(&records).Error; err != nil {
		return nil
	}
	items := make([]model.TranscodeJob, 0, len(records))
	for _, record := range records {
		items = append(items, toJobModel(record))
	}
	return items
}

// ListAssigned 返回已分配任务列表。
//
// 过滤语义约定：
// 1. nodeID > 0 时按节点过滤；
// 2. workerID 非空时按 Worker 过滤；
// 3. 两者都为空时返回全部活跃任务，供失联接管与后台治理复用。
func (r *JobRepository) ListAssigned(ctx context.Context, nodeID uint64, workerID string) []model.TranscodeJob {
	query := applyAssignedJobFilter(
		r.db.WithContext(ctx).Where("status IN ?", []int{model.JobStatusAssigned, model.JobStatusRunning, model.JobStatusUploading}),
		nodeID,
		workerID,
	)
	var records []JobRecord
	if err := query.Find(&records).Error; err != nil {
		return nil
	}
	items := make([]model.TranscodeJob, 0, len(records))
	for _, record := range records {
		items = append(items, toJobModel(record))
	}
	return items
}

// Assign 更新任务分配信息。
func (r *JobRepository) Assign(ctx context.Context, jobID uint64, decision model.DispatchDecision, nodeID uint64, workerID string, workerInstanceID uint64) error {
	result := r.db.WithContext(ctx).Model(&JobRecord{}).Where("job_id = ? AND status = ?", jobID, model.JobStatusQueued).Updates(map[string]any{
		"assigned_node_id":            nodeID,
		"assigned_worker_id":          workerID,
		"executor_worker_instance_id": workerInstanceID,
		"selected_gpu_index":          decision.SelectedGPUIndex,
		"selected_gpu_device_id":      decision.SelectedGPUDeviceID,
		"selected_execution_hwaccel":  decision.SelectedExecutionHW,
		"lease_generation":            decision.LeaseGeneration,
		"attempt_no":                  decision.AttemptNo,
		"lease_owner":                 workerID,
		"status":                      model.JobStatusAssigned,
		"progress_stage":              model.StageQueued,
		"updated_at":                  time.Now(),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrJobAssignConflict
	}
	return nil
}

// UpdateProgress 更新任务进度。
func (r *JobRepository) UpdateProgress(ctx context.Context, jobID uint64, progressPermille int, stage string) error {
	status := model.JobStatusRunning
	if progressPermille >= 900 && progressPermille < 1000 {
		status = model.JobStatusUploading
	}
	return r.db.WithContext(ctx).Model(&JobRecord{}).Where("job_id = ?", jobID).Updates(map[string]any{
		"progress_permille": progressPermille,
		"progress_stage":    stage,
		"status":            status,
		"updated_at":        time.Now(),
	}).Error
}

// MarkCompleted 标记任务完成。
func (r *JobRepository) MarkCompleted(ctx context.Context, jobID uint64) error {
	return r.db.WithContext(ctx).Model(&JobRecord{}).Where("job_id = ?", jobID).Updates(map[string]any{
		"status":            model.JobStatusCompleted,
		"progress_permille": 1000,
		"progress_stage":    model.StageCompleted,
		"updated_at":        time.Now(),
	}).Error
}

// GetByID 根据 job_id 获取任务信息。
func (r *JobRepository) GetByID(ctx context.Context, jobID uint64) (model.TranscodeJob, bool) {
	return r.FindByJobID(ctx, jobID)
}

// ListAll 返回所有任务列表。
func (r *JobRepository) ListAll(ctx context.Context) []model.TranscodeJob {
	var records []JobRecord
	r.db.WithContext(ctx).Order("job_id DESC").Limit(1000).Find(&records)
	jobs := make([]model.TranscodeJob, 0, len(records))
	for _, rec := range records {
		jobs = append(jobs, toJobModel(rec))
	}
	return jobs
}

func (r *JobRepository) ListPage(ctx context.Context, filter JobListFilter) ([]model.TranscodeJob, int64, error) {
	page := filter.Page
	if page <= 0 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	query := r.db.WithContext(ctx).Model(&JobRecord{})
	if filter.Status > 0 {
		query = query.Where("status = ?", filter.Status)
	}
	if filter.BizKey != "" {
		query = query.Where("biz_key = ?", filter.BizKey)
	}
	if filter.RequestID != "" {
		query = query.Where("request_id = ?", filter.RequestID)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var records []JobRecord
	if err := query.Order("job_id DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&records).Error; err != nil {
		return nil, 0, err
	}

	items := make([]model.TranscodeJob, 0, len(records))
	for _, rec := range records {
		items = append(items, toJobModel(rec))
	}
	return items, total, nil
}

// ResetToQueued 将任务重置为排队状态（用于重试）。
func (r *JobRepository) ResetToQueued(ctx context.Context, jobID uint64) error {
	return r.db.WithContext(ctx).Model(&JobRecord{}).Where("job_id = ?", jobID).Updates(map[string]any{
		"status":                      model.JobStatusQueued,
		"progress_permille":           0,
		"progress_stage":              model.StageQueued,
		"assigned_node_id":            0,
		"assigned_worker_id":          "",
		"executor_worker_instance_id": 0,
		"selected_execution_hwaccel":  "",
		"selected_gpu_index":          0,
		"selected_gpu_device_id":      0,
		"lease_owner":                 "",
		"lease_generation":            0,
		"lease_expire_at":             nil,
		"last_worker_heartbeat_at":    nil,
		"updated_at":                  time.Now(),
	}).Error
}

// ForceTakeover 按节点或 Worker 维度强制接管当前活跃任务。
//
// 注意这里的语义是“回收控制面所有权”，不是宣称旧进程已被停止：
// - 任务会被重新置回 queued，等待后续重新调度；
// - 当前执行实例会被标记为 abandoned，便于审计；
// - 若调用方只提供 nodeID 或 workerID，则按该单一维度收口。
func (r *JobRepository) ForceTakeover(ctx context.Context, nodeID uint64, workerID string, reason string) (ForceTakeoverResult, error) {
	result := ForceTakeoverResult{}
	workerID = strings.TrimSpace(workerID)
	reason = strings.TrimSpace(reason)
	if reason == "" {
		reason = "manual_takeover"
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		jobQuery := applyAssignedJobFilter(
			tx.Model(&JobRecord{}).Where("status IN ?", []int{model.JobStatusAssigned, model.JobStatusRunning, model.JobStatusUploading}),
			nodeID,
			workerID,
		)

		var jobIDs []uint64
		if err := jobQuery.Pluck("job_id", &jobIDs).Error; err != nil {
			return err
		}
		result.MatchedJobTotal = len(jobIDs)
		if len(jobIDs) == 0 {
			return nil
		}

		now := time.Now()
		update := map[string]any{
			"status":                      model.JobStatusQueued,
			"progress_permille":           0,
			"progress_stage":              model.StageQueued,
			"assigned_node_id":            0,
			"assigned_worker_id":          "",
			"executor_worker_instance_id": 0,
			"selected_execution_hwaccel":  "",
			"selected_gpu_index":          0,
			"selected_gpu_device_id":      0,
			"lease_owner":                 "",
			"lease_generation":            0,
			"lease_expire_at":             nil,
			"last_worker_heartbeat_at":    nil,
			"updated_at":                  now,
		}
		jobUpdate := tx.Model(&JobRecord{}).
			Where("job_id IN ? AND status IN ?", jobIDs, []int{model.JobStatusAssigned, model.JobStatusRunning, model.JobStatusUploading}).
			Updates(update)
		if jobUpdate.Error != nil {
			return jobUpdate.Error
		}
		result.ResetJobTotal = int(jobUpdate.RowsAffected)

		executionUpdate := tx.Model(&JobExecutionRecord{}).
			Where("job_id IN ? AND status IN ?", jobIDs, []int{1, 2, 3}).
			Updates(map[string]any{
				"status":            6,
				"failure_reason":    reason,
				"recoverable_flag":  true,
				"finished_at":       now,
				"last_heartbeat_at": now,
				"updated_at":        now,
			})
		if executionUpdate.Error != nil {
			return executionUpdate.Error
		}
		result.AbandonedExecutionTotal = int(executionUpdate.RowsAffected)
		return nil
	})
	return result, err
}

func applyAssignedJobFilter(query *gorm.DB, nodeID uint64, workerID string) *gorm.DB {
	if nodeID > 0 {
		query = query.Where("assigned_node_id = ?", nodeID)
	}
	if workerID = strings.TrimSpace(workerID); workerID != "" {
		query = query.Where("assigned_worker_id = ?", workerID)
	}
	return query
}

// EnsureSegmentTemplateSnapshot 为任务锁定首次执行时使用的分片命名模板。
//
// 运行时全局模板可以热更新，但单个任务一旦开始执行，就必须继续沿用首次锁定的模板，
// 否则后续重试、动态清单生成和对象路径审计都会发生漂移。
func (r *JobRepository) EnsureSegmentTemplateSnapshot(ctx context.Context, jobID uint64, template string) (string, error) {
	template = strings.TrimSpace(template)
	if template == "" {
		template = "{job_id}-{rendition_key}-{media_type}-{number}.m4s"
	}

	var effective string
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var record JobRecord
		if err := tx.Where("job_id = ?", jobID).Take(&record).Error; err != nil {
			return err
		}
		if strings.TrimSpace(record.SegmentTemplate) != "" {
			effective = record.SegmentTemplate
			return nil
		}
		if err := tx.Model(&JobRecord{}).Where("job_id = ?", jobID).Updates(map[string]any{
			"segment_template": template,
			"updated_at":       time.Now(),
		}).Error; err != nil {
			return err
		}
		effective = template
		return nil
	})
	return effective, err
}

// MarkCanceled 标记任务为已取消。
func (r *JobRepository) MarkCanceled(ctx context.Context, jobID uint64) error {
	return r.db.WithContext(ctx).Model(&JobRecord{}).Where("job_id = ?", jobID).Updates(map[string]any{
		"status":         model.JobStatusCanceled,
		"progress_stage": model.StageFailed,
		"updated_at":     time.Now(),
	}).Error
}

// MarkFailed 标记任务为失败。
func (r *JobRepository) MarkFailed(ctx context.Context, jobID uint64, errorCode string, errorMessage string) error {
	return r.db.WithContext(ctx).Model(&JobRecord{}).Where("job_id = ?", jobID).Updates(map[string]any{
		"status":         model.JobStatusFailed,
		"progress_stage": model.StageFailed,
		"error_code":     errorCode,
		"error_message":  errorMessage,
		"updated_at":     time.Now(),
	}).Error
}

// ListByStatus 按状态查询任务列表。
func (r *JobRepository) ListByStatus(ctx context.Context, status int, limit int) []model.TranscodeJob {
	if limit <= 0 {
		limit = 100
	}
	var records []JobRecord
	if err := r.db.WithContext(ctx).Where("status = ?", status).Order("job_id DESC").Limit(limit).Find(&records).Error; err != nil {
		return nil
	}
	jobs := make([]model.TranscodeJob, 0, len(records))
	for _, rec := range records {
		jobs = append(jobs, toJobModel(rec))
	}
	return jobs
}

// CountByStatus 按状态统计任务数量。
func (r *JobRepository) CountByStatus(ctx context.Context, status int) int64 {
	var count int64
	r.db.WithContext(ctx).Model(&JobRecord{}).Where("status = ?", status).Count(&count)
	return count
}

// CountActive 统计当前仍占用调度/执行资源的任务数。
//
// 口径统一为 Assigned / Running / Uploading，
// 供全局并发上限控制和后台观测复用。
func (r *JobRepository) CountActive(ctx context.Context) int64 {
	var count int64
	r.db.WithContext(ctx).Model(&JobRecord{}).
		Where("status IN ?", []int{model.JobStatusAssigned, model.JobStatusRunning, model.JobStatusUploading}).
		Count(&count)
	return count
}

// ListExpiredRunning 查询运行超时的任务。
//
// 返回状态为 Assigned 或 Running 且最后心跳超过 timeout 的任务。
func (r *JobRepository) ListExpiredRunning(ctx context.Context, timeout time.Duration) []model.TranscodeJob {
	cutoff := time.Now().Add(-timeout)
	var records []JobRecord
	if err := r.db.WithContext(ctx).
		Where("status IN ? AND last_worker_heartbeat_at < ? AND last_worker_heartbeat_at > ?",
			[]int{model.JobStatusAssigned, model.JobStatusRunning}, cutoff, time.Time{}).
		Find(&records).Error; err != nil {
		return nil
	}
	jobs := make([]model.TranscodeJob, 0, len(records))
	for _, rec := range records {
		jobs = append(jobs, toJobModel(rec))
	}
	return jobs
}

func marshalRenditions(renditions []model.RenditionOption) string {
	if len(renditions) == 0 {
		return ""
	}
	data, _ := json.Marshal(renditions)
	return string(data)
}

func unmarshalRenditions(data string) []model.RenditionOption {
	if data == "" {
		return nil
	}
	var renditions []model.RenditionOption
	if err := json.Unmarshal([]byte(data), &renditions); err != nil {
		return nil
	}
	return renditions
}
