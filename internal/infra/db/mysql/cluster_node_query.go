package mysql

import "context"

// FindByNodeID 根据节点 ID 查询节点。
func (r *ClusterNodeRepository) FindByNodeID(ctx context.Context, nodeID uint64) (ClusterNodeRecord, bool) {
	var record ClusterNodeRecord
	if err := r.db.WithContext(ctx).Where("node_id = ?", nodeID).Take(&record).Error; err != nil {
		return ClusterNodeRecord{}, false
	}
	return record, true
}
