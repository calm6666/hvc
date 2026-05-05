package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"hvc/internal/model"

	rediscache "hvc/internal/infra/cache/redis"
)

// StateCache 表示集群热路径状态缓存。
type StateCache struct {
	client *rediscache.Client
}

// NewStateCache 创建热路径状态缓存。
func NewStateCache(client *rediscache.Client) *StateCache {
	return &StateCache{client: client}
}

// SaveHeartbeat 保存心跳。
func (c *StateCache) SaveHeartbeat(ctx context.Context, heartbeat model.WorkerHeartbeat) {
	if c == nil || c.client == nil {
		return
	}
	payload, err := json.Marshal(heartbeat)
	if err != nil {
		return
	}
	key := fmt.Sprintf("hvc:worker:%s:heartbeat", heartbeat.WorkerID)
	_ = c.client.Engine.Set(ctx, key, payload, 0).Err()
}

// SaveNodeMetrics 保存节点指标。
func (c *StateCache) SaveNodeMetrics(ctx context.Context, metrics model.NodeMetrics) {
	if c == nil || c.client == nil {
		return
	}
	payload, err := json.Marshal(metrics)
	if err != nil {
		return
	}
	key := fmt.Sprintf("hvc:node:%d:metrics", metrics.NodeID)
	_ = c.client.Engine.Set(ctx, key, payload, 0).Err()
}

// GetNodeMetrics 获取节点指标。
func (c *StateCache) GetNodeMetrics(ctx context.Context, nodeID uint64) (model.NodeMetrics, bool) {
	if c == nil || c.client == nil {
		return model.NodeMetrics{}, false
	}
	key := fmt.Sprintf("hvc:node:%d:metrics", nodeID)
	payload, err := c.client.Engine.Get(ctx, key).Result()
	if err != nil || payload == "" {
		return model.NodeMetrics{}, false
	}
	var metrics model.NodeMetrics
	if err := json.Unmarshal([]byte(payload), &metrics); err != nil {
		return model.NodeMetrics{}, false
	}
	return metrics, true
}
