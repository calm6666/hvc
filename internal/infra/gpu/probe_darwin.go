//go:build darwin

package gpu

import (
	"strings"

	"hvc/internal/model"
)

// probePlatform 实现 macOS 平台 GPU 能力探测。
//
// 当前优先识别 Apple VideoToolbox 能力，因为这是 macOS 上最直接影响硬编/硬解调度的执行模式。
// 若无法从 system_profiler 等系统命令拿到稳定设备信息，则回退到 ffmpeg 能力探测补齐 codec 与执行类型。
func probePlatform() []ProbeResult {
	output := strings.ToLower(string(commandOutput("system_profiler", "SPDisplaysDataType")))
	results := make([]ProbeResult, 0, 1)
	if output != "" && strings.Contains(output, "chipset model") {
		results = append(results, ProbeResult{
			GPUUUID:          "apple-gpu-0",
			GPUIndex:         0,
			Vendor:           "apple",
			Model:            "apple-gpu",
			ExecutionHWTypes: []string{model.ExecutionHWAppleVideoToolbox},
			SupportsFilter:   true,
		})
	}
	fallback := probeFallback()
	if len(fallback) == 0 {
		return results
	}
	if len(results) == 0 {
		return fallback
	}
	results[0].EncodeCodecs = append(results[0].EncodeCodecs, fallback[0].EncodeCodecs...)
	results[0].DecodeCodecs = append(results[0].DecodeCodecs, fallback[0].DecodeCodecs...)
	results[0].ExecutionHWTypes = append(results[0].ExecutionHWTypes, fallback[0].ExecutionHWTypes...)
	results[0].SupportsFilter = results[0].SupportsFilter || fallback[0].SupportsFilter
	return results
}
