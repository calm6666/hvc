package mysql

import (
	"context"
	"time"

	"hvc/pkg/idgen"
)

// WorkerInstanceRepository 表示 Worker 实例仓储。
//
// 这层负责把“本次启动起来的这个 Worker 进程”落库成一条独立实例记录，
// 让 startup_instance_id、machine_fingerprint、最后心跳时间和退出原因都有稳定落点。
type WorkerInstanceRepository struct {
	db *DB
}

// NewWorkerInstanceRepository 创建 Worker 实例仓储。
func NewWorkerInstanceRepository(db *DB) *WorkerInstanceRepository {
	return &WorkerInstanceRepository{db: db}
}

// EnsureOnline 确保当前 Worker 实例已在数据库中存在并标记为在线。
//
// 若同一 worker_id 已存在记录，则更新为最新启动身份；否则创建新记录。
func (r *WorkerInstanceRepository) EnsureOnline(ctx context.Context, nodeID uint64, workerID, logicalWorkerID, physicalWorkerID, machineFingerprint, startupInstanceID string) (uint64, error) {
	now := time.Now()
	var existing WorkerInstanceRecord
	result := r.db.WithContext(ctx).Where("worker_id = ?", workerID).Limit(1).Find(&existing)
	if result.Error != nil {
		return 0, result.Error
	}
	if result.RowsAffected > 0 {
		return existing.ID, r.db.WithContext(ctx).Model(&WorkerInstanceRecord{}).
			Where("id = ?", existing.ID).
			Updates(map[string]any{
				"node_id":             nodeID,
				"logical_worker_id":   logicalWorkerID,
				"physical_worker_id":  physicalWorkerID,
				"machine_fingerprint": machineFingerprint,
				"startup_instance_id": startupInstanceID,
				"status":              1,
				"start_at":            now,
				"exited_at":           nil,
				"exit_reason":         "",
				"last_heartbeat_at":   now,
				"updated_at":          now,
			}).Error
	}
	record := WorkerInstanceRecord{
		ID:                 idgen.Next(),
		NodeID:             nodeID,
		WorkerID:           workerID,
		LogicalWorkerID:    logicalWorkerID,
		PhysicalWorkerID:   physicalWorkerID,
		MachineFingerprint: machineFingerprint,
		StartupInstanceID:  startupInstanceID,
		Status:             1,
		StartAt:            now,
		ExitedAt:           nil,
		LastHeartbeatAt:    now,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := r.db.WithContext(ctx).Create(&record).Error; err != nil {
		return 0, err
	}
	return record.ID, nil
}

// TouchHeartbeat 更新 Worker 实例心跳时间。
func (r *WorkerInstanceRepository) TouchHeartbeat(ctx context.Context, workerID string) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&WorkerInstanceRecord{}).
		Where("worker_id = ?", workerID).
		Updates(map[string]any{
			"last_heartbeat_at": now,
			"updated_at":        now,
		}).Error
}
