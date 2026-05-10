package model

import "time"

const (
	// ExecutionHWSoftware 表示纯软件执行模式。
	//
	// 这个常量用于统一调度层、Worker 计划层和数据库 selected_execution_hwaccel 字段的取值语义，
	// 避免不同模块分别写 "software"、"cpu"、"soft" 这类不一致的字符串。
	ExecutionHWSoftware = "software"

	// ExecutionHWNVIDIA 表示 NVIDIA 硬件执行模式。
	ExecutionHWNVIDIA = "nvidia"

	// ExecutionHWIntelQSV 表示 Intel QSV 硬件执行模式。
	ExecutionHWIntelQSV = "intel_qsv"

	// ExecutionHWAMDAMF 表示 AMD AMF 硬件执行模式。
	ExecutionHWAMDAMF = "amd_amf"

	// ExecutionHWVAAPI 表示 Linux VAAPI 硬件执行模式。
	ExecutionHWVAAPI = "vaapi"

	// ExecutionHWAppleVideoToolbox 表示 Apple VideoToolbox 硬件执行模式。
	ExecutionHWAppleVideoToolbox = "apple_videotoolbox"
)

// WorkerHeartbeat 表示 Worker 心跳。
type WorkerHeartbeat struct {
	NodeID             uint64    `json:"node_id"`
	WorkerID           string    `json:"worker_id"`
	StartupInstanceID  string    `json:"startup_instance_id,omitempty"`
	MachineFingerprint string    `json:"machine_fingerprint,omitempty"`
	Timestamp          time.Time `json:"timestamp"`
}

// GPUCapability 表示单张 GPU 的能力信息。
//
// 这里故意同时保留“编解码能力列表”和“执行硬件类型列表”两层语义：
// 1. EncodeCodecs / DecodeCodecs 描述这张卡能处理哪些 codec，例如 h264、hevc；
// 2. ExecutionHWTyps 描述这张卡支持哪些执行后端，例如 nvidia、intel_qsv、vaapi；
// 3. 调度偏好 PreferredHWAccel 必须与 ExecutionHWTypes 匹配，而不是拿去和 codec 名称比较。
//
// 这样可以把“编码格式”和“执行模式”从模型层就彻底拆开，避免后续在调度和执行阶段继续混淆。
type GPUCapability struct {
	GPUDeviceID     uint64 `json:"gpu_device_id,omitempty"`
	GPUUUID         string `json:"gpu_uuid"`
	GPUIndex        int    `json:"gpu_index"`
	Vendor          string `json:"vendor,omitempty"`
	Model           string `json:"model,omitempty"`
	DriverVersion   string `json:"driver_version,omitempty"`
	MemoryTotalMB   int    `json:"memory_total_mb,omitempty"`
	GPUMemoryUsedMB int    `json:"gpu_memory_used_mb,omitempty"`
	// GPUMemoryUsagePercent 表示单张 GPU 当前显存使用率。
	//
	// 这个字段是运行时采样值，不是静态能力；
	// 调度器会优先基于它做多卡节点内的选卡决策。
	GPUMemoryUsagePercent int `json:"gpu_memory_usage_percent,omitempty"`
	// GPUUtilizationPercent 表示单张 GPU 当前核心利用率。
	//
	// 当前优先从 nvidia-smi 等主机侧数据源采样；采不到时保留为 0。
	GPUUtilizationPercent int `json:"gpu_utilization_percent,omitempty"`
	// ActiveSessions 表示当前节点上这张卡正在承载的活跃转码会话数。
	//
	// 这个值用于节点内多卡调度，优先减少“第一张卡被打满、其它卡空闲”的问题。
	ActiveSessions   int      `json:"active_sessions,omitempty"`
	EncodeCodecs     []string `json:"encode_codecs,omitempty"`
	DecodeCodecs     []string `json:"decode_codecs,omitempty"`
	ExecutionHWTypes []string `json:"execution_hw_types,omitempty"`
	MaxSessions      int      `json:"max_sessions,omitempty"`
	SupportsFilter   bool     `json:"supports_filter,omitempty"`
}

// NodeMetrics 表示节点实时指标。
type NodeMetrics struct {
	NodeID                  uint64          `json:"node_id"`
	CPUUsagePercent         int             `json:"cpu_usage_percent"`
	MemoryUsagePercent      int             `json:"memory_usage_percent"`
	GPUMemoryUsagePercent   int             `json:"gpu_memory_usage_percent"`
	UploadQueueDepth        int             `json:"upload_queue_depth"`
	ActiveTranscodeSessions int             `json:"active_transcode_sessions"`
	GPUCapabilities         []GPUCapability `json:"gpu_capabilities,omitempty"`
	Timestamp               time.Time       `json:"timestamp"`
}

// DispatchCandidate 表示调度候选节点。
type DispatchCandidate struct {
	NodeID                    uint64
	NodeName                  string
	Enabled                   bool
	Quarantined               bool
	Draining                  bool
	SupportsHardwareWatermark bool
	MaxTranscodeSessions      int
	MaxUploadConcurrency      int
	MetricsAvailable          bool
	MetricsFresh              bool
	Online                    bool
	LastMetricsAt             time.Time
	LastHeartbeatAt           time.Time
	Metrics                   NodeMetrics
}

// DispatchDecision 表示调度结果。
//
// 调度结果需要同时给出：
// 1. 节点；
// 2. 选中的 GPU 索引；
// 3. 稳定 GPU 设备 ID；
// 4. 最终选中的执行模式。
//
// 其中 SelectedExecutionHW 必须与数据库 selected_execution_hwaccel 语义一致，
// 后续 Worker 执行计划和任务回写也应复用同一套常量值。
type DispatchDecision struct {
	NodeID              uint64
	SelectedGPUIndex    int
	SelectedGPUDeviceID uint64
	SelectedExecutionHW string
	LeaseGeneration     uint64
	AttemptNo           int
}
