package scheduler

import (
	"testing"

	"hvc/internal/config"
	"hvc/internal/model"
)

func TestMergeGPUActiveSessions(t *testing.T) {
	metrics := model.NodeMetrics{
		NodeID: 1,
		GPUCapabilities: []model.GPUCapability{
			{GPUIndex: 0, ActiveSessions: 1},
			{GPUIndex: 1, ActiveSessions: 2},
		},
	}

	merged := mergeGPUActiveSessions(metrics, map[int]int{
		0: 3,
		1: 5,
	})

	if merged.GPUCapabilities[0].ActiveSessions != 3 {
		t.Fatalf("unexpected gpu 0 active sessions: %d", merged.GPUCapabilities[0].ActiveSessions)
	}
	if merged.GPUCapabilities[1].ActiveSessions != 5 {
		t.Fatalf("unexpected gpu 1 active sessions: %d", merged.GPUCapabilities[1].ActiveSessions)
	}
}

func TestResolveDispatchQueueFetchLimit(t *testing.T) {
	if value := resolveDispatchQueueFetchLimit(0); value != 0 {
		t.Fatalf("expected limit 0 when no remaining capacity, got %d", value)
	}
	if value := resolveDispatchQueueFetchLimit(1); value != 20 {
		t.Fatalf("expected minimum fetch limit 20, got %d", value)
	}
	if value := resolveDispatchQueueFetchLimit(8); value != 32 {
		t.Fatalf("expected proportional fetch limit 32, got %d", value)
	}
	if value := resolveDispatchQueueFetchLimit(200); value != 500 {
		t.Fatalf("expected capped fetch limit 500, got %d", value)
	}
}

func TestResolveDispatchRemainingCapacity(t *testing.T) {
	cfg := config.DynamicRuntimeConfig{
		Scheduler: config.SchedulerConfig{
			MaxGlobalTranscodeSessions: 10,
		},
	}
	if value := resolveDispatchRemainingCapacity(cfg, 3); value != 7 {
		t.Fatalf("expected remaining capacity 7, got %d", value)
	}
	if value := resolveDispatchRemainingCapacity(cfg, 15); value != 0 {
		t.Fatalf("expected remaining capacity floor 0, got %d", value)
	}
	cfg.Scheduler.MaxGlobalTranscodeSessions = 0
	if value := resolveDispatchRemainingCapacity(cfg, 999); value <= 0 {
		t.Fatalf("expected unlimited mode to return positive capacity, got %d", value)
	}
}

func TestBuildJobRequestCarriesRenditionsAndOverrideHWAccel(t *testing.T) {
	manager := &Manager{}
	job := model.TranscodeJob{
		RequestID:       "req-1",
		SourceURL:       "https://example.com/source.mp4",
		ProfileID:       7,
		Priority:        3,
		EnableWatermark: true,
		Renditions: []model.RenditionOption{
			{
				Name:       "1080p",
				Width:      1920,
				Height:     1080,
				VideoCodec: "h265",
			},
		},
	}
	override := model.TranscodeJobRequestOverride{
		OverridePreferredHWAccel: model.ExecutionHWNVIDIA,
	}

	req := manager.buildJobRequest(job, override)

	if req.RequestID != job.RequestID || req.SourceURL != job.SourceURL || req.ProfileID != job.ProfileID {
		t.Fatalf("unexpected basic fields in request: %+v", req)
	}
	if !req.EnableWatermark {
		t.Fatal("expected enable_watermark to be preserved")
	}
	if req.ScheduleOptions == nil || req.ScheduleOptions.PreferredHWAccel != model.ExecutionHWNVIDIA {
		t.Fatalf("expected preferred hwaccel override to be preserved, got %+v", req.ScheduleOptions)
	}
	if len(req.Renditions) != 1 || req.Renditions[0].VideoCodec != "h265" {
		t.Fatalf("expected renditions to be preserved, got %+v", req.Renditions)
	}

	job.Renditions[0].VideoCodec = "h264"
	if req.Renditions[0].VideoCodec != "h265" {
		t.Fatal("expected buildJobRequest to copy renditions instead of aliasing source slice")
	}
}
