package mysql

import (
	"context"
	"strings"
	"time"

	"hvc/pkg/idgen"
)

// CallbackConfigRepository 表示回调配置仓储。
type CallbackConfigRepository struct {
	db *DB
}

// NewCallbackConfigRepository 创建回调配置仓储。
func NewCallbackConfigRepository(db *DB) *CallbackConfigRepository {
	return &CallbackConfigRepository{db: db}
}

// ListEnabled 返回已启用回调配置，按优先级降序。
func (r *CallbackConfigRepository) ListEnabled(ctx context.Context) []CallbackConfigRecord {
	var records []CallbackConfigRecord
	if err := r.db.WithContext(ctx).Where("enabled = ?", true).Order("priority desc, updated_at desc").Find(&records).Error; err != nil {
		return nil
	}
	return records
}

// ListAll 返回全部回调配置。
func (r *CallbackConfigRepository) ListAll(ctx context.Context) []CallbackConfigRecord {
	var records []CallbackConfigRecord
	if err := r.db.WithContext(ctx).Order("priority desc, updated_at desc").Find(&records).Error; err != nil {
		return nil
	}
	return records
}

// ListPage 返回回调配置分页列表。
func (r *CallbackConfigRepository) ListPage(ctx context.Context, page, pageSize int, callbackName string, enabled *bool) ([]CallbackConfigRecord, int64, error) {
	page, pageSize = normalizeAdminPage(page, pageSize)
	query := r.db.WithContext(ctx).Model(&CallbackConfigRecord{})
	if callbackName = strings.TrimSpace(callbackName); callbackName != "" {
		query = query.Where("callback_name LIKE ?", "%"+callbackName+"%")
	}
	if enabled != nil {
		query = query.Where("enabled = ?", *enabled)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var records []CallbackConfigRecord
	if err := query.Order("priority desc, updated_at desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&records).Error; err != nil {
		return nil, 0, err
	}
	return records, total, nil
}

// Save 保存回调配置。
func (r *CallbackConfigRepository) Save(ctx context.Context, record CallbackConfigRecord) error {
	if record.CallbackConfigID == 0 {
		record.CallbackConfigID = idgen.Next()
	}
	now := time.Now()
	var existing CallbackConfigRecord
	result := r.db.WithContext(ctx).Where("callback_config_id = ?", record.CallbackConfigID).Limit(1).Find(&existing)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		record.CreatedAt = existing.CreatedAt
		record.UpdatedAt = now
		return r.db.WithContext(ctx).Model(&CallbackConfigRecord{}).
			Where("callback_config_id = ?", record.CallbackConfigID).
			Updates(map[string]any{
				"callback_name":    record.CallbackName,
				"callback_type":    record.CallbackType,
				"target_url":       record.TargetURL,
				"rpc_endpoint":     record.RPCEndpoint,
				"rpc_service_name": record.RPCServiceName,
				"mq_exchange":      record.MQExchange,
				"mq_routing_key":   record.MQRoutingKey,
				"timeout_ms":       record.TimeoutMS,
				"retry_times":      record.RetryTimes,
				"enabled":          record.Enabled,
				"priority":         record.Priority,
				"registry_id":      record.RegistryID,
				"updated_at":       now,
			}).Error
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	record.UpdatedAt = now
	return r.db.WithContext(ctx).Create(&record).Error
}

// SetEnabled 切换回调配置启用状态。
func (r *CallbackConfigRepository) SetEnabled(ctx context.Context, callbackConfigID uint64, enabled bool) error {
	return r.db.WithContext(ctx).Model(&CallbackConfigRecord{}).
		Where("callback_config_id = ?", callbackConfigID).
		Updates(map[string]any{"enabled": enabled, "updated_at": time.Now()}).Error
}
