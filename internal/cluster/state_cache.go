package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"hvc/internal/model"
	"time"

	rediscache "hvc/internal/infra/cache/redis"
)

const (
	// workerHeartbeatTTL 控制心跳在 Redis 热路径中的有效窗口。
	// 超过该时间还没有刷新，后台观测和调度就不应继续把该 Worker 视为在线。
	workerHeartbeatTTL = 2 * time.Minute
	// nodeMetricsTTL 控制节点指标在 Redis 中的保鲜时间。
	// 指标本身允许秒级延迟，但不能无限陈旧。
	nodeMetricsTTL = 30 * time.Second
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
	_ = c.client.Engine.Set(ctx, key, payload, workerHeartbeatTTL).Err()
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
	key := nodeMetricsKey(metrics.NodeID)
	_ = c.client.Engine.Set(ctx, key, payload, nodeMetricsTTL).Err()
}

// GetNodeMetrics 获取节点指标。
func (c *StateCache) GetNodeMetrics(ctx context.Context, nodeID uint64) (model.NodeMetrics, bool) {
	if c == nil || c.client == nil {
		return model.NodeMetrics{}, false
	}
	key := nodeMetricsKey(nodeID)
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

// GetNodeMetricsBatch 批量获取节点指标。
//
// 后台集群总览、实时快照、节点列表这类接口会频繁一次读取多个节点状态。
// 这里直接使用 Redis MGET，把原先 N 次往返压成 1 次，减轻高频轮询下的 Redis RTT 开销。
func (c *StateCache) GetNodeMetricsBatch(ctx context.Context, nodeIDs []uint64) map[uint64]model.NodeMetrics {
	result := make(map[uint64]model.NodeMetrics)
	if c == nil || c.client == nil || len(nodeIDs) == 0 {
		return result
	}

	keys := make([]string, 0, len(nodeIDs))
	orderedNodeIDs := make([]uint64, 0, len(nodeIDs))
	seen := make(map[uint64]struct{}, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		if nodeID == 0 {
			continue
		}
		if _, exists := seen[nodeID]; exists {
			continue
		}
		seen[nodeID] = struct{}{}
		keys = append(keys, nodeMetricsKey(nodeID))
		orderedNodeIDs = append(orderedNodeIDs, nodeID)
	}
	if len(keys) == 0 {
		return result
	}

	values, err := c.client.Engine.MGet(ctx, keys...).Result()
	if err != nil {
		return result
	}
	for index, raw := range values {
		payload, ok := raw.(string)
		if !ok || payload == "" {
			continue
		}
		var metrics model.NodeMetrics
		if err := json.Unmarshal([]byte(payload), &metrics); err != nil {
			continue
		}
		result[orderedNodeIDs[index]] = metrics
	}
	return result
}

// GetJSONSnapshot 读取短 TTL 的 JSON 摘要缓存。
//
// 该能力专门给后台高频 polling 接口复用：
// 1. 值本身就是最终响应 JSON；
// 2. 命中后直接回写，避免重复做对象拼装、再编码一次 JSON；
// 3. TTL 通常只保留 1~3 秒，用于削峰而不是长期缓存。
func (c *StateCache) GetJSONSnapshot(ctx context.Context, key string) (string, bool) {
	if c == nil || c.client == nil || key == "" {
		return "", false
	}
	value, err := c.client.GetString(ctx, key)
	if err != nil || value == "" {
		return "", false
	}
	return value, true
}

// SaveJSONSnapshot 保存短 TTL 的 JSON 摘要缓存。
func (c *StateCache) SaveJSONSnapshot(ctx context.Context, key string, payload string, ttl time.Duration) error {
	if c == nil || c.client == nil || key == "" || payload == "" {
		return nil
	}
	if ttl <= 0 {
		ttl = 2 * time.Second
	}
	return c.client.SetString(ctx, key, payload, ttl)
}

// DeleteJSONSnapshots 删除短 TTL 的 JSON 摘要缓存。
//
// 这类缓存主要服务后台高频 polling 接口。对于节点治理、配置发布这类写操作，
// 仅靠 2~3 秒 TTL 仍然会留下短暂脏读窗口，因此写成功后需要主动删除。
func (c *StateCache) DeleteJSONSnapshots(ctx context.Context, keys ...string) error {
	if c == nil || c.client == nil || len(keys) == 0 {
		return nil
	}
	return c.client.Delete(ctx, keys...)
}

// AdminClusterSummaryCacheKey 返回后台集群摘要缓存 key。
func AdminClusterSummaryCacheKey(name string, localNodeID uint64, version uint64) string {
	return fmt.Sprintf("hvc:admin:cluster:%s:node:%d:version:%d", name, localNodeID, version)
}

// MonitorSnapshotCacheKey 返回后台监控快照缓存 key。
func MonitorSnapshotCacheKey(mode string) string {
	return fmt.Sprintf("hvc:monitor:snapshot:mode:%s", mode)
}

func nodeMetricsKey(nodeID uint64) string {
	return fmt.Sprintf("hvc:node:%d:metrics", nodeID)
}
