package failover

import (
	"time"

	"hvc/internal/cluster"
)

// ShouldTakeOver 判断是否应触发任务接管。
func ShouldTakeOver(lease cluster.LeaseState, now time.Time) bool {
	return !lease.ExpireAt.IsZero() && !lease.ExpireAt.After(now)
}
