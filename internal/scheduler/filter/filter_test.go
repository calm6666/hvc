package filter

import (
	"testing"

	"hvc/internal/config"
	"hvc/internal/model"
)

func TestApply_MatchPreferredExecutionHW(t *testing.T) {
	filter := NewFilter(config.DynamicRuntimeConfig{
		Scheduler: config.SchedulerConfig{
			NodeCPUSafetyLimitPercent:    100,
			NodeMemorySafetyLimitPercent: 100,
			NodeGPUSafetyLimitPercent:    100,
			MaxNodeTranscodeSessions:     100,
			MaxNodeUploadConcurrency:     100,
		},
	})
	candidate := model.DispatchCandidate{
		NodeID:                    1,
		Enabled:                   true,
		SupportsHardwareWatermark: true,
		Metrics: model.NodeMetrics{
			GPUCapabilities: []model.GPUCapability{{
				GPUUUID:          "gpu-1",
				GPUIndex:         0,
				ExecutionHWTypes: []string{model.ExecutionHWNVIDIA},
				EncodeCodecs:     []string{"h264", "hevc"},
			}},
		},
	}
	passed := filter.Apply(model.CreateJobRequest{
		ScheduleOptions: &model.ScheduleOptions{PreferredHWAccel: model.ExecutionHWNVIDIA},
	}, []model.DispatchCandidate{candidate})
	if len(passed) != 1 {
		t.Fatalf("expected candidate to pass, got %d", len(passed))
	}
}

func TestApply_RejectMismatchedPreferredExecutionHW(t *testing.T) {
	filter := NewFilter(config.DynamicRuntimeConfig{
		Scheduler: config.SchedulerConfig{
			NodeCPUSafetyLimitPercent:    100,
			NodeMemorySafetyLimitPercent: 100,
			NodeGPUSafetyLimitPercent:    100,
			MaxNodeTranscodeSessions:     100,
			MaxNodeUploadConcurrency:     100,
		},
	})
	candidate := model.DispatchCandidate{
		NodeID:  1,
		Enabled: true,
		Metrics: model.NodeMetrics{
			GPUCapabilities: []model.GPUCapability{{
				GPUUUID:          "gpu-1",
				GPUIndex:         0,
				ExecutionHWTypes: []string{model.ExecutionHWNVIDIA},
				EncodeCodecs:     []string{"h264", "hevc"},
			}},
		},
	}
	passed := filter.Apply(model.CreateJobRequest{
		ScheduleOptions: &model.ScheduleOptions{PreferredHWAccel: model.ExecutionHWAppleVideoToolbox},
	}, []model.DispatchCandidate{candidate})
	if len(passed) != 0 {
		t.Fatalf("expected candidate to be filtered out, got %d", len(passed))
	}
}

func TestApply_RejectWhenPreferredGPUAllSessionsFull(t *testing.T) {
	filter := NewFilter(config.DynamicRuntimeConfig{
		Scheduler: config.SchedulerConfig{
			NodeCPUSafetyLimitPercent:    100,
			NodeMemorySafetyLimitPercent: 100,
			NodeGPUSafetyLimitPercent:    100,
			MaxNodeTranscodeSessions:     100,
			MaxNodeUploadConcurrency:     100,
		},
	})
	candidate := model.DispatchCandidate{
		NodeID:  1,
		Enabled: true,
		Metrics: model.NodeMetrics{
			GPUCapabilities: []model.GPUCapability{{
				GPUUUID:          "gpu-1",
				GPUIndex:         0,
				ExecutionHWTypes: []string{model.ExecutionHWNVIDIA},
				EncodeCodecs:     []string{"h264"},
				MaxSessions:      2,
				ActiveSessions:   2,
			}},
		},
	}
	passed := filter.Apply(model.CreateJobRequest{
		ScheduleOptions: &model.ScheduleOptions{PreferredHWAccel: model.ExecutionHWNVIDIA},
	}, []model.DispatchCandidate{candidate})
	if len(passed) != 0 {
		t.Fatalf("expected candidate to be filtered out when preferred gpu is full, got %d", len(passed))
	}
}

func TestApply_RejectWhenHardwareRequiredButAllGPUsFull(t *testing.T) {
	filter := NewFilter(config.DynamicRuntimeConfig{
		Scheduler: config.SchedulerConfig{
			NodeCPUSafetyLimitPercent:    100,
			NodeMemorySafetyLimitPercent: 100,
			NodeGPUSafetyLimitPercent:    100,
			MaxNodeTranscodeSessions:     100,
			MaxNodeUploadConcurrency:     100,
			RequireHardwareEncode:        true,
		},
	})
	candidate := model.DispatchCandidate{
		NodeID:  1,
		Enabled: true,
		Metrics: model.NodeMetrics{
			GPUCapabilities: []model.GPUCapability{
				{
					GPUUUID:          "gpu-1",
					GPUIndex:         0,
					ExecutionHWTypes: []string{model.ExecutionHWNVIDIA},
					MaxSessions:      1,
					ActiveSessions:   1,
				},
				{
					GPUUUID:          "gpu-2",
					GPUIndex:         1,
					ExecutionHWTypes: []string{model.ExecutionHWVAAPI},
					MaxSessions:      3,
					ActiveSessions:   3,
				},
			},
		},
	}
	passed := filter.Apply(model.CreateJobRequest{}, []model.DispatchCandidate{candidate})
	if len(passed) != 0 {
		t.Fatalf("expected candidate to be filtered out when all gpus are full under hardware-required mode, got %d", len(passed))
	}
}
