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
