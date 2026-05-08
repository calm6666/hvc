package hoststats

import "time"

// Snapshot 表示一次主机资源采样结果。
type Snapshot struct {
	CPUUsagePercent       int
	MemoryUsagePercent    int
	GPUMemoryUsagePercent int
	GPUDevices            []GPUDeviceSnapshot
	CollectedAt           time.Time
}

// GPUDeviceSnapshot 表示单张 GPU 的运行时采样结果。
//
// 这里只放“这一刻的负载快照”，不放编解码能力这类相对稳定的信息；
// 能力信息仍由 internal/infra/gpu 负责探测。
type GPUDeviceSnapshot struct {
	GPUIndex              int
	GPUUUID               string
	MemoryUsedMB          int
	MemoryTotalMB         int
	MemoryUsagePercent    int
	GPUUtilizationPercent int
}

// Collector 负责跨平台收集主机资源指标。
//
// 设计目标：
// 1. 不把采样逻辑散落到 Worker / 调度器；
// 2. CPU 采用“前后两次采样差值”模式，避免把瞬时累计时间直接当成使用率；
// 3. 内存和 GPU 显存尽量读取主机真实值，读不到时再优雅降级。
type Collector struct {
	cpu cpuSampler
}

// NewCollector 创建主机指标采样器。
func NewCollector() *Collector {
	return &Collector{
		cpu: newCPUSampler(),
	}
}

// Collect 返回当前主机指标快照。
func (c *Collector) Collect() Snapshot {
	snapshot := Snapshot{
		CollectedAt: time.Now(),
	}
	if c != nil && c.cpu != nil {
		snapshot.CPUUsagePercent = clampPercent(c.cpu.Percent())
	}
	snapshot.MemoryUsagePercent = clampPercent(memoryUsagePercent())
	snapshot.GPUDevices = gpuDeviceSnapshots()
	snapshot.GPUMemoryUsagePercent = clampPercent(aggregateGPUMemoryUsagePercent(snapshot.GPUDevices))
	return snapshot
}

func clampPercent(value int) int {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}
