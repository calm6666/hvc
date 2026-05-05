package worker

import "time"

// RetryPolicy 表示上传重试策略。
type RetryPolicy struct {
	BaseDelay time.Duration
	MaxDelay  time.Duration
	MaxRetry  int
}

// NextDelay 返回下一次重试等待时长。
func (p RetryPolicy) NextDelay(retryCount int) time.Duration {
	if retryCount <= 0 {
		return p.BaseDelay
	}
	delay := p.BaseDelay << retryCount
	if delay > p.MaxDelay {
		return p.MaxDelay
	}
	return delay
}

// ShouldRetry 返回当前错误是否应继续重试。
func (p RetryPolicy) ShouldRetry(retryCount int, permanentFailure bool) bool {
	if permanentFailure {
		return false
	}
	return retryCount < p.MaxRetry
}
