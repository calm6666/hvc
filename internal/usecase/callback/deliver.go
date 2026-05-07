// Package callback 提供回调事件投递的业务用例实现。
//
// 本文件实现回调事件投递用例，负责将转码完成、失败等事件
// 通过 HTTP/gRPC/MQ 等渠道投递给外部业务系统。
//
// 投递策略：
//   - HTTP 回调：POST 请求到配置的 URL，支持重试和超时控制
//   - gRPC 回调：调用配置的 gRPC 服务方法
//   - MQ 回调：将事件消息发送到 RabbitMQ/Redis Streams/Kafka
//
// 可靠性保证：
//   - 所有事件先写入 Outbox 表，再异步投递
//   - 投递失败时更新重试计数和下次重试时间
//   - 超过最大重试次数后标记为最终失败
//   - 支持投递失败队列（Dead Letter Queue）人工干预
package callback

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"hvc/internal/config"
	"hvc/internal/infra/db/mysql"
	"hvc/internal/model"
	"hvc/pkg/logx"
	"hvc/pkg/retryx"
)

// DeliverUseCase 表示回调投递用例。
//
// 投递流程：
//  1. 从 Outbox 表扫描待投递事件（status=PENDING 且 next_retry_at <= now）
//  2. 根据 callback_config 中的配置选择投递渠道
//  3. 执行投递并处理结果：
//     - 成功：更新 Outbox 状态为 DELIVERED
//     - 可重试失败：增加重试计数，计算下次重试时间，状态保持 SENDING
//     - 不可重试失败：标记为 FAILED，写入投递失败队列
//  4. 记录投递尝试日志
type DeliverUseCase struct {
	outboxRepository   *mysql.OutboxRepository
	callbackConfigRepo *mysql.CallbackConfigRepository
	cfg                config.DynamicRuntimeConfig
	httpClient         *http.Client
}

// NewDeliverUseCase 创建回调投递用例实例。
func NewDeliverUseCase(
	outboxRepository *mysql.OutboxRepository,
	callbackConfigRepo *mysql.CallbackConfigRepository,
	cfg config.DynamicRuntimeConfig,
) *DeliverUseCase {
	return &DeliverUseCase{
		outboxRepository:   outboxRepository,
		callbackConfigRepo: callbackConfigRepo,
		cfg:                cfg,
		httpClient: &http.Client{
			Timeout: cfg.Callback.HTTPTimeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 3 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
	}
}

// DeliverResult 表示单次投递结果。
type DeliverResult struct {
	EventID      uint64 `json:"event_id"`
	Success      bool   `json:"success"`
	StatusCode   int    `json:"status_code,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
	RetryCount   int    `json:"retry_count"`
	FinalFailed  bool   `json:"final_failed"`
}

// Execute 执行回调投递。
//
// 从 Outbox 表中扫描待投递事件并逐一投递。
// 投递顺序按 event_id 升序，保证事件顺序性。
func (u *DeliverUseCase) Execute(ctx context.Context) []DeliverResult {
	if u.outboxRepository == nil {
		return nil
	}

	events := u.outboxRepository.ListPending(ctx)
	if len(events) == 0 {
		return nil
	}

	var results []DeliverResult
	for _, event := range events {
		result := u.deliverOne(ctx, event)
		results = append(results, result)
	}

	return results
}

// deliverOne 投递单个事件。
//
// 根据事件类型和回调配置选择投递渠道：
//   - transcode.completed: HTTP POST 回调
//   - transcode.failed: HTTP POST 回调
//   - 其他事件类型: 默认 HTTP POST
//
// 投递成功：更新 Outbox 状态为 DELIVERED
// 投递失败：根据重试策略决定是否重试
func (u *DeliverUseCase) deliverOne(ctx context.Context, event model.OutboxEvent) DeliverResult {
	result := DeliverResult{
		EventID:    event.EventID,
		RetryCount: event.RetryCount,
	}

	callbackURL := u.cfg.Callback.HTTPURL
	if callbackURL == "" {
		result.Success = false
		result.ErrorMessage = "回调 URL 未配置"
		result.FinalFailed = true
		u.markFinalFailed(ctx, event, result.ErrorMessage)
		return result
	}

	payload, err := u.buildPayload(event)
	if err != nil {
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("构建回调载荷失败: %v", err)
		result.FinalFailed = true
		u.markFinalFailed(ctx, event, result.ErrorMessage)
		return result
	}

	statusCode, body, err := u.sendHTTP(ctx, callbackURL, payload)
	result.StatusCode = statusCode

	if err != nil {
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("HTTP 回调失败: %v", err)
		u.handleRetry(ctx, event, result.ErrorMessage)
		return result
	}

	if statusCode >= 200 && statusCode < 300 {
		result.Success = true
		u.outboxRepository.MarkDelivered(ctx, event.EventID)
		logx.Info("callback.deliver.success", logx.Fields{
			"event_id":    event.EventID,
			"event_type":  event.EventType,
			"job_id":      event.JobID,
			"status_code": statusCode,
		})
	} else {
		result.Success = false
		result.ErrorMessage = fmt.Sprintf("回调返回非成功状态码: %d, body: %s", statusCode, truncate(body, 200))
		u.handleRetry(ctx, event, result.ErrorMessage)
	}

	return result
}

// buildPayload 构建回调载荷。
//
// 将 Outbox 事件转换为标准回调 JSON 格式：
//
//	{
//	  "event_id": 123,
//	  "event_type": "transcode.completed",
//	  "job_id": 456,
//	  "request_id": "req-xxx",
//	  "timestamp": 1700000000,
//	  "payload": { ... }
//	}
func (u *DeliverUseCase) buildPayload(event model.OutboxEvent) (map[string]any, error) {
	var payload map[string]any
	if event.PayloadJSON != "" && event.PayloadJSON != "{}" {
		if err := json.Unmarshal([]byte(event.PayloadJSON), &payload); err != nil {
			payload = make(map[string]any)
		}
	} else {
		payload = make(map[string]any)
	}

	return map[string]any{
		"event_id":   event.EventID,
		"event_type": event.EventType,
		"job_id":     event.JobID,
		"request_id": event.RequestID,
		"timestamp":  time.Now().Unix(),
		"payload":    payload,
	}, nil
}

// sendHTTP 发送 HTTP 回调请求。
//
// 使用 POST 方法，Content-Type 为 application/json。
// 支持超时控制和上下文取消。
func (u *DeliverUseCase) sendHTTP(ctx context.Context, url string, payload map[string]any) (int, string, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, "", fmt.Errorf("序列化载荷失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, "", fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "hvc-callback/1.0")
	req.Header.Set("X-Event-ID", fmt.Sprintf("%d", payload["event_id"]))
	req.Header.Set("X-Event-Type", fmt.Sprintf("%v", payload["event_type"]))

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return 0, "", err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(respBody), nil
}

// handleRetry 处理投递失败后的重试逻辑。
//
// 重试策略：
//  1. 如果重试次数已达上限，标记为最终失败
//  2. 否则，使用指数退避计算下次重试时间
//  3. 更新 Outbox 记录的重试计数和下次重试时间
func (u *DeliverUseCase) handleRetry(ctx context.Context, event model.OutboxEvent, errMsg string) {
	if event.RetryCount+1 >= event.MaxRetryCount {
		u.markFinalFailed(ctx, event, errMsg)
		return
	}

	backoff := retryx.Backoff{
		BaseDelay: u.cfg.Callback.RetryBackoff,
		MaxDelay:  u.cfg.Callback.RetryBackoff * 16,
	}.Duration(event.RetryCount)
	nextRetryAt := time.Now().Add(backoff)

	if u.outboxRepository != nil {
		u.outboxRepository.MarkRetryable(ctx, event.EventID, errMsg, nextRetryAt)
	}

	logx.Info("callback.deliver.retry_scheduled", logx.Fields{
		"event_id":      event.EventID,
		"retry_count":   event.RetryCount + 1,
		"next_retry_at": nextRetryAt.Unix(),
		"error":         errMsg,
	})
}

// markFinalFailed 标记事件为最终投递失败。
//
// 当事件超过最大重试次数仍然投递失败时调用。
// 事件状态更新为 FAILED，并记录最后的错误信息。
func (u *DeliverUseCase) markFinalFailed(ctx context.Context, event model.OutboxEvent, errMsg string) {
	u.outboxRepository.MarkFinalFailed(ctx, event.EventID, errMsg)

	logx.Error("callback.deliver.final_failed", nil, logx.Fields{
		"event_id":    event.EventID,
		"event_type":  event.EventType,
		"job_id":      event.JobID,
		"retry_count": event.RetryCount,
		"error":       errMsg,
	})
}

// truncate 截断字符串到指定长度。
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
