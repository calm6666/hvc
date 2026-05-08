package config

import (
	"context"

	"hvc/internal/configcenter"
	"hvc/internal/infra/db/mysql"
)

// RefreshUseCase 表示配置刷新用例。
type RefreshUseCase struct {
	effective                *configcenter.EffectiveConfig
	runtimeConfigRepository  *mysql.RuntimeConfigRepository
}

// NewRefreshUseCase 创建配置刷新用例。
func NewRefreshUseCase(effective *configcenter.EffectiveConfig, runtimeConfigRepository *mysql.RuntimeConfigRepository) *RefreshUseCase {
	return &RefreshUseCase{effective: effective, runtimeConfigRepository: runtimeConfigRepository}
}

// Execute 执行配置刷新。
//
// 第一阶段里，“刷新”最少要做到一件真正有意义的事：
// 把数据库里当前最新已发布配置重新加载进 EffectiveConfig。
func (u *RefreshUseCase) Execute(ctx context.Context) {
	if u.effective == nil || u.runtimeConfigRepository == nil {
		return
	}
	record, ok := u.runtimeConfigRepository.LatestPublished(ctx)
	if !ok {
		return
	}
	u.effective.ReplaceWithVersion(mysql.ToDynamicRuntimeConfig(record), record.ConfigVersion)
}
