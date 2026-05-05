package mysql

import (
	"context"
	"hvc/internal/model"
	"time"
)

func toJobRecord(job model.TranscodeJob) JobRecord {
	return JobRecord{
		JobID:                    job.JobID,
		RequestID:                job.RequestID,
		BizKey:                   job.BizKey,
		Status:                   job.Status,
		Priority:                 job.Priority,
		SourceURL:                job.SourceURL,
		ProfileID:                job.ProfileID,
		SegmentDurationSec:       job.SegmentDurationSec,
		SupportDash:              job.SupportDash,
		SupportHLS:               job.SupportHLS,
		EnableWatermark:          job.EnableWatermark,
		WatermarkImageURL:        job.WatermarkImageURL,
		WatermarkAnchor:          job.WatermarkAnchor,
		WatermarkXRatio:          job.WatermarkXRatio,
		WatermarkYRatio:          job.WatermarkYRatio,
		WatermarkWidthRatio:      job.WatermarkWidthRatio,
		WatermarkOpacity:         job.WatermarkOpacity,
		OutputStorageID:          job.OutputStorageID,
		OutputBasePrefix:         job.OutputBasePrefix,
		AssignedNodeID:           job.AssignedNodeID,
		AssignedWorkerID:         job.AssignedWorkerID,
		ExecutorWorkerInstanceID: job.ExecutorWorkerInstanceID,
		SelectedExecutionHWAccel: job.SelectedExecutionHWAccel,
		SelectedGPUIndex:         job.SelectedGPUIndex,
		SelectedGPUDeviceID:      job.SelectedGPUDeviceID,
		LeaseOwner:               job.LeaseOwner,
		LeaseGeneration:          job.LeaseGeneration,
		AttemptNo:                job.AttemptNo,
		ProgressPermille:         job.ProgressPermille,
		ProgressStage:            job.ProgressStage,
		ErrorCode:                job.ErrorCode,
		ErrorMessage:             job.ErrorMessage,
		CreatedAt:                job.CreatedAt,
		UpdatedAt:                job.UpdatedAt,
	}
}

func toJobModel(record JobRecord) model.TranscodeJob {
	return model.TranscodeJob{
		JobID:                    record.JobID,
		RequestID:                record.RequestID,
		BizKey:                   record.BizKey,
		Status:                   record.Status,
		Priority:                 record.Priority,
		SourceURL:                record.SourceURL,
		ProfileID:                record.ProfileID,
		SegmentDurationSec:       record.SegmentDurationSec,
		SupportDash:              record.SupportDash,
		SupportHLS:               record.SupportHLS,
		EnableWatermark:          record.EnableWatermark,
		WatermarkImageURL:        record.WatermarkImageURL,
		WatermarkAnchor:          record.WatermarkAnchor,
		WatermarkXRatio:          record.WatermarkXRatio,
		WatermarkYRatio:          record.WatermarkYRatio,
		WatermarkWidthRatio:      record.WatermarkWidthRatio,
		WatermarkOpacity:         record.WatermarkOpacity,
		OutputStorageID:          record.OutputStorageID,
		OutputBasePrefix:         record.OutputBasePrefix,
		AssignedNodeID:           record.AssignedNodeID,
		AssignedWorkerID:         record.AssignedWorkerID,
		ExecutorWorkerInstanceID: record.ExecutorWorkerInstanceID,
		SelectedExecutionHWAccel: record.SelectedExecutionHWAccel,
		SelectedGPUIndex:         record.SelectedGPUIndex,
		SelectedGPUDeviceID:      record.SelectedGPUDeviceID,
		LeaseOwner:               record.LeaseOwner,
		LeaseGeneration:          record.LeaseGeneration,
		AttemptNo:                record.AttemptNo,
		ProgressPermille:         record.ProgressPermille,
		ProgressStage:            record.ProgressStage,
		ErrorCode:                record.ErrorCode,
		ErrorMessage:             record.ErrorMessage,
		CreatedAt:                record.CreatedAt,
		UpdatedAt:                record.UpdatedAt,
	}
}

// JobRepository 表示任务仓储。
type JobRepository struct {
	db *DB
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

// ListAssigned 返回已分配给当前 Worker 的任务。
func (r *JobRepository) ListAssigned(ctx context.Context, nodeID uint64, workerID string) []model.TranscodeJob {
	var records []JobRecord
	if err := r.db.WithContext(ctx).
		Where("assigned_node_id = ? AND assigned_worker_id = ? AND status IN ?", nodeID, workerID, []int{model.JobStatusAssigned, model.JobStatusRunning, model.JobStatusUploading}).
		Find(&records).Error; err != nil {
		return nil
	}
	items := make([]model.TranscodeJob, 0, len(records))
	for _, record := range records {
		items = append(items, toJobModel(record))
	}
	return items
}

// Assign 更新任务分配信息。
func (r *JobRepository) Assign(ctx context.Context, jobID uint64, decision model.DispatchDecision, nodeID uint64, workerID string) error {
	return r.db.WithContext(ctx).Model(&JobRecord{}).Where("job_id = ?", jobID).Updates(map[string]any{
		"assigned_node_id":            nodeID,
		"assigned_worker_id":          workerID,
		"selected_gpu_index":          decision.SelectedGPUIndex,
		"selected_gpu_device_id":      decision.SelectedGPUDeviceID,
		"selected_execution_hwaccel":  decision.SelectedExecutionHW,
		"lease_generation":            decision.LeaseGeneration,
		"attempt_no":                  decision.AttemptNo,
		"lease_owner":                 workerID,
		"status":                      model.JobStatusAssigned,
		"progress_stage":              model.StageQueued,
		"updated_at":                  time.Now(),
	}).Error
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
