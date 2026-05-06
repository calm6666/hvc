package scheduler

import (
	"context"
	"time"

	"hvc/internal/cluster"
	"hvc/internal/config"
	"hvc/internal/infra/db/mysql"
	"hvc/internal/model"
	dispatchpkg "hvc/internal/scheduler/dispatch"
	filterpkg "hvc/internal/scheduler/filter"
	"hvc/pkg/logx"
)

// Manager 表示调度模块。
type Manager struct {
	cfg           config.DynamicRuntimeConfig
	nodeID        uint64
	workerID      string
	filter        *filterpkg.Filter
	clusterCache  *cluster.StateCache
	jobRepository *mysql.JobRepository
}

// NewManager 创建调度模块。
func NewManager(cfg config.DynamicRuntimeConfig, nodeID uint64, workerID string, clusterCache *cluster.StateCache, jobRepository *mysql.JobRepository) *Manager {
	return &Manager{
		cfg:           cfg,
		nodeID:        nodeID,
		workerID:      workerID,
		filter:        filterpkg.NewFilter(cfg),
		clusterCache:  clusterCache,
		jobRepository: jobRepository,
	}
}

// Start 启动调度模块。
func (m *Manager) Start(ctx context.Context) error {
	ticker := time.NewTicker(m.cfg.Scheduler.LoopInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			m.dispatchOnce(ctx)
		}
	}
}

func (m *Manager) dispatchOnce(ctx context.Context) {
	jobs := m.jobRepository.ListQueued(ctx)
	if len(jobs) == 0 {
		return
	}
	metrics, ok := m.clusterCache.GetNodeMetrics(ctx, m.nodeID)
	if !ok {
		metrics = model.NodeMetrics{NodeID: m.nodeID}
	}
	candidate := model.DispatchCandidate{
		NodeID:                    m.nodeID,
		Enabled:                   true,
		Quarantined:               false,
		SupportsHardwareWatermark: true,
		Metrics:                   metrics,
	}
	for _, job := range jobs {
		req := model.CreateJobRequest{
			RequestID:       job.RequestID,
			SourceURL:       job.SourceURL,
			ProfileID:       job.ProfileID,
			Priority:        job.Priority,
			EnableWatermark: job.EnableWatermark,
		}
		passed := m.filter.Apply(req, []model.DispatchCandidate{candidate})
		best, ok := dispatchpkg.PickBestCandidate(passed)
		if !ok {
			logx.Info("scheduler.dispatch.skipped", logx.Fields{
				"job_id":     job.JobID,
				"request_id": job.RequestID,
				"reason":     "no_candidate",
			})
			continue
		}
		decision := dispatchpkg.BuildDecision(best, job.LeaseGeneration, job.AttemptNo)
		_ = m.jobRepository.Assign(ctx, job.JobID, decision, m.nodeID, m.workerID)
		logx.Info("scheduler.dispatch.assigned", logx.Fields{
			"job_id":           job.JobID,
			"request_id":       job.RequestID,
			"node_id":          decision.NodeID,
			"lease_generation": decision.LeaseGeneration,
			"attempt_no":       decision.AttemptNo,
			"gpu_index":        decision.SelectedGPUIndex,
		})
	}
}
