package capability

import "hvc/internal/model"

// SupportsCodec 判断节点是否支持指定硬编编码。
//
// 这里仍然保留 codec 维度的判断能力，供“是否支持某种编码格式”这类问题复用。
// 需要注意：这和“是否支持某种执行硬件类型”不是同一个概念，调用方不能混用。
func SupportsCodec(metrics model.NodeMetrics, codec string) bool {
	for _, capability := range metrics.GPUCapabilities {
		for _, supported := range capability.EncodeCodecs {
			if supported == codec {
				return true
			}
		}
	}
	return false
}

// SupportsExecutionHW 判断节点是否支持指定执行硬件类型。
//
// 调度偏好 PreferredHWAccel 必须通过这个维度来判断，而不是再去对比 EncodeCodecs。
// 例如：
// - nvidia / intel_qsv / vaapi / apple_videotoolbox 是执行模式；
// - h264 / hevc / av1 是 codec 能力。
func SupportsExecutionHW(metrics model.NodeMetrics, hwType string) bool {
	for _, capability := range metrics.GPUCapabilities {
		for _, supported := range capability.ExecutionHWTypes {
			if supported == hwType {
				return true
			}
		}
	}
	return false
}
