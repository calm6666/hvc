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
