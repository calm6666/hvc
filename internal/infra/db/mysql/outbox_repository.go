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
	now := time.Now()
	if err := r.db.WithContext(ctx).
		Where("status = ? AND (next_retry_at IS NULL OR next_retry_at <= ?)", model.OutboxStatusPending, now).
		Find(&records).Error; err != nil {
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

// MarkRetryable 标记本次投递失败，并安排下次重试。
func (r *OutboxRepository) MarkRetryable(ctx context.Context, eventID uint64, message string, nextRetryAt time.Time) error {
	return r.db.WithContext(ctx).Model(&OutboxRecord{}).Where("event_id = ?", eventID).Updates(map[string]any{
		"status":             model.OutboxStatusFailed,
		"retry_count":        gormExpr("retry_count + 1"),
		"last_error_message": message,
		"next_retry_at":      nextRetryAt,
		"updated_at":         time.Now(),
	}).Error
}

// MarkFinalFailed 标记事件已达到最终失败状态。
func (r *OutboxRepository) MarkFinalFailed(ctx context.Context, eventID uint64, message string) error {
	return r.db.WithContext(ctx).Model(&OutboxRecord{}).Where("event_id = ?", eventID).Updates(map[string]any{
		"status":             model.OutboxStatusFailed,
		"retry_count":        gormExpr("retry_count + 1"),
		"last_error_message": message,
		"next_retry_at":      time.Time{},
		"updated_at":         time.Now(),
	}).Error
}

// CleanDelivered 清理已投递成功的事件记录。
//
// 删除超过 retainDuration 的已投递记录，防止表无限增长。
// 默认保留 7 天。
func (r *OutboxRepository) CleanDelivered(ctx context.Context, retainDuration time.Duration) int {
	if retainDuration <= 0 {
		retainDuration = 7 * 24 * time.Hour
	}
	cutoff := time.Now().Add(-retainDuration)
	result := r.db.WithContext(ctx).
		Where("status = ? AND updated_at < ?", model.OutboxStatusDelivered, cutoff).
		Delete(&OutboxRecord{})
	return int(result.RowsAffected)
}

// CleanExpiredFailed 清理超过最大重试次数的失败记录。
//
// 这些记录已经无法再重试，可以归档或删除。
func (r *OutboxRepository) CleanExpiredFailed(ctx context.Context) int {
	result := r.db.WithContext(ctx).
		Where("status = ? AND retry_count >= max_retry_count", model.OutboxStatusFailed).
		Delete(&OutboxRecord{})
	return int(result.RowsAffected)
}

// ListRetryable 返回可重试的失败事件。
func (r *OutboxRepository) ListRetryable(ctx context.Context) []model.OutboxEvent {
	var records []OutboxRecord
	now := time.Now()
	if err := r.db.WithContext(ctx).
		Where("status = ? AND next_retry_at <= ? AND retry_count < max_retry_count", model.OutboxStatusFailed, now).
		Find(&records).Error; err != nil {
		return nil
	}
	items := make([]model.OutboxEvent, 0, len(records))
	for _, record := range records {
		items = append(items, toOutboxModel(record))
	}
	return items
}

// ResetToPending 将失败事件重置为待投递状态（手动重试）。
func (r *OutboxRepository) ResetToPending(ctx context.Context, eventID uint64) error {
	return r.db.WithContext(ctx).Model(&OutboxRecord{}).Where("event_id = ?", eventID).Updates(map[string]any{
		"status":        model.OutboxStatusPending,
		"retry_count":   0,
		"next_retry_at": time.Now(),
		"updated_at":    time.Now(),
	}).Error
}
