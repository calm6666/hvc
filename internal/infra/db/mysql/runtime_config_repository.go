package mysql

import (
	"context"
	"time"
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

// Save 保存运行配置版本。
func (r *RuntimeConfigRepository) Save(ctx context.Context, record RuntimeConfigRecord) error {
	return r.db.WithContext(ctx).Create(&record).Error
}

// MarkPublished 切换当前发布版本。
func (r *RuntimeConfigRepository) MarkPublished(ctx context.Context, configVersion uint64, publishedBy string) error {
	now := time.Now()
	if err := r.db.WithContext(ctx).Model(&RuntimeConfigRecord{}).Where("published = ?", true).Updates(map[string]any{"published": false, "updated_at": now}).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&RuntimeConfigRecord{}).Where("config_version = ?", configVersion).Updates(map[string]any{
		"published":    true,
		"published_by": publishedBy,
		"published_at": now,
		"updated_at":   now,
	}).Error
}
