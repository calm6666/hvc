package scheduler

import (
	"context"
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
	jobs := m.jobRepository.ListQueued(ctx)
	if len(jobs) == 0 {
		return
	}
	candidates := m.collectCandidates(ctx)
	if len(candidates) == 0 {
		return
	}
	for _, job := range jobs {
		req := m.buildJobRequest(ctx, job)
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
		preferredHWAccel := m.resolvePreferredHWAccel(ctx, job)
		decision := dispatchpkg.BuildDecision(best, preferredHWAccel, job.LeaseGeneration, job.AttemptNo)
		_ = m.jobRepository.Assign(ctx, job.JobID, decision, best.NodeID, m.workerID, job.ExecutorWorkerInstanceID)
		if m.jobExecutionRepository != nil {
			_ = m.jobExecutionRepository.SaveAssigned(ctx, job.JobID, decision, m.workerID, job.ExecutorWorkerInstanceID)
		}
		if m.shieldTracker != nil {
			m.shieldTracker.RecordSuccess(best.NodeID)
		}
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

	if m.clusterNodeRepository != nil {
		nodes := m.clusterNodeRepository.List(ctx)
		for _, node := range nodes {
			if !node.Enabled {
				continue
			}
			if m.shieldTracker != nil && m.shieldTracker.IsShielded(node.NodeID) {
				continue
			}
			metrics, ok := m.clusterCache.GetNodeMetrics(ctx, node.NodeID)
			if !ok {
				metrics = model.NodeMetrics{NodeID: node.NodeID}
			}
			candidates = append(candidates, model.DispatchCandidate{
				NodeID:                    node.NodeID,
				Enabled:                   node.Enabled,
				Quarantined:               node.Quarantined,
				SupportsHardwareWatermark: node.SupportNVENC || node.SupportQSV || node.SupportAMF,
				Metrics:                   metrics,
			})
		}
	}

	if len(candidates) == 0 {
		metrics, ok := m.clusterCache.GetNodeMetrics(ctx, m.nodeID)
		if !ok {
			metrics = model.NodeMetrics{NodeID: m.nodeID}
		}
		candidates = append(candidates, model.DispatchCandidate{
			NodeID:                    m.nodeID,
			Enabled:                   true,
			Quarantined:               false,
			SupportsHardwareWatermark: true,
			Metrics:                   metrics,
		})
	}

	return candidates
}

// buildJobRequest 从任务模型构造调度请求。
func (m *Manager) buildJobRequest(ctx context.Context, job model.TranscodeJob) model.CreateJobRequest {
	req := model.CreateJobRequest{
		RequestID:       job.RequestID,
		SourceURL:       job.SourceURL,
		ProfileID:       job.ProfileID,
		Priority:        job.Priority,
		EnableWatermark: job.EnableWatermark,
	}
	preferredHWAccel := m.resolvePreferredHWAccel(ctx, job)
	if preferredHWAccel != "" {
		req.ScheduleOptions = &model.ScheduleOptions{PreferredHWAccel: preferredHWAccel}
	}
	return req
}

// resolvePreferredHWAccel 解析任务的硬件加速偏好。
func (m *Manager) resolvePreferredHWAccel(ctx context.Context, job model.TranscodeJob) string {
	if m.jobRequestOverrideRepo != nil {
		if override, ok := m.jobRequestOverrideRepo.FindByJobID(ctx, job.JobID); ok && override.OverridePreferredHWAccel != "" {
			return override.OverridePreferredHWAccel
		}
	}
	return ""
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
