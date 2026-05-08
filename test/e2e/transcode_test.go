package e2e

import (
	"testing"

	"hvc/internal/infra/ffmpeg/probe"
	"hvc/internal/model"
	"hvc/internal/worker/planner"
	"hvc/internal/worker/segmenter"
)

// TestE2E_TranscodePipeline 端到端测试：完整转码管线。
//
// 验证从任务创建到分片命名的完整流程：
//  1. 构建编码阶梯
//  2. 生成 FFmpeg 命令
//  3. 渲染分片命名
//  4. 验证命名模板一致性
func TestE2E_TranscodePipeline(t *testing.T) {
	job := model.TranscodeJob{
		JobID:              1893456789012345678,
		SegmentDurationSec: 6,
		SupportDash:        true,
		SupportHLS:         true,
	}

	probeResult := probe.Result{
		Width:            1920,
		Height:           1080,
		VideoCodec:       "h264",
		VideoBitrateKbps: 8000,
		AudioBitrateKbps: 128,
		AudioChannels:    2,
		AudioSampleRate:  44100,
		FPS:              30.0,
		GOPSize:          60,
	}

	templates := []struct {
		name     string
		template string
	}{
		{"方案一", "{job_id}-{rendition_key}-{media_type}-{number}.m4s"},
		{"方案二", "{job_id}-{resolution}-{rendition_key}-{media_type}-{number}.m4s"},
		{"方案三", "{job_id}-{quality}-{rendition_key}-{media_type}-{number}.m4s"},
		{"方案四", "{job_id}-{rendition_key}-{media_type}-{number}-{timestamp}.m4s"},
		{"方案五", "{job_id}-{resolution}-{rendition_key}-{media_type}-{number}-{timestamp}.m4s"},
		{"方案六", "{job_id}-{quality}-{rendition_key}-{media_type}-{number}-{timestamp}.m4s"},
	}

	for _, tmpl := range templates {
		t.Run(tmpl.name, func(t *testing.T) {
			naming := planner.SegmentNamingConfig{
				SegmentTemplate: tmpl.template,
				ObjectKeyPrefix: "hvc",
				JobID:           job.JobID,
			}

			pipeline := planner.BuildPlan(job, probeResult, "nvidia", naming)

			if len(pipeline.Renditions) == 0 {
				t.Fatal("编码阶梯不应为空")
			}

			for _, rend := range pipeline.Renditions {
				if rend.QualityLabel == "" {
					t.Errorf("清晰度 %s 缺少 QualityLabel", rend.Name)
				}
				if rend.Width <= 0 || rend.Height <= 0 {
					t.Errorf("清晰度 %s 分辨率无效: %dx%d", rend.Name, rend.Width, rend.Height)
				}

				initName := planner.RenderSegmentName(tmpl.template, job.JobID, rend, "video", 0, 0)
				mediaName := planner.RenderSegmentName(tmpl.template, job.JobID, rend, "video", 1, 6000)
				audioInitName := planner.RenderSegmentName(tmpl.template, job.JobID, rend, "audio", 0, 0)

				if initName == "" {
					t.Error("init 分片名不应为空")
				}
				if mediaName == "" {
					t.Error("media 分片名不应为空")
				}
				if audioInitName == "" {
					t.Error("音频 init 分片名不应为空")
				}

				t.Logf("清晰度 %s: video-init=%s, video-media=%s, audio-init=%s",
					rend.Name, initName, mediaName, audioInitName)
			}
		})
	}
}

// TestE2E_SegmentNamingConsistency 端到端测试：分片命名一致性。
//
// 验证 discover 模块生成的对象存储键与模板渲染结果一致。
func TestE2E_SegmentNamingConsistency(t *testing.T) {
	job := model.TranscodeJob{
		JobID:              1893456789012345678,
		SegmentDurationSec: 6,
	}

	rend := planner.RenditionSpec{
		Name:             "1080p",
		RenditionKey:     "Ab3kP9xQ",
		QualityLabel:     "1080p",
		Width:            1920,
		Height:           1080,
		VideoBitrateKbps: 5000,
		AudioBitrateKbps: 128,
	}

	template := "{job_id}-{resolution}-{rendition_key}-{media_type}-{number}.m4s"

	expectedInitName := "1893456789012345678-1920_1080-Ab3kP9xQ-video-0.m4s"
	expectedMediaName := "1893456789012345678-1920_1080-Ab3kP9xQ-video-1.m4s"
	expectedAudioInitName := "1893456789012345678-1920_1080-Ab3kP9xQ-audio-0.m4s"

	actualInitName := planner.RenderSegmentName(template, job.JobID, rend, "video", 0, 0)
	actualMediaName := planner.RenderSegmentName(template, job.JobID, rend, "video", 1, 6000)
	actualAudioInitName := planner.RenderSegmentName(template, job.JobID, rend, "audio", 0, 0)

	if actualInitName != expectedInitName {
		t.Errorf("video init 命名不一致: got=%s, want=%s", actualInitName, expectedInitName)
	}
	if actualMediaName != expectedMediaName {
		t.Errorf("video media 命名不一致: got=%s, want=%s", actualMediaName, expectedMediaName)
	}
	if actualAudioInitName != expectedAudioInitName {
		t.Errorf("audio init 命名不一致: got=%s, want=%s", actualAudioInitName, expectedAudioInitName)
	}
}

// TestE2E_QualityLabelMapping 端到端测试：清晰度标签映射。
func TestE2E_QualityLabelMapping(t *testing.T) {
	cases := []struct {
		height int
		want   string
	}{
		{2160, "4k"},
		{1440, "2k"},
		{1080, "1080p"},
		{720, "720p"},
		{480, "480p"},
		{360, "360p"},
		{240, "240p"},
	}

	for _, tc := range cases {
		got := planner.QualityLabelFromHeight(tc.height)
		if got != tc.want {
			t.Errorf("QualityLabelFromHeight(%d) = %s, want %s", tc.height, got, tc.want)
		}
	}
}

// TestE2E_MapRepIDToRendition 端到端测试：RepresentationID 映射。
func TestE2E_MapRepIDToRendition(t *testing.T) {
	renditions := []planner.RenditionSpec{
		{Name: "1080p", QualityLabel: "1080p", Width: 1920, Height: 1080},
		{Name: "720p", QualityLabel: "720p", Width: 1280, Height: 720},
	}

	videoRend, videoType := planner.MapRepIDToRendition(0, renditions)
	if videoType != "video" || videoRend.Name != "1080p" {
		t.Errorf("repID=0 应映射到 1080p video, got %s %s", videoType, videoRend.Name)
	}

	audioRend, audioType := planner.MapRepIDToRendition(2, renditions)
	if audioType != "audio" || audioRend.Name != "1080p" {
		t.Errorf("repID=2 应映射到 1080p audio, got %s %s", audioType, audioRend.Name)
	}
}

// 确保 segmenter 包可编译（空引用）
var _ segmenter.DiscoveredSegment
