// Package transcode 提供转码任务领域对象。
//
// Job 表示一个转码任务，包含源视频信息、输出规格、调度状态等。
// 领域对象封装了业务规则和状态判断逻辑，不包含持久化操作。
//
// 任务生命周期（A1 回调状态机）：
//
//	CREATED → QUEUED → ASSIGNED → RUNNING → UPLOADING → COMPLETED
//	                                          ↓              ↓
//	                                       FAILED        PUBLISHED（清单物化 + 分片逐片可 GET 且 sha256 通过）
//	                                          ↓              ↓
//	                                    FAILED_RETRYABLE  CALLBACK_SENT（完成回调已由至少一个通道送达）
//
// 语义要点：COMPLETED 只表示"内容已产出"；对外承诺（下游可拉取 + 已收到回调）由
// PUBLISHED 与 CALLBACK_SENT 两级保证。回调必须等**本任务待传分片数归零**才允许发出。
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
//   - COMPLETED: 内容已产出，等"待传分片归零"后进入 PUBLISHED
//   - PUBLISHED: 已发布（清单物化、分片可 GET、sha256 通过），等回调送达
//   - CALLBACK_SENT: 回调已送达（对外承诺兑现的最终终态）
//   - FAILED: 任务失败
//   - CANCELED: 任务被取消
func (j *Job) IsTerminal() bool {
	return j.Status == JobStatusCompleted ||
		j.Status == JobStatusPublished ||
		j.Status == JobStatusCallbackSent ||
		j.Status == JobStatusFailed ||
		j.Status == JobStatusCanceled
}

// CanPublish 判断任务能否从"已完成"进入"已发布"。
//
// 前置条件由调用方保证：本任务待传分片数已归零（见 sql/107 的 completion_payload 暂存列），
// 且清单已物化、每个分片可 GET 且 sha256 校验通过。
func (j *Job) CanPublish() bool {
	return j.Status == JobStatusCompleted
}

// CanSendCallback 判断任务能否从"已发布"进入"回调已送达"。
//
// 只在 PUBLISHED 之后允许发回调：这样下游收到回调去拉清单时，内容一定已经完整可读。
func (j *Job) CanSendCallback() bool {
	return j.Status == JobStatusPublished
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
	case JobStatusPublished:
		return "已发布"
	case JobStatusCallbackSent:
		return "回调已送达"
	default:
		return fmt.Sprintf("未知(%d)", j.Status)
	}
}

// 任务状态常量（与 internal/model/status.go 的取值保持一致）。
const (
	JobStatusCreated      = 1
	JobStatusQueued       = 2
	JobStatusAssigned     = 3
	JobStatusRunning      = 4
	JobStatusUploading    = 5
	JobStatusCompleted    = 6
	JobStatusFailed       = 7
	JobStatusCanceled     = 8
	JobStatusPublished    = 9  // 已发布：清单物化 + 分片可 GET + sha256 通过
	JobStatusCallbackSent = 10 // 回调已送达：至少一个通道成功
)

// 进度阶段常量。
const (
	StageDownloading  = "DOWNLOADING"
	StageProbing      = "PROBING"
	StageTranscoding  = "TRANSCODING"
	StageUploading    = "UPLOADING"
	StageFinalizing   = "FINALIZING"
	StageCompleted    = "COMPLETED"
	StagePublished    = "PUBLISHED"
	StageCallbackSent = "CALLBACK_SENT"
	StageFailed       = "FAILED"
	StageQueued       = "QUEUED"
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
