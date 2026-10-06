package mysql

import (
	"context"
	"strings"
)

// ClusterNodeRepository 表示集群节点仓储。
type ClusterNodeRepository struct {
	db *DB
}

// NewClusterNodeRepository 创建集群节点仓储。
func NewClusterNodeRepository(db *DB) *ClusterNodeRepository {
	return &ClusterNodeRepository{db: db}
}

// List 返回节点列表。
func (r *ClusterNodeRepository) List(ctx context.Context) []ClusterNodeRecord {
	var records []ClusterNodeRecord
	if err := r.db.WithContext(ctx).Order("node_id asc").Find(&records).Error; err != nil {
		return nil
	}
	return records
}

// FindByID 按节点 ID 查询节点主档。
func (r *ClusterNodeRepository) FindByID(ctx context.Context, nodeID uint64) (ClusterNodeRecord, bool) {
	var record ClusterNodeRecord
	if err := r.db.WithContext(ctx).Where("node_id = ?", nodeID).Take(&record).Error; err != nil {
		return ClusterNodeRecord{}, false
	}
	return record, true
}

// ListForAdminSnapshot 返回后台集群总览类接口使用的轻量节点视图。
//
// 这些接口只关心节点身份、角色、启停状态、对外入口、容量上限和最近心跳，
// 不需要把完整主档字段每次都查出来。
func (r *ClusterNodeRepository) ListForAdminSnapshot(ctx context.Context) []ClusterNodeRecord {
	var records []ClusterNodeRecord
	if err := r.db.WithContext(ctx).
		Select("node_id", "node_name", "host_ip", "grpc_host", "http_host", "enabled", "quarantined", "draining", "max_transcode_sessions", "max_upload_concurrency", "node_tags", "last_heartbeat_at").
		Order("node_id asc").
		Find(&records).Error; err != nil {
		return nil
	}
	return records
}

// ListForScheduler 返回调度器候选节点的轻量视图。
//
// 调度器每轮只需要节点身份、管理状态、能力标记和心跳时间，
// 没必要把 CPU / 内存 / 磁盘等完整主档每次都读回来。
func (r *ClusterNodeRepository) ListForScheduler(ctx context.Context) []ClusterNodeRecord {
	var records []ClusterNodeRecord
	if err := r.db.WithContext(ctx).
		Select("node_id", "node_name", "http_host", "enabled", "quarantined", "draining", "node_tags", "support_nvenc", "support_qsv", "support_amf", "max_transcode_sessions", "max_upload_concurrency", "last_heartbeat_at").
		Order("node_id asc").
		Find(&records).Error; err != nil {
		return nil
	}
	return records
}

// ListHeartbeatSnapshot 返回节点心跳快照。
//
// WebSocket 监控快照只需要 node_id 和 last_heartbeat_at，
// 这里直接做瘦查询避免每 3 秒把整行节点主档重拉一遍。
func (r *ClusterNodeRepository) ListHeartbeatSnapshot(ctx context.Context) []ClusterNodeRecord {
	var records []ClusterNodeRecord
	if err := r.db.WithContext(ctx).
		Select("node_id", "last_heartbeat_at").
		Order("node_id asc").
		Find(&records).Error; err != nil {
		return nil
	}
	return records
}

// ListPage 返回节点分页列表。
func (r *ClusterNodeRepository) ListPage(ctx context.Context, page, pageSize int, keyword string, enabled *bool, quarantined *bool, draining *bool) ([]ClusterNodeRecord, int64, error) {
	page, pageSize = normalizeAdminPage(page, pageSize)
	query := r.db.WithContext(ctx).Model(&ClusterNodeRecord{})
	if keyword = strings.TrimSpace(keyword); keyword != "" {
		query = query.Where("node_name LIKE ? OR host_ip LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if enabled != nil {
		query = query.Where("enabled = ?", *enabled)
	}
	if quarantined != nil {
		query = query.Where("quarantined = ?", *quarantined)
	}
	if draining != nil {
		query = query.Where("draining = ?", *draining)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var records []ClusterNodeRecord
	if err := query.Order("node_id asc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&records).Error; err != nil {
		return nil, 0, err
	}
	return records, total, nil
}
