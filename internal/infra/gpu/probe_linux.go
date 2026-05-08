//go:build linux

package gpu

import (
	"strings"

	"hvc/internal/model"
)

// probePlatform 实现 Linux 平台 GPU 能力探测。
//
// Linux 平台的硬件路径最复杂，当前批次优先覆盖：
// 1. NVIDIA（nvidia-smi / ffmpeg nvenc/cuvid）；
// 2. Intel QSV；
// 3. VAAPI；
// 4. AMD 走 VAAPI 或 AMF 能力归一化。
//
// 这里仍采用“系统命令识别设备存在性 + ffmpeg 识别 codec 能力”的组合方式，
// 目的是在不引入重依赖的前提下，把调度所需的最小主链路补齐。
func probePlatform() []ProbeResult {
	results := make([]ProbeResult, 0, 2)
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
