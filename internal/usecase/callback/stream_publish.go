package callback

import (
	"context"

	"github.com/redis/go-redis/v9"
	rediscache "hvc/internal/infra/cache/redis"
)

// StreamPublisher 表示 Redis Streams 发布器。
type StreamPublisher struct {
	client *rediscache.Client
}

// NewStreamPublisher 创建 Redis Streams 发布器。
func NewStreamPublisher(client *rediscache.Client) *StreamPublisher {
	return &StreamPublisher{client: client}
}

// Publish 发布消息。
func (p *StreamPublisher) Publish(ctx context.Context, stream string, payload map[string]any) error {
	return p.client.Engine.XAdd(ctx, &redis.XAddArgs{Stream: stream, Values: payload}).Err()
}
