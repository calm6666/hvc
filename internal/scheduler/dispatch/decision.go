package scheduler

import "hvc/internal/model"

// ScoreCandidate 返回候选节点分值。
func ScoreCandidate(candidate model.DispatchCandidate) int {
	score := 0
	score += candidate.Metrics.ActiveTranscodeSessions * 30
	score += candidate.Metrics.CPUUsagePercent * 15
	score += candidate.Metrics.MemoryUsagePercent * 10
	score += candidate.Metrics.GPUMemoryUsagePercent * 20
	score += candidate.Metrics.UploadQueueDepth * 10
	return score
}

// PickBestCandidate 返回分值最小的候选节点。
func PickBestCandidate(candidates []model.DispatchCandidate) (model.DispatchCandidate, bool) {
	if len(candidates) == 0 {
		return model.DispatchCandidate{}, false
	}
	best := candidates[0]
	bestScore := ScoreCandidate(best)
	for _, candidate := range candidates[1:] {
		score := ScoreCandidate(candidate)
		if score < bestScore {
			best = candidate
			bestScore = score
		}
	}
	return best, true
}

// BuildDecision 生成调度结果。
func BuildDecision(candidate model.DispatchCandidate, previousLeaseGeneration uint64, previousAttemptNo int) model.DispatchDecision {
	selectedGPUIndex := 0
	selectedGPUDeviceID := uint64(0)
	if len(candidate.Metrics.GPUCapabilities) > 0 {
		selectedGPUIndex = candidate.Metrics.GPUCapabilities[0].GPUIndex
		selectedGPUDeviceID = uint64(candidate.Metrics.GPUCapabilities[0].GPUIndex + 1)
	}
	return model.DispatchDecision{
		NodeID:              candidate.NodeID,
		SelectedGPUIndex:    selectedGPUIndex,
		SelectedGPUDeviceID: selectedGPUDeviceID,
		SelectedExecutionHW: "hardware",
		LeaseGeneration:     previousLeaseGeneration + 1,
		AttemptNo:           previousAttemptNo + 1,
	}
}
