package transcode

import (
	"testing"
	"time"

	"hvc/internal/model"
	"hvc/pkg/idgen"
)

func TestCreateJobUseCase_Execute(t *testing.T) {
	idgen.Configure(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), 1, 10, 12)
	useCase := NewCreateJobUseCase()
	result, err := useCase.Execute(TestCreateJobRequest())
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if result.Job.JobID == 0 {
		t.Fatalf("job id should not be zero")
	}
	if result.Job.RequestID == "" {
		t.Fatalf("request id should not be empty")
	}
	if result.Job.Status != 2 {
		t.Fatalf("unexpected job status: %d", result.Job.Status)
	}
}

func TestCreateJobUseCase_ExecuteBuildRequestOverride(t *testing.T) {
	idgen.Configure(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), 1, 10, 12)
	useCase := NewCreateJobUseCase()
	result, err := useCase.Execute(model.CreateJobRequest{
		RequestID:       "req-override-1",
		SourceURL:       "https://example.com/video.mp4",
		ProfileID:       7,
		EnableWatermark: true,
		Watermark: &model.Watermark{
			ImageURL:   "https://example.com/wm.png",
			Anchor:     2,
			XRatio:     0.1,
			YRatio:     0.2,
			WidthRatio: 0.3,
			Opacity:    0.8,
		},
		SegmentOptions: &model.SegmentOptions{SegmentDurationSec: 6, SupportDash: true, SupportHLS: true},
		ThumbnailOptions: &model.ThumbnailOptions{EnableSprite: true, SpriteRows: 4, SpriteCols: 5},
		StorageOptions: &model.StorageOptions{StorageID: 9, BucketPrefix: "vod/", SegmentPrefix: "segments/"},
		ScheduleOptions: &model.ScheduleOptions{PreferredHWAccel: model.ExecutionHWNVIDIA},
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if result.RequestOverride == nil {
		t.Fatalf("request override should not be nil")
	}
	if result.RequestOverride.OverrideProfileID != 7 {
		t.Fatalf("unexpected override profile id: %d", result.RequestOverride.OverrideProfileID)
	}
	if result.RequestOverride.OverridePreferredHWAccel != model.ExecutionHWNVIDIA {
		t.Fatalf("unexpected override hwaccel: %s", result.RequestOverride.OverridePreferredHWAccel)
	}
	if !result.RequestOverride.OverrideEnableWatermark {
		t.Fatalf("override watermark flag should be true")
	}
}
