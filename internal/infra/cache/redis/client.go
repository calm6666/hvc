package redis

import (
	"context"
	"github.com/redis/go-redis/v9"
	"hvc/internal/config"
	"time"
)

// Client 表示 Redis 客户端。
type Client struct {
	Engine redis.UniversalClient
}

// Open 打开 Redis 连接。
func Open(cfg config.RedisConfig) (*Client, error) {
	engine := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:        cfg.Addrs,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  cfg.DialTimeout,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := engine.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return &Client{Engine: engine}, nil
}

// GetString 获取字符串。
func (c *Client) GetString(ctx context.Context, key string) (string, error) {
	value, err := c.Engine.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return value, nil
}
