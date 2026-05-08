package redis

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"hvc/internal/config"
)

const (
	// RuntimeConfigCacheKey 保存当前已发布运行时配置的完整快照。
	RuntimeConfigCacheKey = "hvc:runtime:config:published"
	// RuntimeConfigVersionKey 保存当前已发布运行时配置版本号，便于快速观测和排障。
	RuntimeConfigVersionKey = "hvc:runtime:config:version"
)

// RuntimeConfigCacheTTL 是运行时配置写入 Redis 的默认 TTL。
//
// 这里仍然使用显式失效作为主机制；TTL 只是兜底，避免极端情况下缓存永久陈旧。
const RuntimeConfigCacheTTL = 24 * time.Hour

// RuntimeConfigSnapshot 表示 Redis 中缓存的已发布运行时配置快照。
type RuntimeConfigSnapshot struct {
	ConfigVersion uint64                      `json:"config_version"`
	CachedAt      time.Time                   `json:"cached_at"`
	Config        config.DynamicRuntimeConfig `json:"config"`
}

// RuntimeConfigCache 封装运行时配置的 Redis 优先读缓存。
type RuntimeConfigCache struct {
	client *Client
}

// NewRuntimeConfigCache 创建运行时配置缓存访问器。
func NewRuntimeConfigCache(client *Client) *RuntimeConfigCache {
	return &RuntimeConfigCache{client: client}
}

// LoadPublished 从 Redis 读取当前已发布的运行时配置快照。
func (c *RuntimeConfigCache) LoadPublished(ctx context.Context) (RuntimeConfigSnapshot, bool, error) {
	if c == nil || c.client == nil {
		return RuntimeConfigSnapshot{}, false, nil
	}

	payload, err := c.client.GetString(ctx, RuntimeConfigCacheKey)
	if err != nil {
		return RuntimeConfigSnapshot{}, false, err
	}
	if payload == "" {
		return RuntimeConfigSnapshot{}, false, nil
	}

	var snapshot RuntimeConfigSnapshot
	if err := json.Unmarshal([]byte(payload), &snapshot); err != nil {
		return RuntimeConfigSnapshot{}, false, err
	}
	return snapshot, true, nil
}

// SavePublished 将已发布运行时配置快照回填到 Redis。
func (c *RuntimeConfigCache) SavePublished(ctx context.Context, snapshot RuntimeConfigSnapshot) error {
	if c == nil || c.client == nil {
		return nil
	}

	snapshot.CachedAt = time.Now()
	payload, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	if err := c.client.SetString(ctx, RuntimeConfigCacheKey, string(payload), RuntimeConfigCacheTTL); err != nil {
		return err
	}
	return c.client.SetString(ctx, RuntimeConfigVersionKey, jsonUint64(snapshot.ConfigVersion), RuntimeConfigCacheTTL)
}

// InvalidatePublished 删除当前已发布运行时配置缓存。
//
// 发布新版本时先更新数据库，再删 Redis，最后回填最新版本。
// 这样做可以保证其它实例在下一次读取时一定不会拿到旧缓存。
func (c *RuntimeConfigCache) InvalidatePublished(ctx context.Context) error {
	if c == nil || c.client == nil {
		return nil
	}
	return c.client.Delete(ctx, RuntimeConfigCacheKey, RuntimeConfigVersionKey)
}

// GetPublishedVersion 返回当前缓存中的运行时配置版本号。
func (c *RuntimeConfigCache) GetPublishedVersion(ctx context.Context) (uint64, bool, error) {
	if c == nil || c.client == nil {
		return 0, false, nil
	}

	value, err := c.client.GetString(ctx, RuntimeConfigVersionKey)
	if err != nil {
		return 0, false, err
	}
	if value == "" {
		return 0, false, nil
	}
	version, ok := parseUint64(value)
	if !ok {
		return 0, false, nil
	}
	return version, true, nil
}

// GetPublishedTTL 返回已发布运行时配置缓存的剩余 TTL。
func (c *RuntimeConfigCache) GetPublishedTTL(ctx context.Context) (time.Duration, error) {
	if c == nil || c.client == nil {
		return 0, nil
	}
	return c.client.TTL(ctx, RuntimeConfigCacheKey)
}

// RedisServerVersion 返回当前 Redis 服务端版本。
func (c *RuntimeConfigCache) RedisServerVersion(ctx context.Context) string {
	if c == nil || c.client == nil {
		return ""
	}
	return c.client.ServerVersion(ctx)
}

// RedisServerMode 返回当前 Redis 运行模式。
func (c *RuntimeConfigCache) RedisServerMode(ctx context.Context) string {
	if c == nil || c.client == nil {
		return ""
	}
	return c.client.ServerMode(ctx)
}

func jsonUint64(value uint64) string {
	return strconv.FormatUint(value, 10)
}

func parseUint64(value string) (uint64, bool) {
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, false
	}
	return parsed, true
}
