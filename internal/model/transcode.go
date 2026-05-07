package model

import "time"

// CreateJobRequest 表示创建转码任务请求。
type CreateJobRequest struct {
	RequestID        string            `json:"request_id"`
	BizKey           string            `json:"biz_key,omitempty"`
	SourceURL        string            `json:"source_url"`
	ProfileID        uint64            `json:"profile_id,omitempty"`
	Priority         int               `json:"priority,omitempty"`
	EnableWatermark  bool              `json:"enable_watermark,omitempty"`
	Watermark        *Watermark        `json:"watermark,omitempty"`
	VideoOptions     *VideoOptions     `json:"video_options,omitempty"`
	SegmentOptions   *SegmentOptions   `json:"segment_options,omitempty"`
	ThumbnailOptions *ThumbnailOptions `json:"thumbnail_options,omitempty"`
	StorageOptions   *StorageOptions   `json:"storage_options,omitempty"`
	ScheduleOptions  *ScheduleOptions  `json:"schedule_options,omitempty"`
	Renditions       []RenditionOption `json:"renditions,omitempty"`
}

// Watermark 表示水印配置。
type Watermark struct {
	ImageURL        string  `json:"image_url"`
	Anchor          int     `json:"anchor"`
	XRatio          float64 `json:"x_ratio"`
	YRatio          float64 `json:"y_ratio"`
	WidthRatio      float64 `json:"width_ratio"`
	Opacity         float64 `json:"opacity"`
	SafeMarginRatio float64 `json:"safe_margin_ratio,omitempty"`
}

// VideoOptions 表示视频输出选项。
type VideoOptions struct {
	OutputAspectKeep bool   `json:"output_aspect_keep,omitempty"`
	AspectFillMode   string `json:"aspect_fill_mode,omitempty"`
}

// SegmentOptions 表示分片配置。
type SegmentOptions struct {
	SegmentDurationSec int    `json:"segment_duration_sec,omitempty"`
	SupportDash        bool   `json:"support_dash,omitempty"`
	SupportHLS         bool   `json:"support_hls,omitempty"`
	NamingTemplateID   uint64 `json:"naming_template_id,omitempty"`
}

// ThumbnailOptions 表示缩略图配置。
type ThumbnailOptions struct {
	EnableSprite        bool   `json:"enable_sprite,omitempty"`
	SpriteRows          int    `json:"sprite_rows,omitempty"`
	SpriteCols          int    `json:"sprite_cols,omitempty"`
	ThumbIntervalSec    int    `json:"thumb_interval_sec,omitempty"`
	ThumbWidth          int    `json:"thumb_width,omitempty"`
	ThumbHeight         int    `json:"thumb_height,omitempty"`
	SpriteImageFormat   string `json:"sprite_image_format,omitempty"`
	SpriteStoragePrefix string `json:"sprite_storage_prefix,omitempty"`
	EnableBinaryIndex   bool   `json:"enable_binary_index,omitempty"`
	BinaryStoragePrefix string `json:"binary_storage_prefix,omitempty"`
	BinaryMaxSizeBytes  uint64 `json:"binary_max_size_bytes,omitempty"`
}

// StorageOptions 表示对象存储配置。
type StorageOptions struct {
	StorageID     uint64 `json:"storage_id,omitempty"`
	BucketPrefix  string `json:"bucket_prefix,omitempty"`
	SegmentPrefix string `json:"segment_prefix,omitempty"`
}

// ScheduleOptions 表示调度配置。
type ScheduleOptions struct {
	PreferredHWAccel            string `json:"preferred_hwaccel,omitempty"`
	AllowSoftwareDecodeFallback bool   `json:"allow_software_decode_fallback,omitempty"`
	MaxWaitSeconds              int    `json:"max_wait_seconds,omitempty"`
}

// RenditionOption 表示单个输出清晰度配置。
type RenditionOption struct {
	Name             string `json:"name"`
	Width            int    `json:"width"`
	Height           int    `json:"height"`
	VideoCodec       string `json:"video_codec,omitempty"`
	VideoBitrateKbps int    `json:"video_bitrate_kbps,omitempty"`
	VideoMaxrateKbps int    `json:"video_maxrate_kbps,omitempty"`
	VideoBufsizeKbps int    `json:"video_bufsize_kbps,omitempty"`
	Preset           string `json:"preset,omitempty"`
}

// CreateJobResponseData 表示创建任务响应体。
type CreateJobResponseData struct {
	JobID      uint64 `json:"job_id"`
	RequestID  string `json:"request_id"`
	Status     int    `json:"status"`
	StatusName string `json:"status_name"`
}

// TranscodeJobRequestOverride 表示单任务覆盖参数。
//
// 这层对象专门承载“请求级覆盖项”，目的是把“用户这次想怎么覆盖默认模板”
// 和“调度之后最终实际选中了什么执行模式”拆开保存：
// - override_* 表示请求意图；
// - selected_* 表示调度/执行结果。
//
// 这样后续不管是回溯请求来源、做幂等检查、还是分析调度决策偏差，
// 都不会再把两个语义混在同一个字段里。
type TranscodeJobRequestOverride struct {
	ID                               uint64
	JobID                            uint64
	OverrideProfileID                uint64
	OverrideSegmentDurationSec       int
	OverridePreferredHWAccel         string
	OverrideEnableWatermark          bool
	OverrideWatermarkImageURL        string
	OverrideWatermarkAnchor          int
	OverrideWatermarkXRatio          float64
	OverrideWatermarkYRatio          float64
	OverrideWatermarkWidthRatio      float64
	OverrideWatermarkOpacity         float64
	OverrideEnableThumbnailSprite    bool
	OverrideThumbRows                int
	OverrideThumbCols                int
	OverrideThumbIntervalSec         int
	OverrideThumbWidth               int
	OverrideThumbHeight              int
	OverrideThumbImageFormat         string
	OverrideThumbStoragePrefix       string
	OverrideEnableThumbnailBinaryIdx bool
	OverrideThumbBinaryStoragePrefix string
	OverrideThumbBinaryMaxSizeBytes  uint64
	OverrideBucketPrefix             string
	OverrideSegmentPrefix            string
	CreatedAt                        time.Time
}

// ProgressSnapshot 表示任务进度快照。
type ProgressSnapshot struct {
	JobID                uint64    `json:"job_id"`
	Status               int       `json:"status"`
	Stage                string    `json:"stage"`
	ProgressPermille     int       `json:"progress_permille"`
	CurrentFPS           float64   `json:"current_fps,omitempty"`
	CurrentBitrateKbps   float64   `json:"current_bitrate_kbps,omitempty"`
	CurrentSpeed         float64   `json:"current_speed,omitempty"`
	ElapsedMS            int64     `json:"elapsed_ms,omitempty"`
	EstimatedRemainingMS int64     `json:"estimated_remaining_ms,omitempty"`
	UpdatedAt            time.Time `json:"updated_at,omitempty"`
}

// TranscodeJob 表示转码任务。
type TranscodeJob struct {
	JobID                    uint64
	RequestID                string
	BizKey                   string
	Status                   int
	Priority                 int
	SourceURL                string
	ProfileID                uint64
	SegmentDurationSec       int
	SupportDash              bool
	SupportHLS               bool
	EnableWatermark          bool
	WatermarkImageURL        string
	WatermarkAnchor          int
	WatermarkXRatio          float64
	WatermarkYRatio          float64
	WatermarkWidthRatio      float64
	WatermarkOpacity         float64
	WatermarkSafeMarginRatio float64
	EnableThumbnailSprite    bool
	ThumbRows                int
	ThumbCols                int
	ThumbIntervalSec         int
	ThumbWidth               int
	ThumbHeight              int
	ThumbImageFormat         string
	ThumbStoragePrefix       string
	EnableThumbnailBinaryIndex bool
	ThumbBinaryStoragePrefix string
	ThumbBinaryMaxSizeBytes  uint64
	Renditions               []RenditionOption
	OutputStorageID          uint64
	OutputBasePrefix         string
	AssignedNodeID           uint64
	AssignedWorkerID         string
	ExecutorWorkerInstanceID uint64
	SelectedExecutionHWAccel string
	SelectedGPUIndex         int
	SelectedGPUDeviceID      uint64
	LeaseOwner               string
	LeaseGeneration          uint64
	AttemptNo                int
	ProgressPermille         int
	ProgressStage            string
	ErrorCode                string
	ErrorMessage             string
	CreatedAt                time.Time
	UpdatedAt                time.Time
}
