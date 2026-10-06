package mysql

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// RuntimeConfigRepository 表示运行配置仓储。
type RuntimeConfigRepository struct {
	db *DB
}

// NewRuntimeConfigRepository 创建运行配置仓储。
func NewRuntimeConfigRepository(db *DB) *RuntimeConfigRepository {
	return &RuntimeConfigRepository{db: db}
}

// LatestPublished 查询最新已发布运行配置。
func (r *RuntimeConfigRepository) LatestPublished(ctx context.Context) (RuntimeConfigRecord, bool) {
	var record RuntimeConfigRecord
	if err := r.db.WithContext(ctx).Where("published = ?", true).Order("config_version desc").Take(&record).Error; err != nil {
		return RuntimeConfigRecord{}, false
	}
	return record, true
}

// ListVersionsPage 返回运行配置版本分页列表。
func (r *RuntimeConfigRepository) ListVersionsPage(ctx context.Context, page, pageSize int, published *bool) ([]RuntimeConfigRecord, int64, error) {
	page, pageSize = normalizeAdminPage(page, pageSize)
	query := r.db.WithContext(ctx).Model(&RuntimeConfigRecord{})
	if published != nil {
		query = query.Where("published = ?", *published)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var records []RuntimeConfigRecord
	if err := query.Order("config_version desc").Offset((page - 1) * pageSize).Limit(pageSize).Find(&records).Error; err != nil {
		return nil, 0, err
	}
	return records, total, nil
}

// Save 保存运行配置版本。
func (r *RuntimeConfigRepository) Save(ctx context.Context, record RuntimeConfigRecord) error {
	return r.db.WithContext(ctx).Create(&record).Error
}

// EnsureBootstrapPublished 在没有已发布版本时写入一条首启默认版本。
func (r *RuntimeConfigRepository) EnsureBootstrapPublished(ctx context.Context, record RuntimeConfigRecord) error {
	var count int64
	if err := r.db.WithContext(ctx).Model(&RuntimeConfigRecord{}).Where("published = ?", true).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	record.Published = true
	return r.db.WithContext(ctx).Create(&record).Error
}

// NormalizeBootstrapPublicGRPCDefault 把历史“首启默认自动开启 public gRPC”的 bootstrap 初始版本纠正为关闭。
//
// 只修正程序自动生成、且从未进入后台配置流转的那一类初始记录：
// - published_by = bootstrap
// - config_source = bootstrap_local
// - change_summary = bootstrap initial runtime config
//
// 这样可以避免影响管理员后续显式发布过的版本。
func (r *RuntimeConfigRepository) NormalizeBootstrapPublicGRPCDefault(ctx context.Context) (bool, error) {
	now := time.Now()
	result := r.db.WithContext(ctx).Model(&RuntimeConfigRecord{}).
		Where("published = ?", true).
		Where("published_by = ?", "bootstrap").
		Where("config_source = ?", "bootstrap_local").
		Where("change_summary = ?", "bootstrap initial runtime config").
		Where("public_grpc_enabled = ?", true).
		Updates(map[string]any{
			"public_grpc_enabled": false,
			"updated_at":          now,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

// MarkPublished 切换当前发布版本。
func (r *RuntimeConfigRepository) MarkPublished(ctx context.Context, configVersion uint64, publishedBy string) error {
	now := time.Now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&RuntimeConfigRecord{}).Where("published = ?", true).Updates(map[string]any{
			"published":  false,
			"updated_at": now,
		}).Error; err != nil {
			return err
		}
		result := tx.Model(&RuntimeConfigRecord{}).Where("config_version = ?", configVersion).Updates(map[string]any{
			"published":    true,
			"published_by": publishedBy,
			"published_at": now,
			"updated_at":   now,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

// SwitchPublished 切换当前发布版本，并返回切换前的已发布版本。
//
// 之所以显式返回旧版本，是为了让上层在“数据库已切换、但缓存或进程内快照刷新失败”时，
// 能把发布状态安全回滚，避免出现接口返回失败但数据库实际上已经切版本的半成功状态。
func (r *RuntimeConfigRepository) SwitchPublished(ctx context.Context, configVersion uint64, publishedBy string) (RuntimeConfigRecord, bool, error) {
	var previous RuntimeConfigRecord
	var previousExists bool
	now := time.Now()

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("published = ?", true).Order("config_version desc").Take(&previous).Error; err == nil {
			previousExists = true
		} else if err != nil && err != gorm.ErrRecordNotFound {
			return err
		}

		if err := tx.Model(&RuntimeConfigRecord{}).Where("published = ?", true).Updates(map[string]any{
			"published":  false,
			"updated_at": now,
		}).Error; err != nil {
			return err
		}

		result := tx.Model(&RuntimeConfigRecord{}).Where("config_version = ?", configVersion).Updates(map[string]any{
			"published":    true,
			"published_by": publishedBy,
			"published_at": now,
			"updated_at":   now,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
	return previous, previousExists, err
}

// RestorePublished 回滚一次失败的发布切换。
func (r *RuntimeConfigRepository) RestorePublished(ctx context.Context, failedVersion uint64, previous RuntimeConfigRecord, previousExists bool) error {
	now := time.Now()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if failedVersion > 0 {
			if err := tx.Model(&RuntimeConfigRecord{}).Where("config_version = ?", failedVersion).Updates(map[string]any{
				"published":    false,
				"published_by": "",
				"published_at": nil,
				"updated_at":   now,
			}).Error; err != nil {
				return err
			}
		}
		if !previousExists || previous.ConfigVersion == 0 {
			return nil
		}
		return tx.Model(&RuntimeConfigRecord{}).Where("config_version = ?", previous.ConfigVersion).Updates(map[string]any{
			"published":    true,
			"published_by": previous.PublishedBy,
			"published_at": previous.PublishedAt,
			"updated_at":   now,
		}).Error
	})
}
