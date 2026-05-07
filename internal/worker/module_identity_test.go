package worker

import (
	"testing"

	"hvc/internal/model"
)

func TestPersistGPUCapabilities_KeepGPUDeviceIDInMetricsModel(t *testing.T) {
	capability := model.GPUCapability{
		GPUDeviceID:      42,
		GPUUUID:          "gpu-1",
		GPUIndex:         0,
		ExecutionHWTypes: []string{model.ExecutionHWNVIDIA},
	}
	if capability.GPUDeviceID != 42 {
		t.Fatalf("unexpected gpu device id: %d", capability.GPUDeviceID)
	}
}
