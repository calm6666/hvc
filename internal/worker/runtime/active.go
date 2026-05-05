package runtime

import "hvc/internal/model"

// Active 表示活跃执行会话。
type Active struct {
	Job model.TranscodeJob
}
