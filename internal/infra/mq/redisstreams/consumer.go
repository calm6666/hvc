// Package redisstreams 提供 Redis Streams 消息队列的消费者实现。
//
// 基于 Redis Streams 的 Consumer Group 机制实现消息消费：
//   - 支持多消费者组，同一消息只被组内一个消费者处理
//   - 支持消息确认（ACK），处理完成后必须确认
//   - 支持未确认消息的自动重新投递（XPENDING + XCLAIM）
//   - 支持阻塞读取，减少轮询开销
//
// 集群模式：多个 Worker 实例消费同一个 Stream，消息自动负载均衡
// 单机模式：单个 Worker 实例消费，所有消息顺序处理
package redisstreams

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	rediscache "hvc/internal/infra/cache/redis"
	"hvc/pkg/logx"
)

// Consumer 表示 Redis Streams 消费者。
type Consumer struct {
	client   *rediscache.Client
	group    string
	consumer string
}

// NewConsumer 创建 Redis Streams 消费者。
//
// 参数：
//   - client: Redis 客户端
//   - group: 消费者组名称，同一组内的消费者共同消费消息
//   - consumer: 消费者名称，组内唯一标识
func NewConsumer(client *rediscache.Client, group string, consumer string) *Consumer {
	return &Consumer{
		client:   client,
		group:    group,
		consumer: consumer,
	}
}

// EnsureGroup 确保消费者组存在。
//
// 如果消费者组不存在则创建，已存在时忽略 BUSYGROUP 错误。
// 必须在开始消费前调用。
//
// 参数：
//   - stream: Stream 名称
//   - startID: 起始消息 ID，"0" 表示从最早的消息开始，"$" 表示只消费新消息
func (c *Consumer) EnsureGroup(ctx context.Context, stream string, startID string) error {
	err := c.client.Engine.XGroupCreateMkStream(ctx, stream, c.group, startID).Err()
	if err != nil {
		errStr := err.Error()
		if len(errStr) > 10 && errStr[:10] == "BUSYGROUP " {
			return nil
		}
		return fmt.Errorf("创建消费者组失败: %w", err)
	}
	logx.Info("redisstreams.group_created", logx.Fields{
		"stream":   stream,
		"group":    c.group,
		"start_id": startID,
	})
	return nil
}

// Message 表示从 Redis Streams 读取的消息。
type Message struct {
	ID     string
	Stream string
	Values map[string]any
}

// Read 从 Stream 中读取消息。
//
// 使用 XREADGROUP 命令以消费者组模式读取，
// 确保同一组内每条消息只被一个消费者处理。
//
// 参数：
//   - ctx: 上下文
//   - stream: Stream 名称
//   - count: 一次最多读取的消息数
//   - block: 阻塞等待时间，0 表示不阻塞
func (c *Consumer) Read(ctx context.Context, stream string, count int64, block time.Duration) ([]Message, error) {
	result, err := c.client.Engine.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    c.group,
		Consumer: c.consumer,
		Streams:  []string{stream, ">"},
		Count:    count,
		Block:    block,
	}).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, fmt.Errorf("读取消息失败: %w", err)
	}

	var messages []Message
	for _, xStream := range result {
		for _, xMessage := range xStream.Messages {
			messages = append(messages, Message{
				ID:     xMessage.ID,
				Stream: xStream.Stream,
				Values: xMessage.Values,
			})
		}
	}
	return messages, nil
}

// Ack 确认消息已处理。
//
// 消息确认后从 Pending 列表中移除，不再被重新投递。
// 必须在消息处理成功后调用。
func (c *Consumer) Ack(ctx context.Context, stream string, messageIDs ...string) error {
	if len(messageIDs) == 0 {
		return nil
	}
	return c.client.Engine.XAck(ctx, stream, c.group, messageIDs...).Err()
}

// ClaimPending 认领超时未确认的消息。
//
// 从消费者组中查找超过 minIdleTime 未确认的消息，
// 将其转移给当前消费者重新处理。
//
// 适用于消费者崩溃后消息恢复的场景。
func (c *Consumer) ClaimPending(ctx context.Context, stream string, minIdleTime time.Duration, count int64) ([]Message, error) {
	pendingResult, err := c.client.Engine.XPendingExt(ctx, &redis.XPendingExtArgs{
		Stream: stream,
		Group:  c.group,
		Start:  "-",
		End:    "+",
		Count:  count,
		Idle:   minIdleTime,
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("查询待处理消息失败: %w", err)
	}

	if len(pendingResult) == 0 {
		return nil, nil
	}

	var ids []string
	for _, p := range pendingResult {
		ids = append(ids, p.ID)
	}

	claimed, err := c.client.Engine.XClaim(ctx, &redis.XClaimArgs{
		Stream:   stream,
		Group:    c.group,
		Consumer: c.consumer,
		MinIdle:  minIdleTime,
		Messages: ids,
	}).Result()
	if err != nil {
		return nil, fmt.Errorf("认领消息失败: %w", err)
	}

	var messages []Message
	for _, xMessage := range claimed {
		messages = append(messages, Message{
			ID:     xMessage.ID,
			Stream: stream,
			Values: xMessage.Values,
		})
	}
	return messages, nil
}

// ConsumeLoop 持续消费消息的循环。
//
// 典型用法：
//
//	consumer.ConsumeLoop(ctx, "hvc:transcode:jobs", 10, 5*time.Second, func(ctx context.Context, msg Message) error {
//	    // 处理消息...
//	    return nil
//	})
//
// 处理函数返回 nil 时自动 ACK，返回错误时记录日志但不 ACK，
// 消息将在后续被重新投递。
func (c *Consumer) ConsumeLoop(ctx context.Context, stream string, batchSize int64, blockTimeout time.Duration, handler func(ctx context.Context, msg Message) error) {
	logx.Info("redisstreams.consume_loop_started", logx.Fields{
		"stream":   stream,
		"group":    c.group,
		"consumer": c.consumer,
	})

	for {
		select {
		case <-ctx.Done():
			logx.Info("redisstreams.consume_loop_stopped", logx.Fields{
				"stream": stream,
			})
			return
		default:
		}

		messages, err := c.Read(ctx, stream, batchSize, blockTimeout)
		if err != nil {
			logx.Error("redisstreams.read_failed", err, logx.Fields{
				"stream": stream,
			})
			time.Sleep(time.Second)
			continue
		}

		for _, msg := range messages {
			if err := handler(ctx, msg); err != nil {
				logx.Error("redisstreams.handle_failed", err, logx.Fields{
					"stream":    stream,
					"message_id": msg.ID,
				})
				continue
			}
			if err := c.Ack(ctx, stream, msg.ID); err != nil {
				logx.Error("redisstreams.ack_failed", err, logx.Fields{
					"stream":    stream,
					"message_id": msg.ID,
				})
			}
		}
	}
}

// ParseMessage 将消息 Values 解析为指定类型。
func ParseMessage[T any](msg Message) (T, error) {
	var result T
	data, err := json.Marshal(msg.Values)
	if err != nil {
		return result, fmt.Errorf("序列化消息失败: %w", err)
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return result, fmt.Errorf("反序列化消息失败: %w", err)
	}
	return result, nil
}
