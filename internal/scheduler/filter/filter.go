package filter

import (
	"hvc/internal/config"
	"hvc/internal/model"
)

type Filter struct {
	cfg config.DynamicRuntimeConfig
}

func NewFilter(cfg config.DynamicRuntimeConfig) *Filter {
	return &Filter{cfg: cfg}
}

// Apply 返回通过硬约束校验的候选节点。
//
// 过滤维度包括：
//  1. 节点启用与隔离状态（Enabled / Quarantined）
//  2. CPU / 内存 / GPU 显存安全水位
//  3. 并发会话数与上传队列深度
//  4. 水印硬件能力校验
//  5. 请求指定的 PreferredHWAccel 与节点 GPU 执行模式匹配
//  6. 请求指定的编码格式（codec）与节点 GPU 编码能力匹配
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
		if !f.matchHWAccel(req, candidate) {
			continue
		}
		if !f.matchCodec(req, candidate) {
			continue
		}
		passed = append(passed, candidate)
	}
	return passed
}

// matchHWAccel 校验请求指定的硬件加速类型是否被候选节点支持。
//
// PreferredHWAccel 表示"希望走哪一种执行模式"（如 nvidia、intel_qsv），
// 必须拿它去匹配 GPUCapability.ExecutionHWTypes，而不是和 EncodeCodecs 混在一起比较。
func (f *Filter) matchHWAccel(req model.CreateJobRequest, candidate model.DispatchCandidate) bool {
	if req.ScheduleOptions == nil || req.ScheduleOptions.PreferredHWAccel == "" {
		return true
	}
	for _, capability := range candidate.Metrics.GPUCapabilities {
		for _, hwType := range capability.ExecutionHWTypes {
			if hwType == req.ScheduleOptions.PreferredHWAccel {
				return true
			}
		}
	}
	return false
}

// matchCodec 校验请求指定的编码格式是否被候选节点的 GPU 支持。
//
// 当请求携带 VideoOptions 或 Renditions 中指定了 video_codec 时，
// 需要确保候选节点至少有一张 GPU 能编码该格式。
// 如果请求未指定编码格式，则默认 h264，不做强制校验。
func (f *Filter) matchCodec(req model.CreateJobRequest, candidate model.DispatchCandidate) bool {
	codec := resolveRequiredCodec(req)
	if codec == "" {
		return true
	}
	for _, capability := range candidate.Metrics.GPUCapabilities {
		for _, encCodec := range capability.EncodeCodecs {
			if encCodec == codec {
				return true
			}
		}
	}
	if f.cfg.Scheduler.AllowSoftwareDecodeFallback {
		return true
	}
	return false
}

// resolveRequiredCodec 从请求中提取需要校验的编码格式。
//
// 优先级：Renditions[0].VideoCodec > VideoOptions 中的默认编码 > 空（不校验）
func resolveRequiredCodec(req model.CreateJobRequest) string {
	if len(req.Renditions) > 0 && req.Renditions[0].VideoCodec != "" {
		return req.Renditions[0].VideoCodec
	}
	return ""
}
