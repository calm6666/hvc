package model

import "time"

// LeaseRenewRequest 表示任务续租请求。
type LeaseRenewRequest struct {
	JobID           uint64    `json:"job_id"`
	WorkerID        string    `json:"worker_id"`
	LeaseGeneration uint64    `json:"lease_generation"`
	ExpireAt        time.Time `json:"expire_at"`
}

// SegmentUploadFailedRequest 表示分片上传失败回传请求。
type SegmentUploadFailedRequest struct {
	SegmentID     uint64 `json:"segment_id"`
	RetryCount    int    `json:"retry_count"`
	ErrorMessage  string `json:"error_message"`
}
