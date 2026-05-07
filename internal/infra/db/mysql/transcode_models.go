package mysql

import "time"

// TranscodeJobRequestOverrideRecord 表示单任务覆盖参数表映射。
type TranscodeJobRequestOverrideRecord struct {
	ID                               uint64    `gorm:"column:id;primaryKey"`
	JobID                            uint64    `gorm:"column:job_id"`
	OverrideProfileID                uint64    `gorm:"column:override_profile_id"`
	OverrideSegmentDurationSec       int       `gorm:"column:override_segment_duration_sec"`
	OverridePreferredHWAccel         string    `gorm:"column:override_preferred_hwaccel"`
	OverrideEnableWatermark          bool      `gorm:"column:override_enable_watermark"`
	OverrideWatermarkImageURL        string    `gorm:"column:override_watermark_image_url"`
	OverrideWatermarkAnchor          int       `gorm:"column:override_watermark_anchor"`
	OverrideWatermarkXRatio          float64   `gorm:"column:override_watermark_x_ratio"`
	OverrideWatermarkYRatio          float64   `gorm:"column:override_watermark_y_ratio"`
	OverrideWatermarkWidthRatio      float64   `gorm:"column:override_watermark_width_ratio"`
	OverrideWatermarkOpacity         float64   `gorm:"column:override_watermark_opacity"`
	OverrideEnableThumbnailSprite    bool      `gorm:"column:override_enable_thumbnail_sprite"`
	OverrideThumbRows                int       `gorm:"column:override_thumb_rows"`
	OverrideThumbCols                int       `gorm:"column:override_thumb_cols"`
	OverrideThumbIntervalSec         int       `gorm:"column:override_thumb_interval_sec"`
	OverrideThumbWidth               int       `gorm:"column:override_thumb_width"`
	OverrideThumbHeight              int       `gorm:"column:override_thumb_height"`
	OverrideThumbImageFormat         string    `gorm:"column:override_thumb_image_format"`
	OverrideThumbStoragePrefix       string    `gorm:"column:override_thumb_storage_prefix"`
	OverrideEnableThumbnailBinaryIdx bool      `gorm:"column:override_enable_thumbnail_binary_index"`
	OverrideThumbBinaryStoragePrefix string    `gorm:"column:override_thumb_binary_storage_prefix"`
	OverrideThumbBinaryMaxSizeBytes  uint64    `gorm:"column:override_thumb_binary_max_size_bytes"`
	OverrideBucketPrefix             string    `gorm:"column:override_bucket_prefix"`
	OverrideSegmentPrefix            string    `gorm:"column:override_segment_prefix"`
	OverrideCallbackURL              string    `gorm:"column:override_callback_url"`
	CreatedAt                        time.Time `gorm:"column:created_at"`
}

func (TranscodeJobRequestOverrideRecord) TableName() string {
	return "t_transcode_job_request_override"
}

// TranscodeProfileRecord 表示转码模板主表映射。
type TranscodeProfileRecord struct {
	ProfileID          uint64    `gorm:"column:profile_id;primaryKey"`
	ProfileName        string    `gorm:"column:profile_name"`
	BizCode            string    `gorm:"column:biz_code"`
	ContainerFormat    string    `gorm:"column:container_format"`
	SegmentDurationSec int       `gorm:"column:segment_duration_sec"`
	VideoCodec         string    `gorm:"column:video_codec"`
	AudioCodec         string    `gorm:"column:audio_codec"`
	Enabled            bool      `gorm:"column:enabled"`
	CreatedAt          time.Time `gorm:"column:created_at"`
	UpdatedAt          time.Time `gorm:"column:updated_at"`
}

func (TranscodeProfileRecord) TableName() string { return "t_transcode_profile" }

// TranscodeProfileRenditionRecord 表示转码模板清晰度档位表映射。
type TranscodeProfileRenditionRecord struct {
	ProfileRenditionID uint64    `gorm:"column:profile_rendition_id;primaryKey"`
	ProfileID          uint64    `gorm:"column:profile_id"`
	RenditionName      string    `gorm:"column:rendition_name"`
	Enabled            bool      `gorm:"column:enabled"`
	OutWidth           int       `gorm:"column:out_width"`
	OutHeight          int       `gorm:"column:out_height"`
	VideoBitrateKbps   int       `gorm:"column:video_bitrate_kbps"`
	AudioBitrateKbps   int       `gorm:"column:audio_bitrate_kbps"`
	FPS                float64   `gorm:"column:fps"`
	CreatedAt          time.Time `gorm:"column:created_at"`
	UpdatedAt          time.Time `gorm:"column:updated_at"`
}

func (TranscodeProfileRenditionRecord) TableName() string { return "t_transcode_profile_rendition" }

// StorageConfigRecord 表示对象存储配置表映射。
type StorageConfigRecord struct {
	StorageID       uint64    `gorm:"column:storage_id;primaryKey"`
	StorageName     string    `gorm:"column:storage_name"`
	ProviderType    string    `gorm:"column:provider_type"`
	Endpoint        string    `gorm:"column:endpoint"`
	BucketName      string    `gorm:"column:bucket_name"`
	RegionName      string    `gorm:"column:region_name"`
	AccessKeyID     string    `gorm:"column:access_key_id"`
	SecretAccessKey string    `gorm:"column:secret_access_key"`
	BasePrefix      string    `gorm:"column:base_prefix"`
	Enabled         bool      `gorm:"column:enabled"`
	Priority        int       `gorm:"column:priority"`
	CreatedAt       time.Time `gorm:"column:created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at"`
}

func (StorageConfigRecord) TableName() string { return "t_storage_config" }

// TranscodeRenditionRecord 表示任务清晰度子任务表映射。
type TranscodeRenditionRecord struct {
	RenditionID       uint64    `gorm:"column:rendition_id;primaryKey"`
	JobID             uint64    `gorm:"column:job_id"`
	RenditionName     string    `gorm:"column:rendition_name"`
	Status            int       `gorm:"column:status"`
	OutWidth          int       `gorm:"column:out_width"`
	OutHeight         int       `gorm:"column:out_height"`
	VideoCodec        string    `gorm:"column:video_codec"`
	AudioCodec        string    `gorm:"column:audio_codec"`
	VideoBitrateKbps  int       `gorm:"column:video_bitrate_kbps"`
	AudioBitrateKbps  int       `gorm:"column:audio_bitrate_kbps"`
	SegmentCountVideo int       `gorm:"column:segment_count_video"`
	SegmentCountAudio int       `gorm:"column:segment_count_audio"`
	ProgressPermille  int       `gorm:"column:progress_permille"`
	ErrorCode         string    `gorm:"column:error_code"`
	ErrorMessage      string    `gorm:"column:error_message"`
	CreatedAt         time.Time `gorm:"column:created_at"`
	UpdatedAt         time.Time `gorm:"column:updated_at"`
}

func (TranscodeRenditionRecord) TableName() string { return "t_transcode_rendition" }

// TranscodeThumbnailSpriteRecord 表示任务雪碧图表映射。
type TranscodeThumbnailSpriteRecord struct {
	SpriteID           uint64    `gorm:"column:sprite_id;primaryKey"`
	JobID              uint64    `gorm:"column:job_id"`
	SpriteNo           int       `gorm:"column:sprite_no"`
	RowsCount          int       `gorm:"column:rows_count"`
	ColsCount          int       `gorm:"column:cols_count"`
	ThumbCount         int       `gorm:"column:thumb_count"`
	ThumbWidth         int       `gorm:"column:thumb_width"`
	ThumbHeight        int       `gorm:"column:thumb_height"`
	ImageFormat        string    `gorm:"column:image_format"`
	ObjectKey          string    `gorm:"column:object_key"`
	ObjectSizeBytes    uint64    `gorm:"column:object_size_bytes"`
	UploadStatus       int       `gorm:"column:upload_status"`
	UploadErrorMessage string    `gorm:"column:upload_error_message"`
	CreatedAt          time.Time `gorm:"column:created_at"`
	UpdatedAt          time.Time `gorm:"column:updated_at"`
}

func (TranscodeThumbnailSpriteRecord) TableName() string { return "t_transcode_thumbnail_sprite" }

// TranscodeThumbnailBinRecord 表示缩略图二进制索引包表映射。
type TranscodeThumbnailBinRecord struct {
	BinID           uint64    `gorm:"column:bin_id;primaryKey"`
	JobID           uint64    `gorm:"column:job_id"`
	BinNo           int       `gorm:"column:bin_no"`
	ItemCount       int       `gorm:"column:item_count"`
	MaxSizeBytes    uint64    `gorm:"column:max_size_bytes"`
	ActualSizeBytes uint64    `gorm:"column:actual_size_bytes"`
	ObjectKey       string    `gorm:"column:object_key"`
	CreatedAt       time.Time `gorm:"column:created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at"`
}

func (TranscodeThumbnailBinRecord) TableName() string { return "t_transcode_thumbnail_bin" }

// TranscodeThumbnailItemRecord 表示缩略图索引项表映射。
type TranscodeThumbnailItemRecord struct {
	ItemID         uint64    `gorm:"column:item_id;primaryKey"`
	JobID          uint64    `gorm:"column:job_id"`
	ItemIndex      int       `gorm:"column:item_index"`
	CaptureTimeMS  uint64    `gorm:"column:capture_time_ms"`
	SpriteNo       int       `gorm:"column:sprite_no"`
	SpriteRowIndex int       `gorm:"column:sprite_row_index"`
	SpriteColIndex int       `gorm:"column:sprite_col_index"`
	BinNo          int       `gorm:"column:bin_no"`
	BinItemIndex   int       `gorm:"column:bin_item_index"`
	Base64Length   int       `gorm:"column:base64_length"`
	ThumbSHA256    string    `gorm:"column:thumb_sha256"`
	CreatedAt      time.Time `gorm:"column:created_at"`
	UpdatedAt      time.Time `gorm:"column:updated_at"`
}

func (TranscodeThumbnailItemRecord) TableName() string { return "t_transcode_thumbnail_item" }
