package mysql

import (
	"context"
	"strings"
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

// ListPage 返回 bootstrap 配置源绑定分页列表。
func (r *ConfigCenterBindingRepository) ListPage(ctx context.Context, page, pageSize int, providerType string, enabled *bool) ([]ConfigCenterBindingRecord, int64, error) {
	page, pageSize = normalizeAdminPage(page, pageSize)
	query := r.db.WithContext(ctx).Model(&ConfigCenterBindingRecord{})
	if providerType = strings.TrimSpace(providerType); providerType != "" {
		query = query.Where("provider_type = ?", providerType)
	}
	if enabled != nil {
		query = query.Where("enabled = ?", *enabled)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var records []ConfigCenterBindingRecord
	if err := query.Order("priority desc, updated_at desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&records).Error; err != nil {
		return nil, 0, err
	}
	return records, total, nil
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
