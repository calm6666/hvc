package executor

import (
	"hvc/internal/model"
)

// ToSnapshot 把执行进度转换为任务进度快照。
func ToSnapshot(progress model.TranscodeProgress, status int) model.ProgressSnapshot {
	return model.ProgressSnapshot{
		JobID:                progress.JobID,
		Status:               status,
		Stage:                progress.Stage,
		ProgressPermille:     int(progress.Percent * 10),
		CurrentFPS:           progress.FPS,
		CurrentBitrateKbps:   progress.BitrateKbps,
		CurrentSpeed:         progress.Speed,
		ElapsedMS:            progress.TimeMS,
		EstimatedRemainingMS: 0,
	}
}
