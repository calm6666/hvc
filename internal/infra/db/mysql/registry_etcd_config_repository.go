package mysql

import (
	"context"
	"strings"
	"time"

	"hvc/pkg/idgen"
)

// RegistryEtcdConfigRepository 表示 etcd 注册配置仓储。
type RegistryEtcdConfigRepository struct {
	db *DB
}

// NewRegistryEtcdConfigRepository 创建 etcd 注册配置仓储。
func NewRegistryEtcdConfigRepository(db *DB) *RegistryEtcdConfigRepository {
	return &RegistryEtcdConfigRepository{db: db}
}

// ListAll 返回全部 etcd 注册配置。
func (r *RegistryEtcdConfigRepository) ListAll(ctx context.Context) []RegistryEtcdConfigRecord {
	var records []RegistryEtcdConfigRecord
	if err := r.db.WithContext(ctx).Order("priority desc, updated_at desc").Find(&records).Error; err != nil {
		return nil
	}
	return records
}

// ListPage 返回 etcd 注册配置分页列表。
func (r *RegistryEtcdConfigRepository) ListPage(ctx context.Context, page, pageSize int, registryName string, enabled *bool) ([]RegistryEtcdConfigRecord, int64, error) {
	page, pageSize = normalizeAdminPage(page, pageSize)
	query := r.db.WithContext(ctx).Model(&RegistryEtcdConfigRecord{})
	if registryName = strings.TrimSpace(registryName); registryName != "" {
		query = query.Where("registry_name LIKE ?", "%"+registryName+"%")
	}
	if enabled != nil {
		query = query.Where("enabled = ?", *enabled)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var records []RegistryEtcdConfigRecord
	if err := query.Order("priority desc, updated_at desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&records).Error; err != nil {
		return nil, 0, err
	}
	return records, total, nil
}

// Save 保存 etcd 注册配置。
func (r *RegistryEtcdConfigRepository) Save(ctx context.Context, record RegistryEtcdConfigRecord) error {
	if record.RegistryID == 0 {
		record.RegistryID = idgen.Next()
	}
	now := time.Now()
	var existing RegistryEtcdConfigRecord
	result := r.db.WithContext(ctx).Where("registry_id = ?", record.RegistryID).Limit(1).Find(&existing)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		record.CreatedAt = existing.CreatedAt
		record.UpdatedAt = now
		return r.db.WithContext(ctx).Model(&RegistryEtcdConfigRecord{}).
			Where("registry_id = ?", record.RegistryID).
			Updates(map[string]any{
				"registry_name":     record.RegistryName,
				"endpoints":         record.Endpoints,
				"service_namespace": record.ServiceNamespace,
				"lease_ttl_sec":     record.LeaseTTLSec,
				"dial_timeout_ms":   record.DialTimeoutMS,
				"enabled":           record.Enabled,
				"priority":          record.Priority,
				"updated_at":        now,
			}).Error
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	record.UpdatedAt = now
	return r.db.WithContext(ctx).Create(&record).Error
}

// SetEnabled 切换 etcd 注册配置启用状态。
func (r *RegistryEtcdConfigRepository) SetEnabled(ctx context.Context, registryID uint64, enabled bool) error {
	return r.db.WithContext(ctx).Model(&RegistryEtcdConfigRecord{}).
		Where("registry_id = ?", registryID).
		Updates(map[string]any{"enabled": enabled, "updated_at": time.Now()}).Error
}

// FindByID 根据主键查询 etcd 注册配置。
func (r *RegistryEtcdConfigRepository) FindByID(ctx context.Context, registryID uint64) (RegistryEtcdConfigRecord, bool) {
	var record RegistryEtcdConfigRecord
	if err := r.db.WithContext(ctx).Where("registry_id = ?", registryID).Take(&record).Error; err != nil {
		return RegistryEtcdConfigRecord{}, false
	}
	return record, true
}
