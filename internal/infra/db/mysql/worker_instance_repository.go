package mysql

import (
	"context"
	"strings"
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

// WorkerInstanceListFilter 表示 Worker 实例列表筛选条件。
type WorkerInstanceListFilter struct {
	Page       int
	PageSize   int
	NodeID     uint64
	WorkerID   string
	Status     int
	OnlineOnly *bool
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
func (r *WorkerInstanceRepository) TouchHeartbeat(ctx context.Context, workerID string, heartbeatAt time.Time) error {
	if heartbeatAt.IsZero() {
		heartbeatAt = time.Now()
	}
	return r.db.WithContext(ctx).Model(&WorkerInstanceRecord{}).
		Where("worker_id = ?", workerID).
		Updates(map[string]any{
			"last_heartbeat_at": heartbeatAt,
			"updated_at":        time.Now(),
		}).Error
}

// MarkOfflineByHeartbeatTimeout 按心跳超时将仍标记为在线的 Worker 实例收敛为离线。
//
// 这样后台看到的 worker 状态不会长期停留在“在线假象”，
// 也为后续集群治理接口提供稳定的状态基线。
func (r *WorkerInstanceRepository) MarkOfflineByHeartbeatTimeout(ctx context.Context, timeout time.Duration) (int64, error) {
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	cutoff := time.Now().Add(-timeout)
	result := r.db.WithContext(ctx).Model(&WorkerInstanceRecord{}).
		Where("status = ? AND last_heartbeat_at < ?", 1, cutoff).
		Updates(map[string]any{
			"status":      2,
			"exit_reason": "heartbeat_timeout",
			"updated_at":  time.Now(),
		})
	return result.RowsAffected, result.Error
}

// MarkOffline 把指定 Worker 实例手动标记为离线。
//
// 该动作属于控制面治理操作：
// 1. 立即把状态从 online 收口为 offline；
// 2. 不伪造 exited_at，因为这不代表进程真的退出；
// 3. reason 用于后台审计与后续排障。
func (r *WorkerInstanceRepository) MarkOffline(ctx context.Context, workerID string, reason string) error {
	now := time.Now()
	if strings.TrimSpace(reason) == "" {
		reason = "manual_offline"
	}
	return r.db.WithContext(ctx).Model(&WorkerInstanceRecord{}).
		Where("worker_id = ?", workerID).
		Updates(map[string]any{
			"status":      2,
			"exit_reason": reason,
			"updated_at":  now,
		}).Error
}

// MarkExited 把指定 Worker 实例标记为已退出。
func (r *WorkerInstanceRepository) MarkExited(ctx context.Context, workerID string, reason string) error {
	now := time.Now()
	if strings.TrimSpace(reason) == "" {
		reason = "manual_exit"
	}
	return r.db.WithContext(ctx).Model(&WorkerInstanceRecord{}).
		Where("worker_id = ?", workerID).
		Updates(map[string]any{
			"status":      3,
			"exited_at":   &now,
			"exit_reason": reason,
			"updated_at":  now,
		}).Error
}

// ListPage 分页查询 Worker 实例列表。
func (r *WorkerInstanceRepository) ListPage(ctx context.Context, filter WorkerInstanceListFilter) ([]WorkerInstanceRecord, int64, error) {
	page, pageSize := normalizeAdminPage(filter.Page, filter.PageSize)
	query := r.db.WithContext(ctx).Model(&WorkerInstanceRecord{})
	if filter.NodeID > 0 {
		query = query.Where("node_id = ?", filter.NodeID)
	}
	if filter.Status > 0 {
		query = query.Where("status = ?", filter.Status)
	}
	if workerID := strings.TrimSpace(filter.WorkerID); workerID != "" {
		query = query.Where("worker_id LIKE ? OR logical_worker_id LIKE ? OR physical_worker_id LIKE ?", "%"+workerID+"%", "%"+workerID+"%", "%"+workerID+"%")
	}
	if filter.OnlineOnly != nil {
		grace := time.Now().Add(-2 * time.Minute)
		if *filter.OnlineOnly {
			query = query.Where("status = ? AND last_heartbeat_at >= ?", 1, grace)
		} else {
			query = query.Where("status <> ? OR last_heartbeat_at < ?", 1, grace)
		}
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var records []WorkerInstanceRecord
	if err := query.Order("last_heartbeat_at desc, id desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&records).Error; err != nil {
		return nil, 0, err
	}
	return records, total, nil
}

// ListAll 返回全部 Worker 实例记录。
func (r *WorkerInstanceRepository) ListAll(ctx context.Context) []WorkerInstanceRecord {
	var records []WorkerInstanceRecord
	if err := r.db.WithContext(ctx).Order("last_heartbeat_at desc, id desc").Find(&records).Error; err != nil {
		return nil
	}
	return records
}

// CountSummary 返回 Worker 总数和在线数，供总览接口直接复用数据库聚合结果。
func (r *WorkerInstanceRepository) CountSummary(ctx context.Context, onlineAfter time.Time) (int, int) {
	if r == nil {
		return 0, 0
	}

	type row struct {
		Total  int64 `gorm:"column:total"`
		Online int64 `gorm:"column:online"`
	}

	var result row
	if err := r.db.WithContext(ctx).
		Model(&WorkerInstanceRecord{}).
		Select(
			"COUNT(*) AS total, "+
				"SUM(CASE WHEN status = 1 AND last_heartbeat_at >= ? THEN 1 ELSE 0 END) AS online",
			onlineAfter,
		).
		Scan(&result).Error; err != nil {
		return 0, 0
	}
	return int(result.Total), int(result.Online)
}

// CountByNodeSummary 返回按节点聚合的 Worker 总数和在线数。
func (r *WorkerInstanceRepository) CountByNodeSummary(ctx context.Context, onlineAfter time.Time) (map[uint64]int, map[uint64]int) {
	totalByNode := make(map[uint64]int)
	onlineByNode := make(map[uint64]int)
	if r == nil {
		return totalByNode, onlineByNode
	}

	type row struct {
		NodeID uint64 `gorm:"column:node_id"`
		Total  int    `gorm:"column:total"`
		Online int    `gorm:"column:online"`
	}

	var rows []row
	if err := r.db.WithContext(ctx).
		Model(&WorkerInstanceRecord{}).
		Select(
			"node_id, COUNT(*) AS total, "+
				"SUM(CASE WHEN status = 1 AND last_heartbeat_at >= ? THEN 1 ELSE 0 END) AS online",
			onlineAfter,
		).
		Group("node_id").
		Scan(&rows).Error; err != nil {
		return totalByNode, onlineByNode
	}
	for _, item := range rows {
		totalByNode[item.NodeID] = item.Total
		onlineByNode[item.NodeID] = item.Online
	}
	return totalByNode, onlineByNode
}

// FindByWorkerID 按稳定 worker_id 查询实例记录。
//
// 后台治理接口和集群远程控制链路需要先定位“这个 worker 当前属于哪台节点”，
// 因此这里提供精确查询，避免上层再走模糊分页列表做二次筛选。
func (r *WorkerInstanceRepository) FindByWorkerID(ctx context.Context, workerID string) (WorkerInstanceRecord, bool) {
	var record WorkerInstanceRecord
	if err := r.db.WithContext(ctx).Where("worker_id = ?", strings.TrimSpace(workerID)).Take(&record).Error; err != nil {
		return WorkerInstanceRecord{}, false
	}
	return record, true
}
