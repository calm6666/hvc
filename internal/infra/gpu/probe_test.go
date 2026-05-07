package gpu

import (
	"reflect"
	"testing"

	"hvc/internal/model"
)

func TestToGPUCapabilities_NormalizeExecutionTypes(t *testing.T) {
	capabilities := ToGPUCapabilities([]ProbeResult{
		{
			GPUUUID:          "",
			GPUIndex:         1,
			EncodeCodecs:     []string{"hevc", "h264", "hevc"},
			DecodeCodecs:     []string{"h264", "h264"},
			ExecutionHWTypes: []string{model.ExecutionHWNVIDIA, model.ExecutionHWNVIDIA, model.ExecutionHWVAAPI},
			MaxSessions:      8,
			SupportsFilter:   true,
		},
	})
	if len(capabilities) != 1 {
		t.Fatalf("unexpected capability count: %d", len(capabilities))
	}
	if capabilities[0].GPUUUID != "nvidia-1" {
		t.Fatalf("unexpected fallback uuid: %s", capabilities[0].GPUUUID)
	}
	if !reflect.DeepEqual(capabilities[0].EncodeCodecs, []string{"h264", "hevc"}) {
		t.Fatalf("unexpected encode codecs: %#v", capabilities[0].EncodeCodecs)
	}
	if !reflect.DeepEqual(capabilities[0].ExecutionHWTypes, []string{"nvidia", "vaapi"}) {
		t.Fatalf("unexpected execution hw types: %#v", capabilities[0].ExecutionHWTypes)
	}
}
