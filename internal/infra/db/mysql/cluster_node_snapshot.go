package mysql

import (
	"context"
	"time"
)

// SaveSnapshot 保存一份当前集群节点指标快照到节点表衍生字段。
func (r *ClusterNodeRepository) SaveSnapshot(ctx context.Context, nodeID uint64, cpuCores, memoryTotalMB, maxTranscodeSessions, maxUploadConcurrency int, tags string) error {
	return r.db.WithContext(ctx).Model(&ClusterNodeRecord{}).Where("node_id = ?", nodeID).Updates(map[string]any{
		"cpu_cores":               cpuCores,
		"memory_total_mb":         memoryTotalMB,
		"max_transcode_sessions":  maxTranscodeSessions,
		"max_upload_concurrency":  maxUploadConcurrency,
		"node_tags":               tags,
		"updated_at":              time.Now(),
	}).Error
}
