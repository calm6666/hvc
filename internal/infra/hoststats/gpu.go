package hoststats

import (
	"strconv"
	"strings"
)

// aggregateGPUMemoryUsagePercent 返回所有 GPU 的平均显存使用率。
//
// 这里聚合值仍然保留，供节点级过滤器做快速硬阈值判断；
// 更细粒度的“节点内选哪张卡”则依赖每张卡自己的 runtime snapshot。
func aggregateGPUMemoryUsagePercent(items []GPUDeviceSnapshot) int {
	if len(items) == 0 {
		return 0
	}
	total := 0
	count := 0
	for _, item := range items {
		if item.MemoryUsagePercent < 0 {
			continue
		}
		total += item.MemoryUsagePercent
		count++
	}
	if count == 0 {
		return 0
	}
	return total / count
}

// gpuDeviceSnapshots 返回当前机器所有可采样 GPU 的运行时快照。
//
// 当前阶段优先支持 nvidia-smi，因为这是项目当前最主要的生产形态。
// 采不到时直接返回空列表，让调度器回退到节点级指标和会话数，而不是中断 Worker。
func gpuDeviceSnapshots() []GPUDeviceSnapshot {
	output := commandOutput("nvidia-smi", "--query-gpu=index,uuid,memory.used,memory.total,utilization.gpu", "--format=csv,noheader,nounits")
	lines := splitNonEmptyLines(output)
	if len(lines) == 0 {
		return nil
	}
	items := make([]GPUDeviceSnapshot, 0, len(lines))
	for _, line := range lines {
		parts := strings.Split(line, ",")
		if len(parts) < 5 {
			continue
		}
		gpuIndex, err0 := strconv.Atoi(strings.TrimSpace(parts[0]))
		usedMB, err1 := strconv.Atoi(strings.TrimSpace(parts[2]))
		totalMB, err2 := strconv.Atoi(strings.TrimSpace(parts[3]))
		utilPercent, err3 := strconv.Atoi(strings.TrimSpace(parts[4]))
		if err0 != nil || err1 != nil || err2 != nil || err3 != nil || totalMB <= 0 || usedMB < 0 {
			continue
		}
		items = append(items, GPUDeviceSnapshot{
			GPUIndex:              gpuIndex,
			GPUUUID:               strings.TrimSpace(parts[1]),
			MemoryUsedMB:          usedMB,
			MemoryTotalMB:         totalMB,
			MemoryUsagePercent:    int((int64(usedMB) * 100) / int64(totalMB)),
			GPUUtilizationPercent: utilPercent,
		})
	}
	return items
}
