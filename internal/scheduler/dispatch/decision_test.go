package scheduler

import (
	"testing"

	"hvc/internal/model"
)

func TestBuildDecision_UseStableGPUDeviceID(t *testing.T) {
	decision := BuildDecision(model.DispatchCandidate{
		NodeID: 1,
		Metrics: model.NodeMetrics{
			GPUCapabilities: []model.GPUCapability{{
				GPUDeviceID:      99,
				GPUUUID:          "gpu-1",
				GPUIndex:         3,
				ExecutionHWTypes: []string{model.ExecutionHWNVIDIA},
			}},
		},
	}, model.ExecutionHWNVIDIA, 7, 2)
	if decision.SelectedGPUDeviceID != 99 {
		t.Fatalf("unexpected gpu device id: %d", decision.SelectedGPUDeviceID)
	}
	if decision.SelectedGPUIndex != 3 {
		t.Fatalf("unexpected gpu index: %d", decision.SelectedGPUIndex)
	}
	if decision.SelectedExecutionHW != model.ExecutionHWNVIDIA {
		t.Fatalf("unexpected execution hw: %s", decision.SelectedExecutionHW)
	}
}

func TestBuildDecision_NoGPUFallsBackToSoftware(t *testing.T) {
	decision := BuildDecision(model.DispatchCandidate{NodeID: 1}, "", 2, 4)
	if decision.SelectedGPUIndex != -1 {
		t.Fatalf("expected no gpu index, got %d", decision.SelectedGPUIndex)
	}
	if decision.SelectedGPUDeviceID != 0 {
		t.Fatalf("expected no gpu device id, got %d", decision.SelectedGPUDeviceID)
	}
	if decision.SelectedExecutionHW != model.ExecutionHWSoftware {
		t.Fatalf("expected software fallback, got %s", decision.SelectedExecutionHW)
	}
}

func TestBuildDecision_PicksLessLoadedGPU(t *testing.T) {
	decision := BuildDecision(model.DispatchCandidate{
		NodeID: 1,
		Metrics: model.NodeMetrics{
			GPUCapabilities: []model.GPUCapability{
				{
					GPUDeviceID:           101,
					GPUUUID:               "gpu-busy",
					GPUIndex:              0,
					ExecutionHWTypes:      []string{model.ExecutionHWNVIDIA},
					ActiveSessions:        3,
					GPUMemoryUsagePercent: 90,
					GPUUtilizationPercent: 95,
				},
				{
					GPUDeviceID:           102,
					GPUUUID:               "gpu-idle",
					GPUIndex:              1,
					ExecutionHWTypes:      []string{model.ExecutionHWNVIDIA},
					ActiveSessions:        1,
					GPUMemoryUsagePercent: 30,
					GPUUtilizationPercent: 20,
				},
			},
		},
	}, model.ExecutionHWNVIDIA, 1, 0)
	if decision.SelectedGPUDeviceID != 102 {
		t.Fatalf("expected less loaded gpu, got %d", decision.SelectedGPUDeviceID)
	}
	if decision.SelectedGPUIndex != 1 {
		t.Fatalf("expected gpu index 1, got %d", decision.SelectedGPUIndex)
	}
}
