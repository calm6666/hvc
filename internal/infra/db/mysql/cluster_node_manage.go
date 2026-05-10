package mysql

import (
	"context"
	"time"
)

// EnsureLocalNode 确保本机节点在节点表里存在。
func (r *ClusterNodeRepository) EnsureLocalNode(ctx context.Context, nodeID uint64, nodeName, hostIP, httpHost string) error {
	var record ClusterNodeRecord
	now := time.Now()
	result := r.db.WithContext(ctx).Where("node_id = ?", nodeID).Limit(1).Find(&record)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		return r.db.WithContext(ctx).Model(&ClusterNodeRecord{}).Where("node_id = ?", nodeID).Updates(map[string]any{
			"node_name":            nodeName,
			"host_ip":              hostIP,
			"http_host":            httpHost,
			"enabled":              true,
			"last_state_change_at": now,
			"updated_at":           now,
			"last_heartbeat_at":    now,
		}).Error
	}
	record = ClusterNodeRecord{
		NodeID:             nodeID,
		NodeName:           nodeName,
		HostIP:             hostIP,
		HTTPHost:           httpHost,
		Enabled:            true,
		Quarantined:        false,
		Draining:           false,
		LastStateChangeAt:  now,
		CreatedAt:          now,
		UpdatedAt:          now,
		LastHeartbeatAt:    now,
		CapacityGeneration: 1,
	}
	return r.db.WithContext(ctx).Create(&record).Error
}

// SetEnabled 切换节点启用状态。
func (r *ClusterNodeRepository) SetEnabled(ctx context.Context, nodeID uint64, enabled bool) error {
	return r.db.WithContext(ctx).Model(&ClusterNodeRecord{}).Where("node_id = ?", nodeID).Updates(map[string]any{
		"enabled":    enabled,
		"updated_at": time.Now(),
	}).Error
}

// SetQuarantined 切换节点隔离状态。
func (r *ClusterNodeRepository) SetQuarantined(ctx context.Context, nodeID uint64, quarantined bool, reason string) error {
	return r.db.WithContext(ctx).Model(&ClusterNodeRecord{}).Where("node_id = ?", nodeID).Updates(map[string]any{
		"quarantined":          quarantined,
		"quarantine_reason":    reason,
		"last_state_change_at": time.Now(),
		"updated_at":           time.Now(),
	}).Error
}

// SetDraining 切换节点排空状态。
//
// 排空用于运维维护窗口：
// 1. 已在跑的任务继续执行；
// 2. 新任务调度时直接过滤该节点；
// 3. 与 quarantined 区别在于它更偏“有序摘流量”而不是“故障隔离”。
func (r *ClusterNodeRepository) SetDraining(ctx context.Context, nodeID uint64, draining bool, reason string) error {
	return r.db.WithContext(ctx).Model(&ClusterNodeRecord{}).Where("node_id = ?", nodeID).Updates(map[string]any{
		"draining":             draining,
		"drain_reason":         reason,
		"last_state_change_at": time.Now(),
		"updated_at":           time.Now(),
	}).Error
}

// TouchHeartbeat 更新节点最近一次心跳时间。
//
// 节点在线判定不能只依赖 Redis 中的短 TTL metrics，
// 这里把心跳同时落到节点主档，便于后台排障和调度器在 metrics 缺失时做保守判断。
func (r *ClusterNodeRepository) TouchHeartbeat(ctx context.Context, nodeID uint64, heartbeatAt time.Time) error {
	if heartbeatAt.IsZero() {
		heartbeatAt = time.Now()
	}
	return r.db.WithContext(ctx).Model(&ClusterNodeRecord{}).Where("node_id = ?", nodeID).Updates(map[string]any{
		"last_heartbeat_at": heartbeatAt,
		"updated_at":        time.Now(),
	}).Error
}
