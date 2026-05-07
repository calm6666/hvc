// Package dto 提供 gRPC 接口的数据传输对象定义。
package dto

import (
	"time"

	"hvc/internal/model"
)

// WorkerHeartbeatPayload Worker 心跳上报载荷。
type WorkerHeartbeatPayload struct {
	NodeID             uint64 `json:"node_id"`
	WorkerID           string `json:"worker_id"`
	StartupInstanceID  string `json:"startup_instance_id"`
	MachineFingerprint string `json:"machine_fingerprint"`
	TimestampMS        int64  `json:"timestamp_ms"`
}

// ToModel 将 DTO 转换为领域模型。
func (p *WorkerHeartbeatPayload) ToModel() model.HeartbeatRequest {
	return model.HeartbeatRequest{
		NodeID:             p.NodeID,
		WorkerID:           p.WorkerID,
		StartupInstanceID:  p.StartupInstanceID,
		MachineFingerprint: p.MachineFingerprint,
		Timestamp:          time.UnixMilli(p.TimestampMS),
	}
}

// WorkerMetricsPayload Worker 指标上报载荷。
type WorkerMetricsPayload struct {
	NodeID                  uint64                `json:"node_id"`
	WorkerID                string                `json:"worker_id"`
	StartupInstanceID       string                `json:"startup_instance_id"`
	MachineFingerprint      string                `json:"machine_fingerprint"`
	CPUUsagePercent         int                   `json:"cpu_usage_percent"`
	MemoryUsagePercent      int                   `json:"memory_usage_percent"`
	GPUMemoryUsagePercent   int                   `json:"gpu_memory_usage_percent"`
	UploadQueueDepth        int                   `json:"upload_queue_depth"`
	ActiveTranscodeSessions int                   `json:"active_transcode_sessions"`
	GPUCapabilities         []model.GPUCapability `json:"gpu_capabilities,omitempty"`
	TimestampMS             int64                 `json:"timestamp_ms"`
}

// ToModel 将 DTO 转换为领域模型。
func (p *WorkerMetricsPayload) ToModel() model.MetricsRequest {
	return model.MetricsRequest{
		NodeID:                  p.NodeID,
		WorkerID:                p.WorkerID,
		StartupInstanceID:       p.StartupInstanceID,
		MachineFingerprint:      p.MachineFingerprint,
		CPUUsagePercent:         p.CPUUsagePercent,
		MemoryUsagePercent:      p.MemoryUsagePercent,
		GPUMemoryUsagePercent:   p.GPUMemoryUsagePercent,
		UploadQueueDepth:        p.UploadQueueDepth,
		ActiveTranscodeSessions: p.ActiveTranscodeSessions,
		GPUCapabilities:         p.GPUCapabilities,
		Timestamp:               time.UnixMilli(p.TimestampMS),
	}
}

// LeaseRenewPayload 租约续租载荷。
type LeaseRenewPayload struct {
	JobID           uint64    `json:"job_id"`
	WorkerID        string    `json:"worker_id"`
	LeaseGeneration uint64    `json:"lease_generation"`
	ExpireAt        time.Time `json:"expire_at"`
}

// ToModel 将 DTO 转换为领域模型。
func (p *LeaseRenewPayload) ToModel() model.LeaseRenewRequest {
	return model.LeaseRenewRequest{
		JobID:           p.JobID,
		WorkerID:        p.WorkerID,
		LeaseGeneration: p.LeaseGeneration,
		ExpireAt:        p.ExpireAt,
	}
}

// SegmentUploadResultPayload 分片上传结果载荷。
type SegmentUploadResultPayload struct {
	SegmentID       uint64 `json:"segment_id"`
	JobID           uint64 `json:"job_id"`
	ObjectKey       string `json:"object_key"`
	ObjectETag      string `json:"object_etag"`
	ObjectSizeBytes int64  `json:"object_size_bytes"`
	Success         bool   `json:"success"`
	ErrorMessage    string `json:"error_message,omitempty"`
	RetryCount      int    `json:"retry_count"`
}

// ToUploadResult 转换为 UploadResult 模型。
func (p *SegmentUploadResultPayload) ToUploadResult() model.UploadResult {
	return model.UploadResult{
		SegmentID:       p.SegmentID,
		ObjectETag:      p.ObjectETag,
		ObjectSizeBytes: uint64(p.ObjectSizeBytes),
	}
}
