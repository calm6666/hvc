package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"hvc/internal/model"
)

// ProgressStore 表示任务进度缓存。
type ProgressStore struct {
	client *Client
}

// NewProgressStore 创建任务进度缓存。
func NewProgressStore(client *Client) *ProgressStore {
	return &ProgressStore{client: client}
}

// Save 保存任务进度。
func (s *ProgressStore) Save(ctx context.Context, snapshot model.ProgressSnapshot) {
	if s == nil || s.client == nil {
		return
	}
	if snapshot.UpdatedAt.IsZero() {
		snapshot.UpdatedAt = time.Now()
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return
	}
	key := fmt.Sprintf("hvc:job:%d:progress", snapshot.JobID)
	_ = s.client.Engine.Set(ctx, key, payload, 0).Err()
}

// Get 获取任务进度。
func (s *ProgressStore) Get(ctx context.Context, jobID uint64) (model.ProgressSnapshot, bool) {
	if s == nil || s.client == nil {
		return model.ProgressSnapshot{}, false
	}
	key := fmt.Sprintf("hvc:job:%d:progress", jobID)
	payload, err := s.client.Engine.Get(ctx, key).Result()
	if err != nil || payload == "" {
		return model.ProgressSnapshot{}, false
	}
	var snapshot model.ProgressSnapshot
	if err := json.Unmarshal([]byte(payload), &snapshot); err != nil {
		return model.ProgressSnapshot{}, false
	}
	return snapshot, true
}
