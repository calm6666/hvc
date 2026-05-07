// Package dto 提供 HTTP 接口的数据传输对象定义。
package dto

import "hvc/internal/model"

// CreateJobRequest 创建转码任务请求 DTO。
type CreateJobRequest struct {
	RequestID        string               `json:"request_id" binding:"required"`
	BizKey           string               `json:"biz_key"`
	SourceURL        string               `json:"source_url" binding:"required"`
	ProfileID        uint64               `json:"profile_id"`
	Priority         int                  `json:"priority"`
	EnableWatermark  bool                 `json:"enable_watermark"`
	Watermark        *WatermarkDTO        `json:"watermark,omitempty"`
	VideoOptions     *VideoOptionsDTO     `json:"video_options,omitempty"`
	SegmentOptions   *SegmentOptionsDTO   `json:"segment_options,omitempty"`
	ThumbnailOptions *ThumbnailOptionsDTO `json:"thumbnail_options,omitempty"`
	StorageOptions   *StorageOptionsDTO   `json:"storage_options,omitempty"`
	ScheduleOptions  *ScheduleOptionsDTO  `json:"schedule_options,omitempty"`
	Renditions       []RenditionOptionDTO `json:"renditions,omitempty"`
	CallbackURL      string               `json:"callback_url"`
}

// WatermarkDTO 水印配置 DTO。
type WatermarkDTO struct {
	ImageURL        string  `json:"image_url"`
	Anchor          int     `json:"anchor"`
	XRatio          float64 `json:"x_ratio"`
	YRatio          float64 `json:"y_ratio"`
	WidthRatio      float64 `json:"width_ratio"`
	Opacity         float64 `json:"opacity"`
	SafeMarginRatio float64 `json:"safe_margin_ratio,omitempty"`
}

// VideoOptionsDTO 视频输出选项 DTO。
type VideoOptionsDTO struct {
	OutputAspectKeep bool   `json:"output_aspect_keep"`
	AspectFillMode   string `json:"aspect_fill_mode"`
}

// SegmentOptionsDTO 分片配置 DTO。
type SegmentOptionsDTO struct {
	SegmentDurationSec int    `json:"segment_duration_sec"`
	SupportDash        bool   `json:"support_dash"`
	SupportHLS         bool   `json:"support_hls"`
	NamingTemplateID   uint64 `json:"naming_template_id"`
}

// ThumbnailOptionsDTO 缩略图配置 DTO。
type ThumbnailOptionsDTO struct {
	EnableSprite        bool   `json:"enable_sprite"`
	SpriteRows          int    `json:"sprite_rows"`
	SpriteCols          int    `json:"sprite_cols"`
	ThumbIntervalSec    int    `json:"thumb_interval_sec"`
	ThumbWidth          int    `json:"thumb_width"`
	ThumbHeight         int    `json:"thumb_height"`
	SpriteImageFormat   string `json:"sprite_image_format"`
	SpriteStoragePrefix string `json:"sprite_storage_prefix"`
	EnableBinaryIndex   bool   `json:"enable_binary_index"`
	BinaryStoragePrefix string `json:"binary_storage_prefix"`
	BinaryMaxSizeBytes  uint64 `json:"binary_max_size_bytes"`
}

// StorageOptionsDTO 对象存储配置 DTO。
type StorageOptionsDTO struct {
	StorageID     uint64 `json:"storage_id"`
	BucketPrefix  string `json:"bucket_prefix"`
	SegmentPrefix string `json:"segment_prefix"`
}

// ScheduleOptionsDTO 调度配置 DTO。
type ScheduleOptionsDTO struct {
	PreferredHWAccel            string `json:"preferred_hwaccel"`
	AllowSoftwareDecodeFallback bool   `json:"allow_software_decode_fallback"`
	MaxWaitSeconds              int    `json:"max_wait_seconds"`
}

// RenditionOptionDTO 输出清晰度配置 DTO。
type RenditionOptionDTO struct {
	Name             string `json:"name"`
	Width            int    `json:"width"`
	Height           int    `json:"height"`
	VideoCodec       string `json:"video_codec"`
	VideoBitrateKbps int    `json:"video_bitrate_kbps"`
	VideoMaxrateKbps int    `json:"video_maxrate_kbps"`
	VideoBufsizeKbps int    `json:"video_bufsize_kbps"`
	Preset           string `json:"preset"`
}

// ToModel 将 DTO 转换为领域模型。
func (r *CreateJobRequest) ToModel() model.CreateJobRequest {
	req := model.CreateJobRequest{
		RequestID:       r.RequestID,
		BizKey:          r.BizKey,
		SourceURL:       r.SourceURL,
		ProfileID:       r.ProfileID,
		Priority:        r.Priority,
		EnableWatermark: r.EnableWatermark,
		CallbackURL:     r.CallbackURL,
	}

	if r.Watermark != nil {
		req.Watermark = &model.Watermark{
			ImageURL:        r.Watermark.ImageURL,
			Anchor:          r.Watermark.Anchor,
			XRatio:          r.Watermark.XRatio,
			YRatio:          r.Watermark.YRatio,
			WidthRatio:      r.Watermark.WidthRatio,
			Opacity:         r.Watermark.Opacity,
			SafeMarginRatio: r.Watermark.SafeMarginRatio,
		}
	}

	if r.VideoOptions != nil {
		req.VideoOptions = &model.VideoOptions{
			OutputAspectKeep: r.VideoOptions.OutputAspectKeep,
			AspectFillMode:   r.VideoOptions.AspectFillMode,
		}
	}

	if r.SegmentOptions != nil {
		req.SegmentOptions = &model.SegmentOptions{
			SegmentDurationSec: r.SegmentOptions.SegmentDurationSec,
			SupportDash:        r.SegmentOptions.SupportDash,
			SupportHLS:         r.SegmentOptions.SupportHLS,
			NamingTemplateID:   r.SegmentOptions.NamingTemplateID,
		}
	}

	if r.ThumbnailOptions != nil {
		req.ThumbnailOptions = &model.ThumbnailOptions{
			EnableSprite:        r.ThumbnailOptions.EnableSprite,
			SpriteRows:          r.ThumbnailOptions.SpriteRows,
			SpriteCols:          r.ThumbnailOptions.SpriteCols,
			ThumbIntervalSec:    r.ThumbnailOptions.ThumbIntervalSec,
			ThumbWidth:          r.ThumbnailOptions.ThumbWidth,
			ThumbHeight:         r.ThumbnailOptions.ThumbHeight,
			SpriteImageFormat:   r.ThumbnailOptions.SpriteImageFormat,
			SpriteStoragePrefix: r.ThumbnailOptions.SpriteStoragePrefix,
			EnableBinaryIndex:   r.ThumbnailOptions.EnableBinaryIndex,
			BinaryStoragePrefix: r.ThumbnailOptions.BinaryStoragePrefix,
			BinaryMaxSizeBytes:  r.ThumbnailOptions.BinaryMaxSizeBytes,
		}
	}

	if r.StorageOptions != nil {
		req.StorageOptions = &model.StorageOptions{
			StorageID:     r.StorageOptions.StorageID,
			BucketPrefix:  r.StorageOptions.BucketPrefix,
			SegmentPrefix: r.StorageOptions.SegmentPrefix,
		}
	}

	if r.ScheduleOptions != nil {
		req.ScheduleOptions = &model.ScheduleOptions{
			PreferredHWAccel:            r.ScheduleOptions.PreferredHWAccel,
			AllowSoftwareDecodeFallback: r.ScheduleOptions.AllowSoftwareDecodeFallback,
			MaxWaitSeconds:              r.ScheduleOptions.MaxWaitSeconds,
		}
	}

	if len(r.Renditions) > 0 {
		req.Renditions = make([]model.RenditionOption, 0, len(r.Renditions))
		for _, r := range r.Renditions {
			req.Renditions = append(req.Renditions, model.RenditionOption{
				Name:             r.Name,
				Width:            r.Width,
				Height:           r.Height,
				VideoCodec:       r.VideoCodec,
				VideoBitrateKbps: r.VideoBitrateKbps,
				VideoMaxrateKbps: r.VideoMaxrateKbps,
				VideoBufsizeKbps: r.VideoBufsizeKbps,
				Preset:           r.Preset,
			})
		}
	}

	return req
}

// CreateJobResponse 创建转码任务响应 DTO。
type CreateJobResponse = model.Response

// ProgressQueryRequest 进度查询请求 DTO。
type ProgressQueryRequest struct {
	RequestID string `form:"request_id" json:"request_id"`
	JobID     uint64 `form:"job_id" json:"job_id"`
}

// ProgressResponse 进度查询响应 DTO。
type ProgressResponse struct {
	JobID                uint64  `json:"job_id"`
	RequestID            string  `json:"request_id"`
	Status               int     `json:"status"`
	StatusName           string  `json:"status_name"`
	Stage                string  `json:"stage"`
	ProgressPermille     int     `json:"progress_permille"`
	CurrentFPS           float64 `json:"current_fps,omitempty"`
	CurrentBitrateKbps   float64 `json:"current_bitrate_kbps,omitempty"`
	CurrentSpeed         float64 `json:"current_speed,omitempty"`
	ElapsedMS            int64   `json:"elapsed_ms,omitempty"`
	EstimatedRemainingMS int64   `json:"estimated_remaining_ms,omitempty"`
}

// JobListRequest 任务列表请求 DTO。
type JobListRequest struct {
	Page     int    `form:"page" json:"page"`
	PageSize int    `form:"page_size" json:"page_size"`
	Status   int    `form:"status" json:"status"`
	BizKey   string `form:"biz_key" json:"biz_key"`
}

// JobListResponse 任务列表响应 DTO。
type JobListResponse struct {
	Total int              `json:"total"`
	Items []JobListItemDTO `json:"items"`
}

// JobListItemDTO 任务列表项 DTO。
type JobListItemDTO struct {
	JobID            uint64 `json:"job_id"`
	RequestID        string `json:"request_id"`
	BizKey           string `json:"biz_key"`
	SourceURL        string `json:"source_url"`
	Status           int    `json:"status"`
	StatusName       string `json:"status_name"`
	ProgressPermille int    `json:"progress_permille"`
	Stage            string `json:"stage"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}

// RetryJobRequest 重试任务请求 DTO。
type RetryJobRequest struct {
	JobID uint64 `json:"job_id" binding:"required"`
}

// JobDetailResponse 任务详情响应 DTO。
type JobDetailResponse struct {
	JobID            uint64 `json:"job_id"`
	RequestID        string `json:"request_id"`
	BizKey           string `json:"biz_key"`
	SourceURL        string `json:"source_url"`
	Status           int    `json:"status"`
	StatusName       string `json:"status_name"`
	ProgressPermille int    `json:"progress_permille"`
	Stage            string `json:"stage"`
	ProfileID        uint64 `json:"profile_id"`
	EnableWatermark  bool   `json:"enable_watermark"`
	SegmentDuration  int    `json:"segment_duration_sec"`
	SupportDash      bool   `json:"support_dash"`
	SupportHLS       bool   `json:"support_hls"`
	AssignedNodeID   uint64 `json:"assigned_node_id"`
	AssignedWorkerID string `json:"assigned_worker_id"`
	CreatedAt        string `json:"created_at"`
	UpdatedAt        string `json:"updated_at"`
}
