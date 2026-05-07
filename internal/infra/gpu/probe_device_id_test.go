package gpu

import "testing"

func TestToGPUCapabilities_KeepGPUDeviceID(t *testing.T) {
	capabilities := ToGPUCapabilities([]ProbeResult{{
		GPUDeviceID:      123,
		GPUUUID:          "gpu-uuid-1",
		GPUIndex:         0,
		ExecutionHWTypes: []string{"nvidia"},
	}})
	if len(capabilities) != 1 {
		t.Fatalf("unexpected capability count: %d", len(capabilities))
	}
	if capabilities[0].GPUDeviceID != 123 {
		t.Fatalf("unexpected gpu device id: %d", capabilities[0].GPUDeviceID)
	}
}
