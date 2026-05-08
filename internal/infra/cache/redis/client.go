package redis

import (
	"context"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"hvc/internal/config"
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

// SetString 写入字符串值并设置过期时间。
//
// expire=0 表示不设置 TTL，由调用方自行管理失效时机。
func (c *Client) SetString(ctx context.Context, key string, value string, expire time.Duration) error {
	if c == nil || c.Engine == nil {
		return nil
	}
	return c.Engine.Set(ctx, key, value, expire).Err()
}

// Delete 删除指定 key。
func (c *Client) Delete(ctx context.Context, keys ...string) error {
	if c == nil || c.Engine == nil || len(keys) == 0 {
		return nil
	}
	return c.Engine.Del(ctx, keys...).Err()
}

// TTL 返回 key 剩余过期时间。
func (c *Client) TTL(ctx context.Context, key string) (time.Duration, error) {
	if c == nil || c.Engine == nil {
		return 0, nil
	}
	return c.Engine.TTL(ctx, key).Result()
}

// ServerInfo 查询 Redis INFO 指定分组原始文本。
//
// 这里保留最底层的 INFO 能力，方便上层按需扩展更多观测字段，
// 但业务层通常只需要再调用 ServerVersion/ServerMode 这些语义化方法。
func (c *Client) ServerInfo(ctx context.Context, section string) string {
	if c == nil || c.Engine == nil {
		return ""
	}

	result, err := c.Engine.Info(ctx, section).Result()
	if err != nil {
		return ""
	}
	return result
}

// ServerVersion 返回 Redis 服务端版本号。
func (c *Client) ServerVersion(ctx context.Context) string {
	return parseRedisInfoField(c.ServerInfo(ctx, "server"), "redis_version")
}

// ServerMode 返回 Redis 当前运行模式，例如 standalone/cluster/sentinel。
func (c *Client) ServerMode(ctx context.Context) string {
	return parseRedisInfoField(c.ServerInfo(ctx, "server"), "redis_mode")
}

func parseRedisInfoField(info string, field string) string {
	if strings.TrimSpace(info) == "" || strings.TrimSpace(field) == "" {
		return ""
	}

	prefix := field + ":"
	for _, line := range strings.Split(info, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}
