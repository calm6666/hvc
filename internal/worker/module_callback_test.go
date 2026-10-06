package worker

import (
	"context"
	"testing"

	"hvc/internal/config"
	ffprobe "hvc/internal/infra/ffmpeg/probe"
	"hvc/internal/model"
	"hvc/internal/worker/segmenter"
)

func TestBuildCompletedPayloadUsesExtensionlessManifestRoutes(t *testing.T) {
	module := &Module{
		cfg: config.DynamicRuntimeConfig{},
	}
	payload := module.buildCompletedPayload(context.Background(), model.TranscodeJob{
		JobID:              123,
		RequestID:          "req-1",
		SourceURL:          "https://example.com/video.mp4",
		SegmentDurationSec: 6,
		SupportDash:        true,
		SupportHLS:         true,
	}, ffprobe.Result{}, segmenter.Result{
		Segments: []segmenter.DiscoveredSegment{
			{
				RenditionName:    "1080p",
				RenditionKey:     "1080p",
				QualityLabel:     "1080p",
				Width:            1920,
				Height:           1080,
				VideoCodec:       "h264",
				VideoBitrateKbps: 5000,
				AudioBitrateKbps: 128,
				IsInit:           true,
				ObjectKey:        "init.m4s",
				FileSize:         1024,
			},
		},
	})
	if len(payload.Renditions) != 1 {
		t.Fatalf("unexpected rendition count: %d", len(payload.Renditions))
	}
	rendition := payload.Renditions[0]
	if rendition.ManifestDashURL != "/v1/manifest/dash/123" {
		t.Fatalf("unexpected manifest dash url: %s", rendition.ManifestDashURL)
	}
	if rendition.ManifestHLSURL != "/v1/manifest/hls/123" {
		t.Fatalf("unexpected manifest hls url: %s", rendition.ManifestHLSURL)
	}
	if rendition.ManifestHLSVariantURL != "/v1/manifest/hls/123/1080p" {
		t.Fatalf("unexpected manifest hls variant url: %s", rendition.ManifestHLSVariantURL)
	}
}
