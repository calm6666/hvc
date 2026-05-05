package segmenter

import "hvc/internal/model"

// Discover 发现分片结果。
func Discover(job model.TranscodeJob) Result {
	_ = job
	return Result{Count: 1}
}
