package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	rediscache "hvc/internal/infra/cache/redis"
)

// LeaseState 表示任务租约状态。
type LeaseState struct {
	JobID           uint64    `json:"job_id"`
	WorkerID        string    `json:"worker_id"`
	LeaseGeneration uint64    `json:"lease_generation"`
	ExpireAt        time.Time `json:"expire_at"`
}

// LeaseCache 表示任务租约缓存。
type LeaseCache struct {
	client *rediscache.Client
}

// NewLeaseCache 创建任务租约缓存。
func NewLeaseCache(client *rediscache.Client) *LeaseCache {
	return &LeaseCache{client: client}
}

// Save 保存租约状态。
func (c *LeaseCache) Save(ctx context.Context, lease LeaseState) {
	if c == nil || c.client == nil {
		return
	}
	payload, err := json.Marshal(lease)
	if err != nil {
		return
	}
	key := fmt.Sprintf("hvc:job:%d:lease", lease.JobID)
	_ = c.client.Engine.Set(ctx, key, payload, 0).Err()
}

// Get 获取租约状态。
func (c *LeaseCache) Get(ctx context.Context, jobID uint64) (LeaseState, bool) {
	if c == nil || c.client == nil {
		return LeaseState{}, false
	}
	key := fmt.Sprintf("hvc:job:%d:lease", jobID)
	payload, err := c.client.Engine.Get(ctx, key).Result()
	if err != nil || payload == "" {
		return LeaseState{}, false
	}
	var lease LeaseState
	if err := json.Unmarshal([]byte(payload), &lease); err != nil {
		return LeaseState{}, false
	}
	return lease, true
}
