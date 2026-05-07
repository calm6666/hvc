package model

import "time"

// HeartbeatRequest 表示心跳上报请求。
type HeartbeatRequest struct {
	NodeID             uint64    `json:"node_id"`
	WorkerID           string    `json:"worker_id"`
	StartupInstanceID  string    `json:"startup_instance_id,omitempty"`
	MachineFingerprint string    `json:"machine_fingerprint,omitempty"`
	Timestamp          time.Time `json:"timestamp"`
}

// MetricsRequest 表示节点指标上报请求。
type MetricsRequest struct {
	NodeID                  uint64          `json:"node_id"`
	WorkerID                string          `json:"worker_id,omitempty"`
	StartupInstanceID       string          `json:"startup_instance_id,omitempty"`
	MachineFingerprint      string          `json:"machine_fingerprint,omitempty"`
	CPUUsagePercent         int             `json:"cpu_usage_percent"`
	MemoryUsagePercent      int             `json:"memory_usage_percent"`
	GPUMemoryUsagePercent   int             `json:"gpu_memory_usage_percent"`
	UploadQueueDepth        int             `json:"upload_queue_depth"`
	ActiveTranscodeSessions int             `json:"active_transcode_sessions"`
	GPUCapabilities         []GPUCapability `json:"gpu_capabilities,omitempty"`
	Timestamp               time.Time       `json:"timestamp"`
}
