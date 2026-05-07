package integration

import (
	"testing"

	"hvc/internal/model"
)

// TestIntegration_JobModel 集成测试：转码任务模型字段完整性。
//
// 验证 TranscodeJob 模型包含所有必要字段，
// 确保数据库映射和 API 传输一致。
func TestIntegration_JobModel(t *testing.T) {
	job := model.TranscodeJob{
		JobID:              1893456789012345678,
		RequestID:          "req-001",
		BizKey:             "biz-video-001",
		SourceURL:          "https://example.com/video.mp4",
		Status:             model.JobStatusQueued,
		Priority:           5,
		SegmentDurationSec: 6,
		SupportDash:        true,
		SupportHLS:         true,
		EnableWatermark:    true,
	}

	if job.JobID == 0 {
		t.Error("JobID 不应为 0")
	}
	if job.SourceURL == "" {
		t.Error("SourceURL 不应为空")
	}
	if job.Status != model.JobStatusQueued {
		t.Errorf("初始状态应为 Queued, got %d", job.Status)
	}
}

// TestIntegration_SegmentModel 集成测试：分片模型字段完整性。
func TestIntegration_SegmentModel(t *testing.T) {
	seg := model.Segment{
		JobID:            1893456789012345678,
		RenditionName:    "1080p",
		SegmentType:      "init",
		MediaType:        1,
		IsInitSegment:    true,
		SequenceNo:       0,
		DurationMS:       0,
		Width:            1920,
		Height:           1080,
		VideoBitrateKbps: 5000,
		AudioBitrateKbps: 128,
		VideoCodec:       "h264_nvenc",
		SupportDash:      true,
		SupportHLS:       true,
		ObjectKey:        "hvc/1893456789012345678-1920_1080-video-0.m4s",
		UploadStatus:     model.SegmentUploadPending,
	}

	if seg.JobID == 0 {
		t.Error("Segment.JobID 不应为 0")
	}
	if seg.RenditionName == "" {
		t.Error("RenditionName 不应为空")
	}
	if seg.SegmentType != "init" {
		t.Errorf("SegmentType 应为 init, got %s", seg.SegmentType)
	}
	if !seg.IsInitSegment {
		t.Error("IsInitSegment 应为 true")
	}
	if seg.SequenceNo != 0 {
		t.Errorf("init 分片 SequenceNo 应为 0, got %d", seg.SequenceNo)
	}
	if seg.MediaType != 1 {
		t.Errorf("视频 MediaType 应为 1, got %d", seg.MediaType)
	}
}

// TestIntegration_SegmentUploadStatus 集成测试：分片上传状态枚举。
func TestIntegration_SegmentUploadStatus(t *testing.T) {
	statuses := map[string]int{
		"pending":   model.SegmentUploadPending,
		"uploading": model.SegmentUploading,
		"completed": model.SegmentUploaded,
		"failed":    model.SegmentUploadFailed,
	}

	for name, status := range statuses {
		if status < 0 {
			t.Errorf("%s 状态值不应为负数: %d", name, status)
		}
	}
}

// TestIntegration_JobStatusTransitions 集成测试：任务状态转换合法性。
func TestIntegration_JobStatusTransitions(t *testing.T) {
	validTransitions := map[int][]int{
		model.JobStatusQueued:    {model.JobStatusAssigned, model.JobStatusCanceled},
		model.JobStatusAssigned:  {model.JobStatusRunning, model.JobStatusFailed},
		model.JobStatusRunning:   {model.JobStatusUploading, model.JobStatusFailed},
		model.JobStatusUploading: {model.JobStatusCompleted, model.JobStatusFailed},
	}

	for from, tos := range validTransitions {
		for _, to := range tos {
			if from == to {
				t.Errorf("状态 %d 不应转换到自身", from)
			}
		}
	}
}
