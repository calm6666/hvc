package redislock

import (
	"context"
	"time"

	rediscache "hvc/internal/infra/cache/redis"
)

// Lock 表示分布式锁。
type Lock struct {
	client *rediscache.Client
}

// NewLock 创建分布式锁。
func NewLock(client *rediscache.Client) *Lock {
	return &Lock{client: client}
}

// Acquire 获取锁。
func (l *Lock) Acquire(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	return l.client.Engine.SetNX(ctx, key, value, ttl).Result()
}
