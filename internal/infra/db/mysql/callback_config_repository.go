package mysql

import "context"

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

// Save 保存回调配置。
func (r *CallbackConfigRepository) Save(ctx context.Context, record CallbackConfigRecord) error {
	return r.db.WithContext(ctx).Save(&record).Error
}

// SetEnabled 切换回调配置启用状态。
func (r *CallbackConfigRepository) SetEnabled(ctx context.Context, callbackConfigID uint64, enabled bool) error {
	return r.db.WithContext(ctx).Model(&CallbackConfigRecord{}).Where("callback_config_id = ?", callbackConfigID).Update("enabled", enabled).Error
}
