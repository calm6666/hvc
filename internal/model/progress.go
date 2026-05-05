package model

import "time"

// TranscodeProgress 表示转码进度。
type TranscodeProgress struct {
	JobID         uint64    `json:"job_id"`
	RenditionID   uint64    `json:"rendition_id,omitempty"`
	Stage         string    `json:"stage"`
	Frame         int64     `json:"frame,omitempty"`
	FPS           float64   `json:"fps,omitempty"`
	BitrateKbps   float64   `json:"bitrate_kbps,omitempty"`
	TimeMS        int64     `json:"time_ms,omitempty"`
	Speed         float64   `json:"speed,omitempty"`
	Percent       float64   `json:"percent,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
}
