package mysql

import (
	"context"
	"time"
)

// EnsureLocalNode 确保本机节点在节点表里存在。
func (r *ClusterNodeRepository) EnsureLocalNode(ctx context.Context, nodeID uint64, nodeName, hostIP, httpHost string) error {
	var record ClusterNodeRecord
	now := time.Now()
	if err := r.db.WithContext(ctx).Where("node_id = ?", nodeID).Take(&record).Error; err == nil {
		return r.db.WithContext(ctx).Model(&ClusterNodeRecord{}).Where("node_id = ?", nodeID).Updates(map[string]any{
			"node_name":      nodeName,
			"host_ip":        hostIP,
			"http_host":      httpHost,
			"enabled":        true,
			"updated_at":     now,
			"last_heartbeat_at": now,
		}).Error
	}
	record = ClusterNodeRecord{
		NodeID:            nodeID,
		NodeName:          nodeName,
		HostIP:            hostIP,
		HTTPHost:          httpHost,
		Enabled:           true,
		Quarantined:       false,
		CreatedAt:         now,
		UpdatedAt:         now,
		LastHeartbeatAt:   now,
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
		"quarantined":        quarantined,
		"quarantine_reason":  reason,
		"last_state_change_at": time.Now(),
		"updated_at":         time.Now(),
	}).Error
}
