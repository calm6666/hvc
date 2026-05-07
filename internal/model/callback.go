package model

import "time"

// Outbox 事件状态常量，表示回调事件从创建到投递完成的完整状态。
const (
	OutboxStatusPending   = 1 // 待投递
	OutboxStatusSending   = 2 // 投递中
	OutboxStatusDelivered = 3 // 已投递
	OutboxStatusFailed    = 4 // 投递失败
)

// OutboxEvent 表示回调事件。
type OutboxEvent struct {
	EventID           uint64
	EventType         string
	JobID             uint64
	RequestID         string
	PayloadJSON       string
	Status            int
	RetryCount        int
	MaxRetryCount     int
	NextRetryAt       time.Time
	LastErrorMessage  string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}
