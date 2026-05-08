package mysql

import (
	"context"
	"errors"
	"time"
)

var ErrLeaseRenewRejected = errors.New("lease renew rejected")

// RenewLease 更新任务租约。
func (r *JobRepository) RenewLease(ctx context.Context, jobID uint64, workerID string, leaseGeneration uint64) error {
	result := r.db.WithContext(ctx).Model(&JobRecord{}).
		Where("job_id = ? AND assigned_worker_id = ? AND lease_generation = ?", jobID, workerID, leaseGeneration).
		Updates(map[string]any{
			"updated_at": time.Now(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrLeaseRenewRejected
	}
	return nil
}
