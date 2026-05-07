package mysql

import "context"

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
