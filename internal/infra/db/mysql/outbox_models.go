package mysql

import "time"

// OutboxRecord 表示回调事件表记录。
type OutboxRecord struct {
	EventID          uint64     `gorm:"column:event_id;primaryKey"`
	EventType        string     `gorm:"column:event_type"`
	JobID            uint64     `gorm:"column:job_id"`
	RequestID        string     `gorm:"column:request_id"`
	PayloadJSON      string     `gorm:"column:payload_json"`
	Status           int        `gorm:"column:delivery_status"`
	RetryCount       int        `gorm:"column:retry_count"`
	MaxRetryCount    int        `gorm:"column:max_retry_count"`
	NextRetryAt      *time.Time `gorm:"column:next_retry_at"`
	LastErrorMessage string     `gorm:"column:last_error_message"`
	CreatedAt        time.Time  `gorm:"column:created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at"`
}

func (OutboxRecord) TableName() string { return "t_event_outbox" }

// DeliveryFailureQueueRecord 表示外部投递失败队列表映射。
type DeliveryFailureQueueRecord struct {
	FailureID        uint64     `gorm:"column:failure_id;primaryKey"`
	EventID          uint64     `gorm:"column:event_id"`
	FailureStage     string     `gorm:"column:failure_stage"`
	FailureCode      string     `gorm:"column:failure_code"`
	FailureMessage   string     `gorm:"column:failure_message"`
	CallbackConfigID uint64     `gorm:"column:callback_config_id"`
	CallbackTarget   string     `gorm:"column:callback_target"`
	RetryCount       int        `gorm:"column:retry_count"`
	NextRetryAt      *time.Time `gorm:"column:next_retry_at"`
	Resolved         bool       `gorm:"column:resolved"`
	CreatedAt        time.Time  `gorm:"column:created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at"`
}

func (DeliveryFailureQueueRecord) TableName() string { return "t_delivery_failure_queue" }
