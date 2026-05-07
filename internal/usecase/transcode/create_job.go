package transcode

import (
	"strings"
	"time"

	"hvc/internal/callback"
	"hvc/internal/model"
	"hvc/pkg/idgen"
)

// CreateJobResult 表示创建任务结果。
type CreateJobResult struct {
	Job             model.TranscodeJob
	RequestOverride *model.TranscodeJobRequestOverride
}

// CreateJobUseCase 表示创建任务用例。
type CreateJobUseCase struct{}

// NewCreateJobUseCase 创建创建任务用例。
func NewCreateJobUseCase() *CreateJobUseCase {
	return &CreateJobUseCase{}
}

// Execute 执行创建任务。
func (u *CreateJobUseCase) Execute(req model.CreateJobRequest) (CreateJobResult, error) {
	if strings.TrimSpace(req.RequestID) == "" {
		return CreateJobResult{}, ErrRequestIDRequired
	}
	if strings.TrimSpace(req.SourceURL) == "" {
		return CreateJobResult{}, ErrSourceURLRequired
	}
	if callbackURL := strings.TrimSpace(req.CallbackURL); callbackURL != "" {
		if _, ok := callback.ParseTaskCallbackTarget(callbackURL); !ok {
			return CreateJobResult{}, ErrInvalidCallbackURL
		}
	}
	if req.VideoOptions != nil && strings.TrimSpace(req.VideoOptions.AspectFillMode) != "" {
		return CreateJobResult{}, ErrUnsupportedVideoOptions
	}
	if req.SegmentOptions != nil && req.SegmentOptions.NamingTemplateID != 0 {
		return CreateJobResult{}, ErrUnsupportedNamingTemplate
	}
	if req.StorageOptions != nil {
		if req.StorageOptions.StorageID != 0 {
			return CreateJobResult{}, ErrUnsupportedStorageID
		}
		if strings.TrimSpace(req.StorageOptions.SegmentPrefix) != "" {
			return CreateJobResult{}, ErrUnsupportedSegmentPrefix
		}
	}
	if req.ScheduleOptions != nil {
		if req.ScheduleOptions.MaxWaitSeconds > 0 {
			return CreateJobResult{}, ErrUnsupportedMaxWaitSeconds
		}
		if req.ScheduleOptions.AllowSoftwareDecodeFallback {
			return CreateJobResult{}, ErrUnsupportedSoftDecodeFlag
		}
	}

	now := time.Now()
	job := model.TranscodeJob{
		JobID:            idgen.Next(),
		RequestID:        req.RequestID,
		BizKey:           req.BizKey,
		Status:           model.JobStatusQueued,
		Priority:         req.Priority,
		SourceURL:        req.SourceURL,
		ProfileID:        req.ProfileID,
		EnableWatermark:  req.EnableWatermark,
		Renditions:       req.Renditions,
		SupportDash:      req.SegmentOptions == nil || req.SegmentOptions.SupportDash,
		SupportHLS:       req.SegmentOptions == nil || req.SegmentOptions.SupportHLS,
		OutputStorageID:  0,
		OutputBasePrefix: "",
		ProgressPermille: 0,
		ProgressStage:    model.StageQueued,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	var requestOverride *model.TranscodeJobRequestOverride
	if req.ProfileID != 0 || req.ScheduleOptions != nil || req.Watermark != nil || req.SegmentOptions != nil || req.ThumbnailOptions != nil || req.StorageOptions != nil || strings.TrimSpace(req.CallbackURL) != "" {
		requestOverride = &model.TranscodeJobRequestOverride{
			ID:        idgen.Next(),
			JobID:     job.JobID,
			CreatedAt: now,
		}
	}

	if req.SegmentOptions != nil {
		job.SegmentDurationSec = req.SegmentOptions.SegmentDurationSec
		if requestOverride != nil {
			requestOverride.OverrideSegmentDurationSec = req.SegmentOptions.SegmentDurationSec
		}
	}
	if req.StorageOptions != nil {
		job.OutputBasePrefix = req.StorageOptions.BucketPrefix
		if requestOverride != nil {
			requestOverride.OverrideBucketPrefix = req.StorageOptions.BucketPrefix
		}
	}
	if req.ScheduleOptions != nil && requestOverride != nil {
		requestOverride.OverridePreferredHWAccel = req.ScheduleOptions.PreferredHWAccel
	}
	if req.ProfileID != 0 && requestOverride != nil {
		requestOverride.OverrideProfileID = req.ProfileID
	}
	if req.EnableWatermark && requestOverride != nil {
		requestOverride.OverrideEnableWatermark = true
	}
	if req.Watermark != nil {
		job.WatermarkImageURL = req.Watermark.ImageURL
		job.WatermarkAnchor = req.Watermark.Anchor
		job.WatermarkXRatio = req.Watermark.XRatio
		job.WatermarkYRatio = req.Watermark.YRatio
		job.WatermarkWidthRatio = req.Watermark.WidthRatio
		job.WatermarkOpacity = req.Watermark.Opacity
		job.WatermarkSafeMarginRatio = req.Watermark.SafeMarginRatio
		if requestOverride != nil {
			requestOverride.OverrideWatermarkImageURL = req.Watermark.ImageURL
			requestOverride.OverrideWatermarkAnchor = req.Watermark.Anchor
			requestOverride.OverrideWatermarkXRatio = req.Watermark.XRatio
			requestOverride.OverrideWatermarkYRatio = req.Watermark.YRatio
			requestOverride.OverrideWatermarkWidthRatio = req.Watermark.WidthRatio
			requestOverride.OverrideWatermarkOpacity = req.Watermark.Opacity
		}
	}
	if req.ThumbnailOptions != nil {
		job.EnableThumbnailSprite = req.ThumbnailOptions.EnableSprite
		job.ThumbRows = req.ThumbnailOptions.SpriteRows
		job.ThumbCols = req.ThumbnailOptions.SpriteCols
		job.ThumbIntervalSec = req.ThumbnailOptions.ThumbIntervalSec
		job.ThumbWidth = req.ThumbnailOptions.ThumbWidth
		job.ThumbHeight = req.ThumbnailOptions.ThumbHeight
		job.ThumbImageFormat = req.ThumbnailOptions.SpriteImageFormat
		job.ThumbStoragePrefix = req.ThumbnailOptions.SpriteStoragePrefix
		job.EnableThumbnailBinaryIndex = req.ThumbnailOptions.EnableBinaryIndex
		job.ThumbBinaryStoragePrefix = req.ThumbnailOptions.BinaryStoragePrefix
		job.ThumbBinaryMaxSizeBytes = req.ThumbnailOptions.BinaryMaxSizeBytes
		if requestOverride != nil {
			requestOverride.OverrideEnableThumbnailSprite = req.ThumbnailOptions.EnableSprite
			requestOverride.OverrideThumbRows = req.ThumbnailOptions.SpriteRows
			requestOverride.OverrideThumbCols = req.ThumbnailOptions.SpriteCols
			requestOverride.OverrideThumbIntervalSec = req.ThumbnailOptions.ThumbIntervalSec
			requestOverride.OverrideThumbWidth = req.ThumbnailOptions.ThumbWidth
			requestOverride.OverrideThumbHeight = req.ThumbnailOptions.ThumbHeight
			requestOverride.OverrideThumbImageFormat = req.ThumbnailOptions.SpriteImageFormat
			requestOverride.OverrideThumbStoragePrefix = req.ThumbnailOptions.SpriteStoragePrefix
			requestOverride.OverrideEnableThumbnailBinaryIdx = req.ThumbnailOptions.EnableBinaryIndex
			requestOverride.OverrideThumbBinaryStoragePrefix = req.ThumbnailOptions.BinaryStoragePrefix
			requestOverride.OverrideThumbBinaryMaxSizeBytes = req.ThumbnailOptions.BinaryMaxSizeBytes
		}
	}
	if requestOverride != nil {
		requestOverride.OverrideCallbackURL = strings.TrimSpace(req.CallbackURL)
	}

	return CreateJobResult{Job: job, RequestOverride: requestOverride}, nil
}
