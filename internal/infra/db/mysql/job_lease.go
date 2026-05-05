package mysql

import (
	"context"
	"time"
)

// RenewLease 更新任务租约。
func (r *JobRepository) RenewLease(ctx context.Context, jobID uint64, workerID string, leaseGeneration uint64) error {
	return r.db.WithContext(ctx).Model(&JobRecord{}).
		Where("job_id = ? AND assigned_worker_id = ? AND lease_generation = ?", jobID, workerID, leaseGeneration).
		Updates(map[string]any{
			"updated_at": time.Now(),
		}).Error
}
