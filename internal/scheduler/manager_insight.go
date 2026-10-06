package scheduler

import (
	"context"
	"sort"
	"strings"
	"time"

	"hvc/internal/config"
	"hvc/internal/infra/db/mysql"
	"hvc/internal/model"
	dispatchpkg "hvc/internal/scheduler/dispatch"
	filterpkg "hvc/internal/scheduler/filter"
)

// DispatchInsight 表示当前调度器视角下的集群快照。
//
// 这个结构主要供后台运维接口消费，用于解释：
// 1. 当前有哪些候选节点；
// 2. 为什么某些节点被过滤；
// 3. 如果此刻分配一个任务，Top-K 池和推荐结果会是什么。
type DispatchInsight struct {
	GeneratedAt                time.Time
	TopK                       int
	DispatchPolicy             string
	MaxGlobalTranscodeSessions int
	ActiveExecutionCount       int64
	QueuedJobCount             int64
	CandidateTotal             int
	PassedCandidateTotal       int
	RecommendedNodeID          uint64
	RecommendedDecision        *model.DispatchDecision
	Candidates                 []CandidateInsight
}

// CandidateInsight 表示单个候选节点的调度评估结果。
type CandidateInsight struct {
	NodeID                    uint64
	NodeName                  string
	NodeRole                  string
	Enabled                   bool
	Quarantined               bool
	Draining                  bool
	ControlPlane              bool
	AdminAccessible           bool
	Shielded                  bool
	SupportsHardwareWatermark bool
	MaxTranscodeSessions      int
	MaxUploadConcurrency      int
	MetricsAvailable          bool
	MetricsFresh              bool
	Online                    bool
	OnlineSignalSource        string
	SchedulerReady            bool
	StateReason               string
	LastMetricsAt             time.Time
	LastHeartbeatAt           time.Time
	Score                     int
	ScoreRank                 int
	FilterPassed              bool
	FilterReasons             []string
	InTopKPool                bool
	DecisionPreview           *model.DispatchDecision
	Metrics                   model.NodeMetrics
}

type candidateState struct {
	candidate          model.DispatchCandidate
	shielded           bool
	nodeRole           string
	controlPlane       bool
	adminAccessible    bool
	onlineSignalSource string
	schedulerReady     bool
	stateReason        string
}

// BuildInsight 构建当前调度器快照。
func (m *Manager) BuildInsight(ctx context.Context, req model.CreateJobRequest) DispatchInsight {
	cfg := m.currentConfig()
	states := m.collectCandidateStates(ctx, cfg)
	filter := filterpkg.NewFilter(cfg)

	evaluations := make(map[uint64]filterpkg.Evaluation, len(states))
	passed := make([]model.DispatchCandidate, 0, len(states))
	for _, state := range states {
		evaluation := filter.Evaluate(req, state.candidate)
		if state.shielded {
			evaluation.Passed = false
			evaluation.Reasons = append([]string{"node_shielded_by_failover"}, evaluation.Reasons...)
		}
		evaluations[state.candidate.NodeID] = evaluation
		if evaluation.Passed {
			passed = append(passed, state.candidate)
		}
	}

	sort.Slice(passed, func(i, j int) bool {
		left := dispatchpkg.ScoreCandidate(passed[i])
		right := dispatchpkg.ScoreCandidate(passed[j])
		if left == right {
			return passed[i].NodeID < passed[j].NodeID
		}
		return left < right
	})

	topK := m.topK
	if topK <= 0 {
		topK = 3
	}
	if topK > len(passed) {
		topK = len(passed)
	}

	topKSet := make(map[uint64]struct{}, topK)
	scoreRank := make(map[uint64]int, len(passed))
	for idx, candidate := range passed {
		scoreRank[candidate.NodeID] = idx + 1
		if idx < topK {
			topKSet[candidate.NodeID] = struct{}{}
		}
	}

	var recommendedDecision *model.DispatchDecision
	var recommendedNodeID uint64
	preferredHWAccel := ""
	if req.ScheduleOptions != nil {
		preferredHWAccel = req.ScheduleOptions.PreferredHWAccel
	}
	if len(passed) > 0 {
		decision := dispatchpkg.BuildDecision(passed[0], preferredHWAccel, 0, 0)
		recommendedDecision = &decision
		recommendedNodeID = passed[0].NodeID
	}

	items := make([]CandidateInsight, 0, len(states))
	for _, state := range states {
		evaluation := evaluations[state.candidate.NodeID]
		item := CandidateInsight{
			NodeID:                    state.candidate.NodeID,
			NodeName:                  state.candidate.NodeName,
			NodeRole:                  state.nodeRole,
			Enabled:                   state.candidate.Enabled,
			Quarantined:               state.candidate.Quarantined,
			Draining:                  state.candidate.Draining,
			ControlPlane:              state.controlPlane,
			AdminAccessible:           state.adminAccessible,
			Shielded:                  state.shielded,
			SupportsHardwareWatermark: state.candidate.SupportsHardwareWatermark,
			MaxTranscodeSessions:      state.candidate.MaxTranscodeSessions,
			MaxUploadConcurrency:      state.candidate.MaxUploadConcurrency,
			MetricsAvailable:          state.candidate.MetricsAvailable,
			MetricsFresh:              state.candidate.MetricsFresh,
			Online:                    state.candidate.Online,
			OnlineSignalSource:        state.onlineSignalSource,
			SchedulerReady:            state.schedulerReady,
			StateReason:               state.stateReason,
			LastMetricsAt:             state.candidate.LastMetricsAt,
			LastHeartbeatAt:           state.candidate.LastHeartbeatAt,
			Score:                     dispatchpkg.ScoreCandidate(state.candidate),
			ScoreRank:                 scoreRank[state.candidate.NodeID],
			FilterPassed:              evaluation.Passed,
			FilterReasons:             evaluation.Reasons,
			Metrics:                   state.candidate.Metrics,
		}
		if _, ok := topKSet[state.candidate.NodeID]; ok {
			item.InTopKPool = true
		}
		if evaluation.Passed {
			decision := dispatchpkg.BuildDecision(state.candidate, preferredHWAccel, 0, 0)
			item.DecisionPreview = &decision
		}
		items = append(items, item)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].FilterPassed != items[j].FilterPassed {
			return items[i].FilterPassed
		}
		if items[i].Score != items[j].Score {
			return items[i].Score < items[j].Score
		}
		return items[i].NodeID < items[j].NodeID
	})

	queuedJobCount := int64(0)
	if m.jobRepository != nil {
		queuedJobCount = m.jobRepository.CountByStatus(ctx, model.JobStatusQueued)
	}

	return DispatchInsight{
		GeneratedAt:                time.Now(),
		TopK:                       topK,
		DispatchPolicy:             "top_k_random",
		MaxGlobalTranscodeSessions: cfg.Scheduler.MaxGlobalTranscodeSessions,
		ActiveExecutionCount:       m.activeExecutionCount(ctx, cfg),
		QueuedJobCount:             queuedJobCount,
		CandidateTotal:             len(states),
		PassedCandidateTotal:       len(passed),
		RecommendedNodeID:          recommendedNodeID,
		RecommendedDecision:        recommendedDecision,
		Candidates:                 items,
	}
}

func (m *Manager) activeExecutionCount(ctx context.Context, cfg config.DynamicRuntimeConfig) int64 {
	if m.jobExecutionRepository != nil {
		activeAfter := time.Now().Add(-cfg.Scheduler.WorkerHeartbeatTimeout)
		return m.jobExecutionRepository.CountActiveExecutions(ctx, activeAfter)
	}
	if m.jobRepository != nil {
		return m.jobRepository.CountActive(ctx)
	}
	return 0
}

func (m *Manager) collectCandidateStates(ctx context.Context, cfg config.DynamicRuntimeConfig) []candidateState {
	now := time.Now()
	metricsFreshAfter := now.Add(-cfg.Scheduler.WorkerHeartbeatTimeout)
	onlineGrace := resolveNodeOnlineGracePeriod(cfg)
	activeGPUUsageByNode := make(map[uint64]map[int]int)
	shieldSnapshot := m.shieldSnapshot()
	items := make([]candidateState, 0)

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
			metrics, ok := metricsByNode[node.NodeID]
			if !ok {
				metrics = model.NodeMetrics{NodeID: node.NodeID}
			}
			metrics = mergeGPUActiveSessions(metrics, activeGPUUsageByNode[node.NodeID])
			lastMetricsAt := metrics.Timestamp
			_, shielded := shieldSnapshot[node.NodeID]
			nodeRole := schedulerNodeRoleFromTags(node.NodeTags)
			controlPlane, adminAccessible := schedulerEvaluateNodeManagementCapability(nodeRole, node.Enabled, node.HTTPHost)
			candidate := model.DispatchCandidate{
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
			}
			schedulerReady, stateReason := evaluateSchedulerCandidateReadiness(cfg, node, candidate)
			items = append(items, candidateState{
				shielded:           shielded,
				candidate:          candidate,
				nodeRole:           nodeRole,
				controlPlane:       controlPlane,
				adminAccessible:    adminAccessible,
				onlineSignalSource: resolveSchedulerOnlineSignalSource(candidate),
				schedulerReady:     schedulerReady,
				stateReason:        stateReason,
			})
		}
	}

	if len(items) > 0 {
		return items
	}

	metrics, ok := m.clusterCache.GetNodeMetrics(ctx, m.nodeID)
	if !ok {
		metrics = model.NodeMetrics{NodeID: m.nodeID}
	}
	if m.jobExecutionRepository != nil {
		activeGPUUsageByNode = m.jobExecutionRepository.CountActiveGPUUsageByNode(ctx, []uint64{m.nodeID}, metricsFreshAfter)
	}
	metrics = mergeGPUActiveSessions(metrics, activeGPUUsageByNode[m.nodeID])
	candidate := model.DispatchCandidate{
		NodeID:                    m.nodeID,
		NodeName:                  m.workerID,
		Enabled:                   true,
		Quarantined:               false,
		Draining:                  false,
		SupportsHardwareWatermark: true,
		MaxTranscodeSessions:      cfg.Scheduler.MaxNodeTranscodeSessions,
		MaxUploadConcurrency:      cfg.Scheduler.MaxNodeUploadConcurrency,
		MetricsAvailable:          ok,
		MetricsFresh:              ok && !metrics.Timestamp.IsZero() && !metrics.Timestamp.Before(metricsFreshAfter),
		Online:                    true,
		LastMetricsAt:             metrics.Timestamp,
		LastHeartbeatAt:           now,
		Metrics:                   metrics,
	}
	schedulerReady, stateReason := evaluateSchedulerCandidateReadiness(cfg, mysql.ClusterNodeRecord{
		NodeID:               m.nodeID,
		NodeName:             m.workerID,
		Enabled:              true,
		MaxTranscodeSessions: cfg.Scheduler.MaxNodeTranscodeSessions,
		MaxUploadConcurrency: cfg.Scheduler.MaxNodeUploadConcurrency,
		NodeTags:             "mode:standalone",
		LastHeartbeatAt:      now,
	}, candidate)
	items = append(items, candidateState{
		candidate:          candidate,
		nodeRole:           "standalone",
		controlPlane:       true,
		adminAccessible:    false,
		onlineSignalSource: resolveSchedulerOnlineSignalSource(candidate),
		schedulerReady:     schedulerReady,
		stateReason:        stateReason,
	})
	return items
}

func (m *Manager) shieldSnapshot() map[uint64]struct{} {
	result := make(map[uint64]struct{})
	if m.shieldTracker == nil {
		return result
	}
	for nodeID := range m.shieldTracker.Snapshot() {
		result[nodeID] = struct{}{}
	}
	return result
}

func schedulerNodeRoleFromTags(tags string) string {
	for _, item := range strings.Split(tags, ",") {
		item = strings.TrimSpace(item)
		if strings.HasPrefix(item, "mode:") {
			return strings.TrimPrefix(item, "mode:")
		}
	}
	return "unknown"
}

func schedulerEvaluateNodeManagementCapability(nodeRole string, enabled bool, httpHost string) (bool, bool) {
	adminAccessible := enabled && strings.TrimSpace(httpHost) != ""
	controlPlane := false
	switch nodeRole {
	case "standalone", "cluster-control", "cluster-allinone":
		controlPlane = enabled
	}
	return controlPlane, adminAccessible
}

func resolveSchedulerOnlineSignalSource(candidate model.DispatchCandidate) string {
	if candidate.MetricsFresh && !candidate.LastMetricsAt.IsZero() {
		return "metrics"
	}
	if !candidate.LastHeartbeatAt.IsZero() {
		return "node_heartbeat"
	}
	return "unknown"
}

func evaluateSchedulerCandidateReadiness(cfg config.DynamicRuntimeConfig, node mysql.ClusterNodeRecord, candidate model.DispatchCandidate) (bool, string) {
	if !node.Enabled {
		return false, "node_disabled"
	}
	if node.Quarantined {
		return false, "node_quarantined"
	}
	if node.Draining {
		return false, "node_draining"
	}
	if !candidate.Online {
		return false, "node_offline"
	}
	if !candidate.MetricsAvailable {
		return false, "metrics_missing"
	}
	if !candidate.MetricsFresh {
		return false, "metrics_stale"
	}
	if candidate.Metrics.CPUUsagePercent >= cfg.Scheduler.NodeCPUSafetyLimitPercent {
		return false, "cpu_limit_reached"
	}
	if candidate.Metrics.MemoryUsagePercent >= cfg.Scheduler.NodeMemorySafetyLimitPercent {
		return false, "memory_limit_reached"
	}
	if candidate.Metrics.GPUMemoryUsagePercent >= cfg.Scheduler.NodeGPUSafetyLimitPercent {
		return false, "gpu_memory_limit_reached"
	}
	if candidate.Metrics.ActiveTranscodeSessions >= candidate.MaxTranscodeSessions {
		return false, "node_session_limit_reached"
	}
	if candidate.Metrics.UploadQueueDepth >= candidate.MaxUploadConcurrency {
		return false, "upload_queue_limit_reached"
	}
	return true, "ready"
}
