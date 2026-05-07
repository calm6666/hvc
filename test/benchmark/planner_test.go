package benchmark

import (
	"testing"

	"hvc/internal/model"
	"hvc/internal/infra/ffmpeg/probe"
	"hvc/internal/worker/planner"
)

func BenchmarkBuildPlan(b *testing.B) {
	job := createBenchmarkJob()
	probeResult := createBenchmarkProbeResult()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		planner.BuildPlan(job, probeResult, "nvidia", planner.DefaultSegmentNamingConfig())
	}
}

func BenchmarkBuildFFmpegArgs(b *testing.B) {
	job := createBenchmarkJob()
	probeResult := createBenchmarkProbeResult()
	pipeline := planner.BuildPlan(job, probeResult, "nvidia", planner.DefaultSegmentNamingConfig())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		planner.BuildFFmpegArgs(pipeline)
	}
}

func BenchmarkCalculateWidth(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for _, aspect := range []float64{16.0 / 9, 4.0 / 3, 21.0 / 9, 2.0} {
			planner.CalculateWidth(1080, aspect)
			planner.CalculateWidth(720, aspect)
			planner.CalculateWidth(480, aspect)
		}
	}
}

func createBenchmarkJob() model.TranscodeJob {
	return model.TranscodeJob{
		JobID:              1,
		RequestID:          "bench-001",
		SourceURL:          "https://example.com/bench.mp4",
		SegmentDurationSec: 6,
		SupportDash:        true,
		SupportHLS:         true,
		EnableWatermark:    false,
	}
}

func createBenchmarkProbeResult() probe.Result {
	return probe.Result{
		VideoCodec:       "h264",
		AudioCodec:       "aac",
		Width:            3840,
		Height:           2160,
		FPS:              30.0,
		DurationMS:       60000,
		VideoBitrateKbps: 20000,
		AudioBitrateKbps: 128,
		AudioChannels:    2,
		AudioSampleRate:  44100,
	}
}
