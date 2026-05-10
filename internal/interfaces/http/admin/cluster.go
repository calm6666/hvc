package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	clusterstate "hvc/internal/cluster"
	"hvc/internal/cluster/membership"
	"hvc/internal/config"
	"hvc/internal/configcenter"
	rediscache "hvc/internal/infra/cache/redis"
	"hvc/internal/infra/db/mysql"
	"hvc/internal/model"
	schedulercore "hvc/internal/scheduler"
	"hvc/pkg/logx"
)

const clusterNodeOnlineGracePeriod = 2 * time.Minute

// ClusterHandler 处理后台集群管理与观测接口。
type ClusterHandler struct {
	nodeRepository          *mysql.ClusterNodeRepository
	gpuRepository           *mysql.GPUDeviceRepository
	stateCache              *clusterstate.StateCache
	registry                *membership.Registry
	effectiveConfig         *configcenter.EffectiveConfig
	runtimeConfigRepository *mysql.RuntimeConfigRepository
	runtimeConfigCache      *rediscache.RuntimeConfigCache
	jobRepository           *mysql.JobRepository
	jobExecutionRepository  *mysql.JobExecutionRepository
	workerInstanceRepo      *mysql.WorkerInstanceRepository
	schedulerManager        *schedulercore.Manager
	db                      *mysql.DB
	internalGRPCConfig      config.InternalGRPCConfig
	localNodeID             uint64
	localHTTPAddress        string
}

// ClusterNodeGPUView 表示后台节点详情中的单张 GPU 视图。
//
// 数据来源分两部分：
// 1. 数据库存量主档，回答“这张卡是谁、是否可调度”；
// 2. 最新 metrics 中的 runtime_capability，回答“这一刻这张卡上报了什么能力”。
type ClusterNodeGPUView struct {
	mysql.NodeGPUDeviceRecord
	RuntimeCapability *model.GPUCapability `json:"runtime_capability,omitempty"`
}

// ClusterNodeGPUSummary 表示单节点 GPU 汇总信息。
type ClusterNodeGPUSummary struct {
	Total                int `json:"total"`
	HealthyTotal         int `json:"healthy_total"`
	SchedulableTotal     int `json:"schedulable_total"`
	MaxTranscodeSessions int `json:"max_transcode_sessions"`
}

// ClusterNodeView 表示后台节点聚合视图。
//
// 这个视图面向后台页面和巡检接口，避免前端自己再把 node 表、GPU 表、Redis metrics 拼起来。
type ClusterNodeView struct {
	mysql.ClusterNodeRecord
	MetricsAvailable bool                  `json:"metrics_available"`
	MetricsFresh     bool                  `json:"metrics_fresh"`
	OnlineEstimate   bool                  `json:"online_estimate"`
	SchedulerReady   bool                  `json:"scheduler_ready"`
	StateReason      string                `json:"state_reason"`
	LastMetricsAt    *time.Time            `json:"last_metrics_at,omitempty"`
	Metrics          *model.NodeMetrics    `json:"metrics,omitempty"`
	GPUSummary       ClusterNodeGPUSummary `json:"gpu_summary"`
	GPUDevices       []ClusterNodeGPUView  `json:"gpu_devices"`
}

type clusterOverviewNodeSummary struct {
	NodeID                  uint64 `json:"node_id"`
	NodeName                string `json:"node_name"`
	NodeRole                string `json:"node_role"`
	Enabled                 bool   `json:"enabled"`
	Quarantined             bool   `json:"quarantined"`
	Draining                bool   `json:"draining"`
	ControlPlane            bool   `json:"control_plane"`
	AdminAccessible         bool   `json:"admin_accessible"`
	OnlineEstimate          bool   `json:"online_estimate"`
	GPUTotal                int    `json:"gpu_total"`
	GPUSchedulableTotal     int    `json:"gpu_schedulable_total"`
	ActiveTranscodeSessions int    `json:"active_transcode_sessions"`
	UploadQueueDepth        int    `json:"upload_queue_depth"`
}

// NewClusterHandler 创建后台集群处理器。
func NewClusterHandler(
	nodeRepository *mysql.ClusterNodeRepository,
	gpuRepository *mysql.GPUDeviceRepository,
	stateCache *clusterstate.StateCache,
	registry *membership.Registry,
	effectiveConfig *configcenter.EffectiveConfig,
	runtimeConfigRepository *mysql.RuntimeConfigRepository,
	runtimeConfigCache *rediscache.RuntimeConfigCache,
	jobRepository *mysql.JobRepository,
	jobExecutionRepository *mysql.JobExecutionRepository,
	workerInstanceRepo *mysql.WorkerInstanceRepository,
	schedulerManager *schedulercore.Manager,
	db *mysql.DB,
	internalGRPCConfig config.InternalGRPCConfig,
	localNodeID uint64,
	localHTTPAddress string,
) *ClusterHandler {
	return &ClusterHandler{
		nodeRepository:          nodeRepository,
		gpuRepository:           gpuRepository,
		stateCache:              stateCache,
		registry:                registry,
		effectiveConfig:         effectiveConfig,
		runtimeConfigRepository: runtimeConfigRepository,
		runtimeConfigCache:      runtimeConfigCache,
		jobRepository:           jobRepository,
		jobExecutionRepository:  jobExecutionRepository,
		workerInstanceRepo:      workerInstanceRepo,
		schedulerManager:        schedulerManager,
		db:                      db,
		internalGRPCConfig:      internalGRPCConfig,
		localNodeID:             localNodeID,
		localHTTPAddress:        localHTTPAddress,
	}
}

// ListNodes 返回节点列表。
func (h *ClusterHandler) ListNodes(w http.ResponseWriter, r *http.Request) {
	page, pageSize := parsePageParams(r)
	items, total, err := h.nodeRepository.ListPage(
		r.Context(),
		page,
		pageSize,
		r.URL.Query().Get("keyword"),
		parseOptionalBool(r.URL.Query().Get("enabled")),
		parseOptionalBool(r.URL.Query().Get("quarantined")),
		parseOptionalBool(r.URL.Query().Get("draining")),
	)
	if err != nil {
		logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "query nodes failed"})
		return
	}
	nodeViews := h.buildNodeViews(r.Context(), items)
	result := make([]clusterNodeListItemView, 0, len(nodeViews))
	for _, item := range nodeViews {
		result = append(result, toClusterNodeListItemView(item))
	}
	writePageResponse(w, page, pageSize, total, result)
}

// GetNodeDetail 返回节点详情。
func (h *ClusterHandler) GetNodeDetail(w http.ResponseWriter, r *http.Request) {
	nodeID, _ := strconv.ParseUint(r.URL.Query().Get("node_id"), 10, 64)
	if nodeID == 0 {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "node_id is required"})
		return
	}
	item, ok := h.nodeRepository.FindByNodeID(r.Context(), nodeID)
	if !ok {
		logx.WriteJSON(w, http.StatusNotFound, model.Response{Code: 404, Message: "node not found"})
		return
	}
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: toClusterNodeDetailView(h.buildNodeView(r.Context(), item, h.listGPUByNode(r.Context(), item.NodeID), time.Now()))})
}

// ListNodeMetrics 返回当前节点指标概览。
func (h *ClusterHandler) ListNodeMetrics(w http.ResponseWriter, r *http.Request) {
	nodeID, _ := strconv.ParseUint(r.URL.Query().Get("node_id"), 10, 64)
	page, pageSize := parsePageParams(r)
	metrics := make([]model.NodeMetrics, 0)
	if nodeID > 0 {
		if value, ok := h.stateCache.GetNodeMetrics(r.Context(), nodeID); ok {
			metrics = append(metrics, value)
		}
		paged, total := pageSlice(metrics, page, pageSize)
		writePageResponse(w, page, pageSize, total, paged)
		return
	}

	items := h.nodeRepository.List(r.Context())
	metrics = make([]model.NodeMetrics, 0, len(items))
	for _, item := range items {
		if value, ok := h.stateCache.GetNodeMetrics(r.Context(), item.NodeID); ok {
			metrics = append(metrics, value)
		}
	}
	paged, total := pageSlice(metrics, page, pageSize)
	writePageResponse(w, page, pageSize, total, paged)
}

// ListMembers 返回当前内部成员注册表快照。
func (h *ClusterHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	h.compactWorkerStatuses(r.Context())
	page, pageSize := parsePageParams(r)
	items := h.buildMemberViews(r.Context())
	paged, total := pageSlice(items, page, pageSize)
	writePageResponse(w, page, pageSize, total, paged)
}

// ListWorkers 返回 Worker 实例分页列表。
//
// 该接口用于回答“当前集群里到底有哪些执行进程在线、最后一次心跳是什么时候、对应哪台节点”。
func (h *ClusterHandler) ListWorkers(w http.ResponseWriter, r *http.Request) {
	if h.workerInstanceRepo == nil {
		writePageResponse(w, 1, 20, 0, []clusterWorkerListItemView{})
		return
	}

	h.compactWorkerStatuses(r.Context())
	page, pageSize := parsePageParams(r)
	nodeID, _ := strconv.ParseUint(r.URL.Query().Get("node_id"), 10, 64)
	status, _ := strconv.Atoi(r.URL.Query().Get("status"))
	items, total, err := h.workerInstanceRepo.ListPage(r.Context(), mysql.WorkerInstanceListFilter{
		Page:       page,
		PageSize:   pageSize,
		NodeID:     nodeID,
		WorkerID:   r.URL.Query().Get("worker_id"),
		Status:     status,
		OnlineOnly: parseOptionalBool(r.URL.Query().Get("online_only")),
	})
	if err != nil {
		logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "query worker instances failed"})
		return
	}

	now := time.Now()
	result := make([]clusterWorkerListItemView, 0, len(items))
	for _, item := range items {
		result = append(result, toClusterWorkerListItemView(item, now))
	}
	writePageResponse(w, page, pageSize, total, result)
}

// SchedulerInsight 返回当前调度器视角下的候选节点、过滤原因与推荐结果。
//
// 注意这里的推荐结果是“当前快照下的确定性预览”：
// 真正调度时仍然会在 Top-K 池内随机挑一个，以避免热点节点被持续打满。
func (h *ClusterHandler) SchedulerInsight(w http.ResponseWriter, r *http.Request) {
	if h.schedulerManager == nil {
		logx.WriteJSON(w, http.StatusServiceUnavailable, model.Response{Code: 503, Message: "scheduler not initialized"})
		return
	}

	req := model.CreateJobRequest{
		EnableWatermark: parseOptionalBoolValue(r.URL.Query().Get("enable_watermark")),
	}
	if preferredHWAccel := r.URL.Query().Get("preferred_hw_accel"); preferredHWAccel != "" {
		req.ScheduleOptions = &model.ScheduleOptions{PreferredHWAccel: preferredHWAccel}
	}
	if videoCodec := r.URL.Query().Get("video_codec"); videoCodec != "" {
		req.Renditions = []model.RenditionOption{{VideoCodec: videoCodec}}
	}

	insight := h.schedulerManager.BuildInsight(r.Context(), req)
	logx.WriteJSON(w, http.StatusOK, model.Response{
		Code:    0,
		Message: "ok",
		Data:    toClusterSchedulerInsightView(insight),
	})
}

// Topology 返回集群拓扑与控制面摘要。
//
// 该接口面向后台集群运维页，重点回答三个问题：
// 1. 当前节点是什么角色；
// 2. 哪些节点具备控制面能力；
// 3. 哪些节点具备后台管理访问入口。
func (h *ClusterHandler) Topology(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cfg := h.currentConfig()
	h.compactWorkerStatuses(ctx)
	nodes := h.nodeRepository.List(ctx)
	nodeViews := h.buildNodeViews(ctx, nodes)
	writeItemsResponse(w, h.buildMemberViews(ctx), map[string]any{
		"mode":     resolveRuntimeMode(cfg),
		"topology": h.topologySummary(ctx, cfg, nodeViews),
	})
}

// ResourceDistribution 返回节点级资源承载分布。
//
// 这类数据后台非常常用，但以前要靠前端自己拼 node/list + metrics + worker/list + scheduler insight，
// 既重又容易口径不一致。这里统一下推成一个运维视角接口。
func (h *ClusterHandler) ResourceDistribution(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cfg := h.currentConfig()
	if h.nodeRepository == nil {
		writeItemsResponse(w, []clusterNodeResourceDistributionView{}, map[string]any{
			"mode":         resolveRuntimeMode(cfg),
			"generated_at": time.Now(),
		})
		return
	}
	nodes := h.nodeRepository.List(ctx)
	nodeViews := h.buildNodeViews(ctx, nodes)
	activeAfter := time.Now().Add(-resolveSchedulerMetricsGrace(cfg))

	nodeIDs := make([]uint64, 0, len(nodes))
	for _, item := range nodes {
		nodeIDs = append(nodeIDs, item.NodeID)
	}

	activeGPUUsageByNode := make(map[uint64]map[int]int)
	if h.jobExecutionRepository != nil && len(nodeIDs) > 0 {
		activeGPUUsageByNode = h.jobExecutionRepository.CountActiveGPUUsageByNode(ctx, nodeIDs, activeAfter)
	}
	activeJobCountByNode := h.activeJobCountByNode(ctx)

	items := make([]clusterNodeResourceDistributionView, 0, len(nodeViews))
	for _, item := range nodeViews {
		view := clusterNodeResourceDistributionView{
			NodeID:                  item.NodeID,
			NodeName:                item.NodeName,
			NodeRole:                nodeRoleFromTags(item.NodeTags),
			Enabled:                 item.Enabled,
			Quarantined:             item.Quarantined,
			Draining:                item.Draining,
			OnlineEstimate:          item.OnlineEstimate,
			SchedulerReady:          item.SchedulerReady,
			StateReason:             item.StateReason,
			MetricsAvailable:        item.MetricsAvailable,
			MetricsFresh:            item.MetricsFresh,
			LastMetricsAt:           item.LastMetricsAt,
			LastHeartbeatAt:         item.LastHeartbeatAt,
			ActiveExecutionTotal:    activeJobCountByNode[item.NodeID],
			MaxTranscodeSessions:    item.MaxTranscodeSessions,
			MaxUploadConcurrency:    item.MaxUploadConcurrency,
			GPUs:                    make([]clusterNodeResourceGPUView, 0, len(item.GPUDevices)),
		}
		if item.Metrics != nil {
			view.ActiveTranscodeSessions = item.Metrics.ActiveTranscodeSessions
			view.UploadQueueDepth = item.Metrics.UploadQueueDepth
			view.CPUUsagePercent = item.Metrics.CPUUsagePercent
			view.MemoryUsagePercent = item.Metrics.MemoryUsagePercent
			view.GPUMemoryUsagePercent = item.Metrics.GPUMemoryUsagePercent
		}
		view.RemainingTranscodeCapacity = item.MaxTranscodeSessions - view.ActiveTranscodeSessions
		if view.RemainingTranscodeCapacity < 0 {
			view.RemainingTranscodeCapacity = 0
		}
		view.RemainingUploadCapacity = item.MaxUploadConcurrency - view.UploadQueueDepth
		if view.RemainingUploadCapacity < 0 {
			view.RemainingUploadCapacity = 0
		}

		activeByGPU := activeGPUUsageByNode[item.NodeID]
		for _, gpu := range item.GPUDevices {
			gpuView := clusterNodeResourceGPUView{
				GPUIndex:      gpu.GPUIndex,
				GPUUUID:       gpu.GPUUUID,
				Model:         gpu.Model,
				Vendor:        gpu.Vendor,
				MemoryTotalMB: gpu.MemoryTotalMB,
				Schedulable:   gpu.Schedulable,
				Healthy:       gpu.Healthy,
			}
			if gpu.RuntimeCapability != nil {
				gpuView.GPUMemoryUsagePercent = gpu.RuntimeCapability.GPUMemoryUsagePercent
				gpuView.GPUUtilizationPercent = gpu.RuntimeCapability.GPUUtilizationPercent
				gpuView.ActiveSessions = gpu.RuntimeCapability.ActiveSessions
				gpuView.MaxSessions = gpu.RuntimeCapability.MaxSessions
			}
			if activeByGPU != nil {
				if sessions, ok := activeByGPU[gpu.GPUIndex]; ok {
					gpuView.ActiveSessions = sessions
				}
			}
			view.GPUActiveSessionTotal += gpuView.ActiveSessions
			view.GPUs = append(view.GPUs, gpuView)
		}
		items = append(items, view)
	}

	writeItemsResponse(w, items, map[string]any{
		"mode":       resolveRuntimeMode(cfg),
		"generated_at": time.Now(),
	})
}

// Overview 返回集群运行模式、节点/GPU 概览和配置版本信息。
//
// 该接口面向运维与排障，尽量一次返回最常用的关键状态，
// 让后台页面和巡检脚本都不需要再自己拼装多次查询结果。
func (h *ClusterHandler) Overview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cfg := h.currentConfig()
	h.compactWorkerStatuses(ctx)
	nodes := h.nodeRepository.List(ctx)
	members := h.registry.List(ctx)
	nodeViews := h.buildNodeViews(ctx, nodes)
	workerTotal, workerOnlineTotal := h.workerSummary(ctx)

	nodeEnabledCount := 0
	nodeQuarantinedCount := 0
	nodeDrainingCount := 0
	nodeOnlineCount := 0
	activeSessions := 0
	uploadQueueDepth := 0
	for _, node := range nodeViews {
		if node.Enabled {
			nodeEnabledCount++
		}
		if node.Quarantined {
			nodeQuarantinedCount++
		}
		if node.Draining {
			nodeDrainingCount++
		}
		if node.OnlineEstimate {
			nodeOnlineCount++
		}
		if node.Metrics != nil {
			activeSessions += node.Metrics.ActiveTranscodeSessions
			uploadQueueDepth += node.Metrics.UploadQueueDepth
		}
	}

	gpuTotal := 0
	gpuHealthyCount := 0
	gpuSchedulableCount := 0
	maxTranscodeSessions := 0
	for _, node := range nodeViews {
		gpuTotal += node.GPUSummary.Total
		gpuHealthyCount += node.GPUSummary.HealthyTotal
		gpuSchedulableCount += node.GPUSummary.SchedulableTotal
		maxTranscodeSessions += node.GPUSummary.MaxTranscodeSessions
	}

	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{
		"mode":          resolveRuntimeMode(cfg),
		"topology":      h.topologySummary(ctx, cfg, nodeViews),
		"module_status": h.moduleStatus(cfg),
		"scheduler":     h.schedulerSummary(ctx, cfg),
		"internal_grpc": map[string]any{
			"listen_address": h.internalGRPCConfig.ListenAddress,
		},
		"cluster": map[string]any{
			"node_total":             len(nodes),
			"node_enabled_total":     nodeEnabledCount,
			"node_quarantined_total": nodeQuarantinedCount,
			"node_draining_total":    nodeDrainingCount,
			"node_online_total":      nodeOnlineCount,
			"member_total":           len(members),
			"worker_total":           workerTotal,
			"worker_online_total":    workerOnlineTotal,
			"gpu_total":              gpuTotal,
			"gpu_healthy_total":      gpuHealthyCount,
			"gpu_schedulable_total":  gpuSchedulableCount,
			"active_sessions":        activeSessions,
			"upload_queue_depth":     uploadQueueDepth,
			"max_transcode_sessions": maxTranscodeSessions,
			"nodes":                  h.buildOverviewNodeSummaries(nodeViews),
		},
		"version": h.versionSummary(ctx),
	}})
}

// Realtime 返回后台轮询友好的实时集群快照。
//
// 与 overview 相比，这里直接带上节点聚合明细和任务状态计数，
// 后台页面可以单接口完成“模式、版本、资源、任务、节点”的实时刷新。
func (h *ClusterHandler) Realtime(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cfg := h.currentConfig()
	h.compactWorkerStatuses(ctx)
	nodes := h.nodeRepository.List(ctx)
	nodeViews := h.buildNodeViews(ctx, nodes)
	members := h.registry.List(ctx)
	workerTotal, workerOnlineTotal := h.workerSummary(ctx)

	activeSessions := 0
	uploadQueueDepth := 0
	nodeEnabledCount := 0
	nodeQuarantinedCount := 0
	nodeDrainingCount := 0
	nodeOnlineCount := 0
	gpuTotal := 0
	gpuHealthyTotal := 0
	gpuSchedulableTotal := 0
	maxTranscodeSessions := 0
	for _, node := range nodeViews {
		if node.Enabled {
			nodeEnabledCount++
		}
		if node.Quarantined {
			nodeQuarantinedCount++
		}
		if node.Draining {
			nodeDrainingCount++
		}
		if node.OnlineEstimate {
			nodeOnlineCount++
		}
		if node.Metrics != nil {
			activeSessions += node.Metrics.ActiveTranscodeSessions
			uploadQueueDepth += node.Metrics.UploadQueueDepth
		}
		gpuTotal += node.GPUSummary.Total
		gpuHealthyTotal += node.GPUSummary.HealthyTotal
		gpuSchedulableTotal += node.GPUSummary.SchedulableTotal
		maxTranscodeSessions += node.GPUSummary.MaxTranscodeSessions
	}

	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{
		"mode":          resolveRuntimeMode(cfg),
		"topology":      h.topologySummary(ctx, cfg, nodeViews),
		"module_status": h.moduleStatus(cfg),
		"scheduler":     h.schedulerSummary(ctx, cfg),
		"internal_grpc": map[string]any{
			"listen_address": h.internalGRPCConfig.ListenAddress,
		},
		"version": h.versionSummary(ctx),
		"queue":   h.queueSummary(ctx),
		"cluster": map[string]any{
			"node_total":             len(nodeViews),
			"node_enabled_total":     nodeEnabledCount,
			"node_quarantined_total": nodeQuarantinedCount,
			"node_draining_total":    nodeDrainingCount,
			"node_online_total":      nodeOnlineCount,
			"member_total":           len(members),
			"worker_total":           workerTotal,
			"worker_online_total":    workerOnlineTotal,
			"gpu_total":              gpuTotal,
			"gpu_healthy_total":      gpuHealthyTotal,
			"gpu_schedulable_total":  gpuSchedulableTotal,
			"active_sessions":        activeSessions,
			"upload_queue_depth":     uploadQueueDepth,
			"max_transcode_sessions": maxTranscodeSessions,
		},
		"nodes": buildRealtimeNodeViews(nodeViews),
	}})
}

// SetNodeEnabled 切换节点启用状态。
func (h *ClusterHandler) SetNodeEnabled(w http.ResponseWriter, r *http.Request) {
	var req struct {
		NodeID  uint64 `json:"node_id"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "invalid request"})
		return
	}
	if err := h.nodeRepository.SetEnabled(r.Context(), req.NodeID, req.Enabled); err != nil {
		logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "update node enabled failed"})
		return
	}
	writeAdminAudit(r.Context(), r, "cluster.node.enable", "cluster_node", "node", 0, "ok")
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok"})
}

// SetNodeQuarantined 切换节点隔离状态。
func (h *ClusterHandler) SetNodeQuarantined(w http.ResponseWriter, r *http.Request) {
	var req struct {
		NodeID      uint64 `json:"node_id"`
		Quarantined bool   `json:"quarantined"`
		Reason      string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "invalid request"})
		return
	}
	if err := h.nodeRepository.SetQuarantined(r.Context(), req.NodeID, req.Quarantined, req.Reason); err != nil {
		logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "update node quarantined failed"})
		return
	}
	writeAdminAudit(r.Context(), r, "cluster.node.quarantine", "cluster_node", "node", 0, "ok")
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok"})
}

// SetNodeDraining 切换节点排空状态。
func (h *ClusterHandler) SetNodeDraining(w http.ResponseWriter, r *http.Request) {
	var req struct {
		NodeID   uint64 `json:"node_id"`
		Draining bool   `json:"draining"`
		Reason   string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "invalid request"})
		return
	}
	if err := h.nodeRepository.SetDraining(r.Context(), req.NodeID, req.Draining, req.Reason); err != nil {
		logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "update node draining failed"})
		return
	}
	writeAdminAudit(r.Context(), r, "cluster.node.drain", "cluster_node", "node", 0, "ok")
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok"})
}

// SetWorkerOffline 手动将 Worker 标记为离线。
//
// 这里先做“状态治理”，不直接远程杀进程：
// - 适用于控制面先收口错误状态；
// - 调度与后台视图会立刻把它视为非在线实例；
// - 真正的远程进程管理后续应走独立 agent/daemon 控制通道。
func (h *ClusterHandler) SetWorkerOffline(w http.ResponseWriter, r *http.Request) {
	if h.workerInstanceRepo == nil {
		logx.WriteJSON(w, http.StatusServiceUnavailable, model.Response{Code: 503, Message: "worker repository not initialized"})
		return
	}
	var req struct {
		WorkerID string `json:"worker_id"`
		Reason   string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "invalid request"})
		return
	}
	if req.WorkerID == "" {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "worker_id is required"})
		return
	}
	if err := h.workerInstanceRepo.MarkOffline(r.Context(), req.WorkerID, req.Reason); err != nil {
		logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "update worker offline failed"})
		return
	}
	writeAdminAudit(r.Context(), r, "cluster.worker.offline", "worker_instance", req.WorkerID, 0, "ok")
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok"})
}

// SetWorkerExited 手动将 Worker 标记为已退出。
func (h *ClusterHandler) SetWorkerExited(w http.ResponseWriter, r *http.Request) {
	if h.workerInstanceRepo == nil {
		logx.WriteJSON(w, http.StatusServiceUnavailable, model.Response{Code: 503, Message: "worker repository not initialized"})
		return
	}
	var req struct {
		WorkerID string `json:"worker_id"`
		Reason   string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "invalid request"})
		return
	}
	if req.WorkerID == "" {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "worker_id is required"})
		return
	}
	if err := h.workerInstanceRepo.MarkExited(r.Context(), req.WorkerID, req.Reason); err != nil {
		logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "update worker exited failed"})
		return
	}
	writeAdminAudit(r.Context(), r, "cluster.worker.exit", "worker_instance", req.WorkerID, 0, "ok")
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok"})
}

// ForceTakeoverJobs 手动强制接管节点或 Worker 上的活跃任务。
//
// 该接口用于以下场景：
// 1. 节点仍有残留 running/assigned 任务，但控制面已确认需要立即回收；
// 2. Worker 状态已离线/异常，不能继续等租约自然过期；
// 3. 运维需要把任务立即重新排队到其他可用节点重新执行。
func (h *ClusterHandler) ForceTakeoverJobs(w http.ResponseWriter, r *http.Request) {
	var req struct {
		NodeID   uint64 `json:"node_id"`
		WorkerID string `json:"worker_id"`
		Reason   string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "invalid request"})
		return
	}
	if req.NodeID == 0 && req.WorkerID == "" {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "node_id or worker_id is required"})
		return
	}
	if h.jobRepository == nil {
		logx.WriteJSON(w, http.StatusServiceUnavailable, model.Response{Code: 503, Message: "job repository not initialized"})
		return
	}

	result, err := h.jobRepository.ForceTakeover(r.Context(), req.NodeID, req.WorkerID, req.Reason)
	if err != nil {
		logx.Error("admin.cluster.force_takeover.failed", err, logx.Fields{
			"node_id":   req.NodeID,
			"worker_id": req.WorkerID,
		})
		logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "force takeover failed"})
		return
	}

	targetID := req.WorkerID
	if req.NodeID > 0 {
		targetID = strconv.FormatUint(req.NodeID, 10)
	}
	if req.NodeID > 0 && req.WorkerID != "" {
		targetID = "node:" + strconv.FormatUint(req.NodeID, 10) + "|worker:" + req.WorkerID
	}
	writeAdminAudit(r.Context(), r, "cluster.job.takeover", "transcode_job", targetID, 0, "ok")
	logx.WriteJSON(w, http.StatusOK, model.Response{
		Code:    0,
		Message: "ok",
		Data:    result,
	})
}

func (h *ClusterHandler) currentConfig() config.DynamicRuntimeConfig {
	if h.effectiveConfig == nil {
		return config.DynamicRuntimeConfig{}
	}
	return h.effectiveConfig.Snapshot()
}

func resolveRuntimeMode(cfg config.DynamicRuntimeConfig) string {
	if cfg.IsStandalone() {
		return "standalone"
	}
	if cfg.IsClusterControl() {
		return "cluster-control"
	}
	if cfg.IsClusterWorker() {
		return "cluster-worker"
	}
	if cfg.IsClusterAllInOne() {
		return "cluster-allinone"
	}
	return "custom"
}

func (h *ClusterHandler) buildNodeViews(ctx context.Context, nodes []mysql.ClusterNodeRecord) []ClusterNodeView {
	now := time.Now()
	items := make([]ClusterNodeView, 0, len(nodes))
	allGPUs := h.listAllGPUs(ctx)
	gpuByNode := make(map[uint64][]mysql.NodeGPUDeviceRecord, len(nodes))
	for _, gpu := range allGPUs {
		gpuByNode[gpu.NodeID] = append(gpuByNode[gpu.NodeID], gpu)
	}
	for _, node := range nodes {
		items = append(items, h.buildNodeView(ctx, node, gpuByNode[node.NodeID], now))
	}
	return items
}

func (h *ClusterHandler) buildNodeView(ctx context.Context, node mysql.ClusterNodeRecord, gpus []mysql.NodeGPUDeviceRecord, now time.Time) ClusterNodeView {
	cfg := h.currentConfig()
	view := ClusterNodeView{
		ClusterNodeRecord: node,
		GPUDevices:        make([]ClusterNodeGPUView, 0, len(gpus)),
	}

	metrics, metricsOK := h.stateCache.GetNodeMetrics(ctx, node.NodeID)
	if metricsOK {
		view.MetricsAvailable = true
		view.Metrics = &metrics
		metricsAt := metrics.Timestamp
		view.LastMetricsAt = &metricsAt
		view.MetricsFresh = now.Sub(metricsAt) <= resolveSchedulerMetricsGrace(cfg)
	}
	view.OnlineEstimate = estimateNodeOnline(node, view.LastMetricsAt, now)
	view.SchedulerReady, view.StateReason = evaluateSchedulerReadiness(cfg, node, view)

	capabilityByIdentity := buildCapabilityLookup(metrics)
	for _, gpu := range gpus {
		runtimeCapability := lookupRuntimeCapability(capabilityByIdentity, gpu)
		view.GPUDevices = append(view.GPUDevices, ClusterNodeGPUView{
			NodeGPUDeviceRecord: gpu,
			RuntimeCapability:   runtimeCapability,
		})
		view.GPUSummary.Total++
		view.GPUSummary.MaxTranscodeSessions += gpu.MaxTranscodeSessions
		if gpu.Healthy {
			view.GPUSummary.HealthyTotal++
		}
		if gpu.Schedulable {
			view.GPUSummary.SchedulableTotal++
		}
	}
	return view
}

func (h *ClusterHandler) buildOverviewNodeSummaries(items []ClusterNodeView) []clusterOverviewNodeSummary {
	result := make([]clusterOverviewNodeSummary, 0, len(items))
	for _, item := range items {
		nodeRole := nodeRoleFromTags(item.NodeTags)
		controlPlane, adminAccessible := evaluateNodeManagementCapability(nodeRole, item.Enabled, item.HTTPHost)
		summary := clusterOverviewNodeSummary{
			NodeID:              item.NodeID,
			NodeName:            item.NodeName,
			NodeRole:            nodeRole,
			Enabled:             item.Enabled,
			Quarantined:         item.Quarantined,
			Draining:            item.Draining,
			ControlPlane:        controlPlane,
			AdminAccessible:     adminAccessible,
			OnlineEstimate:      item.OnlineEstimate,
			GPUTotal:            item.GPUSummary.Total,
			GPUSchedulableTotal: item.GPUSummary.SchedulableTotal,
		}
		if item.Metrics != nil {
			summary.ActiveTranscodeSessions = item.Metrics.ActiveTranscodeSessions
			summary.UploadQueueDepth = item.Metrics.UploadQueueDepth
		}
		result = append(result, summary)
	}
	return result
}

func (h *ClusterHandler) listAllGPUs(ctx context.Context) []mysql.NodeGPUDeviceRecord {
	if h.gpuRepository == nil {
		return nil
	}
	return h.gpuRepository.ListAll(ctx)
}

func (h *ClusterHandler) listGPUByNode(ctx context.Context, nodeID uint64) []mysql.NodeGPUDeviceRecord {
	if h.gpuRepository == nil {
		return nil
	}
	return h.gpuRepository.ListByNodeID(ctx, nodeID)
}

func (h *ClusterHandler) moduleStatus(cfg config.DynamicRuntimeConfig) map[string]any {
	return map[string]any{
		"http_enabled":          cfg.Mode.EnableHTTPServer,
		"public_grpc_enabled":   cfg.Mode.EnableGRPCServer,
		"internal_grpc_enabled": h.internalGRPCConfig.Enabled && h.internalGRPCConfig.ListenAddress != "",
		"mq_consumer_enabled":   cfg.Mode.EnableMQConsumer,
		"scheduler_enabled":     cfg.Mode.EnableScheduler,
		"worker_enabled":        cfg.Mode.EnableWorker,
		"callback_enabled":      cfg.Mode.EnableCallback,
	}
}

func (h *ClusterHandler) schedulerSummary(ctx context.Context, cfg config.DynamicRuntimeConfig) map[string]any {
	activeExecutions := int64(0)
	if h.jobExecutionRepository != nil {
		activeExecutions = h.jobExecutionRepository.CountActiveExecutions(ctx, time.Now().Add(-cfg.Scheduler.WorkerHeartbeatTimeout))
	}
	remaining := int64(cfg.Scheduler.MaxGlobalTranscodeSessions) - activeExecutions
	if remaining < 0 {
		remaining = 0
	}
	return map[string]any{
		"dynamic_concurrency_control":    cfg.Scheduler.DynamicConcurrencyControl,
		"max_global_transcode_sessions":  cfg.Scheduler.MaxGlobalTranscodeSessions,
		"max_node_transcode_sessions":    cfg.Scheduler.MaxNodeTranscodeSessions,
		"worker_heartbeat_timeout_sec":   int(cfg.Scheduler.WorkerHeartbeatTimeout / time.Second),
		"active_execution_total":         activeExecutions,
		"remaining_execution_capacity":   remaining,
		"require_hardware_encode":        cfg.Scheduler.RequireHardwareEncode,
		"allow_software_decode_fallback": cfg.Scheduler.AllowSoftwareDecodeFallback,
	}
}

func (h *ClusterHandler) versionSummary(ctx context.Context) map[string]any {
	var dbConfigVersion uint64
	if record, ok := h.runtimeConfigRepository.LatestPublished(ctx); ok {
		dbConfigVersion = record.ConfigVersion
	}

	cacheConfigVersion, cacheVersionExists, _ := h.runtimeConfigCache.GetPublishedVersion(ctx)
	cacheTTL, _ := h.runtimeConfigCache.GetPublishedTTL(ctx)
	cacheTTLSeconds := int(cacheTTL / time.Second)
	if cacheTTLSeconds < 0 {
		cacheTTLSeconds = 0
	}

	return map[string]any{
		"mysql_server_version":         h.db.ServerVersion(ctx),
		"redis_server_version":         h.runtimeConfigCache.RedisServerVersion(ctx),
		"redis_mode":                   h.runtimeConfigCache.RedisServerMode(ctx),
		"runtime_config_db_version":    dbConfigVersion,
		"runtime_config_cache_version": cacheConfigVersion,
		"runtime_config_cache_exists":  cacheVersionExists,
		"runtime_config_cache_ttl_sec": cacheTTLSeconds,
	}
}

func (h *ClusterHandler) queueSummary(ctx context.Context) map[string]any {
	if h.jobRepository == nil {
		return map[string]any{
			"queued_jobs":    0,
			"assigned_jobs":  0,
			"running_jobs":   0,
			"uploading_jobs": 0,
			"completed_jobs": 0,
			"failed_jobs":    0,
			"canceled_jobs":  0,
		}
	}

	return map[string]any{
		"queued_jobs":    h.jobRepository.CountByStatus(ctx, model.JobStatusQueued),
		"assigned_jobs":  h.jobRepository.CountByStatus(ctx, model.JobStatusAssigned),
		"running_jobs":   h.jobRepository.CountByStatus(ctx, model.JobStatusRunning),
		"uploading_jobs": h.jobRepository.CountByStatus(ctx, model.JobStatusUploading),
		"completed_jobs": h.jobRepository.CountByStatus(ctx, model.JobStatusCompleted),
		"failed_jobs":    h.jobRepository.CountByStatus(ctx, model.JobStatusFailed),
		"canceled_jobs":  h.jobRepository.CountByStatus(ctx, model.JobStatusCanceled),
	}
}

func (h *ClusterHandler) activeJobCountByNode(ctx context.Context) map[uint64]int {
	result := make(map[uint64]int)
	if h.jobRepository == nil {
		return result
	}
	items := h.jobRepository.ListAssigned(ctx, 0, "")
	for _, item := range items {
		if item.AssignedNodeID == 0 {
			continue
		}
		result[item.AssignedNodeID]++
	}
	return result
}

func (h *ClusterHandler) workerSummary(ctx context.Context) (int, int) {
	if h.workerInstanceRepo == nil {
		return 0, 0
	}
	items := h.workerInstanceRepo.ListAll(ctx)
	total := len(items)
	online := 0
	now := time.Now()
	for _, item := range items {
		if item.Status == 1 && !item.LastHeartbeatAt.IsZero() && now.Sub(item.LastHeartbeatAt) <= clusterNodeOnlineGracePeriod {
			online++
		}
	}
	return total, online
}

func (h *ClusterHandler) compactWorkerStatuses(ctx context.Context) {
	if h.workerInstanceRepo == nil {
		return
	}
	if _, err := h.workerInstanceRepo.MarkOfflineByHeartbeatTimeout(ctx, resolveWorkerOfflineTimeout(h.currentConfig())); err != nil {
		logx.Error("admin.cluster.worker.compact_status_failed", err, nil)
	}
}

func resolveWorkerOfflineTimeout(cfg config.DynamicRuntimeConfig) time.Duration {
	if cfg.Scheduler.WorkerHeartbeatTimeout > 0 {
		return cfg.Scheduler.WorkerHeartbeatTimeout * 2
	}
	return clusterNodeOnlineGracePeriod
}

func (h *ClusterHandler) buildMemberViews(ctx context.Context) []clusterMemberView {
	nodes := h.nodeRepository.List(ctx)
	registryItems := h.registry.List(ctx)
	workersByNode, onlineWorkersByNode := h.workerSummaryByNode(ctx)
	registryByNode := make(map[uint64]membership.Node, len(registryItems))
	for _, item := range registryItems {
		registryByNode[item.NodeID] = item
	}

	result := make([]clusterMemberView, 0, len(nodes)+len(registryItems))
	seen := make(map[uint64]struct{}, len(nodes)+len(registryItems))
	now := time.Now()
	for _, node := range nodes {
		seen[node.NodeID] = struct{}{}
		var registryHeartbeatAt *time.Time
		source := "node_table"
		if registryItem, ok := registryByNode[node.NodeID]; ok {
			value := registryItem.LastHeartbeatAt
			registryHeartbeatAt = &value
			source = "node_table+registry"
		}

		var lastMetricsAt *time.Time
		if metrics, ok := h.stateCache.GetNodeMetrics(ctx, node.NodeID); ok && !metrics.Timestamp.IsZero() {
			value := metrics.Timestamp
			lastMetricsAt = &value
		}

		view := clusterMemberView{
			NodeID:              node.NodeID,
			NodeName:            node.NodeName,
			NodeRole:            nodeRoleFromTags(node.NodeTags),
			Host:                firstNonEmpty(node.HTTPHost, node.GRPCHost, node.HostIP),
			HostIP:              node.HostIP,
			GRPCHost:            node.GRPCHost,
			HTTPHost:            node.HTTPHost,
			Enabled:             node.Enabled,
			Quarantined:         node.Quarantined,
			Draining:            node.Draining,
			ControlPlane:        isControlPlaneNode(node.NodeTags, node.Enabled),
			AdminAccessible:     node.Enabled && strings.TrimSpace(node.HTTPHost) != "",
			OnlineEstimate:      estimateNodeOnline(node, lastMetricsAt, now),
			Source:              source,
			LastHeartbeatAt:     node.LastHeartbeatAt,
			RegistryHeartbeatAt: registryHeartbeatAt,
			LastMetricsAt:       lastMetricsAt,
			WorkerTotal:         workersByNode[node.NodeID],
			WorkerOnlineTotal:   onlineWorkersByNode[node.NodeID],
		}
		result = append(result, view)
	}

	for _, item := range registryItems {
		if _, ok := seen[item.NodeID]; ok {
			continue
		}
		result = append(result, toClusterMemberView(item))
	}
	return result
}

func (h *ClusterHandler) workerSummaryByNode(ctx context.Context) (map[uint64]int, map[uint64]int) {
	totalByNode := make(map[uint64]int)
	onlineByNode := make(map[uint64]int)
	if h.workerInstanceRepo == nil {
		return totalByNode, onlineByNode
	}
	items := h.workerInstanceRepo.ListAll(ctx)
	now := time.Now()
	for _, item := range items {
		totalByNode[item.NodeID]++
		if item.Status == 1 && !item.LastHeartbeatAt.IsZero() && now.Sub(item.LastHeartbeatAt) <= clusterNodeOnlineGracePeriod {
			onlineByNode[item.NodeID]++
		}
	}
	return totalByNode, onlineByNode
}

func estimateNodeOnline(node mysql.ClusterNodeRecord, lastMetricsAt *time.Time, now time.Time) bool {
	if lastMetricsAt != nil && !lastMetricsAt.IsZero() && now.Sub(*lastMetricsAt) <= clusterNodeOnlineGracePeriod {
		return true
	}
	if !node.LastHeartbeatAt.IsZero() && now.Sub(node.LastHeartbeatAt) <= clusterNodeOnlineGracePeriod {
		return true
	}
	return false
}

func (h *ClusterHandler) topologySummary(ctx context.Context, cfg config.DynamicRuntimeConfig, nodeViews []ClusterNodeView) map[string]any {
	controlPlaneTotal := 0
	adminAccessibleTotal := 0
	workerOnlyTotal := 0
	controlPlaneNodes := make([]map[string]any, 0)
	for _, item := range nodeViews {
		nodeRole := nodeRoleFromTags(item.NodeTags)
		controlPlane, adminAccessible := evaluateNodeManagementCapability(nodeRole, item.Enabled, item.HTTPHost)
		if controlPlane {
			controlPlaneTotal++
			controlPlaneNodes = append(controlPlaneNodes, map[string]any{
				"node_id":          item.NodeID,
				"node_name":        item.NodeName,
				"node_role":        nodeRole,
				"http_host":        item.HTTPHost,
				"online_estimate":  item.OnlineEstimate,
				"admin_accessible": adminAccessible,
			})
		}
		if adminAccessible {
			adminAccessibleTotal++
		}
		if nodeRole == "cluster-worker" {
			workerOnlyTotal++
		}
	}

	currentNodeRole := resolveRuntimeMode(cfg)
	currentIsControlPlane, currentAdminAccessible := evaluateNodeManagementCapability(currentNodeRole, true, h.localHTTPAddress)
	return map[string]any{
		"current_node_id":          h.localNodeID,
		"current_node_role":        currentNodeRole,
		"current_is_control_plane": currentIsControlPlane,
		"current_admin_accessible": currentAdminAccessible,
		"current_http_address":     h.localHTTPAddress,
		"control_plane_total":      controlPlaneTotal,
		"admin_accessible_total":   adminAccessibleTotal,
		"worker_only_total":        workerOnlyTotal,
		"control_plane_nodes":      controlPlaneNodes,
		"generated_at":             time.Now(),
		"db_context_ok":            h.nodeRepository != nil && h.db != nil && ctx != nil,
	}
}

func nodeRoleFromTags(tags string) string {
	for _, item := range strings.Split(tags, ",") {
		item = strings.TrimSpace(item)
		if strings.HasPrefix(item, "mode:") {
			return strings.TrimPrefix(item, "mode:")
		}
	}
	return "unknown"
}

func isControlPlaneNode(tags string, enabled bool) bool {
	if !enabled {
		return false
	}
	switch nodeRoleFromTags(tags) {
	case "standalone", "cluster-control", "cluster-allinone":
		return true
	default:
		return false
	}
}

func buildCapabilityLookup(metrics model.NodeMetrics) map[string]model.GPUCapability {
	if len(metrics.GPUCapabilities) == 0 {
		return nil
	}

	result := make(map[string]model.GPUCapability, len(metrics.GPUCapabilities)*2)
	for _, capability := range metrics.GPUCapabilities {
		if capability.GPUUUID != "" {
			result["uuid:"+capability.GPUUUID] = capability
		}
		result["index:"+strconv.Itoa(capability.GPUIndex)] = capability
	}
	return result
}

func lookupRuntimeCapability(index map[string]model.GPUCapability, gpu mysql.NodeGPUDeviceRecord) *model.GPUCapability {
	if len(index) == 0 {
		return nil
	}
	if gpu.GPUUUID != "" {
		if capability, ok := index["uuid:"+gpu.GPUUUID]; ok {
			copy := capability
			return &copy
		}
	}
	if capability, ok := index["index:"+strconv.Itoa(gpu.GPUIndex)]; ok {
		copy := capability
		return &copy
	}
	return nil
}

func evaluateSchedulerReadiness(cfg config.DynamicRuntimeConfig, node mysql.ClusterNodeRecord, view ClusterNodeView) (bool, string) {
	if !node.Enabled {
		return false, "node_disabled"
	}
	if node.Quarantined {
		return false, "node_quarantined"
	}
	if node.Draining {
		return false, "node_draining"
	}
	if !view.OnlineEstimate {
		return false, "node_offline"
	}
	if !view.MetricsAvailable {
		return false, "metrics_missing"
	}
	if !view.MetricsFresh {
		return false, "metrics_stale"
	}
	if view.Metrics != nil {
		if view.Metrics.CPUUsagePercent >= cfg.Scheduler.NodeCPUSafetyLimitPercent {
			return false, "cpu_limit_reached"
		}
		if view.Metrics.MemoryUsagePercent >= cfg.Scheduler.NodeMemorySafetyLimitPercent {
			return false, "memory_limit_reached"
		}
		if view.Metrics.GPUMemoryUsagePercent >= cfg.Scheduler.NodeGPUSafetyLimitPercent {
			return false, "gpu_memory_limit_reached"
		}
		if view.Metrics.ActiveTranscodeSessions >= cfg.Scheduler.MaxNodeTranscodeSessions {
			return false, "node_session_limit_reached"
		}
		if view.Metrics.UploadQueueDepth >= cfg.Scheduler.MaxNodeUploadConcurrency {
			return false, "upload_queue_limit_reached"
		}
	}
	return true, "ready"
}

func resolveSchedulerMetricsGrace(cfg config.DynamicRuntimeConfig) time.Duration {
	if cfg.Scheduler.WorkerHeartbeatTimeout > 0 {
		return cfg.Scheduler.WorkerHeartbeatTimeout
	}
	return 30 * time.Second
}

func buildRealtimeNodeViews(items []ClusterNodeView) []clusterNodeListItemView {
	result := make([]clusterNodeListItemView, 0, len(items))
	for _, item := range items {
		result = append(result, toClusterNodeListItemView(item))
	}
	return result
}
