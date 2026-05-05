package redisstreams

import (
	"context"

	"github.com/redis/go-redis/v9"
	rediscache "hvc/internal/infra/cache/redis"
)

// Producer 表示 Redis Streams 生产者。
type Producer struct {
	client *rediscache.Client
}

// NewProducer 创建 Redis Streams 生产者。
func NewProducer(client *rediscache.Client) *Producer {
	return &Producer{client: client}
}

// Publish 发布消息。
func (p *Producer) Publish(ctx context.Context, stream string, values map[string]any) error {
	return p.client.Engine.XAdd(ctx, &redis.XAddArgs{Stream: stream, Values: values}).Err()
}
