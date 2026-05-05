package config

import (
	"context"

	"hvc/internal/configcenter"
)

// RefreshUseCase 表示配置刷新用例。
type RefreshUseCase struct {
	effective *configcenter.EffectiveConfig
}

// NewRefreshUseCase 创建配置刷新用例。
func NewRefreshUseCase(effective *configcenter.EffectiveConfig) *RefreshUseCase {
	return &RefreshUseCase{effective: effective}
}

// Execute 执行配置刷新。
func (u *RefreshUseCase) Execute(ctx context.Context, cfg any) {
	_ = ctx
	if runtimeConfig, ok := cfg.(interface{ }); ok {
		_ = runtimeConfig
	}
}
