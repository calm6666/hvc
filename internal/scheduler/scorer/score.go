package scorer

import "hvc/internal/model"

// ScoreNode 计算节点分值。
func ScoreNode(metrics model.NodeMetrics) int {
	score := 0
	score += metrics.ActiveTranscodeSessions * 30
	score += metrics.CPUUsagePercent * 15
	score += metrics.MemoryUsagePercent * 10
	score += metrics.GPUMemoryUsagePercent * 20
	score += metrics.UploadQueueDepth * 10
	return score
}
