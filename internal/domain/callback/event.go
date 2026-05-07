// Package callback 提供回调事件领域对象。
//
// Event 表示一个需要投递给外部系统的回调事件，
// 例如转码完成通知、转码失败通知等。
//
// 回调投递流程：
//  1. 业务操作完成后创建 Event 并写入 Outbox 表
//  2. 异步投递器从 Outbox 表扫描待投递事件
//  3. 根据回调配置选择投递渠道（HTTP/gRPC/MQ）
//  4. 投递成功标记为 DELIVERED，失败则重试
//  5. 超过最大重试次数后标记为 FAILED
package callback

import "time"

// Event 表示回调事件领域对象。
type Event struct {
	EventID     uint64
	EventType   string
	JobID       uint64
	RequestID   string
	PayloadJSON string
	Status      string
	RetryCount  int
	MaxRetry    int
	CreatedAt   time.Time
	NextRetryAt time.Time
}

// EventStatus 定义事件状态常量。
const (
	EventStatusPending   = "PENDING"
	EventStatusSending   = "SENDING"
	EventStatusDelivered = "DELIVERED"
	EventStatusFailed    = "FAILED"
)

// IsPending 判断事件是否待投递。
func (e *Event) IsPending() bool {
	return e.Status == EventStatusPending
}

// CanRetry 判断事件是否可以重试。
func (e *Event) CanRetry() bool {
	return e.RetryCount < e.MaxRetry
}

// IsFinalFailed 判断事件是否已最终失败。
func (e *Event) IsFinalFailed() bool {
	return e.Status == EventStatusFailed && e.RetryCount >= e.MaxRetry
}

// MarkRetry 标记事件为重试中。
func (e *Event) MarkRetry() {
	e.RetryCount++
	e.Status = EventStatusPending
}

// MarkDelivered 标记事件为已投递。
func (e *Event) MarkDelivered() {
	e.Status = EventStatusDelivered
}

// MarkFailed 标记事件为投递失败。
func (e *Event) MarkFailed() {
	if e.CanRetry() {
		e.Status = EventStatusPending
	} else {
		e.Status = EventStatusFailed
	}
}

// EventTypeCompleted 转码完成事件类型。
const EventTypeCompleted = "transcode.completed"

// EventTypeFailed 转码失败事件类型。
const EventTypeFailed = "transcode.failed"
