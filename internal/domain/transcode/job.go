// Package transcode 提供转码任务领域对象。
//
// Job 表示一个转码任务，包含源视频信息、输出规格、调度状态等。
// 领域对象封装了业务规则和状态判断逻辑，不包含持久化操作。
//
// 任务生命周期：
//
//	CREATED → QUEUED → ASSIGNED → RUNNING → UPLOADING → COMPLETED
//	                                          ↓
//	                                       FAILED
//	                                          ↓
//	                                    FAILED_RETRYABLE
//
// 支持集群模式和单机模式：
//   - 集群模式：任务可被调度到任意节点执行
//   - 单机模式：任务在本机执行，跳过调度环节
package transcode

import (
	"fmt"
	"time"
)

// Job 表示转码任务领域对象。
type Job struct {
	JobID                uint64
	RequestID            string
	BizKey               string
	SourceURL            string
	ProfileID            uint64
	Priority             int
	Status               int
	ProgressPermille     int
	ProgressStage        string
	SelectedExecutionHWAccel string
	SelectedGPUIndex     int
	AssignedNodeID       uint64
	AssignedWorkerID     string
	LeaseGeneration      uint64
	LeaseExpireAt        time.Time
	EnableWatermark      bool
	SegmentDurationSec   int
	SupportDash          bool
	SupportHLS           bool
	OutputBasePrefix     string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// IsTerminal 判断任务是否处于终态。
//
// 终态任务不会再被调度器处理：
//   - COMPLETED: 任务成功完成
//   - FAILED: 任务失败
//   - CANCELED: 任务被取消
func (j *Job) IsTerminal() bool {
	return j.Status == JobStatusCompleted ||
		j.Status == JobStatusFailed ||
		j.Status == JobStatusCanceled
}

// IsRunning 判断任务是否正在执行。
func (j *Job) IsRunning() bool {
	return j.Status == JobStatusRunning ||
		j.Status == JobStatusUploading
}

// CanRetry 判断任务是否可以重试。
func (j *Job) CanRetry() bool {
	return j.Status == JobStatusFailed
}

// CanCancel 判断任务是否可以取消。
func (j *Job) CanCancel() bool {
	return j.Status == JobStatusCreated ||
		j.Status == JobStatusQueued ||
		j.Status == JobStatusAssigned
}

// IsLeaseExpired 判断任务租约是否已过期。
func (j *Job) IsLeaseExpired() bool {
	if j.LeaseExpireAt.IsZero() {
		return false
	}
	return time.Now().After(j.LeaseExpireAt)
}

// AssignTo 将任务分配给指定节点和 Worker。
func (j *Job) AssignTo(nodeID uint64, workerID string, leaseTTL time.Duration) {
	j.AssignedNodeID = nodeID
	j.AssignedWorkerID = workerID
	j.LeaseGeneration++
	j.LeaseExpireAt = time.Now().Add(leaseTTL)
	j.Status = JobStatusAssigned
}

// StatusName 返回任务状态的中文描述。
func (j *Job) StatusName() string {
	switch j.Status {
	case JobStatusCreated:
		return "已创建"
	case JobStatusQueued:
		return "排队中"
	case JobStatusAssigned:
		return "已分配"
	case JobStatusRunning:
		return "转码中"
	case JobStatusUploading:
		return "上传中"
	case JobStatusCompleted:
		return "已完成"
	case JobStatusFailed:
		return "失败"
	case JobStatusCanceled:
		return "已取消"
	default:
		return fmt.Sprintf("未知(%d)", j.Status)
	}
}

// 任务状态常量。
const (
	JobStatusCreated   = 1
	JobStatusQueued    = 2
	JobStatusAssigned  = 3
	JobStatusRunning   = 4
	JobStatusUploading = 5
	JobStatusCompleted = 6
	JobStatusFailed    = 7
	JobStatusCanceled  = 8
)

// 进度阶段常量。
const (
	StageDownloading = "DOWNLOADING"
	StageProbing     = "PROBING"
	StageTranscoding = "TRANSCODING"
	StageUploading   = "UPLOADING"
	StageFinalizing  = "FINALIZING"
	StageCompleted   = "COMPLETED"
	StageFailed      = "FAILED"
	StageQueued      = "QUEUED"
)

// 硬件执行类型常量。
const (
	ExecutionHWSoftware = "software"
	ExecutionHWNVIDIA   = "nvidia"
	ExecutionHWQSV      = "qsv"
	ExecutionHWAMF      = "amf"
	ExecutionHVVAAPI    = "vaapi"
	ExecutionHWVideoToolbox = "videotoolbox"
)
