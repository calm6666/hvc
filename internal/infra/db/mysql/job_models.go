package mysql

import "time"

// JobRecord 表示任务表记录。
type JobRecord struct {
	JobID                      uint64     `gorm:"column:job_id;primaryKey"`
	RequestID                  string     `gorm:"column:request_id"`
	BizKey                     string     `gorm:"column:biz_key"`
	Mode                       int        `gorm:"column:mode"`
	CompletionPayload          string     `gorm:"column:completion_payload"` // A1: 完成+回调 payload 暂存（非空=还没传完、尚未发布）
	Status                     int        `gorm:"column:status"`
	Priority                   int        `gorm:"column:priority"`
	SourceURL                  string     `gorm:"column:source_url"`
	SourceProtocol             int        `gorm:"column:source_protocol"`
	ProfileID                  uint64     `gorm:"column:profile_id"`
	JobConfigVersion           uint64     `gorm:"column:job_config_version"`
	SegmentDurationSec         int        `gorm:"column:segment_duration_sec"`
	SegmentTemplate            string     `gorm:"column:segment_template"`
	SupportDash                bool       `gorm:"column:support_dash"`
	SupportHLS                 bool       `gorm:"column:support_hls"`
	EnableWatermark            bool       `gorm:"column:enable_watermark"`
	WatermarkImageURL          string     `gorm:"column:watermark_image_url"`
	WatermarkAnchor            int        `gorm:"column:watermark_anchor"`
	WatermarkXRatio            float64    `gorm:"column:watermark_x_ratio"`
	WatermarkYRatio            float64    `gorm:"column:watermark_y_ratio"`
	WatermarkWidthRatio        float64    `gorm:"column:watermark_width_ratio"`
	WatermarkOpacity           float64    `gorm:"column:watermark_opacity"`
	WatermarkSafeMarginRatio   float64    `gorm:"column:watermark_safe_margin_ratio"`
	EnableThumbnailSprite      bool       `gorm:"column:enable_thumbnail_sprite"`
	ThumbRows                  int        `gorm:"column:thumb_rows"`
	ThumbCols                  int        `gorm:"column:thumb_cols"`
	ThumbIntervalSec           int        `gorm:"column:thumb_interval_sec"`
	ThumbWidth                 int        `gorm:"column:thumb_width"`
	ThumbHeight                int        `gorm:"column:thumb_height"`
	ThumbImageFormat           string     `gorm:"column:thumb_image_format"`
	ThumbStoragePrefix         string     `gorm:"column:thumb_storage_prefix"`
	EnableThumbnailBinaryIndex bool       `gorm:"column:enable_thumbnail_binary_index"`
	ThumbBinaryStoragePrefix   string     `gorm:"column:thumb_binary_storage_prefix"`
	ThumbBinaryMaxSizeBytes    uint64     `gorm:"column:thumb_binary_max_size_bytes"`
	RenditionsJSON             string     `gorm:"column:renditions_json;type:text"`
	OutputStorageID            uint64     `gorm:"column:output_storage_id"`
	OutputBasePrefix           string     `gorm:"column:output_base_prefix"`
	AssignedNodeID             uint64     `gorm:"column:assigned_node_id"`
	AssignedWorkerID           string     `gorm:"column:assigned_worker_id"`
	ExecutorWorkerInstanceID   uint64     `gorm:"column:executor_worker_instance_id"`
	SelectedExecutionHWAccel   string     `gorm:"column:selected_execution_hwaccel"`
	SelectedGPUIndex           int        `gorm:"column:selected_gpu_index"`
	SelectedGPUDeviceID        uint64     `gorm:"column:selected_gpu_device_id"`
	LeaseOwner                 string     `gorm:"column:lease_owner"`
	LeaseGeneration            uint64     `gorm:"column:lease_generation"`
	AttemptNo                  int        `gorm:"column:attempt_no"`
	LeaseExpireAt              *time.Time `gorm:"column:lease_expire_at"`
	// LastWorkerHeartbeatAt 为历史兼容字段。
	//
	// 当前任务执行链路的真实心跳权威来源已经切换到
	// `t_transcode_job_execution.last_heartbeat_at`，这里保留字段只是为了兼容既有表结构
	// 和旧数据迁移，不应再作为新的失联判定依据。
	LastWorkerHeartbeatAt *time.Time `gorm:"column:last_worker_heartbeat_at"`
	ProgressPermille      int        `gorm:"column:progress_permille"`
	ProgressStage         string     `gorm:"column:progress_stage"`
	LastRetryMode         string     `gorm:"column:last_retry_mode"`
	ErrorCode             string     `gorm:"column:error_code"`
	ErrorMessage          string     `gorm:"column:error_message"`
	CreatedAt             time.Time  `gorm:"column:created_at"`
	UpdatedAt             time.Time  `gorm:"column:updated_at"`
}

func (JobRecord) TableName() string { return "t_transcode_job" }

// SegmentRecord 表示分片表记录。
type SegmentRecord struct {
	SegmentID          uint64    `gorm:"column:segment_id;primaryKey"`
	JobID              uint64    `gorm:"column:job_id"`
	RenditionID        uint64    `gorm:"column:rendition_id"`
	RenditionName      string    `gorm:"column:rendition_name"`
	RenditionKey       string    `gorm:"column:rendition_key"`
	MediaType          int       `gorm:"column:media_type"`
	IsInitSegment      bool      `gorm:"column:is_init_segment"`
	SequenceNo         int       `gorm:"column:sequence_no"`
	DurationMS         int       `gorm:"column:duration_ms"`
	Width              int       `gorm:"column:width"`
	Height             int       `gorm:"column:height"`
	VideoBitrateKbps   int       `gorm:"column:video_bitrate_kbps"`
	AudioBitrateKbps   int       `gorm:"column:audio_bitrate_kbps"`
	VideoCodec         string    `gorm:"column:video_codec"`
	AudioCodec         string    `gorm:"column:audio_codec"`
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
