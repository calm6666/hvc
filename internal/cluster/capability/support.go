package capability

import "hvc/internal/model"

// SupportsCodec 判断节点是否支持指定硬编编码。
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
