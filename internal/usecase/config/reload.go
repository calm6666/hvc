package config

import (
	"context"

	"hvc/internal/configcenter"
	"hvc/internal/infra/db/mysql"
)

// ReloadUseCase 表示配置重载用例。
type ReloadUseCase struct {
	effective               *configcenter.EffectiveConfig
	runtimeConfigRepository *mysql.RuntimeConfigRepository
	namingTemplateRepo      *mysql.NamingTemplateRepository
}

// NewReloadUseCase 创建配置重载用例。
func NewReloadUseCase(effective *configcenter.EffectiveConfig, runtimeConfigRepository *mysql.RuntimeConfigRepository, namingTemplateRepo *mysql.NamingTemplateRepository) *ReloadUseCase {
	return &ReloadUseCase{
		effective:               effective,
		runtimeConfigRepository: runtimeConfigRepository,
		namingTemplateRepo:      namingTemplateRepo,
	}
}

// Execute 执行配置重载。
//
// 当前第一阶段把 reload 和 refresh 的落点保持一致：
// 都是重新读取数据库里已发布版本并替换当前生效配置。
func (u *ReloadUseCase) Execute(ctx context.Context) {
	if u.effective == nil || u.runtimeConfigRepository == nil {
		return
	}
	record, ok := u.runtimeConfigRepository.LatestPublished(ctx)
	if !ok {
		return
	}
	cfg := mysql.ApplyLatestNamingTemplate(ctx, u.namingTemplateRepo, mysql.ToDynamicRuntimeConfig(record))
	u.effective.ReplaceWithVersion(cfg, record.ConfigVersion)
}
