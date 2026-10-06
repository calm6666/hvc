package scheduler

import (
	"context"
	"errors"
	"time"

	"hvc/internal/cluster"
	"hvc/internal/config"
	"hvc/internal/configcenter"
	"hvc/internal/infra/db/mysql"
	"hvc/internal/model"
	dispatchpkg "hvc/internal/scheduler/dispatch"
	"hvc/internal/scheduler/failover"
	filterpkg "hvc/internal/scheduler/filter"
	"hvc/pkg/logx"
)

// Manager 表示调度模块。
//
// 调度模块负责：
//  1. 周期性扫描排队任务；
//  2. 从集群中收集所有可用节点作为候选；
//  3. 通过过滤器筛选合规候选；
//  4. 使用 Top-K 随机策略选择最优节点；
//  5. 将任务分配给选中节点；
//  6. 定期扫描过期租约并触发故障接管。
type Manager struct {
	cfg                    config.DynamicRuntimeConfig
	effectiveConfig        *configcenter.EffectiveConfig
	nodeID                 uint64
	workerID               string
	clusterCache           *cluster.StateCache
	leaseCache             *cluster.LeaseCache
	jobRepository          *mysql.JobRepository
	jobRequestOverrideRepo *mysql.JobRequestOverrideRepository
	jobExecutionRepository *mysql.JobExecutionRepository
	clusterNodeRepository  *mysql.ClusterNodeRepository
	shieldTracker          *failover.ShieldTracker
	topK                   int
}

const schedulerNodeOnlineGraceMultiplier = 2

// NewManager 创建调度模块。
func NewManager(cfg config.DynamicRuntimeConfig, effectiveConfig *configcenter.EffectiveConfig, nodeID uint64, workerID string, clusterCache *cluster.StateCache, jobRepository *mysql.JobRepository, jobRequestOverrideRepo *mysql.JobRequestOverrideRepository, jobExecutionRepository *mysql.JobExecutionRepository) *Manager {
	return &Manager{
		cfg:                    cfg,
		effectiveConfig:        effectiveConfig,
		nodeID:                 nodeID,
		workerID:               workerID,
		clusterCache:           clusterCache,
		jobRepository:          jobRepository,
		jobRequestOverrideRepo: jobRequestOverrideRepo,
		jobExecutionRepository: jobExecutionRepository,
		shieldTracker:          failover.NewShieldTracker(failover.DefaultShieldConfig()),
		topK:                   3,
	}
}

// SetLeaseCache 注入租约缓存。
func (m *Manager) SetLeaseCache(leaseCache *cluster.LeaseCache) {
	m.leaseCache = leaseCache
}

// SetClusterNodeRepository 注入集群节点仓储。
func (m *Manager) SetClusterNodeRepository(repo *mysql.ClusterNodeRepository) {
	m.clusterNodeRepository = repo
}

// SetTopK 设置 Top-K 随机选择参数。
func (m *Manager) SetTopK(k int) {
	if k > 0 {
		m.topK = k
	}
}

// Start 启动调度模块。
func (m *Manager) Start(ctx context.Context) error {
	for {
		cfg := m.currentConfig()
		if cfg.Mode.EnableScheduler {
			m.dispatchOnce(ctx, cfg)
			m.failoverOnce(ctx, cfg)
		}
		interval := cfg.Scheduler.LoopInterval
		if interval <= 0 {
			interval = 3 * time.Second
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil
		case <-timer.C:
		}
	}
}

// dispatchOnce 执行一次调度循环。
//
// 流程：
//  1. 拉取排队任务；
//  2. 收集集群中所有可用节点作为候选；
//  3. 对每个任务，过滤合规候选并选择最优节点；
//  4. 将任务分配给选中节点。
func (m *Manager) dispatchOnce(ctx context.Context, cfg config.DynamicRuntimeConfig) {
	activeExecutions := m.activeExecutionCount(ctx, cfg)
	remainingCapacity := resolveDispatchRemainingCapacity(cfg, activeExecutions)
	if remainingCapacity <= 0 {
		logx.Info("scheduler.dispatch.skipped", logx.Fields{
			"reason": "max_global_transcode_sessions_reached",
			"limit":  cfg.Scheduler.MaxGlobalTranscodeSessions,
		})
		return
	}
	jobs := m.jobRepository.ListQueuedForDispatch(ctx, resolveDispatchQueueFetchLimit(remainingCapacity))
	if len(jobs) == 0 {
		return
	}
	jobOverrideByID := m.listJobOverrides(ctx, jobs)
	candidates := m.collectCandidates(ctx)
	if len(candidates) == 0 {
		return
	}
	for _, job := range jobs {
		if remainingCapacity <= 0 {
			return
		}
		req := m.buildJobRequest(job, jobOverrideByID[job.JobID])
		passed := filterpkg.NewFilter(cfg).Apply(req, candidates)
		if len(passed) == 0 {
			logx.Info("scheduler.dispatch.skipped", logx.Fields{
				"job_id":     job.JobID,
				"request_id": job.RequestID,
				"reason":     "no_candidate_passed_filter",
			})
			continue
		}
		best, ok := dispatchpkg.PickTopKRandom(passed, m.topK)
		if !ok {
			logx.Info("scheduler.dispatch.skipped", logx.Fields{
				"job_id":     job.JobID,
				"request_id": job.RequestID,
				"reason":     "no_candidate",
			})
			continue
		}
		preferredHWAccel := m.resolvePreferredHWAccel(jobOverrideByID[job.JobID])
		decision := dispatchpkg.BuildDecision(best, preferredHWAccel, job.LeaseGeneration, job.AttemptNo)
		if err := m.jobRepository.Assign(ctx, job.JobID, decision, best.NodeID, m.workerID, job.ExecutorWorkerInstanceID); err != nil {
			if errors.Is(err, mysql.ErrJobAssignConflict) {
				logx.Info("scheduler.dispatch.skipped", logx.Fields{
					"job_id":     job.JobID,
					"request_id": job.RequestID,
					"reason":     "job_assign_conflict",
				})
				continue
			}
			logx.Error("scheduler.dispatch.assign_failed", err, logx.Fields{
				"job_id":     job.JobID,
				"request_id": job.RequestID,
				"node_id":    best.NodeID,
			})
			continue
		}
		if m.jobExecutionRepository != nil {
			if err := m.jobExecutionRepository.SaveAssigned(ctx, job.JobID, decision, m.workerID, job.ExecutorWorkerInstanceID); err != nil {
				logx.Error("scheduler.dispatch.execution_save_failed", err, logx.Fields{
					"job_id":     job.JobID,
					"request_id": job.RequestID,
					"node_id":    best.NodeID,
				})
			}
		}
		if m.shieldTracker != nil {
			m.shieldTracker.RecordSuccess(best.NodeID)
		}
		remainingCapacity--
		logx.Info("scheduler.dispatch.assigned", logx.Fields{
			"job_id":             job.JobID,
			"request_id":         job.RequestID,
			"node_id":            decision.NodeID,
			"lease_generation":   decision.LeaseGeneration,
			"attempt_no":         decision.AttemptNo,
			"gpu_index":          decision.SelectedGPUIndex,
			"selected_execution": decision.SelectedExecutionHW,
			"candidate_count":    len(passed),
		})
	}
}

// collectCandidates 从集群中收集所有可用节点作为调度候选。
//
// 数据来源：
//   - 集群节点表（t_cluster_node）：获取节点启用/隔离状态
//   - Redis 热路径缓存：获取节点实时指标
//   - 故障屏蔽跟踪器：过滤被隔离的节点
func (m *Manager) collectCandidates(ctx context.Context) []model.DispatchCandidate {
	candidates := make([]model.DispatchCandidate, 0)
	activeGPUUsageByNode := make(map[uint64]map[int]int)
	now := time.Now()
	currentCfg := m.currentConfig()
	metricsFreshAfter := now.Add(-currentCfg.Scheduler.WorkerHeartbeatTimeout)
	onlineGrace := resolveNodeOnlineGracePeriod(currentCfg)

	if m.clusterNodeRepository != nil {
		nodes := m.clusterNodeRepository.ListForScheduler(ctx)
		nodeIDs := make([]uint64, 0, len(nodes))
		for _, node := range nodes {
			nodeIDs = append(nodeIDs, node.NodeID)
		}
		metricsByNode := m.getNodeMetricsBatch(ctx, nodeIDs)
		if m.jobExecutionRepository != nil && len(nodes) > 0 {
			activeGPUUsageByNode = m.jobExecutionRepository.CountActiveGPUUsageByNode(ctx, nodeIDs, metricsFreshAfter)
		}
		for _, node := range nodes {
			if !node.Enabled {
				continue
			}
			if m.shieldTracker != nil && m.shieldTracker.IsShielded(node.NodeID) {
				continue
			}
			metrics, ok := metricsByNode[node.NodeID]
			if !ok {
				metrics = model.NodeMetrics{NodeID: node.NodeID}
			}
			metrics = mergeGPUActiveSessions(metrics, activeGPUUsageByNode[node.NodeID])
			lastMetricsAt := metrics.Timestamp
			candidates = append(candidates, model.DispatchCandidate{
				NodeID:                    node.NodeID,
				NodeName:                  node.NodeName,
				Enabled:                   node.Enabled,
				Quarantined:               node.Quarantined,
				Draining:                  node.Draining,
				SupportsHardwareWatermark: node.SupportNVENC || node.SupportQSV || node.SupportAMF,
				MaxTranscodeSessions:      node.MaxTranscodeSessions,
				MaxUploadConcurrency:      node.MaxUploadConcurrency,
				MetricsAvailable:          ok,
				MetricsFresh:              ok && !lastMetricsAt.IsZero() && !lastMetricsAt.Before(metricsFreshAfter),
				Online:                    isNodeOnlineCandidate(node.LastHeartbeatAt, lastMetricsAt, now, onlineGrace),
				LastMetricsAt:             lastMetricsAt,
				LastHeartbeatAt:           node.LastHeartbeatAt,
				Metrics:                   metrics,
			})
		}
	}

	if len(candidates) == 0 {
		metrics, ok := m.clusterCache.GetNodeMetrics(ctx, m.nodeID)
		if !ok {
			metrics = model.NodeMetrics{NodeID: m.nodeID}
		}
		if m.jobExecutionRepository != nil {
			activeGPUUsageByNode = m.jobExecutionRepository.CountActiveGPUUsageByNode(ctx, []uint64{m.nodeID}, metricsFreshAfter)
		}
		metrics = mergeGPUActiveSessions(metrics, activeGPUUsageByNode[m.nodeID])
		candidates = append(candidates, model.DispatchCandidate{
			NodeID:                    m.nodeID,
			NodeName:                  m.workerID,
			Enabled:                   true,
			Quarantined:               false,
			Draining:                  false,
			SupportsHardwareWatermark: true,
			MaxTranscodeSessions:      currentCfg.Scheduler.MaxNodeTranscodeSessions,
			MaxUploadConcurrency:      currentCfg.Scheduler.MaxNodeUploadConcurrency,
			MetricsAvailable:          ok,
			MetricsFresh:              ok && !metrics.Timestamp.IsZero() && !metrics.Timestamp.Before(metricsFreshAfter),
			Online:                    true,
			LastMetricsAt:             metrics.Timestamp,
			LastHeartbeatAt:           now,
			Metrics:                   metrics,
		})
	}

	return candidates
}

func (m *Manager) getNodeMetricsBatch(ctx context.Context, nodeIDs []uint64) map[uint64]model.NodeMetrics {
	if m.clusterCache == nil || len(nodeIDs) == 0 {
		return map[uint64]model.NodeMetrics{}
	}
	return m.clusterCache.GetNodeMetricsBatch(ctx, nodeIDs)
}

func mergeGPUActiveSessions(metrics model.NodeMetrics, activeByGPU map[int]int) model.NodeMetrics {
	if len(metrics.GPUCapabilities) == 0 || len(activeByGPU) == 0 {
		return metrics
	}
	for idx := range metrics.GPUCapabilities {
		metrics.GPUCapabilities[idx].ActiveSessions = activeByGPU[metrics.GPUCapabilities[idx].GPUIndex]
	}
	return metrics
}

// buildJobRequest 从任务模型和预取的 override 构造调度请求。
func (m *Manager) buildJobRequest(job model.TranscodeJob, override model.TranscodeJobRequestOverride) model.CreateJobRequest {
	req := model.CreateJobRequest{
		RequestID:       job.RequestID,
		SourceURL:       job.SourceURL,
		ProfileID:       job.ProfileID,
		Priority:        job.Priority,
		EnableWatermark: job.EnableWatermark,
		Renditions:      append([]model.RenditionOption(nil), job.Renditions...),
	}
	preferredHWAccel := m.resolvePreferredHWAccel(override)
	if preferredHWAccel != "" {
		req.ScheduleOptions = &model.ScheduleOptions{PreferredHWAccel: preferredHWAccel}
	}
	return req
}

// resolvePreferredHWAccel 解析任务的硬件加速偏好。
func (m *Manager) resolvePreferredHWAccel(override model.TranscodeJobRequestOverride) string {
	return override.OverridePreferredHWAccel
}

func (m *Manager) listJobOverrides(ctx context.Context, jobs []model.TranscodeJob) map[uint64]model.TranscodeJobRequestOverride {
	result := make(map[uint64]model.TranscodeJobRequestOverride)
	if m.jobRequestOverrideRepo == nil || len(jobs) == 0 {
		return result
	}
	jobIDs := make([]uint64, 0, len(jobs))
	for _, job := range jobs {
		jobIDs = append(jobIDs, job.JobID)
	}
	return m.jobRequestOverrideRepo.ListByJobIDs(ctx, jobIDs)
}

// failoverOnce 执行一次故障接管循环。
//
// 扫描过期租约并将对应任务重置为排队状态，
// 同时恢复已到隔离期限的节点。
func (m *Manager) failoverOnce(ctx context.Context, cfg config.DynamicRuntimeConfig) {
	if m.leaseCache == nil {
		return
	}
	takeoverCount := failover.TakeoverExpiredLeases(ctx, m.jobRepository, m.leaseCache, m.shieldTracker, cfg.Scheduler.WorkerHeartbeatTimeout)
	if takeoverCount > 0 {
		logx.Info("scheduler.failover.takeover_completed", logx.Fields{
			"takeover_count": takeoverCount,
		})
	}
	if m.shieldTracker != nil {
		recovered := m.shieldTracker.RecoverShieldedNodes()
		if len(recovered) > 0 {
			logx.Info("scheduler.failover.nodes_recovered", logx.Fields{
				"recovered_node_ids": recovered,
			})
		}
	}
}

func (m *Manager) currentConfig() config.DynamicRuntimeConfig {
	if m.effectiveConfig == nil {
		return m.cfg
	}
	return m.effectiveConfig.Snapshot()
}

func (m *Manager) reachedGlobalCapacity(ctx context.Context, cfg config.DynamicRuntimeConfig) bool {
	return resolveDispatchRemainingCapacity(cfg, m.activeExecutionCount(ctx, cfg)) <= 0
}

func resolveDispatchRemainingCapacity(cfg config.DynamicRuntimeConfig, activeExecutions int64) int64 {
	if cfg.Scheduler.MaxGlobalTranscodeSessions <= 0 {
		return 1<<62 - 1
	}
	remaining := int64(cfg.Scheduler.MaxGlobalTranscodeSessions) - activeExecutions
	if remaining < 0 {
		return 0
	}
	return remaining
}

func resolveDispatchQueueFetchLimit(remainingCapacity int64) int {
	if remainingCapacity <= 0 {
		return 0
	}
	limit := remainingCapacity * 4
	if limit < 20 {
		limit = 20
	}
	if limit > 500 {
		limit = 500
	}
	return int(limit)
}

func resolveNodeOnlineGracePeriod(cfg config.DynamicRuntimeConfig) time.Duration {
	if cfg.Scheduler.WorkerHeartbeatTimeout > 0 {
		return cfg.Scheduler.WorkerHeartbeatTimeout * schedulerNodeOnlineGraceMultiplier
	}
	return 2 * time.Minute
}

func isNodeOnlineCandidate(lastHeartbeatAt time.Time, lastMetricsAt time.Time, now time.Time, grace time.Duration) bool {
	if !lastMetricsAt.IsZero() && now.Sub(lastMetricsAt) <= grace {
		return true
	}
	if !lastHeartbeatAt.IsZero() && now.Sub(lastHeartbeatAt) <= grace {
		return true
	}
	return false
}
