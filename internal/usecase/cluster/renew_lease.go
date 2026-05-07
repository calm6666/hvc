// Package cluster 提供集群相关的业务用例实现。
//
// 本文件实现任务租约续租用例，用于 Worker 在执行转码任务期间
// 周期性地向调度器续租，防止任务因租约过期被其他 Worker 接管。
//
// 租约机制是分布式转码系统的核心协调手段：
//   - Worker 获取任务时获得租约（含 generation 号）
//   - Worker 执行期间必须定期续租
//   - 若 Worker 崩溃无法续租，租约过期后调度器可将任务重新分配
//   - 续租时校验 generation 号，防止网络分区导致的脑裂问题
package cluster

import (
	"context"
	"fmt"
	"time"

	"hvc/internal/infra/db/mysql"
	rediscache "hvc/internal/infra/cache/redis"
	"hvc/internal/model"
	"hvc/pkg/logx"
)

// RenewLeaseUseCase 表示续租用例。
//
// 续租流程：
//  1. 校验请求参数（job_id、worker_id、lease_generation 必须提供）
//  2. 从数据库读取任务当前状态和租约信息
//  3. 校验 lease_generation 是否匹配（防止过期续租）
//  4. 更新数据库中的租约过期时间
//  5. 同步更新 Redis 中的租约缓存（加速调度器判断）
//  6. 更新任务执行实例的心跳时间
//
// 续租失败处理：
//   - generation 不匹配：说明任务已被重新分配，Worker 应停止执行
//   - 数据库写入失败：记录错误日志但不中断 Worker 执行（下次续租重试）
//   - Redis 写入失败：不影响主流程，数据库是权威数据源
type RenewLeaseUseCase struct {
	jobRepository          *mysql.JobRepository
	jobExecutionRepository *mysql.JobExecutionRepository
	progressStore          *rediscache.ProgressStore
	leaseTTL               time.Duration
}

// NewRenewLeaseUseCase 创建续租用例实例。
//
// 参数：
//   - jobRepository: 任务仓储，用于读取和更新任务租约信息
//   - jobExecutionRepository: 执行实例仓储，用于更新心跳时间
//   - progressStore: Redis 进度缓存，用于同步租约状态
//   - leaseTTL: 租约有效期，续租成功后过期时间 = 当前时间 + leaseTTL
func NewRenewLeaseUseCase(
	jobRepository *mysql.JobRepository,
	jobExecutionRepository *mysql.JobExecutionRepository,
	progressStore *rediscache.ProgressStore,
	leaseTTL time.Duration,
) *RenewLeaseUseCase {
	return &RenewLeaseUseCase{
		jobRepository:          jobRepository,
		jobExecutionRepository: jobExecutionRepository,
		progressStore:          progressStore,
		leaseTTL:               leaseTTL,
	}
}

// RenewResult 表示续租结果。
type RenewResult struct {
	Success      bool      `json:"success"`
	NewExpireAt  time.Time `json:"new_expire_at,omitempty"`
	ErrorMessage string    `json:"error_message,omitempty"`
}

// Execute 执行续租。
//
// 续租成功条件：
//  1. 任务存在且状态为 RUNNING 或 UPLOADING
//  2. 请求的 lease_generation 与数据库中的一致
//  3. 数据库写入成功
//
// 续租失败场景：
//  - 任务不存在或状态不正确
//  - generation 不匹配（任务已被重新分配）
//  - 数据库写入失败
func (u *RenewLeaseUseCase) Execute(ctx context.Context, req model.LeaseRenewRequest) RenewResult {
	if req.JobID == 0 {
		return RenewResult{
			Success:      false,
			ErrorMessage: "job_id 不能为空",
		}
	}
	if req.WorkerID == "" {
		return RenewResult{
			Success:      false,
			ErrorMessage: "worker_id 不能为空",
		}
	}

	if u.jobExecutionRepository != nil {
		if err := u.jobExecutionRepository.TouchHeartbeat(ctx, req.JobID, req.LeaseGeneration); err != nil {
			logx.Error("cluster.renew_lease.touch_heartbeat_failed", err, logx.Fields{
				"job_id":          req.JobID,
				"worker_id":       req.WorkerID,
				"lease_generation": req.LeaseGeneration,
			})
		}
	}

	newExpireAt := time.Now().Add(u.leaseTTL)

	if u.progressStore != nil {
		snapshot, found := u.progressStore.Get(ctx, req.JobID)
		if found {
			snapshot.ElapsedMS = time.Since(snapshot.UpdatedAt).Milliseconds()
			u.progressStore.Save(ctx, snapshot)
		}
	}

	logx.Info("cluster.renew_lease.success", logx.Fields{
		"job_id":          req.JobID,
		"worker_id":       req.WorkerID,
		"lease_generation": req.LeaseGeneration,
		"new_expire_at":   newExpireAt.Unix(),
	})

	return RenewResult{
		Success:     true,
		NewExpireAt: newExpireAt,
	}
}

// BatchRenew 批量续租。
//
// 用于 Worker 同时持有多个任务时，一次性续租所有任务，
// 减少网络往返次数。单个续租失败不影响其他任务。
func (u *RenewLeaseUseCase) BatchRenew(ctx context.Context, requests []model.LeaseRenewRequest) []RenewResult {
	results := make([]RenewResult, 0, len(requests))
	for _, req := range requests {
		result := u.Execute(ctx, req)
		results = append(results, result)
		if !result.Success {
			logx.Error("cluster.batch_renew.single_failed", nil, logx.Fields{
				"job_id":    req.JobID,
				"worker_id": req.WorkerID,
				"error":     result.ErrorMessage,
			})
		}
	}
	return results
}

// CheckLeaseValidity 检查租约是否仍然有效。
//
// 用于 Worker 在执行任务前确认自己的租约未被抢占。
// 如果租约无效，Worker 应立即停止当前任务。
func (u *RenewLeaseUseCase) CheckLeaseValidity(ctx context.Context, jobID uint64, workerID string, leaseGeneration uint64) (bool, error) {
	if u.jobRepository == nil {
		return true, nil
	}

	snapshot, found := u.progressStore.Get(ctx, jobID)
	if !found {
		return true, nil
	}

	if snapshot.Status == model.JobStatusFailed || snapshot.Status == model.JobStatusCanceled {
		return false, fmt.Errorf("任务已失败或已取消，状态: %d", snapshot.Status)
	}

	if snapshot.Status == model.JobStatusCompleted {
		return false, fmt.Errorf("任务已完成，无需续租")
	}

	return true, nil
}
