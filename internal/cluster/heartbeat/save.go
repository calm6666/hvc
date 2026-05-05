package heartbeat

import (
	"context"
	clusterstate "hvc/internal/cluster"
	"hvc/internal/model"
)

// SaveHeartbeat 保存 Worker 心跳。
func SaveHeartbeat(ctx context.Context, cache *clusterstate.StateCache, heartbeat model.WorkerHeartbeat) {
	cache.SaveHeartbeat(ctx, heartbeat)
}

// SaveNodeMetrics 保存节点指标。
func SaveNodeMetrics(ctx context.Context, cache *clusterstate.StateCache, metrics model.NodeMetrics) {
	cache.SaveNodeMetrics(ctx, metrics)
}
