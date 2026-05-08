package scheduler

import (
	"hvc/internal/model"
	"math/rand"
	"sort"
)

// ScoreCandidate 返回候选节点分值。
//
// 分值越低表示节点负载越轻，越应该被优先选中。
// 权重分配：
//   - 活跃转码会话数 × 30：最关键的资源指标
//   - GPU 显存使用率 × 20：GPU 是转码核心资源
//   - CPU 使用率 × 15：软解或混合场景的重要指标
//   - 内存使用率 × 10：辅助指标
//   - 上传队列深度 × 10：I/O 压力指标
func ScoreCandidate(candidate model.DispatchCandidate) int {
	score := 0
	score += candidate.Metrics.ActiveTranscodeSessions * 30
	score += candidate.Metrics.CPUUsagePercent * 15
	score += candidate.Metrics.MemoryUsagePercent * 10
	score += candidate.Metrics.GPUMemoryUsagePercent * 20
	score += candidate.Metrics.UploadQueueDepth * 10
	return score
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
