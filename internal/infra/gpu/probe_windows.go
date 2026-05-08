//go:build windows

package gpu

import (
	"strings"

	"hvc/internal/model"
)

// probePlatform 实现 Windows 平台 GPU 能力探测。
//
// Windows 当前批次优先覆盖 NVIDIA / Intel QSV / AMD AMF 三条执行路径。
// 因为仓库里还没有 WMI、DXGI 等重依赖封装，所以先采用最小化策略：
// - 优先尝试 nvidia-smi 识别 NVIDIA 设备；
// - 其余能力统一从 ffmpeg 输出里推断。
func probePlatform() []ProbeResult {
	results := make([]ProbeResult, 0, 1)
	nvidiaOutput := strings.TrimSpace(string(commandOutput("nvidia-smi", "--query-gpu=index,uuid,name,driver_version,memory.total", "--format=csv,noheader,nounits")))
	if nvidiaOutput != "" {
		for _, line := range strings.Split(nvidiaOutput, "\n") {
			parts := strings.Split(line, ",")
			if len(parts) < 5 {
				continue
			}
			results = append(results, ProbeResult{
				GPUUUID:          strings.TrimSpace(parts[1]),
				GPUIndex:         parseIndex(parts[0]),
				Vendor:           "nvidia",
				Model:            strings.TrimSpace(parts[2]),
				DriverVersion:    strings.TrimSpace(parts[3]),
				MemoryTotalMB:    parseIndex(parts[4]),
				ExecutionHWTypes: []string{model.ExecutionHWNVIDIA},
				SupportsFilter:   true,
			})
		}
	}
	fallback := probeFallback()
	if len(fallback) == 0 {
		return results
	}
	if len(results) == 0 {
		return fallback
	}
	for i := range results {
		results[i].EncodeCodecs = append(results[i].EncodeCodecs, fallback[0].EncodeCodecs...)
		results[i].DecodeCodecs = append(results[i].DecodeCodecs, fallback[0].DecodeCodecs...)
		results[i].ExecutionHWTypes = append(results[i].ExecutionHWTypes, fallback[0].ExecutionHWTypes...)
		results[i].SupportsFilter = results[i].SupportsFilter || fallback[0].SupportsFilter
	}
	return results
}
