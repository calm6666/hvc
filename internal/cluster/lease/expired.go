package lease

import "hvc/internal/cluster"

// Expired 判断租约是否已过期。
func Expired(lease cluster.LeaseState, nowUnixMilli int64) bool {
	return lease.ExpireAt.UnixMilli() <= nowUnixMilli
}
