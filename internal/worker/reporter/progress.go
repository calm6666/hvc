package reporter

import (
	"time"

	"hvc/internal/model"
)

type ProgressEvent struct {
	JobID            uint64  `json:"job_id"`
	Status           int     `json:"status"`
	Stage            string  `json:"stage"`
	ProgressPermille int     `json:"progress_permille"`
	CurrentFPS       float64 `json:"current_fps,omitempty"`
	CurrentBitrate   float64 `json:"current_bitrate_kbps,omitempty"`
	CurrentSpeed     float64 `json:"current_speed,omitempty"`
	ElapsedMS        int64   `json:"elapsed_ms,omitempty"`
	EstimatedRemain  int64   `json:"estimated_remaining_ms,omitempty"`
	Timestamp        int64   `json:"timestamp"`
}

func NewProgressEvent(snapshot model.ProgressSnapshot) ProgressEvent {
	return ProgressEvent{
		JobID:            snapshot.JobID,
		Status:           snapshot.Status,
		Stage:            snapshot.Stage,
		ProgressPermille: snapshot.ProgressPermille,
		CurrentFPS:       snapshot.CurrentFPS,
		CurrentBitrate:   snapshot.CurrentBitrateKbps,
		CurrentSpeed:     snapshot.CurrentSpeed,
		ElapsedMS:        snapshot.ElapsedMS,
		EstimatedRemain:  snapshot.EstimatedRemainingMS,
		Timestamp:        time.Now().UnixMilli(),
	}
}
