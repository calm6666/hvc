package filter

import (
	"fmt"
	"hvc/internal/config"
	"hvc/internal/model"
	"time"
)

// Filter 根据运行配置对候选节点进行硬约束过滤。
type Filter struct {
	cfg config.DynamicRuntimeConfig
}

// Evaluation 表示单个候选节点的过滤结论。
//
// Apply 仍然用于真正的调度流程；
// Evaluate 额外暴露拒绝原因，供后台调度洞察接口直接复用。
type Evaluation struct {
	Passed  bool
	Reasons []string
}

// NewFilter 创建节点过滤器。
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
		if !f.Evaluate(req, candidate).Passed {
			continue
		}
		passed = append(passed, candidate)
	}
	return passed
}

// Evaluate 返回候选节点是否通过过滤，以及具体拒绝原因。
func (f *Filter) Evaluate(req model.CreateJobRequest, candidate model.DispatchCandidate) Evaluation {
	reasons := make([]string, 0, 8)
	now := time.Now()
	heartbeatGrace := resolveHeartbeatGracePeriod(f.cfg)

	if !candidate.Enabled {
		reasons = append(reasons, "node_disabled")
	}
	if candidate.Quarantined {
		reasons = append(reasons, "node_quarantined")
	}
	if candidate.Draining {
		reasons = append(reasons, "node_draining")
	}
	if !candidate.Online {
		reasons = append(reasons, "node_offline")
	}
	if !candidate.MetricsAvailable {
		reasons = append(reasons, "metrics_missing")
	}
	if !candidate.MetricsFresh {
		reasons = append(reasons, "metrics_stale")
	}
	if !candidate.LastHeartbeatAt.IsZero() && now.Sub(candidate.LastHeartbeatAt) > heartbeatGrace {
		reasons = append(reasons, fmt.Sprintf("heartbeat_stale(>%s)", heartbeatGrace.String()))
	}
	if limit := f.cfg.Scheduler.NodeCPUSafetyLimitPercent; limit > 0 && candidate.Metrics.CPUUsagePercent >= limit {
		reasons = append(reasons, fmt.Sprintf("cpu_usage_percent(%d)>=limit(%d)", candidate.Metrics.CPUUsagePercent, limit))
	}
	if limit := f.cfg.Scheduler.NodeMemorySafetyLimitPercent; limit > 0 && candidate.Metrics.MemoryUsagePercent >= limit {
		reasons = append(reasons, fmt.Sprintf("memory_usage_percent(%d)>=limit(%d)", candidate.Metrics.MemoryUsagePercent, limit))
	}
	if limit := f.cfg.Scheduler.NodeGPUSafetyLimitPercent; limit > 0 && candidate.Metrics.GPUMemoryUsagePercent >= limit {
		reasons = append(reasons, fmt.Sprintf("gpu_memory_usage_percent(%d)>=limit(%d)", candidate.Metrics.GPUMemoryUsagePercent, limit))
	}
	if limit := f.cfg.Scheduler.MaxNodeTranscodeSessions; limit > 0 && candidate.Metrics.ActiveTranscodeSessions >= limit {
		reasons = append(reasons, fmt.Sprintf("active_transcode_sessions(%d)>=limit(%d)", candidate.Metrics.ActiveTranscodeSessions, limit))
	}
	if limit := f.cfg.Scheduler.MaxNodeUploadConcurrency; limit > 0 && candidate.Metrics.UploadQueueDepth >= limit {
		reasons = append(reasons, fmt.Sprintf("upload_queue_depth(%d)>=limit(%d)", candidate.Metrics.UploadQueueDepth, limit))
	}
	if req.EnableWatermark && f.cfg.Scheduler.RequireHardwareWatermark && !candidate.SupportsHardwareWatermark {
		reasons = append(reasons, "hardware_watermark_required_but_not_supported")
	}
	if !f.matchHWAccel(req, candidate) {
		reasons = append(reasons, "preferred_hwaccel_not_supported")
	}
	if !f.matchGPUCapacity(req, candidate) {
		reasons = append(reasons, "no_gpu_capacity_for_request")
	}
	if !f.matchCodec(req, candidate) {
		reasons = append(reasons, "codec_not_supported")
	}

	return Evaluation{
		Passed:  len(reasons) == 0,
		Reasons: reasons,
	}
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

// matchGPUCapacity 校验候选节点内是否至少还有一张可承载新任务的 GPU。
//
// 约束：
// 1. 如果请求显式指定 PreferredHWAccel，则必须至少有一张支持该执行模式且未满会话的卡；
// 2. 如果系统配置要求硬编，则也必须至少有一张未满会话的卡；
// 3. 如果系统允许软编兜底，则“节点没有卡 / 卡已满”不作为硬过滤条件，留给后续 software 路径处理。
func (f *Filter) matchGPUCapacity(req model.CreateJobRequest, candidate model.DispatchCandidate) bool {
	preferredHW := ""
	if req.ScheduleOptions != nil {
		preferredHW = req.ScheduleOptions.PreferredHWAccel
	}

	requireGPU := preferredHW != "" || f.cfg.Scheduler.RequireHardwareEncode
	if len(candidate.Metrics.GPUCapabilities) == 0 {
		return !requireGPU
	}

	for _, capability := range candidate.Metrics.GPUCapabilities {
		if preferredHW != "" && !supportsExecutionHW(capability, preferredHW) {
			continue
		}
		if capability.MaxSessions > 0 && capability.ActiveSessions >= capability.MaxSessions {
			continue
		}
		return true
	}

	return !requireGPU
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

func supportsExecutionHW(capability model.GPUCapability, preferredHWAccel string) bool {
	for _, hwType := range capability.ExecutionHWTypes {
		if hwType == preferredHWAccel {
			return true
		}
	}
	return false
}

func resolveHeartbeatGracePeriod(cfg config.DynamicRuntimeConfig) time.Duration {
	if cfg.Scheduler.WorkerHeartbeatTimeout > 0 {
		return cfg.Scheduler.WorkerHeartbeatTimeout * 2
	}
	return time.Minute
}
