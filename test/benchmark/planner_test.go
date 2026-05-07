package benchmark

import (
	"testing"

	"hvc/internal/infra/ffmpeg/probe"
	"hvc/internal/model"
	"hvc/internal/worker/planner"
)

// BenchmarkBuildRenditions 基准测试：构建编码阶梯。
//
// 测量从源视频信息生成多清晰度输出规格的性能。
// 正常情况下应在微秒级完成。
func BenchmarkBuildRenditions(b *testing.B) {
	job := model.TranscodeJob{
		JobID:              1893456789012345678,
		SegmentDurationSec: 6,
		SupportDash:        true,
		SupportHLS:         true,
	}

	probeResult := mockProbeResult(1920, 1080)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		planner.BuildPlan(job, probeResult, "nvidia", planner.DefaultSegmentNamingConfig())
	}
}

// BenchmarkRenderSegmentName 基准测试：分片命名模板渲染。
//
// 测量模板占位符替换的性能。
// 六种模板方案均需在纳秒级完成。
func BenchmarkRenderSegmentName(b *testing.B) {
	rend := planner.RenditionSpec{
		Name:             "1080p",
		QualityLabel:     "1080p",
		Width:            1920,
		Height:           1080,
		VideoCodec:       "h264_nvenc",
		VideoBitrateKbps: 5000,
	}

	templates := []struct {
		name     string
		template string
	}{
		{"方案一", "{job_id}-{media_type}-{number}.m4s"},
		{"方案二", "{job_id}-{resolution}-{media_type}-{number}.m4s"},
		{"方案三", "{job_id}-{quality}-{media_type}-{number}.m4s"},
		{"方案四", "{job_id}-{media_type}-{number}-{timestamp}.m4s"},
		{"方案五", "{job_id}-{resolution}-{media_type}-{number}-{timestamp}.m4s"},
		{"方案六", "{job_id}-{quality}-{media_type}-{number}-{timestamp}.m4s"},
	}

	for _, tmpl := range templates {
		b.Run(tmpl.name, func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				planner.RenderSegmentName(tmpl.template, 1893456789012345678, rend, "video", 1, 6000)
			}
		})
	}
}

// BenchmarkQualityLabelFromHeight 基准测试：清晰度标签映射。
func BenchmarkQualityLabelFromHeight(b *testing.B) {
	heights := []int{2160, 1440, 1080, 720, 480, 360, 240}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, h := range heights {
			planner.QualityLabelFromHeight(h)
		}
	}
}

// BenchmarkMapRepIDToRendition 基准测试：RepresentationID 映射。
func BenchmarkMapRepIDToRendition(b *testing.B) {
	renditions := []planner.RenditionSpec{
		{Name: "1080p", QualityLabel: "1080p", Width: 1920, Height: 1080},
		{Name: "720p", QualityLabel: "720p", Width: 1280, Height: 720},
		{Name: "480p", QualityLabel: "480p", Width: 854, Height: 480},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for repID := 0; repID < 6; repID++ {
			planner.MapRepIDToRendition(repID, renditions)
		}
	}
}

// BenchmarkCalculateWidth 基准测试：动态分辨率计算。
func BenchmarkCalculateWidth(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		planner.CalculateWidth(1080, 16.0/9.0)
		planner.CalculateWidth(720, 16.0/9.0)
		planner.CalculateWidth(480, 4.0/3.0)
	}
}

func mockProbeResult(width, height int) probe.Result {
	return probe.Result{
		Width:            width,
		Height:           height,
		VideoCodec:       "h264",
		VideoBitrateKbps: 8000,
		AudioBitrateKbps: 128,
		AudioChannels:    2,
		AudioSampleRate:  44100,
		FPS:              30.0,
		GOPSize:          60,
	}
}
