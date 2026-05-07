// Package redislock 提供基于 Redis 的分布式锁实现。
//
// 支持单机 Redis 和 Redis Cluster 两种部署模式，底层统一使用 go-redis UniversalClient。
// 锁的实现采用 SETNX + TTL 经典方案，并通过 Lua 脚本保证释放和续期的原子性。
//
// 使用方式：
//
//	lock := redislock.NewLock(redisClient)
//	acquired, err := lock.Acquire(ctx, "my-lock", "unique-value", 30*time.Second)
//	if acquired {
//	    defer lock.Release(ctx, "my-lock", "unique-value")
//	    // 执行业务逻辑...
//	}
package redislock

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	rediscache "hvc/internal/infra/cache/redis"
)

// Lock 表示基于 Redis 的分布式锁。
//
// 核心设计原则：
//  1. 加锁使用 SETNX（仅当 key 不存在时设置），配合 TTL 防止死锁
//  2. 释放锁使用 Lua 脚本，先比较 value 再删除，防止误删其他持有者的锁
//  3. 续期锁使用 Lua 脚本，先比较 value 再重置 TTL，保证只有锁持有者能续期
//  4. value 必须全局唯一（建议使用 UUID 或雪花 ID），用于标识锁的持有者
type Lock struct {
	client *rediscache.Client
}

// NewLock 创建分布式锁实例。
func NewLock(client *rediscache.Client) *Lock {
	return &Lock{client: client}
}

// Acquire 尝试获取分布式锁。
//
// 参数：
//   - ctx: 上下文，用于控制超时和取消
//   - key: 锁的 Redis 键名，建议使用 "hvc:lock:{业务标识}" 格式
//   - value: 锁持有者的唯一标识，用于后续安全释放和续期
//   - ttl: 锁的自动过期时间，防止持有者崩溃后死锁
//
// 返回值：
//   - acquired: 是否成功获取锁
//   - err: Redis 操作错误（非竞争失败）
func (l *Lock) Acquire(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	return l.client.Engine.SetNX(ctx, key, value, ttl).Result()
}

// TryLock 带重试的分布式锁获取。
//
// 在指定的时间窗口内，按照指定间隔反复尝试获取锁，直到成功或超时。
// 适用于锁竞争激烈的场景，避免单次 SETNX 失败就放弃。
//
// 参数：
//   - ctx: 上下文
//   - key: 锁键名
//   - value: 持有者唯一标识
//   - ttl: 锁过期时间
//   - retryInterval: 重试间隔
//   - timeout: 总超时时间，超时后放弃获取
func (l *Lock) TryLock(ctx context.Context, key string, value string, ttl time.Duration, retryInterval time.Duration, timeout time.Duration) (bool, error) {
	deadline := time.Now().Add(timeout)
	for {
		acquired, err := l.Acquire(ctx, key, value, ttl)
		if err != nil {
			return false, err
		}
		if acquired {
			return true, nil
		}
		if time.Now().After(deadline) {
			return false, nil
		}
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(retryInterval):
		}
	}
}

// Release 安全释放分布式锁。
//
// 使用 Lua 脚本保证原子性：只有当 key 存在且 value 匹配时才删除，
// 避免误删其他锁持有者重新获取的锁。
//
// Lua 脚本逻辑：
//
//	if redis.call("GET", KEYS[1]) == ARGV[1] then
//	    return redis.call("DEL", KEYS[1])
//	else
//	    return 0
//	end
//
// 返回值：
//   - released: 是否成功释放（true 表示确实是本持有者释放的）
//   - err: Redis 操作错误
func (l *Lock) Release(ctx context.Context, key string, value string) (bool, error) {
	script := redis.NewScript(`
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("DEL", KEYS[1])
		else
			return 0
		end
	`)
	result, err := script.Run(ctx, l.client.Engine, []string{key}, value).Int64()
	if err != nil {
		return false, fmt.Errorf("release lock failed: %w", err)
	}
	return result == 1, nil
}

// Extend 续期分布式锁。
//
// 使用 Lua 脚本保证原子性：只有当 key 存在且 value 匹配时才重置 TTL，
// 防止续期了其他持有者的锁。
//
// 适用于长时间任务需要在执行过程中延长锁持有时间的场景。
// 建议在任务执行期间以小于 TTL/2 的间隔定期续期。
//
// Lua 脚本逻辑：
//
//	if redis.call("GET", KEYS[1]) == ARGV[1] then
//	    return redis.call("PEXPIRE", KEYS[1], ARGV[2])
//	else
//	    return 0
//	end
//
// 返回值：
//   - extended: 是否成功续期
//   - err: Redis 操作错误
func (l *Lock) Extend(ctx context.Context, key string, value string, ttl time.Duration) (bool, error) {
	script := redis.NewScript(`
		if redis.call("GET", KEYS[1]) == ARGV[1] then
			return redis.call("PEXPIRE", KEYS[1], ARGV[2])
		else
			return 0
		end
	`)
	ttlMS := int64(ttl / time.Millisecond)
	result, err := script.Run(ctx, l.client.Engine, []string{key}, value, ttlMS).Int64()
	if err != nil {
		return false, fmt.Errorf("extend lock failed: %w", err)
	}
	return result == 1, nil
}

// IsHeld 检查锁是否被持有。
//
// 注意：此方法只检查 key 是否存在，不验证持有者身份。
// 在高并发场景下，返回结果可能已过时，仅适用于监控和调试。
func (l *Lock) IsHeld(ctx context.Context, key string) (bool, error) {
	result, err := l.client.Engine.Exists(ctx, key).Result()
	if err != nil {
		return false, err
	}
	return result > 0, nil
}

// GetHolder 获取锁的当前持有者标识。
//
// 返回空字符串表示锁未被任何人持有。
func (l *Lock) GetHolder(ctx context.Context, key string) (string, error) {
	result, err := l.client.Engine.Get(ctx, key).Result()
	if err == redis.Nil {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return result, nil
}

// GetTTL 获取锁的剩余过期时间。
//
// 返回值：
//   - ttl: 剩余时间，-2 表示 key 不存在，-1 表示 key 存在但没有关联过期时间
//   - err: Redis 操作错误
func (l *Lock) GetTTL(ctx context.Context, key string) (time.Duration, error) {
	return l.client.Engine.TTL(ctx, key).Result()
}

// ForceRelease 强制释放锁，不验证持有者身份。
//
// 危险操作！仅在以下场景使用：
//  1. 管理员手动干预死锁
//  2. 节点故障恢复时清理残留锁
//  3. 测试环境清理
//
// 生产环境应优先使用 Release 方法。
func (l *Lock) ForceRelease(ctx context.Context, key string) error {
	return l.client.Engine.Del(ctx, key).Err()
}
