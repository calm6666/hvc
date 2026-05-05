package retryx

import "time"

// Backoff 表示退避策略。
type Backoff struct {
	BaseDelay time.Duration
	MaxDelay  time.Duration
}

// Duration 返回指定次数的等待时长。
func (b Backoff) Duration(retryCount int) time.Duration {
	if retryCount <= 0 {
		return b.BaseDelay
	}
	delay := b.BaseDelay << retryCount
	if delay > b.MaxDelay {
		return b.MaxDelay
	}
	return delay
}
