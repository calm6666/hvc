package mysql

import "time"

// JobRecord 表示任务表记录。
type JobRecord struct {
	JobID                    uint64    `gorm:"column:job_id;primaryKey"`
	RequestID                string    `gorm:"column:request_id"`
	BizKey                   string    `gorm:"column:biz_key"`
	Status                   int       `gorm:"column:status"`
	Priority                 int       `gorm:"column:priority"`
	SourceURL                string    `gorm:"column:source_url"`
	ProfileID                uint64    `gorm:"column:profile_id"`
	SegmentDurationSec       int       `gorm:"column:segment_duration_sec"`
	SupportDash              bool      `gorm:"column:support_dash"`
	SupportHLS               bool      `gorm:"column:support_hls"`
	EnableWatermark          bool      `gorm:"column:enable_watermark"`
	WatermarkImageURL        string    `gorm:"column:watermark_image_url"`
	WatermarkAnchor          int       `gorm:"column:watermark_anchor"`
	WatermarkXRatio          float64   `gorm:"column:watermark_x_ratio"`
	WatermarkYRatio          float64   `gorm:"column:watermark_y_ratio"`
	WatermarkWidthRatio      float64   `gorm:"column:watermark_width_ratio"`
	WatermarkOpacity         float64   `gorm:"column:watermark_opacity"`
	OutputStorageID          uint64    `gorm:"column:output_storage_id"`
	OutputBasePrefix         string    `gorm:"column:output_base_prefix"`
	AssignedNodeID           uint64    `gorm:"column:assigned_node_id"`
	AssignedWorkerID         string    `gorm:"column:assigned_worker_id"`
	ExecutorWorkerInstanceID uint64    `gorm:"column:executor_worker_instance_id"`
	SelectedExecutionHWAccel string    `gorm:"column:selected_execution_hwaccel"`
	SelectedGPUIndex         int       `gorm:"column:selected_gpu_index"`
	SelectedGPUDeviceID      uint64    `gorm:"column:selected_gpu_device_id"`
	LeaseOwner               string    `gorm:"column:lease_owner"`
	LeaseGeneration          uint64    `gorm:"column:lease_generation"`
	AttemptNo                int       `gorm:"column:attempt_no"`
	ProgressPermille         int       `gorm:"column:progress_permille"`
	ProgressStage            string    `gorm:"column:progress_stage"`
	ErrorCode                string    `gorm:"column:error_code"`
	ErrorMessage             string    `gorm:"column:error_message"`
	CreatedAt                time.Time `gorm:"column:created_at"`
	UpdatedAt                time.Time `gorm:"column:updated_at"`
}

func (JobRecord) TableName() string { return "t_transcode_job" }

// SegmentRecord 表示分片表记录。
type SegmentRecord struct {
	SegmentID          uint64    `gorm:"column:segment_id;primaryKey"`
	JobID              uint64    `gorm:"column:job_id"`
	RenditionID        uint64    `gorm:"column:rendition_id"`
	MediaType          int       `gorm:"column:media_type"`
	IsInitSegment      bool      `gorm:"column:is_init_segment"`
	SequenceNo         int       `gorm:"column:sequence_no"`
	DurationMS         int       `gorm:"column:duration_ms"`
	SupportDash        bool      `gorm:"column:support_dash"`
	SupportHLS         bool      `gorm:"column:support_hls"`
	CodecName          string    `gorm:"column:codec_name"`
	ObjectKey          string    `gorm:"column:object_key"`
	ObjectSizeBytes    uint64    `gorm:"column:object_size_bytes"`
	ObjectETag         string    `gorm:"column:object_etag"`
	SHA256             string    `gorm:"column:sha256"`
	StartPTSMS         int64     `gorm:"column:start_pts_ms"`
	EndPTSMS           int64     `gorm:"column:end_pts_ms"`
	UploadStatus       int       `gorm:"column:upload_status"`
	UploadRetryCount   int       `gorm:"column:upload_retry_count"`
	UploadErrorMessage string    `gorm:"column:upload_error_message"`
	CreatedAt          time.Time `gorm:"column:created_at"`
	UpdatedAt          time.Time `gorm:"column:updated_at"`
}

func (SegmentRecord) TableName() string { return "t_transcode_segment" }

// OutboxRecord 表示回调事件表记录。
type OutboxRecord struct {
	EventID          uint64    `gorm:"column:event_id;primaryKey"`
	EventType        string    `gorm:"column:event_type"`
	JobID            uint64    `gorm:"column:job_id"`
	RequestID        string    `gorm:"column:request_id"`
	PayloadJSON      string    `gorm:"column:payload_json"`
	Status           int       `gorm:"column:status"`
	RetryCount       int       `gorm:"column:retry_count"`
	MaxRetryCount    int       `gorm:"column:max_retry_count"`
	NextRetryAt      time.Time `gorm:"column:next_retry_at"`
	LastErrorMessage string    `gorm:"column:last_error_message"`
	CreatedAt        time.Time `gorm:"column:created_at"`
	UpdatedAt        time.Time `gorm:"column:updated_at"`
}

func (OutboxRecord) TableName() string { return "t_event_outbox" }
