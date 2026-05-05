package model

import "time"

// UploadTask 表示待上传分片任务。
type UploadTask struct {
	SegmentID   uint64    `json:"segment_id"`
	JobID       uint64    `json:"job_id"`
	RenditionID uint64    `json:"rendition_id"`
	ObjectKey   string    `json:"object_key"`
	LocalPath   string    `json:"local_path"`
	RetryCount  int       `json:"retry_count"`
	CreatedAt   time.Time `json:"created_at"`
}

// UploadResult 表示上传结果。
type UploadResult struct {
	SegmentID       uint64 `json:"segment_id"`
	Success         bool   `json:"success"`
	ObjectETag      string `json:"object_etag,omitempty"`
	ObjectSizeBytes uint64 `json:"object_size_bytes,omitempty"`
	ErrorMessage    string `json:"error_message,omitempty"`
}
