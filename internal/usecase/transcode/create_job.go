package transcode

import (
	"hvc/internal/model"
	"hvc/pkg/idgen"
	"strings"
	"time"
)

// CreateJobResult 表示创建任务结果。
type CreateJobResult struct {
	Job model.TranscodeJob
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

	now := time.Now()
	job := model.TranscodeJob{
		JobID:            idgen.Next(),
		RequestID:        req.RequestID,
		BizKey:           req.BizKey,
		Status:           model.JobStatusQueued,
		Priority:         req.Priority,
		SourceURL:        req.SourceURL,
		ProfileID:        req.ProfileID,
		SupportDash:      req.SegmentOptions == nil || req.SegmentOptions.SupportDash,
		SupportHLS:       req.SegmentOptions == nil || req.SegmentOptions.SupportHLS,
		EnableWatermark:  req.EnableWatermark,
		ProgressPermille: 0,
		ProgressStage:    model.StageQueued,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if req.SegmentOptions != nil {
		job.SegmentDurationSec = req.SegmentOptions.SegmentDurationSec
	}
	if req.StorageOptions != nil {
		job.OutputStorageID = req.StorageOptions.StorageID
		job.OutputBasePrefix = req.StorageOptions.BucketPrefix
	}
	if req.ScheduleOptions != nil {
		job.SelectedExecutionHWAccel = req.ScheduleOptions.PreferredHWAccel
	}
	if req.Watermark != nil {
		job.WatermarkImageURL = req.Watermark.ImageURL
		job.WatermarkAnchor = req.Watermark.Anchor
		job.WatermarkXRatio = req.Watermark.XRatio
		job.WatermarkYRatio = req.Watermark.YRatio
		job.WatermarkWidthRatio = req.Watermark.WidthRatio
		job.WatermarkOpacity = req.Watermark.Opacity
	}

	return CreateJobResult{Job: job}, nil
}
