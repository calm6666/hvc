package config

import "context"

// ReloadUseCase 表示配置重载用例。
type ReloadUseCase struct {}

// Execute 执行配置重载。
func (u *ReloadUseCase) Execute(ctx context.Context) { _ = ctx }
