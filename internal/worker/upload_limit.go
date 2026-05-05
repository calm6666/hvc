package worker

import "hvc/internal/config"

// CanAcceptUpload 判断是否允许继续扩大上传并发。
func CanAcceptUpload(cfg config.RuntimeConfig, uploadQueueDepth int) bool {
	return uploadQueueDepth < cfg.Scheduler.MaxNodeUploadConcurrency
}

// EffectiveUploadConcurrency 返回单任务有效上传并发。
func EffectiveUploadConcurrency(cfg config.RuntimeConfig, uploadQueueDepth int) int {
	limit := cfg.Worker.SingleJobUploadConcurrency
	if limit <= 0 {
		return 1
	}
	if cfg.Scheduler.DynamicConcurrencyControl && uploadQueueDepth >= cfg.Scheduler.MaxNodeUploadConcurrency/2 {
		if limit == 1 {
			return 1
		}
		return limit / 2
	}
	return limit
}
