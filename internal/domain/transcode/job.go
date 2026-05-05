package transcode

import "time"

// Job 表示转码任务领域对象。
type Job struct {
	JobID                    uint64
	RequestID                string
	SourceURL                string
	Status                   int
	Priority                 int
	ProfileID                uint64
	AssignedNodeID           uint64
	AssignedWorkerID         string
	SelectedExecutionHWAccel string
	SelectedGPUIndex         int
	ProgressPermille         int
	ProgressStage            string
	CreatedAt                time.Time
	UpdatedAt                time.Time
}
