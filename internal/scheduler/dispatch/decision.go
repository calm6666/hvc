package scheduler

import (
	"hvc/internal/model"
	"math/rand"
	"sort"
)

// ScoreCandidate 返回候选节点分值。
//
// 分值越低表示节点负载越轻，越应该被优先选中。
// 当前实现不再只看“绝对会话数”，而是优先看“当前负载 / 可承载容量”。
// 这样多 GPU / 大容量节点不会因为绝对会话数更高就被过早打压，
// 能更合理地利用单机多卡与集群多机的整体吞吐能力。
func ScoreCandidate(candidate model.DispatchCandidate) int {
	score := 0
	if !candidate.MetricsFresh {
		score += 1_000_000
	}
	if !candidate.MetricsAvailable {
		score += 1_000_000
	}
	if !candidate.Online {
		score += 1_000_000
	}
	score += normalizedLoadScore(candidate.Metrics.ActiveTranscodeSessions, candidate.MaxTranscodeSessions) * 35
	score += candidate.Metrics.GPUMemoryUsagePercent * 25
	score += candidate.Metrics.CPUUsagePercent * 15
	score += candidate.Metrics.MemoryUsagePercent * 10
	score += normalizedLoadScore(candidate.Metrics.UploadQueueDepth, candidate.MaxUploadConcurrency) * 10
	score += residualCapacityPenalty(candidate.Metrics.ActiveTranscodeSessions, candidate.MaxTranscodeSessions) * 10
	score += uploadResidualCapacityPenalty(candidate.Metrics.UploadQueueDepth, candidate.MaxUploadConcurrency) * 5
	score += gpuScarcityPenalty(candidate.Metrics.GPUCapabilities)
	return score
}

func normalizedLoadScore(current, capacity int) int {
	if current <= 0 {
		return 0
	}
	if capacity <= 0 {
		return current * 10
	}
	return current * 100 / capacity
}

// scoredCandidate 内部结构，用于排序和 Top-K 选择。
type scoredCandidate struct {
	candidate model.DispatchCandidate
	score     int
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

// PickTopKRandom 从候选节点中选择 Top-K 个最优节点，再随机选一个返回。
//
// 策略说明：
//  1. 将所有候选节点按分值从小到大排序；
//  2. 取前 K 个（K = min(topK, len(candidates))）作为候选池；
//  3. 从候选池中随机选一个。
//
// 这样既保证选中的节点负载较轻，又避免所有任务都集中到同一节点，
// 实现负载均衡与随机分散的平衡。
func PickTopKRandom(candidates []model.DispatchCandidate, topK int) (model.DispatchCandidate, bool) {
	if len(candidates) == 0 {
		return model.DispatchCandidate{}, false
	}
	if topK <= 0 {
		topK = 3
	}
	scored := make([]scoredCandidate, 0, len(candidates))
	for _, c := range candidates {
		scored = append(scored, scoredCandidate{candidate: c, score: ScoreCandidate(c)})
	}
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score < scored[j].score
	})
	k := topK
	if k > len(scored) {
		k = len(scored)
	}
	picked := scored[rand.Intn(k)]
	return picked.candidate, true
}

// BuildDecision 生成调度结果。
//
// 当前策略已经收敛为“先选节点，再在节点内选最合适的 GPU”：
// 1. 若请求指定 PreferredHWAccel，则优先挑能满足该执行模式的卡；
// 2. 节点内多卡按 active_sessions / gpu_memory_usage / gpu_utilization 综合评分；
// 3. 若节点没有 GPU，则明确回落为 software，并把 selected_gpu_index 写成 -1。
func BuildDecision(candidate model.DispatchCandidate, preferredHWAccel string, previousLeaseGeneration uint64, previousAttemptNo int) model.DispatchDecision {
	selectedGPUIndex := -1
	selectedGPUDeviceID := uint64(0)
	selectedExecutionHW := model.ExecutionHWSoftware
	if capability, ok := pickBestGPU(candidate.Metrics.GPUCapabilities, preferredHWAccel); ok {
		selectedGPUIndex = capability.GPUIndex
		selectedGPUDeviceID = capability.GPUDeviceID
		if selectedGPUDeviceID == 0 {
			selectedGPUDeviceID = uint64(capability.GPUIndex + 1)
		}
		selectedExecutionHW = resolveExecutionHW(capability, preferredHWAccel)
	}
	return model.DispatchDecision{
		NodeID:              candidate.NodeID,
		SelectedGPUIndex:    selectedGPUIndex,
		SelectedGPUDeviceID: selectedGPUDeviceID,
		SelectedExecutionHW: selectedExecutionHW,
		LeaseGeneration:     previousLeaseGeneration + 1,
		AttemptNo:           previousAttemptNo + 1,
	}
}

func pickBestGPU(capabilities []model.GPUCapability, preferredHWAccel string) (model.GPUCapability, bool) {
	if len(capabilities) == 0 {
		return model.GPUCapability{}, false
	}

	filtered := make([]model.GPUCapability, 0, len(capabilities))
	for _, capability := range capabilities {
		if preferredHWAccel != "" && !supportsExecutionHW(capability, preferredHWAccel) {
			continue
		}
		if capability.MaxSessions > 0 && capability.ActiveSessions >= capability.MaxSessions {
			continue
		}
		filtered = append(filtered, capability)
	}
	if len(filtered) == 0 {
		for _, capability := range capabilities {
			if capability.MaxSessions > 0 && capability.ActiveSessions >= capability.MaxSessions {
				continue
			}
			filtered = append(filtered, capability)
		}
	}
	if len(filtered) == 0 {
		return model.GPUCapability{}, false
	}

	best := filtered[0]
	bestScore := scoreGPU(best)
	for _, capability := range filtered[1:] {
		score := scoreGPU(capability)
		if score < bestScore || (score == bestScore && capability.GPUIndex < best.GPUIndex) {
			best = capability
			bestScore = score
		}
	}
	return best, true
}

func scoreGPU(capability model.GPUCapability) int {
	score := 0
	score += capability.ActiveSessions * 10000
	score += capability.GPUMemoryUsagePercent * 100
	score += capability.GPUUtilizationPercent * 10
	score += capability.GPUIndex
	return score
}

func gpuScarcityPenalty(capabilities []model.GPUCapability) int {
	if len(capabilities) == 0 {
		return 500
	}
	schedulable := 0
	totalFreeSlots := 0
	for _, capability := range capabilities {
		if capability.MaxSessions > 0 && capability.ActiveSessions >= capability.MaxSessions {
			continue
		}
		schedulable++
		if capability.MaxSessions > 0 {
			totalFreeSlots += capability.MaxSessions - capability.ActiveSessions
			continue
		}
		totalFreeSlots += 1
	}
	if schedulable == 0 {
		return 50_000
	}
	penalty := 0
	penalty += (len(capabilities) - schedulable) * 300
	if totalFreeSlots > 0 {
		penalty += 1000 / totalFreeSlots
	}
	if schedulable == 1 {
		penalty += 250
	}
	return penalty
}

func residualCapacityPenalty(current, capacity int) int {
	if capacity <= 0 {
		return 0
	}
	remaining := capacity - current
	switch {
	case remaining <= 0:
		return 1000
	case remaining == 1:
		return 300
	case remaining == 2:
		return 120
	default:
		return 0
	}
}

func uploadResidualCapacityPenalty(current, capacity int) int {
	if capacity <= 0 {
		return 0
	}
	remaining := capacity - current
	switch {
	case remaining <= 0:
		return 500
	case remaining == 1:
		return 150
	default:
		return 0
	}
}

func supportsExecutionHW(capability model.GPUCapability, preferredHWAccel string) bool {
	for _, hwType := range capability.ExecutionHWTypes {
		if hwType == preferredHWAccel {
			return true
		}
	}
	return false
}

func resolveExecutionHW(capability model.GPUCapability, preferredHWAccel string) string {
	if preferredHWAccel != "" && supportsExecutionHW(capability, preferredHWAccel) {
		return preferredHWAccel
	}
	if len(capability.ExecutionHWTypes) > 0 {
		return capability.ExecutionHWTypes[0]
	}
	return model.ExecutionHWSoftware
}
