package cluster

import (
	"context"

	clusterstate "hvc/internal/cluster"
)

// SaveLeaseUseCase 表示租约保存用例。
type SaveLeaseUseCase struct {
	leaseCache *clusterstate.LeaseCache
}

// NewSaveLeaseUseCase 创建租约保存用例。
func NewSaveLeaseUseCase(leaseCache *clusterstate.LeaseCache) *SaveLeaseUseCase {
	return &SaveLeaseUseCase{leaseCache: leaseCache}
}

// Execute 保存租约。
func (u *SaveLeaseUseCase) Execute(ctx context.Context, lease clusterstate.LeaseState) {
	u.leaseCache.Save(ctx, lease)
}
