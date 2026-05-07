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
			ImageURL:        "https://example.com/wm.png",
			Anchor:          2,
			XRatio:          0.1,
			YRatio:          0.2,
			WidthRatio:      0.3,
			Opacity:         0.8,
			SafeMarginRatio: 0.05,
		},
		SegmentOptions:   &model.SegmentOptions{SegmentDurationSec: 6, SupportDash: true, SupportHLS: true},
		ThumbnailOptions: &model.ThumbnailOptions{EnableSprite: true, SpriteRows: 4, SpriteCols: 5, ThumbIntervalSec: 8, ThumbWidth: 320, ThumbHeight: 180, SpriteImageFormat: "jpg", SpriteStoragePrefix: "thumbs/"},
		StorageOptions:   &model.StorageOptions{BucketPrefix: "vod/"},
		ScheduleOptions:  &model.ScheduleOptions{PreferredHWAccel: model.ExecutionHWNVIDIA},
		Renditions: []model.RenditionOption{
			{Name: "720p", Width: 1280, Height: 720, VideoBitrateKbps: 2800},
		},
		CallbackURL: "https://callback.example.com/task/req-override-1",
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
	if result.RequestOverride.OverrideCallbackURL != "https://callback.example.com/task/req-override-1" {
		t.Fatalf("unexpected override callback url: %s", result.RequestOverride.OverrideCallbackURL)
	}
	if len(result.Job.Renditions) != 1 || result.Job.Renditions[0].Name != "720p" {
		t.Fatalf("renditions should be written into job")
	}
	if result.Job.WatermarkSafeMarginRatio != 0.05 {
		t.Fatalf("unexpected watermark safe margin: %v", result.Job.WatermarkSafeMarginRatio)
	}
	if !result.Job.EnableThumbnailSprite || result.Job.ThumbRows != 4 || result.Job.ThumbCols != 5 {
		t.Fatalf("thumbnail sprite fields should be written into job")
	}
	if result.Job.OutputBasePrefix != "vod/" {
		t.Fatalf("unexpected output base prefix: %s", result.Job.OutputBasePrefix)
	}
}

func TestCreateJobUseCase_ExecuteSupportsGRPCAndMQCallbackURL(t *testing.T) {
	idgen.Configure(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), 1, 10, 12)
	useCase := NewCreateJobUseCase()

	grpcResult, err := useCase.Execute(model.CreateJobRequest{
		RequestID:   "req-grpc-callback",
		SourceURL:   "https://example.com/video.mp4",
		CallbackURL: "grpc://127.0.0.1:9000/transcode.callback.Service/Notify",
	})
	if err != nil {
		t.Fatalf("grpc callback should be accepted: %v", err)
	}
	if grpcResult.RequestOverride == nil || grpcResult.RequestOverride.OverrideCallbackURL == "" {
		t.Fatalf("grpc callback override should be persisted")
	}

	mqResult, err := useCase.Execute(model.CreateJobRequest{
		RequestID:   "req-mq-callback",
		SourceURL:   "https://example.com/video.mp4",
		CallbackURL: "mq://callback.exchange/transcode.job.completed",
	})
	if err != nil {
		t.Fatalf("mq callback should be accepted: %v", err)
	}
	if mqResult.RequestOverride == nil || mqResult.RequestOverride.OverrideCallbackURL == "" {
		t.Fatalf("mq callback override should be persisted")
	}
}

func TestCreateJobUseCase_ExecuteRejectsInvalidCallbackURL(t *testing.T) {
	idgen.Configure(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), 1, 10, 12)
	useCase := NewCreateJobUseCase()

	_, err := useCase.Execute(model.CreateJobRequest{
		RequestID:   "req-invalid-callback",
		SourceURL:   "https://example.com/video.mp4",
		CallbackURL: "redis://127.0.0.1:6379/transcode.job.completed",
	})
	if err != ErrInvalidCallbackURL {
		t.Fatalf("expected ErrInvalidCallbackURL, got %v", err)
	}
}

func TestCreateJobUseCase_ExecuteRejectsUnsupportedFields(t *testing.T) {
	idgen.Configure(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC), 1, 10, 12)
	useCase := NewCreateJobUseCase()

	tests := []struct {
		name string
		req  model.CreateJobRequest
		err  error
	}{
		{
			name: "naming template",
			req: model.CreateJobRequest{
				RequestID:      "req-naming-template",
				SourceURL:      "https://example.com/video.mp4",
				SegmentOptions: &model.SegmentOptions{NamingTemplateID: 1},
			},
			err: ErrUnsupportedNamingTemplate,
		},
		{
			name: "storage id",
			req: model.CreateJobRequest{
				RequestID:      "req-storage-id",
				SourceURL:      "https://example.com/video.mp4",
				StorageOptions: &model.StorageOptions{StorageID: 9},
			},
			err: ErrUnsupportedStorageID,
		},
		{
			name: "segment prefix",
			req: model.CreateJobRequest{
				RequestID:      "req-segment-prefix",
				SourceURL:      "https://example.com/video.mp4",
				StorageOptions: &model.StorageOptions{SegmentPrefix: "segments/"},
			},
			err: ErrUnsupportedSegmentPrefix,
		},
		{
			name: "max wait seconds",
			req: model.CreateJobRequest{
				RequestID:       "req-max-wait",
				SourceURL:       "https://example.com/video.mp4",
				ScheduleOptions: &model.ScheduleOptions{MaxWaitSeconds: 30},
			},
			err: ErrUnsupportedMaxWaitSeconds,
		},
		{
			name: "soft decode flag",
			req: model.CreateJobRequest{
				RequestID:       "req-soft-decode",
				SourceURL:       "https://example.com/video.mp4",
				ScheduleOptions: &model.ScheduleOptions{AllowSoftwareDecodeFallback: true},
			},
			err: ErrUnsupportedSoftDecodeFlag,
		},
		{
			name: "video options",
			req: model.CreateJobRequest{
				RequestID:    "req-video-options",
				SourceURL:    "https://example.com/video.mp4",
				VideoOptions: &model.VideoOptions{AspectFillMode: "crop"},
			},
			err: ErrUnsupportedVideoOptions,
		},
	}

	for _, tt := range tests {
		_, err := useCase.Execute(tt.req)
		if err != tt.err {
			t.Fatalf("%s: expected %v, got %v", tt.name, tt.err, err)
		}
	}
}
