package filter

import (
	"hvc/internal/config"
	"hvc/internal/model"
)

// Filter 表示调度过滤器。
type Filter struct {
	cfg config.DynamicRuntimeConfig
}

// NewFilter 创建调度过滤器。
func NewFilter(cfg config.DynamicRuntimeConfig) *Filter {
	return &Filter{cfg: cfg}
}

// Apply 返回通过硬约束校验的候选节点。
func (f *Filter) Apply(req model.CreateJobRequest, candidates []model.DispatchCandidate) []model.DispatchCandidate {
	passed := make([]model.DispatchCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if !candidate.Enabled || candidate.Quarantined {
			continue
		}
		if candidate.Metrics.CPUUsagePercent >= f.cfg.Scheduler.NodeCPUSafetyLimitPercent {
			continue
		}
		if candidate.Metrics.MemoryUsagePercent >= f.cfg.Scheduler.NodeMemorySafetyLimitPercent {
			continue
		}
		if candidate.Metrics.GPUMemoryUsagePercent >= f.cfg.Scheduler.NodeGPUSafetyLimitPercent {
			continue
		}
		if candidate.Metrics.ActiveTranscodeSessions >= f.cfg.Scheduler.MaxNodeTranscodeSessions {
			continue
		}
		if candidate.Metrics.UploadQueueDepth >= f.cfg.Scheduler.MaxNodeUploadConcurrency {
			continue
		}
		if req.EnableWatermark && f.cfg.Scheduler.RequireHardwareWatermark && !candidate.SupportsHardwareWatermark {
			continue
		}
		if req.ScheduleOptions != nil && req.ScheduleOptions.PreferredHWAccel != "" {
			matched := false
			for _, capability := range candidate.Metrics.GPUCapabilities {
				for _, codec := range capability.EncodeCodecs {
					if codec == req.ScheduleOptions.PreferredHWAccel {
						matched = true
						break
					}
				}
				if matched {
					break
				}
			}
			if !matched {
				continue
			}
		}
		passed = append(passed, candidate)
	}
	return passed
}
