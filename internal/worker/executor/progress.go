package executor

import (
	"hvc/internal/model"
)

func ToSnapshot(p model.TranscodeProgress, status int) model.ProgressSnapshot {
	estimatedRemainingMS := int64(0)
	if p.Speed > 0 && p.Percent < 100 {
		estimatedRemainingMS = int64(float64(p.TimeMS) / p.Speed * (100.0 - p.Percent) / p.Percent)
	}
	return model.ProgressSnapshot{
		JobID:                p.JobID,
		Status:               status,
		Stage:                p.Stage,
		ProgressPermille:     int(p.Percent * 10),
		CurrentFPS:           p.FPS,
		CurrentBitrateKbps:   p.BitrateKbps,
		CurrentSpeed:         p.Speed,
		ElapsedMS:            p.TimeMS,
		EstimatedRemainingMS: estimatedRemainingMS,
	}
}
