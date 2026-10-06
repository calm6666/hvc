package mysql

import (
	"context"
	"hvc/internal/model"
	"strings"
	"time"
)

func toOutboxRecord(event model.OutboxEvent) OutboxRecord {
	maxRetryCount := effectiveOutboxMaxRetryCount(event.MaxRetryCount)
	return OutboxRecord{
		EventID:          event.EventID,
		EventType:        event.EventType,
		JobID:            event.JobID,
		RequestID:        event.RequestID,
		PayloadJSON:      event.PayloadJSON,
		Status:           event.Status,
		RetryCount:       event.RetryCount,
		MaxRetryCount:    maxRetryCount,
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
		MaxRetryCount:    effectiveOutboxMaxRetryCount(record.MaxRetryCount),
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
	if err := r.db.WithContext(ctx).Create(&record).Error; err != nil {
		if !isLegacyOutboxSchemaError(err) {
			return err
		}
		return r.db.WithContext(ctx).Table(record.TableName()).Create(map[string]any{
			"event_id":        record.EventID,
			"job_id":          record.JobID,
			"event_type":      record.EventType,
			"payload_json":    record.PayloadJSON,
			"delivery_status": record.Status,
			"retry_count":     record.RetryCount,
			"next_retry_at":   record.NextRetryAt,
			"created_at":      record.CreatedAt,
			"updated_at":      record.UpdatedAt,
		}).Error
	}
	return nil
}

// ListPending 返回待投递事件。
func (r *OutboxRepository) ListPending(ctx context.Context) []model.OutboxEvent {
	var records []OutboxRecord
	now := time.Now()
	if err := r.db.WithContext(ctx).
		Where("delivery_status = ? AND (next_retry_at IS NULL OR next_retry_at <= ?)", model.OutboxStatusPending, now).
		Find(&records).Error; err != nil {
		return nil
	}
	items := make([]model.OutboxEvent, 0, len(records))
	for _, record := range records {
		items = append(items, toOutboxModel(record))
	}
	return items
}

// TryMarkSending 尝试以 CAS 方式抢占事件投递权。
//
// 只有当前状态仍等于 expectedStatus 时才会更新为 sending，
// 这样多个节点并发扫 outbox 时，同一事件只能被一个节点真正拿到。
func (r *OutboxRepository) TryMarkSending(ctx context.Context, eventID uint64, expectedStatus int) bool {
	result := r.db.WithContext(ctx).Model(&OutboxRecord{}).
		Where("event_id = ? AND delivery_status = ?", eventID, expectedStatus).
		Updates(map[string]any{
			"delivery_status": model.OutboxStatusSending,
			"updated_at":      time.Now(),
		})
	return result.Error == nil && result.RowsAffected == 1
}

// MarkDelivered 标记投递成功。
func (r *OutboxRepository) MarkDelivered(ctx context.Context, eventID uint64) error {
	return r.db.WithContext(ctx).Model(&OutboxRecord{}).Where("event_id = ?", eventID).Updates(map[string]any{
		"delivery_status": model.OutboxStatusDelivered,
		"updated_at":      time.Now(),
	}).Error
}

// MarkRetryable 标记本次投递失败，并安排下次重试。
func (r *OutboxRepository) MarkRetryable(ctx context.Context, eventID uint64, message string, nextRetryAt time.Time) error {
	updates := map[string]any{
		"delivery_status":    model.OutboxStatusFailed,
		"retry_count":        gormExpr("retry_count + 1"),
		"last_error_message": message,
		"next_retry_at":      nextRetryAt,
		"updated_at":         time.Now(),
	}
	if err := r.db.WithContext(ctx).Model(&OutboxRecord{}).Where("event_id = ?", eventID).Updates(updates).Error; err != nil {
		if !isMissingOutboxColumn(err, "last_error_message") {
			return err
		}
		delete(updates, "last_error_message")
		return r.db.WithContext(ctx).Table((&OutboxRecord{}).TableName()).Where("event_id = ?", eventID).Updates(updates).Error
	}
	return nil
}

// MarkFinalFailed 标记事件已达到最终失败状态。
func (r *OutboxRepository) MarkFinalFailed(ctx context.Context, eventID uint64, message string) error {
	updates := map[string]any{
		"delivery_status":    model.OutboxStatusFailed,
		"retry_count":        gormExpr("retry_count + 1"),
		"last_error_message": message,
		"next_retry_at":      nil,
		"updated_at":         time.Now(),
	}
	if err := r.db.WithContext(ctx).Model(&OutboxRecord{}).Where("event_id = ?", eventID).Updates(updates).Error; err != nil {
		if !isMissingOutboxColumn(err, "last_error_message") {
			return err
		}
		delete(updates, "last_error_message")
		return r.db.WithContext(ctx).Table((&OutboxRecord{}).TableName()).Where("event_id = ?", eventID).Updates(updates).Error
	}
	return nil
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
		Where("delivery_status = ? AND updated_at < ?", model.OutboxStatusDelivered, cutoff).
		Delete(&OutboxRecord{})
	return int(result.RowsAffected)
}

// CleanExpiredFailed 清理超过最大重试次数的失败记录。
//
// 这些记录已经无法再重试，可以归档或删除。
func (r *OutboxRepository) CleanExpiredFailed(ctx context.Context) int {
	result := r.db.WithContext(ctx).
		Where("delivery_status = ? AND retry_count >= max_retry_count", model.OutboxStatusFailed).
		Delete(&OutboxRecord{})
	if result.Error != nil && isMissingOutboxColumn(result.Error, "max_retry_count") {
		var records []OutboxRecord
		if err := r.db.WithContext(ctx).
			Where("delivery_status = ?", model.OutboxStatusFailed).
			Find(&records).Error; err != nil {
			return 0
		}
		ids := make([]uint64, 0, len(records))
		for _, record := range records {
			if record.RetryCount >= effectiveOutboxMaxRetryCount(record.MaxRetryCount) {
				ids = append(ids, record.EventID)
			}
		}
		if len(ids) == 0 {
			return 0
		}
		deleteResult := r.db.WithContext(ctx).Where("event_id IN ?", ids).Delete(&OutboxRecord{})
		return int(deleteResult.RowsAffected)
	}
	return int(result.RowsAffected)
}

// ListRetryable 返回可重试的失败事件。
func (r *OutboxRepository) ListRetryable(ctx context.Context) []model.OutboxEvent {
	var records []OutboxRecord
	now := time.Now()
	if err := r.db.WithContext(ctx).
		Where("delivery_status = ? AND next_retry_at <= ? AND retry_count < max_retry_count", model.OutboxStatusFailed, now).
		Find(&records).Error; err != nil {
		if !isMissingOutboxColumn(err, "max_retry_count") {
			return nil
		}
		if fallbackErr := r.db.WithContext(ctx).
			Where("delivery_status = ? AND next_retry_at <= ?", model.OutboxStatusFailed, now).
			Find(&records).Error; fallbackErr != nil {
			return nil
		}
		filtered := make([]OutboxRecord, 0, len(records))
		for _, record := range records {
			if record.RetryCount < effectiveOutboxMaxRetryCount(record.MaxRetryCount) {
				filtered = append(filtered, record)
			}
		}
		records = filtered
	}
	items := make([]model.OutboxEvent, 0, len(records))
	for _, record := range records {
		items = append(items, toOutboxModel(record))
	}
	return items
}

// ResetStaleSending 将长时间停留在 sending 的事件回收为可重试失败状态。
//
// 这个兜底用于处理“节点在成功抢占后崩溃/断电”的情况，避免事件永久卡死在 sending。
func (r *OutboxRepository) ResetStaleSending(ctx context.Context, staleBefore time.Time, message string) int {
	if message == "" {
		message = "callback sending timeout"
	}
	updates := map[string]any{
		"delivery_status":    model.OutboxStatusFailed,
		"last_error_message": message,
		"next_retry_at":      time.Now(),
		"updated_at":         time.Now(),
	}
	result := r.db.WithContext(ctx).Model(&OutboxRecord{}).
		Where("delivery_status = ? AND updated_at < ?", model.OutboxStatusSending, staleBefore).
		Updates(updates)
	if result.Error != nil && isMissingOutboxColumn(result.Error, "last_error_message") {
		delete(updates, "last_error_message")
		result = r.db.WithContext(ctx).Table((&OutboxRecord{}).TableName()).
			Where("delivery_status = ? AND updated_at < ?", model.OutboxStatusSending, staleBefore).
			Updates(updates)
	}
	return int(result.RowsAffected)
}

// ResetToPending 将失败事件重置为待投递状态（手动重试）。
func (r *OutboxRepository) ResetToPending(ctx context.Context, eventID uint64) error {
	return r.db.WithContext(ctx).Model(&OutboxRecord{}).Where("event_id = ?", eventID).Updates(map[string]any{
		"delivery_status": model.OutboxStatusPending,
		"retry_count":     0,
		"next_retry_at":   time.Now(),
		"updated_at":      time.Now(),
	}).Error
}

func effectiveOutboxMaxRetryCount(value int) int {
	if value > 0 {
		return value
	}
	return 3
}

func isLegacyOutboxSchemaError(err error) bool {
	if err == nil {
		return false
	}
	return isMissingOutboxColumn(err, "request_id") ||
		isMissingOutboxColumn(err, "max_retry_count") ||
		isMissingOutboxColumn(err, "last_error_message")
}

func isMissingOutboxColumn(err error, column string) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "Unknown column '"+column+"'")
}
