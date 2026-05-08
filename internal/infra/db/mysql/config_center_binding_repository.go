package mysql

import (
	"context"
	"time"

	"hvc/pkg/idgen"
)

// ConfigCenterBindingRepository 表示外部 bootstrap 配置源绑定仓储。
type ConfigCenterBindingRepository struct {
	db *DB
}

// NewConfigCenterBindingRepository 创建外部 bootstrap 配置源绑定仓储。
func NewConfigCenterBindingRepository(db *DB) *ConfigCenterBindingRepository {
	return &ConfigCenterBindingRepository{db: db}
}

// ListAll 返回全部 bootstrap 配置源绑定。
func (r *ConfigCenterBindingRepository) ListAll(ctx context.Context) []ConfigCenterBindingRecord {
	var records []ConfigCenterBindingRecord
	if err := r.db.WithContext(ctx).Order("priority desc, updated_at desc").Find(&records).Error; err != nil {
		return nil
	}
	return records
}

// Save 保存 bootstrap 配置源绑定。
func (r *ConfigCenterBindingRepository) Save(ctx context.Context, record ConfigCenterBindingRecord) error {
	if record.BindingID == 0 {
		record.BindingID = idgen.Next()
	}
	now := time.Now()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	record.UpdatedAt = now
	return r.db.WithContext(ctx).Save(&record).Error
}
