package scheduler

import (
	"testing"

	"hvc/internal/model"
)

func TestBuildDecision_PreferRequestedExecutionHW(t *testing.T) {
	decision := BuildDecision(model.DispatchCandidate{
		NodeID: 1,
		Metrics: model.NodeMetrics{
			GPUCapabilities: []model.GPUCapability{{
				GPUDeviceID:      88,
				GPUUUID:          "gpu-1",
				GPUIndex:         2,
				ExecutionHWTypes: []string{model.ExecutionHWNVIDIA, model.ExecutionHWVAAPI},
			}},
		},
	}, model.ExecutionHWVAAPI, 3, 1)
	if decision.SelectedExecutionHW != model.ExecutionHWVAAPI {
		t.Fatalf("unexpected selected execution hw: %s", decision.SelectedExecutionHW)
	}
}
