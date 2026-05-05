package callback

import "context"

// DeliverUseCase 表示回调投递用例。
type DeliverUseCase struct {}

// Execute 执行投递。
func (u *DeliverUseCase) Execute(ctx context.Context) { _ = ctx }
