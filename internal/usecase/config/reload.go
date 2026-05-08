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
}

// NewReloadUseCase 创建配置重载用例。
func NewReloadUseCase(effective *configcenter.EffectiveConfig, runtimeConfigRepository *mysql.RuntimeConfigRepository) *ReloadUseCase {
	return &ReloadUseCase{effective: effective, runtimeConfigRepository: runtimeConfigRepository}
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
	u.effective.ReplaceWithVersion(mysql.ToDynamicRuntimeConfig(record), record.ConfigVersion)
}
