package model

const (
	JobStatusCreated   = 1
	JobStatusQueued    = 2
	JobStatusAssigned  = 3
	JobStatusRunning   = 4
	JobStatusUploading = 5
	JobStatusCompleted = 6
	JobStatusFailed    = 7
	JobStatusCanceled  = 8
)

const (
	StageQueued       = "QUEUED"
	StageDownloading  = "DOWNLOADING"
	StageProbing      = "PROBING"
	StageTranscoding  = "TRANSCODING"
	StageUploading    = "UPLOADING"
	StageFinalizing   = "FINALIZING"
	StageCompleted    = "COMPLETED"
	StageFailed       = "FAILED"
)
