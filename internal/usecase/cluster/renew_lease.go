package cluster

import "context"

// RenewLeaseUseCase 表示续租用例。
type RenewLeaseUseCase struct {}

// Execute 执行续租。
func (u *RenewLeaseUseCase) Execute(ctx context.Context) { _ = ctx }
