package mysql

import (
	"context"
	"time"

	"hvc/internal/model"
	"hvc/pkg/idgen"
)

// JobExecutionRepository 表示任务执行实例仓储。
//
// 这层负责把“某个任务在某次租约代次上的执行过程”落到 t_transcode_job_execution，
// 让调度分配、开始执行、上传中、完成、失败都能有独立执行实例留痕。
type JobExecutionRepository struct {
	db *DB
}

// NewJobExecutionRepository 创建任务执行实例仓储。
func NewJobExecutionRepository(db *DB) *JobExecutionRepository {
	return &JobExecutionRepository{db: db}
}

// SaveAssigned 在调度分配成功后创建或更新执行实例记录。
func (r *JobExecutionRepository) SaveAssigned(ctx context.Context, jobID uint64, decision model.DispatchDecision, workerID string, workerInstanceID uint64) error {
	now := time.Now()
	var existing JobExecutionRecord
	if err := r.db.WithContext(ctx).Where("job_id = ? AND lease_generation = ?", jobID, decision.LeaseGeneration).Take(&existing).Error; err == nil {
		return r.db.WithContext(ctx).Model(&JobExecutionRecord{}).
			Where("execution_id = ?", existing.ExecutionID).
			Updates(map[string]any{
				"node_id":                    decision.NodeID,
				"worker_instance_id":         workerInstanceID,
				"gpu_device_id":              decision.SelectedGPUDeviceID,
				"selected_gpu_index":         decision.SelectedGPUIndex,
				"selected_execution_hwaccel": decision.SelectedExecutionHW,
				"status":                     1,
				"lease_owner":                workerID,
				"last_heartbeat_at":          now,
				"updated_at":                 now,
			}).Error
	}
	record := JobExecutionRecord{
		ExecutionID:            idgen.Next(),
		JobID:                  jobID,
		AttemptNo:              decision.AttemptNo,
		LeaseGeneration:        decision.LeaseGeneration,
		NodeID:                 decision.NodeID,
		WorkerInstanceID:       workerInstanceID,
		GPUDeviceID:            decision.SelectedGPUDeviceID,
		SelectedGPUIndex:       decision.SelectedGPUIndex,
		SelectedExecutionHWAcc: decision.SelectedExecutionHW,
		Status:                 1,
		LeaseOwner:             workerID,
		LastHeartbeatAt:        now,
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	return r.db.WithContext(ctx).Create(&record).Error
}

// MarkRunning 把执行实例更新为执行中。
func (r *JobExecutionRepository) MarkRunning(ctx context.Context, jobID uint64, leaseGeneration uint64) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&JobExecutionRecord{}).
		Where("job_id = ? AND lease_generation = ?", jobID, leaseGeneration).
		Updates(map[string]any{
			"status":            2,
			"started_at":        now,
			"last_heartbeat_at": now,
			"updated_at":        now,
		}).Error
}

// MarkUploading 把执行实例更新为上传中。
func (r *JobExecutionRepository) MarkUploading(ctx context.Context, jobID uint64, leaseGeneration uint64) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&JobExecutionRecord{}).
		Where("job_id = ? AND lease_generation = ?", jobID, leaseGeneration).
		Updates(map[string]any{
			"status":            3,
			"last_heartbeat_at": now,
			"updated_at":        now,
		}).Error
}

// MarkCompleted 把执行实例更新为成功结束。
func (r *JobExecutionRepository) MarkCompleted(ctx context.Context, jobID uint64, leaseGeneration uint64) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&JobExecutionRecord{}).
		Where("job_id = ? AND lease_generation = ?", jobID, leaseGeneration).
		Updates(map[string]any{
			"status":            4,
			"finished_at":       now,
			"last_heartbeat_at": now,
			"updated_at":        now,
		}).Error
}

// TouchHeartbeat 更新执行实例心跳时间。
func (r *JobExecutionRepository) TouchHeartbeat(ctx context.Context, jobID uint64, leaseGeneration uint64) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&JobExecutionRecord{}).
		Where("job_id = ? AND lease_generation = ?", jobID, leaseGeneration).
		Updates(map[string]any{
			"last_heartbeat_at": now,
			"updated_at":        now,
		}).Error
}

// CountActiveGPUUsageByNode 返回指定节点集合的单卡活跃执行数。
//
// 统计口径：
// 1. 只统计状态仍处于“已租约 / 执行中 / 上传中”的执行实例；
// 2. 只统计 last_heartbeat_at 仍然新鲜的记录，避免陈旧实例长期占用卡位；
// 3. 聚合维度为 node_id + selected_gpu_index，便于调度阶段直接回填到 GPUCapabilities。
func (r *JobExecutionRepository) CountActiveGPUUsageByNode(ctx context.Context, nodeIDs []uint64, activeAfter time.Time) map[uint64]map[int]int {
	type row struct {
		NodeID           uint64 `gorm:"column:node_id"`
		SelectedGPUIndex int    `gorm:"column:selected_gpu_index"`
		Count            int    `gorm:"column:count"`
	}

	result := make(map[uint64]map[int]int)
	if len(nodeIDs) == 0 {
		return result
	}

	var rows []row
	if err := r.db.WithContext(ctx).
		Model(&JobExecutionRecord{}).
		Select("node_id, selected_gpu_index, COUNT(*) AS count").
		Where("node_id IN ? AND status IN ? AND selected_gpu_index >= ? AND last_heartbeat_at >= ?",
			nodeIDs,
			[]int{1, 2, 3},
			0,
			activeAfter,
		).
		Group("node_id, selected_gpu_index").
		Scan(&rows).Error; err != nil {
		return result
	}

	for _, item := range rows {
		if _, ok := result[item.NodeID]; !ok {
			result[item.NodeID] = make(map[int]int)
		}
		result[item.NodeID][item.SelectedGPUIndex] = item.Count
	}
	return result
}
