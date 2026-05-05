package model

import "time"

// WorkerHeartbeat 表示 Worker 心跳。
type WorkerHeartbeat struct {
	NodeID             uint64    `json:"node_id"`
	WorkerID           string    `json:"worker_id"`
	StartupInstanceID  string    `json:"startup_instance_id,omitempty"`
	MachineFingerprint string    `json:"machine_fingerprint,omitempty"`
	Timestamp          time.Time `json:"timestamp"`
}

// GPUCapability 表示单张 GPU 的能力信息。
type GPUCapability struct {
	GPUUUID        string   `json:"gpu_uuid"`
	GPUIndex       int      `json:"gpu_index"`
	EncodeCodecs   []string `json:"encode_codecs,omitempty"`
	DecodeCodecs   []string `json:"decode_codecs,omitempty"`
	MaxSessions    int      `json:"max_sessions,omitempty"`
	SupportsFilter bool     `json:"supports_filter,omitempty"`
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
	Enabled                   bool
	Quarantined               bool
	SupportsHardwareWatermark bool
	Metrics                   NodeMetrics
}

// DispatchDecision 表示调度结果。
type DispatchDecision struct {
	NodeID               uint64
	SelectedGPUIndex     int
	SelectedGPUDeviceID  uint64
	SelectedExecutionHW  string
	LeaseGeneration      uint64
	AttemptNo            int
}
