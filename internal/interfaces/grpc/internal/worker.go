// Package internal 提供 gRPC 内部服务接口定义及默认实现。
//
// 内部服务用于 Worker 与调度器 / 控制面之间的通信，
// 不对外暴露，仅限集群内部节点间调用。
//
// 包含以下接口方法：
//   - ReportHeartbeat: Worker 心跳上报，用于调度器判断节点存活
//   - ReportMetrics: 节点资源指标上报（CPU/GPU/内存/上传队列等）
//   - RenewLease: 任务租约续租，防止任务被重新分配
//   - ReportUploadFailed: 分片上传失败回传
//   - ReportUploadSucceeded: 分片上传成功回传
//
// 集群模式：请求通过 gRPC 发送到调度器节点
// 单机模式：请求直接在本地处理，不经过网络
package internal

import (
	"context"
	"fmt"

	"hvc/internal/cluster"
	"hvc/internal/infra/db/mysql"
	rediscache "hvc/internal/infra/cache/redis"
	"hvc/internal/model"
	"hvc/pkg/logx"
)

// WorkerInternalServer 表示 gRPC 内部服务接口。
type WorkerInternalServer interface {
	ReportHeartbeat(req model.HeartbeatRequest) error
	ReportMetrics(req model.MetricsRequest) error
	RenewLease(req model.LeaseRenewRequest) error
	ReportUploadFailed(req model.SegmentUploadFailedRequest) error
	ReportUploadSucceeded(req model.SegmentUploadedRequest) error
}

// SegmentUploadedRequest 表示分片上传成功回传请求。
type SegmentUploadedRequest = model.UploadResult

// workerInternalServer 是 WorkerInternalServer 的默认实现。
type workerInternalServer struct {
	stateCache        *cluster.StateCache
	progressStore     *rediscache.ProgressStore
	segmentRepository *mysql.SegmentRepository
	jobExecutionRepo  *mysql.JobExecutionRepository
}

// NewWorkerInternalServer 创建 gRPC 内部服务实现。
//
// 参数：
//   - stateCache: 集群状态缓存，用于保存心跳和指标
//   - progressStore: Redis 进度缓存
//   - segmentRepository: 分片仓储，用于更新上传状态
//   - jobExecutionRepo: 执行实例仓储，用于更新心跳
func NewWorkerInternalServer(
	stateCache *cluster.StateCache,
	progressStore *rediscache.ProgressStore,
	segmentRepository *mysql.SegmentRepository,
	jobExecutionRepo *mysql.JobExecutionRepository,
) WorkerInternalServer {
	return &workerInternalServer{
		stateCache:        stateCache,
		progressStore:     progressStore,
		segmentRepository: segmentRepository,
		jobExecutionRepo:  jobExecutionRepo,
	}
}

// ReportHeartbeat 处理 Worker 心跳上报。
//
// 心跳是调度器判断节点存活的核心机制：
//   - 调度器根据心跳新鲜度过滤可用节点
//   - 心跳超时的节点会被自动屏蔽，不再分配新任务
//   - 心跳恢复后自动解除屏蔽
//
// 集群模式：心跳写入共享 Redis，所有调度器实例可见
// 单机模式：心跳写入本地内存缓存
func (s *workerInternalServer) ReportHeartbeat(req model.HeartbeatRequest) error {
	ctx := context.Background()

	if s.stateCache != nil {
		s.stateCache.SaveHeartbeat(ctx, model.WorkerHeartbeat{
			NodeID:             req.NodeID,
			WorkerID:           req.WorkerID,
			StartupInstanceID:  req.StartupInstanceID,
			MachineFingerprint: req.MachineFingerprint,
			Timestamp:          req.Timestamp,
		})
	}

	logx.Info("grpc.internal.heartbeat_received", logx.Fields{
		"node_id":   req.NodeID,
		"worker_id": req.WorkerID,
	})
	return nil
}

// ReportMetrics 处理节点指标上报。
//
// 指标用于调度器的节点选择和负载均衡：
//   - CPU 使用率：判断节点是否过载
//   - 内存使用率：防止 OOM
//   - GPU 显存使用率：限制 GPU 并发会话数
//   - 上传队列深度：防止上传积压
//   - 活跃转码会话数：控制并发任务数
//   - GPU 能力列表：匹配任务硬件需求
//
// 单机模式：指标写入本地内存缓存
// 集群模式：指标写入共享 Redis，所有调度器实例可见
func (s *workerInternalServer) ReportMetrics(req model.MetricsRequest) error {
	ctx := context.Background()

	if s.stateCache != nil {
		s.stateCache.SaveNodeMetrics(ctx, model.NodeMetrics{
			NodeID:                  req.NodeID,
			CPUUsagePercent:         req.CPUUsagePercent,
			MemoryUsagePercent:      req.MemoryUsagePercent,
			GPUMemoryUsagePercent:   req.GPUMemoryUsagePercent,
			UploadQueueDepth:        req.UploadQueueDepth,
			ActiveTranscodeSessions: req.ActiveTranscodeSessions,
			GPUCapabilities:         req.GPUCapabilities,
			Timestamp:               req.Timestamp,
		})
	}

	logx.Info("grpc.internal.metrics_received", logx.Fields{
		"node_id":  req.NodeID,
		"cpu":      req.CPUUsagePercent,
		"mem":      req.MemoryUsagePercent,
		"gpu_mem":  req.GPUMemoryUsagePercent,
		"sessions": req.ActiveTranscodeSessions,
	})
	return nil
}

// RenewLease 处理任务租约续租。
//
// 租约续租是分布式任务协调的关键机制：
//   - Worker 执行任务期间必须定期续租
//   - 续租失败（租约过期）意味着任务可能被重新分配
//   - 调度器通过租约 generation 号防止脑裂
//
// 续租成功后更新执行实例的心跳时间。
func (s *workerInternalServer) RenewLease(req model.LeaseRenewRequest) error {
	ctx := context.Background()

	if s.jobExecutionRepo != nil {
		if err := s.jobExecutionRepo.TouchHeartbeat(ctx, req.JobID, req.LeaseGeneration); err != nil {
			logx.Error("grpc.internal.renew_lease_failed", err, logx.Fields{
				"job_id":  req.JobID,
				"worker_id": req.WorkerID,
			})
			return fmt.Errorf("续租失败: %w", err)
		}
	}

	logx.Info("grpc.internal.lease_renewed", logx.Fields{
		"job_id":           req.JobID,
		"worker_id":        req.WorkerID,
		"lease_generation": req.LeaseGeneration,
	})
	return nil
}

// ReportUploadFailed 处理分片上传失败回传。
//
// 上传失败回传用于调度器感知节点上传健康状况：
//   - 连续上传失败可能触发节点降级或屏蔽
//   - 调度器可据此调整后续任务的节点选择权重
func (s *workerInternalServer) ReportUploadFailed(req model.SegmentUploadFailedRequest) error {
	ctx := context.Background()

	if s.segmentRepository != nil {
		if err := s.segmentRepository.MarkUploadFailed(ctx, req.SegmentID, req.ErrorMessage); err != nil {
			logx.Error("grpc.internal.upload_failed_report_error", err, logx.Fields{
				"segment_id": req.SegmentID,
			})
			return err
		}
	}

	logx.Info("grpc.internal.upload_failed", logx.Fields{
		"segment_id":    req.SegmentID,
		"retry_count":   req.RetryCount,
		"error_message": req.ErrorMessage,
	})
	return nil
}

// ReportUploadSucceeded 处理分片上传成功回传。
//
// 上传成功回传更新分片状态为已上传，记录 ETag 和文件大小。
func (s *workerInternalServer) ReportUploadSucceeded(req model.SegmentUploadedRequest) error {
	ctx := context.Background()

	if s.segmentRepository != nil {
		if err := s.segmentRepository.MarkUploaded(ctx, req.SegmentID, req.ObjectETag, req.ObjectSizeBytes); err != nil {
			logx.Error("grpc.internal.upload_succeeded_report_error", err, logx.Fields{
				"segment_id": req.SegmentID,
			})
			return err
		}
	}

	logx.Info("grpc.internal.upload_succeeded", logx.Fields{
		"segment_id":        req.SegmentID,
		"object_etag":       req.ObjectETag,
		"object_size_bytes": req.ObjectSizeBytes,
	})
	return nil
}
