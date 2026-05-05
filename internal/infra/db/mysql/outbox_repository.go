package mysql

import (
	"context"
	"hvc/internal/model"
	"time"
)

func toOutboxRecord(event model.OutboxEvent) OutboxRecord {
	return OutboxRecord{
		EventID:          event.EventID,
		EventType:        event.EventType,
		JobID:            event.JobID,
		RequestID:        event.RequestID,
		PayloadJSON:      event.PayloadJSON,
		Status:           event.Status,
		RetryCount:       event.RetryCount,
		MaxRetryCount:    event.MaxRetryCount,
		NextRetryAt:      event.NextRetryAt,
		LastErrorMessage: event.LastErrorMessage,
		CreatedAt:        event.CreatedAt,
		UpdatedAt:        event.UpdatedAt,
	}
}

func toOutboxModel(record OutboxRecord) model.OutboxEvent {
	return model.OutboxEvent{
		EventID:          record.EventID,
		EventType:        record.EventType,
		JobID:            record.JobID,
		RequestID:        record.RequestID,
		PayloadJSON:      record.PayloadJSON,
		Status:           record.Status,
		RetryCount:       record.RetryCount,
		MaxRetryCount:    record.MaxRetryCount,
		NextRetryAt:      record.NextRetryAt,
		LastErrorMessage: record.LastErrorMessage,
		CreatedAt:        record.CreatedAt,
		UpdatedAt:        record.UpdatedAt,
	}
}

// OutboxRepository 表示回调事件仓储。
type OutboxRepository struct {
	db *DB
}

// NewOutboxRepository 创建回调事件仓储。
func NewOutboxRepository(db *DB) *OutboxRepository {
	return &OutboxRepository{db: db}
}

// Save 保存事件。
func (r *OutboxRepository) Save(ctx context.Context, event model.OutboxEvent) error {
	record := toOutboxRecord(event)
	return r.db.WithContext(ctx).Create(&record).Error
}

// ListPending 返回待投递事件。
func (r *OutboxRepository) ListPending(ctx context.Context) []model.OutboxEvent {
	var records []OutboxRecord
	if err := r.db.WithContext(ctx).Where("status IN ?", []int{model.OutboxStatusPending, model.OutboxStatusFailed}).Find(&records).Error; err != nil {
		return nil
	}
	items := make([]model.OutboxEvent, 0, len(records))
	for _, record := range records {
		items = append(items, toOutboxModel(record))
	}
	return items
}

// MarkDelivered 标记投递成功。
func (r *OutboxRepository) MarkDelivered(ctx context.Context, eventID uint64) error {
	return r.db.WithContext(ctx).Model(&OutboxRecord{}).Where("event_id = ?", eventID).Updates(map[string]any{
		"status":     model.OutboxStatusDelivered,
		"updated_at": time.Now(),
	}).Error
}

// MarkFailed 标记投递失败。
func (r *OutboxRepository) MarkFailed(ctx context.Context, eventID uint64, message string) error {
	return r.db.WithContext(ctx).Model(&OutboxRecord{}).Where("event_id = ?", eventID).Updates(map[string]any{
		"status":             model.OutboxStatusFailed,
		"retry_count":        gormExpr("retry_count + 1"),
		"last_error_message": message,
		"next_retry_at":      time.Now().Add(2 * time.Second),
		"updated_at":         time.Now(),
	}).Error
}
