package mysql

import "context"

// LoadDynamicRuntimeConfigFromDB 读取数据库中当前已发布的动态运行配置。
func LoadDynamicRuntimeConfigFromDB(ctx context.Context, db *DB) (DynamicRuntimeConfigResult, bool) {
	repo := NewRuntimeConfigRepository(db)
	record, ok := repo.LatestPublished(ctx)
	if !ok {
		return DynamicRuntimeConfigResult{}, false
	}
	namingTemplateRepo := NewNamingTemplateRepository(db)
	cfg := ApplyLatestNamingTemplate(ctx, namingTemplateRepo, ToDynamicRuntimeConfig(record))
	return DynamicRuntimeConfigResult{Record: record, Config: cfg}, true
}

// DynamicRuntimeConfigResult 表示数据库中读取到的动态运行配置结果。
type DynamicRuntimeConfigResult struct {
	Record RuntimeConfigRecord
	Config any
}
