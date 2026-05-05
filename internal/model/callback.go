package model

import "time"

const (
	OutboxStatusPending   = 1
	OutboxStatusSending   = 2
	OutboxStatusDelivered = 3
	OutboxStatusFailed    = 4
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
