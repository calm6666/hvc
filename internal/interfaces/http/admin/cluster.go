package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	clusterv1 "hvc/api/pb/clusterv1"
	"hvc/internal/audit"
	clusterstate "hvc/internal/cluster"
	"hvc/internal/cluster/membership"
	"hvc/internal/config"
	"hvc/internal/configcenter"
	rediscache "hvc/internal/infra/cache/redis"
	"hvc/internal/infra/db/mysql"
	"hvc/internal/model"
	schedulercore "hvc/internal/scheduler"
	"hvc/pkg/logx"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

const clusterNodeOnlineGracePeriod = 2 * time.Minute

const (
	clusterOverviewCacheTTL             = 2 * time.Second
	clusterRealtimeCacheTTL             = 2 * time.Second
	clusterTopologyCacheTTL             = 3 * time.Second
	clusterResourceDistributionCacheTTL = 2 * time.Second
)

var clusterGovernanceAuditActions = []string{
	"cluster.node.enable",
	"cluster.node.quarantine",
	"cluster.node.drain",
	"cluster.worker.offline",
	"cluster.worker.exit",
	"cluster.job.takeover",
}

// ClusterHandler 处理后台集群管理与观测接口。
type ClusterHandler struct {
	nodeRepository          *mysql.ClusterNodeRepository
	gpuRepository           *mysql.GPUDeviceRepository
	stateCache              *clusterstate.StateCache
	registry                *membership.Registry
	effectiveConfig         *configcenter.EffectiveConfig
	runtimeConfigRepository *mysql.RuntimeConfigRepository
	runtimeConfigCache      *rediscache.RuntimeConfigCache
	registryEtcdRepository  *mysql.RegistryEtcdConfigRepository
	jobRepository           *mysql.JobRepository
	jobExecutionRepository  *mysql.JobExecutionRepository
	workerInstanceRepo      *mysql.WorkerInstanceRepository
	schedulerManager        *schedulercore.Manager
	workerController        clusterWorkerController
	auditRepository         *audit.Repository
	db                      *mysql.DB
	internalGRPCConfig      config.InternalGRPCConfig
	nodeMode                string
	localNodeID             uint64
	localHTTPAddress        string
	localAdvertiseIP        string
}

type clusterWorkerController interface {
	SetOffline(ctx context.Context, workerID string, reason string) error
	RequestExit(ctx context.Context, workerID string, reason string) error
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

type clusterNodeStateSnapshot struct {
	Metrics             model.NodeMetrics
	MetricsAvailable    bool
	RegistryHeartbeatAt *time.Time
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
	OnlineSignalSource      string `json:"online_signal_source"`
	SchedulerReady          bool   `json:"scheduler_ready"`
	StateReason             string `json:"state_reason"`
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
	registryEtcdRepository *mysql.RegistryEtcdConfigRepository,
	jobRepository *mysql.JobRepository,
	jobExecutionRepository *mysql.JobExecutionRepository,
	workerInstanceRepo *mysql.WorkerInstanceRepository,
	schedulerManager *schedulercore.Manager,
	workerController clusterWorkerController,
	auditRepository *audit.Repository,
	db *mysql.DB,
	internalGRPCConfig config.InternalGRPCConfig,
	nodeMode string,
	localNodeID uint64,
	localHTTPAddress string,
	localAdvertiseIP string,
) *ClusterHandler {
	return &ClusterHandler{
		nodeRepository:          nodeRepository,
		gpuRepository:           gpuRepository,
		stateCache:              stateCache,
		registry:                registry,
		effectiveConfig:         effectiveConfig,
		runtimeConfigRepository: runtimeConfigRepository,
		runtimeConfigCache:      runtimeConfigCache,
		registryEtcdRepository:  registryEtcdRepository,
		jobRepository:           jobRepository,
		jobExecutionRepository:  jobExecutionRepository,
		workerInstanceRepo:      workerInstanceRepo,
		schedulerManager:        schedulerManager,
		workerController:        workerController,
		auditRepository:         auditRepository,
		db:                      db,
		internalGRPCConfig:      internalGRPCConfig,
		nodeMode:                nodeMode,
		localNodeID:             localNodeID,
		localHTTPAddress:        localHTTPAddress,
		localAdvertiseIP:        localAdvertiseIP,
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
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "查询节点列表失败"})
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
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: "node_id 不能为空"})
		return
	}
	item, ok := h.nodeRepository.FindByNodeID(r.Context(), nodeID)
	if !ok {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 404, Message: "节点不存在"})
		return
	}
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: toClusterNodeDetailView(h.buildNodeView(r.Context(), item, h.listGPUByNode(r.Context(), item.NodeID), time.Now()))})
}

// ListNodeMetrics 返回当前节点指标概览。
func (h *ClusterHandler) ListNodeMetrics(w http.ResponseWriter, r *http.Request) {
	nodeID, _ := strconv.ParseUint(r.URL.Query().Get("node_id"), 10, 64)
	page, pageSize := parsePageParams(r)
	if nodeID > 0 {
		if h.nodeRepository == nil {
			writePageResponse(w, page, pageSize, 0, []model.NodeMetrics{})
			return
		}
		if _, ok := h.nodeRepository.FindByNodeID(r.Context(), nodeID); !ok {
			writePageResponse(w, page, pageSize, 0, []model.NodeMetrics{})
			return
		}
		metrics := make([]model.NodeMetrics, 0, 1)
		if h.stateCache != nil {
			if value, ok := h.stateCache.GetNodeMetrics(r.Context(), nodeID); ok {
				metrics = append(metrics, value)
			}
		}
		writePageResponse(w, page, pageSize, int64(len(metrics)), metrics)
		return
	}

	if h.nodeRepository == nil {
		writePageResponse(w, page, pageSize, 0, []model.NodeMetrics{})
		return
	}
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
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "查询节点指标失败"})
		return
	}
	metrics := make([]model.NodeMetrics, 0, len(items))
	metricsByNode := h.listNodeMetricsSnapshot(r.Context(), collectNodeIDs(items))
	for _, item := range items {
		if value, ok := metricsByNode[item.NodeID]; ok {
			metrics = append(metrics, value)
			continue
		}
		metrics = append(metrics, model.NodeMetrics{NodeID: item.NodeID})
	}
	writePageResponse(w, page, pageSize, total, metrics)
}

// ListMembers 返回当前内部成员注册表快照。
func (h *ClusterHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	h.compactWorkerStatuses(r.Context())
	page, pageSize := parsePageParams(r)
	if h.nodeRepository == nil {
		writePageResponse(w, page, pageSize, 0, []clusterMemberView{})
		return
	}
	// member/list 的稳定分页基准应落在节点主表，而不是把全量成员快照拉出来再切片。
	// 这样可以保证大集群下分页成本跟 page_size 成正比，而不是跟总节点数成正比。
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
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "查询集群成员列表失败"})
		return
	}
	writePageResponse(w, page, pageSize, total, h.buildMemberViewsFromNodes(r.Context(), items))
}

// ListWorkers 返回 Worker 实例分页列表。
//
// 该接口用于回答“当前集群里到底有哪些执行进程在线、最后一次心跳是什么时候、对应哪台节点”。
func (h *ClusterHandler) ListWorkers(w http.ResponseWriter, r *http.Request) {
	page, pageSize := parsePageParams(r)
	if h.workerInstanceRepo == nil {
		writePageResponse(w, page, pageSize, 0, []clusterWorkerListItemView{})
		return
	}

	h.compactWorkerStatuses(r.Context())
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
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "查询 Worker 实例列表失败"})
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
		logx.WriteJSON(w, http.StatusServiceUnavailable, model.Response{Code: 503, Message: "调度器未初始化"})
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
	h.writeCachedClusterResponse(w, r, "topology", clusterTopologyCacheTTL, func(ctx context.Context) model.Response {
		cfg := h.currentConfig()
		h.compactWorkerStatuses(ctx)
		nodes := h.listClusterNodes(ctx)
		nodeViews := h.buildNodeStatusViews(ctx, nodes)
		return model.Response{
			Code:    0,
			Message: "ok",
			Data: map[string]any{
				"items":     h.buildMemberViewsFromPreparedNodes(ctx, nodes, nodeViews),
				"mode":      resolveRuntimeMode(cfg),
				"node_mode": h.nodeMode,
				"topology":  h.topologySummary(ctx, cfg, nodeViews),
			},
		}
	})
}

// ResourceDistribution 返回节点级资源承载分布。
//
// 这类数据后台非常常用，但以前要靠前端自己拼 node/list + metrics + worker/list + scheduler insight，
// 既重又容易口径不一致。这里统一下推成一个运维视角接口。
func (h *ClusterHandler) ResourceDistribution(w http.ResponseWriter, r *http.Request) {
	h.writeCachedClusterResponse(w, r, "resource_distribution", clusterResourceDistributionCacheTTL, func(ctx context.Context) model.Response {
		cfg := h.currentConfig()
		if h.nodeRepository == nil {
			return model.Response{
				Code:    0,
				Message: "ok",
				Data: map[string]any{
					"items":        []clusterNodeResourceDistributionView{},
					"mode":         resolveRuntimeMode(cfg),
					"node_mode":    h.nodeMode,
					"generated_at": time.Now(),
				},
			}
		}
		nodes := h.listClusterNodes(ctx)
		nodeViews := h.buildNodeStatusViews(ctx, nodes)
		activeAfter := time.Now().Add(-resolveSchedulerMetricsGrace(cfg))
		nodeIDs := collectNodeIDs(nodes)
		gpuRecordsByNode := h.groupGPURecordsByNode(h.listGPUsByNodeIDs(ctx, nodeIDs))
		activeGPUUsageByNode := make(map[uint64]map[int]int)
		if h.jobExecutionRepository != nil && len(nodeIDs) > 0 {
			activeGPUUsageByNode = h.jobExecutionRepository.CountActiveGPUUsageByNode(ctx, nodeIDs, activeAfter)
		}
		activeJobCountByNode := h.activeJobCountByNode(ctx)
		items := make([]clusterNodeResourceDistributionView, 0, len(nodeViews))
		for _, item := range nodeViews {
			controlPlane, adminAccessible := evaluateNodeManagementCapability(nodeRoleFromTags(item.NodeTags), item.Enabled, item.HTTPHost)
			view := clusterNodeResourceDistributionView{
				NodeID:               item.NodeID,
				NodeName:             item.NodeName,
				NodeRole:             nodeRoleFromTags(item.NodeTags),
				Enabled:              item.Enabled,
				Quarantined:          item.Quarantined,
				Draining:             item.Draining,
				ControlPlane:         controlPlane,
				AdminAccessible:      adminAccessible,
				OnlineEstimate:       item.OnlineEstimate,
				OnlineSignalSource:   resolveNodeOnlineSignalSource(item),
				SchedulerReady:       item.SchedulerReady,
				StateReason:          item.StateReason,
				MetricsAvailable:     item.MetricsAvailable,
				MetricsFresh:         item.MetricsFresh,
				LastMetricsAt:        item.LastMetricsAt,
				LastHeartbeatAt:      item.LastHeartbeatAt,
				ActiveExecutionTotal: activeJobCountByNode[item.NodeID],
				MaxTranscodeSessions: item.MaxTranscodeSessions,
				MaxUploadConcurrency: item.MaxUploadConcurrency,
				GPUs:                 make([]clusterNodeResourceGPUView, 0, len(item.GPUDevices)),
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
			capabilityByIdentity := buildCapabilityLookup(resolveNodeMetricsValue(item.Metrics))
			for _, gpu := range gpuRecordsByNode[item.NodeID] {
				gpuView := clusterNodeResourceGPUView{
					GPUIndex:      gpu.GPUIndex,
					GPUUUID:       gpu.GPUUUID,
					Model:         gpu.Model,
					Vendor:        gpu.Vendor,
					MemoryTotalMB: gpu.MemoryTotalMB,
					Schedulable:   gpu.Schedulable,
					Healthy:       gpu.Healthy,
				}
				if runtimeCapability := lookupRuntimeCapability(capabilityByIdentity, gpu); runtimeCapability != nil {
					gpuView.GPUMemoryUsagePercent = runtimeCapability.GPUMemoryUsagePercent
					gpuView.GPUUtilizationPercent = runtimeCapability.GPUUtilizationPercent
					gpuView.ActiveSessions = runtimeCapability.ActiveSessions
					gpuView.MaxSessions = runtimeCapability.MaxSessions
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
		return model.Response{
			Code:    0,
			Message: "ok",
			Data: map[string]any{
				"items":        items,
				"mode":         resolveRuntimeMode(cfg),
				"node_mode":    h.nodeMode,
				"generated_at": time.Now(),
			},
		}
	})
}

// Overview 返回集群运行模式、节点/GPU 概览和配置版本信息。
//
// 该接口面向运维与排障，尽量一次返回最常用的关键状态，
// 让后台页面和巡检脚本都不需要再自己拼装多次查询结果。
func (h *ClusterHandler) Overview(w http.ResponseWriter, r *http.Request) {
	h.writeCachedClusterResponse(w, r, "overview", clusterOverviewCacheTTL, func(ctx context.Context) model.Response {
		cfg := h.currentConfig()
		h.compactWorkerStatuses(ctx)
		nodes := h.listClusterNodes(ctx)
		members := h.listRegistryMembers(ctx)
		nodeViews := h.buildNodeStatusViews(ctx, nodes)
		h.attachGPUSummaries(nodeViews, h.listGPUSummaryByNodeIDs(ctx, collectNodeIDs(nodes)))
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
		return model.Response{Code: 0, Message: "ok", Data: map[string]any{
			"mode":          resolveRuntimeMode(cfg),
			"node_mode":     h.nodeMode,
			"topology":      h.topologySummary(ctx, cfg, nodeViews),
			"module_status": h.moduleStatus(cfg),
			"scheduler":     h.schedulerSummary(ctx, cfg),
			"internal_grpc": map[string]any{
				"listen_address": h.internalGRPCConfig.ListenAddress,
			},
			"public_grpc": h.publicGRPCSummary(ctx, cfg),
			"governance":  h.governanceSummary(ctx),
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
		}}
	})
}

// Realtime 返回后台轮询友好的实时集群快照。
//
// 与 overview 相比，这里直接带上节点聚合明细和任务状态计数，
// 后台页面可以单接口完成“模式、版本、资源、任务、节点”的实时刷新。
func (h *ClusterHandler) Realtime(w http.ResponseWriter, r *http.Request) {
	h.writeCachedClusterResponse(w, r, "realtime", clusterRealtimeCacheTTL, func(ctx context.Context) model.Response {
		cfg := h.currentConfig()
		h.compactWorkerStatuses(ctx)
		nodes := h.listClusterNodes(ctx)
		nodeViews := h.buildNodeStatusViews(ctx, nodes)
		h.attachGPUSummaries(nodeViews, h.listGPUSummaryByNodeIDs(ctx, collectNodeIDs(nodes)))
		members := h.listRegistryMembers(ctx)
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
		return model.Response{Code: 0, Message: "ok", Data: map[string]any{
			"mode":          resolveRuntimeMode(cfg),
			"node_mode":     h.nodeMode,
			"topology":      h.topologySummary(ctx, cfg, nodeViews),
			"module_status": h.moduleStatus(cfg),
			"scheduler":     h.schedulerSummary(ctx, cfg),
			"internal_grpc": map[string]any{
				"listen_address": h.internalGRPCConfig.ListenAddress,
			},
			"public_grpc": h.publicGRPCSummary(ctx, cfg),
			"governance":  h.governanceSummary(ctx),
			"version":     h.versionSummary(ctx),
			"queue":       h.queueSummary(ctx),
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
		}}
	})
}

func (h *ClusterHandler) writeCachedClusterResponse(w http.ResponseWriter, r *http.Request, name string, ttl time.Duration, build func(context.Context) model.Response) {
	if payload, ok := h.getCachedClusterResponse(r.Context(), name); ok {
		writeCachedJSON(w, payload)
		return
	}
	response := build(r.Context())
	payload, err := json.Marshal(response)
	if err != nil {
		logx.Error("admin.cluster.cache_response.marshal_failed", err, logx.Fields{"name": name})
		logx.WriteJSON(w, http.StatusOK, response)
		return
	}
	if err := h.saveCachedClusterResponse(r.Context(), name, string(payload), ttl); err != nil {
		logx.Error("admin.cluster.cache_response.save_failed", err, logx.Fields{"name": name})
	}
	writeCachedJSON(w, string(payload))
}

func (h *ClusterHandler) getCachedClusterResponse(ctx context.Context, name string) (string, bool) {
	if h == nil || h.stateCache == nil || strings.TrimSpace(name) == "" {
		return "", false
	}
	return h.stateCache.GetJSONSnapshot(ctx, h.clusterResponseCacheKey(name))
}

func (h *ClusterHandler) saveCachedClusterResponse(ctx context.Context, name string, payload string, ttl time.Duration) error {
	if h == nil || h.stateCache == nil || strings.TrimSpace(name) == "" {
		return nil
	}
	return h.stateCache.SaveJSONSnapshot(ctx, h.clusterResponseCacheKey(name), payload, ttl)
}

func (h *ClusterHandler) clusterResponseCacheKey(name string) string {
	version := uint64(0)
	if h != nil && h.effectiveConfig != nil {
		version = h.effectiveConfig.CurrentVersion()
	}
	return clusterstate.AdminClusterSummaryCacheKey(strings.TrimSpace(name), h.localNodeID, version)
}

func (h *ClusterHandler) invalidateClusterSummaryCaches(ctx context.Context) {
	h.invalidateClusterSummaryCachesByVersions(ctx, h.currentConfigVersion())
}

func (h *ClusterHandler) invalidateClusterSummaryCachesByVersions(ctx context.Context, versions ...uint64) {
	if h == nil || h.stateCache == nil {
		return
	}

	keys := make([]string, 0, len(versions)*4+1)
	seen := make(map[string]struct{}, len(versions)*4+1)
	appendKey := func(key string) {
		key = strings.TrimSpace(key)
		if key == "" {
			return
		}
		if _, exists := seen[key]; exists {
			return
		}
		seen[key] = struct{}{}
		keys = append(keys, key)
	}

	for _, version := range versions {
		appendKey(clusterstate.AdminClusterSummaryCacheKey("overview", h.localNodeID, version))
		appendKey(clusterstate.AdminClusterSummaryCacheKey("realtime", h.localNodeID, version))
		appendKey(clusterstate.AdminClusterSummaryCacheKey("topology", h.localNodeID, version))
		appendKey(clusterstate.AdminClusterSummaryCacheKey("resource_distribution", h.localNodeID, version))
	}
	appendKey(clusterstate.MonitorSnapshotCacheKey(h.nodeMode))
	if len(keys) == 0 {
		return
	}
	if err := h.stateCache.DeleteJSONSnapshots(ctx, keys...); err != nil {
		logx.Error("admin.cluster.cache_response.invalidate_failed", err, logx.Fields{"keys": keys})
	}
}

func (h *ClusterHandler) currentConfigVersion() uint64 {
	if h == nil || h.effectiveConfig == nil {
		return 0
	}
	return h.effectiveConfig.CurrentVersion()
}

func writeCachedJSON(w http.ResponseWriter, payload string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, _ = w.Write([]byte(payload))
}

// SetNodeEnabled 切换节点启用状态。
func (h *ClusterHandler) SetNodeEnabled(w http.ResponseWriter, r *http.Request) {
	if h.nodeRepository == nil {
		logx.WriteJSON(w, http.StatusServiceUnavailable, model.Response{Code: 503, Message: "节点仓储未初始化"})
		return
	}
	var req struct {
		NodeID             uint64 `json:"node_id"`
		Enabled            bool   `json:"enabled"`
		Reason             string `json:"reason"`
		TakeoverActiveJobs bool   `json:"takeover_active_jobs"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if err := h.nodeRepository.SetEnabled(r.Context(), req.NodeID, req.Enabled); err != nil {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "更新节点启用状态失败"})
		return
	}
	takeoverResult, err := h.takeoverNodeJobsIfNeeded(r.Context(), req.NodeID, req.Reason, req.TakeoverActiveJobs && !req.Enabled)
	if err != nil {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: err.Error()})
		return
	}
	writeAdminAudit(r.Context(), r, "cluster.node.enable", "cluster_node", strconv.FormatUint(req.NodeID, 10), 0, formatGovernanceAuditMessage(map[string]any{
		"target_status":        map[bool]string{true: "enabled", false: "disabled"}[req.Enabled],
		"takeover_active_jobs": req.TakeoverActiveJobs && !req.Enabled,
		"takeover_result":      takeoverResult,
	}))
	h.invalidateClusterSummaryCaches(r.Context())
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{
		"node_id":              req.NodeID,
		"enabled":              req.Enabled,
		"target_status":        map[bool]string{true: "enabled", false: "disabled"}[req.Enabled],
		"takeover_active_jobs": req.TakeoverActiveJobs && !req.Enabled,
		"takeover_result":      takeoverResult,
	}})
}

// SetNodeQuarantined 切换节点隔离状态。
func (h *ClusterHandler) SetNodeQuarantined(w http.ResponseWriter, r *http.Request) {
	if h.nodeRepository == nil {
		logx.WriteJSON(w, http.StatusServiceUnavailable, model.Response{Code: 503, Message: "节点仓储未初始化"})
		return
	}
	var req struct {
		NodeID             uint64 `json:"node_id"`
		Quarantined        bool   `json:"quarantined"`
		Reason             string `json:"reason"`
		TakeoverActiveJobs bool   `json:"takeover_active_jobs"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if err := h.nodeRepository.SetQuarantined(r.Context(), req.NodeID, req.Quarantined, req.Reason); err != nil {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "更新节点隔离状态失败"})
		return
	}
	takeoverResult, err := h.takeoverNodeJobsIfNeeded(r.Context(), req.NodeID, req.Reason, req.TakeoverActiveJobs && req.Quarantined)
	if err != nil {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: err.Error()})
		return
	}
	writeAdminAudit(r.Context(), r, "cluster.node.quarantine", "cluster_node", strconv.FormatUint(req.NodeID, 10), 0, formatGovernanceAuditMessage(map[string]any{
		"target_status":        map[bool]string{true: "quarantined", false: "active"}[req.Quarantined],
		"takeover_active_jobs": req.TakeoverActiveJobs && req.Quarantined,
		"takeover_result":      takeoverResult,
	}))
	h.invalidateClusterSummaryCaches(r.Context())
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{
		"node_id":              req.NodeID,
		"quarantined":          req.Quarantined,
		"target_status":        map[bool]string{true: "quarantined", false: "active"}[req.Quarantined],
		"takeover_active_jobs": req.TakeoverActiveJobs && req.Quarantined,
		"takeover_result":      takeoverResult,
	}})
}

// SetNodeDraining 切换节点排空状态。
func (h *ClusterHandler) SetNodeDraining(w http.ResponseWriter, r *http.Request) {
	if h.nodeRepository == nil {
		logx.WriteJSON(w, http.StatusServiceUnavailable, model.Response{Code: 503, Message: "节点仓储未初始化"})
		return
	}
	var req struct {
		NodeID             uint64 `json:"node_id"`
		Draining           bool   `json:"draining"`
		Reason             string `json:"reason"`
		TakeoverActiveJobs bool   `json:"takeover_active_jobs"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if err := h.nodeRepository.SetDraining(r.Context(), req.NodeID, req.Draining, req.Reason); err != nil {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "更新节点排空状态失败"})
		return
	}
	takeoverResult, err := h.takeoverNodeJobsIfNeeded(r.Context(), req.NodeID, req.Reason, req.TakeoverActiveJobs && req.Draining)
	if err != nil {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: err.Error()})
		return
	}
	writeAdminAudit(r.Context(), r, "cluster.node.drain", "cluster_node", strconv.FormatUint(req.NodeID, 10), 0, formatGovernanceAuditMessage(map[string]any{
		"target_status":        map[bool]string{true: "draining", false: "schedulable"}[req.Draining],
		"takeover_active_jobs": req.TakeoverActiveJobs && req.Draining,
		"takeover_result":      takeoverResult,
	}))
	h.invalidateClusterSummaryCaches(r.Context())
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{
		"node_id":              req.NodeID,
		"draining":             req.Draining,
		"target_status":        map[bool]string{true: "draining", false: "schedulable"}[req.Draining],
		"takeover_active_jobs": req.TakeoverActiveJobs && req.Draining,
		"takeover_result":      takeoverResult,
	}})
}

// SetWorkerOffline 手动将 Worker 标记为离线。
func (h *ClusterHandler) SetWorkerOffline(w http.ResponseWriter, r *http.Request) {
	if h.workerInstanceRepo == nil {
		logx.WriteJSON(w, http.StatusServiceUnavailable, model.Response{Code: 503, Message: "Worker 仓储未初始化"})
		return
	}
	var req struct {
		WorkerID           string `json:"worker_id"`
		Reason             string `json:"reason"`
		TakeoverActiveJobs bool   `json:"takeover_active_jobs"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.WorkerID == "" {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: "worker_id 不能为空"})
		return
	}
	controlPath, nodeID, takeoverResult, err := h.applyWorkerOffline(r.Context(), req.WorkerID, req.Reason, req.TakeoverActiveJobs)
	if err != nil {
		statusCode := http.StatusBadGateway
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "不存在") {
			statusCode = http.StatusOK
		}
		logx.WriteJSON(w, statusCode, model.Response{Code: statusCode, Message: err.Error()})
		return
	}
	writeAdminAudit(r.Context(), r, "cluster.worker.offline", "worker_instance", req.WorkerID, 0, formatGovernanceAuditMessage(map[string]any{
		"node_id":              nodeID,
		"control_path":         controlPath,
		"target_status":        "offline",
		"takeover_active_jobs": req.TakeoverActiveJobs,
		"takeover_result":      takeoverResult,
	}))
	h.invalidateClusterSummaryCaches(r.Context())
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{
		"worker_id":            req.WorkerID,
		"node_id":              nodeID,
		"control_path":         controlPath,
		"target_status":        "offline",
		"takeover_active_jobs": req.TakeoverActiveJobs,
		"takeover_result":      takeoverResult,
	}})
}

// SetWorkerExited 手动将 Worker 标记为已退出。
func (h *ClusterHandler) SetWorkerExited(w http.ResponseWriter, r *http.Request) {
	if h.workerInstanceRepo == nil {
		logx.WriteJSON(w, http.StatusServiceUnavailable, model.Response{Code: 503, Message: "Worker 仓储未初始化"})
		return
	}
	var req struct {
		WorkerID           string `json:"worker_id"`
		Reason             string `json:"reason"`
		TakeoverActiveJobs bool   `json:"takeover_active_jobs"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.WorkerID == "" {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: "worker_id 不能为空"})
		return
	}
	controlPath, nodeID, takeoverResult, err := h.applyWorkerExit(r.Context(), req.WorkerID, req.Reason, req.TakeoverActiveJobs)
	if err != nil {
		statusCode := http.StatusBadGateway
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "不存在") {
			statusCode = http.StatusOK
		}
		logx.WriteJSON(w, statusCode, model.Response{Code: statusCode, Message: err.Error()})
		return
	}
	writeAdminAudit(r.Context(), r, "cluster.worker.exit", "worker_instance", req.WorkerID, 0, formatGovernanceAuditMessage(map[string]any{
		"node_id":              nodeID,
		"control_path":         controlPath,
		"target_status":        "exited",
		"takeover_active_jobs": req.TakeoverActiveJobs,
		"takeover_result":      takeoverResult,
	}))
	h.invalidateClusterSummaryCaches(r.Context())
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{
		"worker_id":            req.WorkerID,
		"node_id":              nodeID,
		"control_path":         controlPath,
		"target_status":        "exited",
		"takeover_active_jobs": req.TakeoverActiveJobs,
		"takeover_result":      takeoverResult,
	}})
}

func (h *ClusterHandler) applyWorkerOffline(ctx context.Context, workerID string, reason string, takeoverActiveJobs bool) (string, uint64, *mysql.ForceTakeoverResult, error) {
	record, ok := h.workerInstanceRepo.FindByWorkerID(ctx, workerID)
	if !ok {
		return "", 0, nil, fmt.Errorf("worker 不存在")
	}
	if record.NodeID == h.localNodeID && h.workerController != nil {
		if err := h.workerController.SetOffline(ctx, workerID, reason); err != nil {
			return "", record.NodeID, nil, err
		}
		takeoverResult, err := h.takeoverWorkerJobsIfNeeded(ctx, record.NodeID, workerID, reason, takeoverActiveJobs)
		if err != nil {
			return "", record.NodeID, nil, err
		}
		return "local_worker_module", record.NodeID, takeoverResult, nil
	}
	if err := h.invokeRemoteWorkerControl(ctx, record.NodeID, workerID, reason, "offline"); err != nil {
		return "", record.NodeID, nil, err
	}
	takeoverResult, err := h.takeoverWorkerJobsIfNeeded(ctx, record.NodeID, workerID, reason, takeoverActiveJobs)
	if err != nil {
		return "", record.NodeID, nil, err
	}
	return "internal_grpc", record.NodeID, takeoverResult, nil
}

func (h *ClusterHandler) applyWorkerExit(ctx context.Context, workerID string, reason string, takeoverActiveJobs bool) (string, uint64, *mysql.ForceTakeoverResult, error) {
	record, ok := h.workerInstanceRepo.FindByWorkerID(ctx, workerID)
	if !ok {
		return "", 0, nil, fmt.Errorf("worker 不存在")
	}
	if record.NodeID == h.localNodeID && h.workerController != nil {
		if err := h.workerController.RequestExit(ctx, workerID, reason); err != nil {
			return "", record.NodeID, nil, err
		}
		takeoverResult, err := h.takeoverWorkerJobsIfNeeded(ctx, record.NodeID, workerID, reason, takeoverActiveJobs)
		if err != nil {
			return "", record.NodeID, nil, err
		}
		return "local_worker_module", record.NodeID, takeoverResult, nil
	}
	if err := h.invokeRemoteWorkerControl(ctx, record.NodeID, workerID, reason, "exit"); err != nil {
		return "", record.NodeID, nil, err
	}
	takeoverResult, err := h.takeoverWorkerJobsIfNeeded(ctx, record.NodeID, workerID, reason, takeoverActiveJobs)
	if err != nil {
		return "", record.NodeID, nil, err
	}
	return "internal_grpc", record.NodeID, takeoverResult, nil
}

func (h *ClusterHandler) takeoverWorkerJobsIfNeeded(ctx context.Context, nodeID uint64, workerID string, reason string, enabled bool) (*mysql.ForceTakeoverResult, error) {
	if !enabled {
		return nil, nil
	}
	if h.jobRepository == nil {
		return nil, fmt.Errorf("任务仓储未初始化")
	}
	result, err := h.jobRepository.ForceTakeover(ctx, nodeID, workerID, reason)
	if err != nil {
		return nil, fmt.Errorf("强制接管任务失败: %w", err)
	}
	return &result, nil
}

func (h *ClusterHandler) takeoverNodeJobsIfNeeded(ctx context.Context, nodeID uint64, reason string, enabled bool) (*mysql.ForceTakeoverResult, error) {
	if !enabled {
		return nil, nil
	}
	if h.jobRepository == nil {
		return nil, fmt.Errorf("任务仓储未初始化")
	}
	result, err := h.jobRepository.ForceTakeover(ctx, nodeID, "", reason)
	if err != nil {
		return nil, fmt.Errorf("强制接管任务失败: %w", err)
	}
	return &result, nil
}

func (h *ClusterHandler) invokeRemoteWorkerControl(ctx context.Context, nodeID uint64, workerID string, reason string, action string) error {
	if h.nodeRepository == nil {
		return fmt.Errorf("节点仓储未初始化")
	}
	node, ok := h.nodeRepository.FindByNodeID(ctx, nodeID)
	if !ok {
		return fmt.Errorf("目标节点不存在")
	}
	target, err := resolveInternalGRPCTarget(node.HostIP, node.GRPCHost)
	if err != nil {
		return err
	}
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(callCtx, target, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		return fmt.Errorf("连接目标节点内部 gRPC 失败: %w", err)
	}
	defer func() { _ = conn.Close() }()
	client := clusterv1.NewClusterInternalServiceClient(conn)
	if strings.TrimSpace(h.internalGRPCConfig.SharedToken) != "" {
		callCtx = metadata.AppendToOutgoingContext(callCtx, "x-hvc-internal-token", strings.TrimSpace(h.internalGRPCConfig.SharedToken))
	}
	switch action {
	case "offline":
		resp, err := client.SetWorkerOffline(callCtx, &clusterv1.SetWorkerOfflineRequest{
			WorkerId: workerID,
			Reason:   reason,
		})
		if err != nil {
			return fmt.Errorf("远端 Worker 下线失败: %w", err)
		}
		if resp == nil || !resp.GetAccepted() {
			return fmt.Errorf("远端 Worker 下线请求被拒绝")
		}
		return nil
	case "exit":
		resp, err := client.RequestWorkerExit(callCtx, &clusterv1.RequestWorkerExitRequest{
			WorkerId: workerID,
			Reason:   reason,
		})
		if err != nil {
			return fmt.Errorf("远端 Worker 退出失败: %w", err)
		}
		if resp == nil || !resp.GetAccepted() {
			return fmt.Errorf("远端 Worker 退出请求被拒绝")
		}
		return nil
	default:
		return fmt.Errorf("不支持的 Worker 控制动作: %s", action)
	}
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
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.NodeID == 0 && req.WorkerID == "" {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: "node_id 和 worker_id 至少传一个"})
		return
	}
	if h.jobRepository == nil {
		logx.WriteJSON(w, http.StatusServiceUnavailable, model.Response{Code: 503, Message: "任务仓储未初始化"})
		return
	}

	result, err := h.jobRepository.ForceTakeover(r.Context(), req.NodeID, req.WorkerID, req.Reason)
	if err != nil {
		logx.Error("admin.cluster.force_takeover.failed", err, logx.Fields{
			"node_id":   req.NodeID,
			"worker_id": req.WorkerID,
		})
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "强制接管任务失败"})
		return
	}

	targetID := req.WorkerID
	if req.NodeID > 0 {
		targetID = strconv.FormatUint(req.NodeID, 10)
	}
	if req.NodeID > 0 && req.WorkerID != "" {
		targetID = "node:" + strconv.FormatUint(req.NodeID, 10) + "|worker:" + req.WorkerID
	}
	writeAdminAudit(r.Context(), r, "cluster.job.takeover", "transcode_job", targetID, 0, formatGovernanceAuditMessage(result))
	h.invalidateClusterSummaryCaches(r.Context())
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
	if cfg.Mode.EnableHTTPServer && cfg.Mode.EnableScheduler && cfg.Mode.EnableWorker {
		return "active-http+scheduler+worker"
	}
	if cfg.Mode.EnableHTTPServer && cfg.Mode.EnableScheduler {
		return "active-http+scheduler"
	}
	if cfg.Mode.EnableWorker {
		return "active-worker"
	}
	if cfg.Mode.EnableCallback {
		return "active-callback-only"
	}
	return "custom"
}

func (h *ClusterHandler) buildNodeViews(ctx context.Context, nodes []mysql.ClusterNodeRecord) []ClusterNodeView {
	now := time.Now()
	items := make([]ClusterNodeView, 0, len(nodes))
	nodeIDs := collectNodeIDs(nodes)
	allGPUs := h.listGPUsByNodeIDs(ctx, nodeIDs)
	snapshotByNode := h.buildNodeStateSnapshotMap(ctx, nodes)
	gpuByNode := make(map[uint64][]mysql.NodeGPUDeviceRecord, len(nodes))
	for _, gpu := range allGPUs {
		gpuByNode[gpu.NodeID] = append(gpuByNode[gpu.NodeID], gpu)
	}
	for _, node := range nodes {
		items = append(items, h.buildNodeViewFromSnapshot(node, gpuByNode[node.NodeID], snapshotByNode[node.NodeID], now))
	}
	return items
}

func (h *ClusterHandler) buildNodeStatusViews(ctx context.Context, nodes []mysql.ClusterNodeRecord) []ClusterNodeView {
	now := time.Now()
	items := make([]ClusterNodeView, 0, len(nodes))
	snapshotByNode := h.buildNodeStateSnapshotMap(ctx, nodes)
	for _, node := range nodes {
		items = append(items, h.buildNodeStatusView(node, snapshotByNode[node.NodeID], now))
	}
	return items
}

func (h *ClusterHandler) buildNodeView(ctx context.Context, node mysql.ClusterNodeRecord, gpus []mysql.NodeGPUDeviceRecord, now time.Time) ClusterNodeView {
	return h.buildNodeViewFromSnapshot(node, gpus, h.buildNodeStateSnapshotMap(ctx, []mysql.ClusterNodeRecord{node})[node.NodeID], now)
}

func (h *ClusterHandler) buildNodeStatusView(node mysql.ClusterNodeRecord, snapshot clusterNodeStateSnapshot, now time.Time) ClusterNodeView {
	cfg := h.currentConfig()
	view := ClusterNodeView{
		ClusterNodeRecord: node,
	}

	metrics := snapshot.Metrics
	if snapshot.MetricsAvailable {
		view.MetricsAvailable = true
		view.Metrics = &metrics
		metricsAt := metrics.Timestamp
		view.LastMetricsAt = &metricsAt
		view.MetricsFresh = now.Sub(metricsAt) <= resolveSchedulerMetricsGrace(cfg)
	}
	view.OnlineEstimate = estimateNodeOnlineWithRegistry(node, view.LastMetricsAt, snapshot.RegistryHeartbeatAt, now)
	view.SchedulerReady, view.StateReason = evaluateSchedulerReadiness(cfg, node, view)
	return view
}

func (h *ClusterHandler) buildNodeViewFromSnapshot(node mysql.ClusterNodeRecord, gpus []mysql.NodeGPUDeviceRecord, snapshot clusterNodeStateSnapshot, now time.Time) ClusterNodeView {
	view := h.buildNodeStatusView(node, snapshot, now)
	view.GPUDevices = make([]ClusterNodeGPUView, 0, len(gpus))
	metrics := snapshot.Metrics
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
			OnlineSignalSource:  resolveNodeOnlineSignalSource(item),
			SchedulerReady:      item.SchedulerReady,
			StateReason:         item.StateReason,
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

func (h *ClusterHandler) listClusterNodes(ctx context.Context) []mysql.ClusterNodeRecord {
	if h.nodeRepository == nil {
		return nil
	}
	return h.nodeRepository.ListForAdminSnapshot(ctx)
}

func (h *ClusterHandler) listRegistryMembers(ctx context.Context) []membership.Node {
	if h.registry == nil {
		return nil
	}
	return h.registry.List(ctx)
}

func (h *ClusterHandler) listAllGPUs(ctx context.Context) []mysql.NodeGPUDeviceRecord {
	if h.gpuRepository == nil {
		return nil
	}
	return h.gpuRepository.ListAll(ctx)
}

func (h *ClusterHandler) listGPUsByNodeIDs(ctx context.Context, nodeIDs []uint64) []mysql.NodeGPUDeviceRecord {
	if h.gpuRepository == nil {
		return nil
	}
	if len(nodeIDs) == 0 {
		return nil
	}
	return h.gpuRepository.ListByNodeIDs(ctx, nodeIDs)
}

func (h *ClusterHandler) groupGPURecordsByNode(items []mysql.NodeGPUDeviceRecord) map[uint64][]mysql.NodeGPUDeviceRecord {
	result := make(map[uint64][]mysql.NodeGPUDeviceRecord)
	for _, item := range items {
		result[item.NodeID] = append(result[item.NodeID], item)
	}
	return result
}

func (h *ClusterHandler) listGPUSummaryByNodeIDs(ctx context.Context, nodeIDs []uint64) map[uint64]ClusterNodeGPUSummary {
	result := make(map[uint64]ClusterNodeGPUSummary)
	if h.gpuRepository == nil || len(nodeIDs) == 0 {
		return result
	}
	for nodeID, summary := range h.gpuRepository.SummaryByNodeIDs(ctx, nodeIDs) {
		result[nodeID] = ClusterNodeGPUSummary{
			Total:                summary.Total,
			HealthyTotal:         summary.HealthyTotal,
			SchedulableTotal:     summary.SchedulableTotal,
			MaxTranscodeSessions: summary.MaxTranscodeSessions,
		}
	}
	return result
}

func resolveNodeMetricsValue(metrics *model.NodeMetrics) model.NodeMetrics {
	if metrics == nil {
		return model.NodeMetrics{}
	}
	return *metrics
}

func (h *ClusterHandler) attachGPUSummaries(nodeViews []ClusterNodeView, summaryByNode map[uint64]ClusterNodeGPUSummary) {
	for idx := range nodeViews {
		if summary, ok := summaryByNode[nodeViews[idx].NodeID]; ok {
			nodeViews[idx].GPUSummary = summary
		}
	}
}

func (h *ClusterHandler) listGPUByNode(ctx context.Context, nodeID uint64) []mysql.NodeGPUDeviceRecord {
	if h.gpuRepository == nil {
		return nil
	}
	return h.gpuRepository.ListByNodeID(ctx, nodeID)
}

func (h *ClusterHandler) buildNodeStateSnapshotMap(ctx context.Context, nodes []mysql.ClusterNodeRecord) map[uint64]clusterNodeStateSnapshot {
	result := make(map[uint64]clusterNodeStateSnapshot, len(nodes))
	nodeIDs := collectNodeIDs(nodes)
	metricsByNode := h.listNodeMetricsSnapshot(ctx, nodeIDs)
	registryHeartbeatByNode := h.listRegistryHeartbeatByNode(ctx)
	for _, nodeID := range nodeIDs {
		snapshot := result[nodeID]
		if metrics, ok := metricsByNode[nodeID]; ok {
			snapshot.Metrics = metrics
			snapshot.MetricsAvailable = true
		}
		if heartbeatAt, ok := registryHeartbeatByNode[nodeID]; ok && !heartbeatAt.IsZero() {
			value := heartbeatAt
			snapshot.RegistryHeartbeatAt = &value
		}
		result[nodeID] = snapshot
	}
	return result
}

func (h *ClusterHandler) listNodeMetricsSnapshot(ctx context.Context, nodeIDs []uint64) map[uint64]model.NodeMetrics {
	if h.stateCache == nil || len(nodeIDs) == 0 {
		return map[uint64]model.NodeMetrics{}
	}
	return h.stateCache.GetNodeMetricsBatch(ctx, nodeIDs)
}

func (h *ClusterHandler) listRegistryHeartbeatByNode(ctx context.Context) map[uint64]time.Time {
	result := make(map[uint64]time.Time)
	if h.registry == nil {
		return result
	}
	for _, item := range h.registry.List(ctx) {
		if item.NodeID == 0 || item.LastHeartbeatAt.IsZero() {
			continue
		}
		result[item.NodeID] = item.LastHeartbeatAt
	}
	return result
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

func (h *ClusterHandler) publicGRPCSummary(ctx context.Context, cfg config.DynamicRuntimeConfig) map[string]any {
	effectiveEndpoint := strings.TrimSpace(cfg.GRPC.ListenAddress)
	if effectiveEndpoint == "" {
		effectiveEndpoint = ":9090"
	}
	registryID := h.publishedPublicGRPCRegistryID(ctx)
	summary := map[string]any{
		"enabled":                cfg.Mode.EnableGRPCServer,
		"listen_address":         cfg.GRPC.ListenAddress,
		"effective_endpoint":     effectiveEndpoint,
		"registry_bound":         registryID > 0,
		"registry_id":            registryID,
		"service_name":           "transcode.v1.TranscodePublicService",
		"discovery_mode":         "direct",
		"service_namespace":      "",
		"registry_endpoints":     []string{},
		"registry_enabled":       false,
		"lease_ttl_sec":          0,
		"dial_timeout_ms":        0,
		"registry_key":           "",
		"registry_key_exists":    false,
		"registry_value_json":    "",
		"registry_lease_id":      int64(0),
		"registry_lease_ttl_sec": 0,
	}
	if registryID == 0 {
		return summary
	}

	summary["discovery_mode"] = "etcd"
	if h.registryEtcdRepository == nil {
		return summary
	}
	record, ok := h.registryEtcdRepository.FindByID(ctx, registryID)
	if !ok {
		summary["registry_missing"] = true
		return summary
	}
	summary["registry_enabled"] = record.Enabled
	summary["service_namespace"] = record.ServiceNamespace
	summary["registry_endpoints"] = splitCommaText(record.Endpoints)
	summary["lease_ttl_sec"] = record.LeaseTTLSec
	summary["dial_timeout_ms"] = record.DialTimeoutMS
	inspection := h.inspectPublicGRPCRegistryKey(ctx, record, effectiveEndpoint)
	summary["registry_key"] = inspection.Key
	summary["registry_key_exists"] = inspection.Exists
	summary["registry_value_json"] = inspection.ValueJSON
	summary["registry_lease_id"] = inspection.LeaseID
	summary["registry_lease_ttl_sec"] = inspection.LeaseTTLSeconds
	return summary
}

func (h *ClusterHandler) publishedPublicGRPCRegistryID(ctx context.Context) uint64 {
	if h.runtimeConfigRepository == nil {
		return 0
	}
	record, ok := h.runtimeConfigRepository.LatestPublished(ctx)
	if !ok {
		return 0
	}
	return record.PublicGRPCRegistryID
}

func (h *ClusterHandler) versionSummary(ctx context.Context) map[string]any {
	var dbConfigVersion uint64
	if h.runtimeConfigRepository != nil {
		if record, ok := h.runtimeConfigRepository.LatestPublished(ctx); ok {
			dbConfigVersion = record.ConfigVersion
		}
	}

	cacheConfigVersion := uint64(0)
	cacheVersionExists := false
	cacheTTLSeconds := 0
	if h.runtimeConfigCache != nil {
		cacheConfigVersion, cacheVersionExists, _ = h.runtimeConfigCache.GetPublishedVersion(ctx)
		cacheTTL, _ := h.runtimeConfigCache.GetPublishedTTL(ctx)
		cacheTTLSeconds = int(cacheTTL / time.Second)
		if cacheTTLSeconds < 0 {
			cacheTTLSeconds = 0
		}
	}

	mysqlServerVersion := ""
	if h.db != nil {
		mysqlServerVersion = h.db.ServerVersion(ctx)
	}

	redisServerVersion := ""
	redisMode := ""
	if h.runtimeConfigCache != nil {
		redisServerVersion = h.runtimeConfigCache.RedisServerVersion(ctx)
		redisMode = h.runtimeConfigCache.RedisServerMode(ctx)
	}

	return map[string]any{
		"mysql_server_version":         mysqlServerVersion,
		"redis_server_version":         redisServerVersion,
		"redis_mode":                   redisMode,
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

	counts := h.jobRepository.CountByStatuses(ctx, []int{
		model.JobStatusQueued,
		model.JobStatusAssigned,
		model.JobStatusRunning,
		model.JobStatusUploading,
		model.JobStatusCompleted,
		model.JobStatusFailed,
		model.JobStatusCanceled,
	})

	return map[string]any{
		"queued_jobs":    counts[model.JobStatusQueued],
		"assigned_jobs":  counts[model.JobStatusAssigned],
		"running_jobs":   counts[model.JobStatusRunning],
		"uploading_jobs": counts[model.JobStatusUploading],
		"completed_jobs": counts[model.JobStatusCompleted],
		"failed_jobs":    counts[model.JobStatusFailed],
		"canceled_jobs":  counts[model.JobStatusCanceled],
	}
}

func (h *ClusterHandler) governanceSummary(ctx context.Context) clusterGovernanceSummaryView {
	const windowMinutes = 24 * 60
	result := clusterGovernanceSummaryView{
		WindowMinutes: windowMinutes,
		RecentActions: []clusterGovernanceRecentActionView{},
	}
	if h.auditRepository == nil {
		return result
	}
	since := time.Now().Add(-time.Duration(windowMinutes) * time.Minute)
	items := h.auditRepository.ListRecentByActions(ctx, clusterGovernanceAuditActions, 10, &since)
	result.RecentActionTotal = h.auditRepository.CountByActionsSince(ctx, clusterGovernanceAuditActions, since)
	if len(items) == 0 {
		return result
	}
	result.RecentActions = make([]clusterGovernanceRecentActionView, 0, len(items))
	for _, item := range items {
		view := toClusterGovernanceRecentActionView(item)
		accumulateGovernanceActionStats(&result, view)
		result.RecentActions = append(result.RecentActions, view)
	}
	latest := result.RecentActions[0]
	result.LatestAction = &latest
	return result
}

func accumulateGovernanceActionStats(summary *clusterGovernanceSummaryView, item clusterGovernanceRecentActionView) {
	if summary == nil {
		return
	}
	targetStatus := ""
	if item.Result != nil {
		targetStatus = strings.TrimSpace(item.Result.TargetStatus)
	}
	switch item.ActionName {
	case "cluster.node.enable":
		if targetStatus == "disabled" {
			summary.NodeDisableTotal++
			return
		}
		summary.NodeEnableTotal++
	case "cluster.node.quarantine":
		if targetStatus == "active" {
			summary.NodeUnquarantineTotal++
			return
		}
		summary.NodeQuarantineTotal++
	case "cluster.node.drain":
		if targetStatus == "schedulable" {
			summary.NodeResumeTotal++
			return
		}
		summary.NodeDrainTotal++
	case "cluster.worker.offline":
		summary.WorkerOfflineTotal++
	case "cluster.worker.exit":
		summary.WorkerExitTotal++
	case "cluster.job.takeover":
		summary.JobTakeoverTotal++
	}
}

func (h *ClusterHandler) activeJobCountByNode(ctx context.Context) map[uint64]int {
	if h.jobRepository == nil {
		return map[uint64]int{}
	}
	return h.jobRepository.CountActiveByNode(ctx)
}

func (h *ClusterHandler) workerSummary(ctx context.Context) (int, int) {
	if h.workerInstanceRepo == nil {
		return 0, 0
	}
	return h.workerInstanceRepo.CountSummary(ctx, time.Now().Add(-clusterNodeOnlineGracePeriod))
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
	if h.nodeRepository == nil {
		return nil
	}
	return h.buildMemberViewsFromNodes(ctx, h.listClusterNodes(ctx))
}

func (h *ClusterHandler) buildMemberViewsFromNodes(ctx context.Context, nodes []mysql.ClusterNodeRecord) []clusterMemberView {
	nodeViews := h.buildNodeStatusViews(ctx, nodes)
	return h.buildMemberViewsFromPreparedNodes(ctx, nodes, nodeViews)
}

func (h *ClusterHandler) buildMemberViewsFromPreparedNodes(ctx context.Context, nodes []mysql.ClusterNodeRecord, nodeViews []ClusterNodeView) []clusterMemberView {
	nodeViewByID := make(map[uint64]ClusterNodeView, len(nodeViews))
	for _, item := range nodeViews {
		nodeViewByID[item.NodeID] = item
	}
	workersByNode, onlineWorkersByNode := h.workerSummaryByNode(ctx)
	registryHeartbeatByNode := h.listRegistryHeartbeatByNode(ctx)

	result := make([]clusterMemberView, 0, len(nodes))
	now := time.Now()
	for _, node := range nodes {
		var registryHeartbeatAt *time.Time
		source := "node_table"
		if heartbeatAt, ok := registryHeartbeatByNode[node.NodeID]; ok {
			value := heartbeatAt
			registryHeartbeatAt = &value
			source = "node_table+registry"
		}

		var lastMetricsAt *time.Time
		schedulerReady := false
		stateReason := "node_record_missing"
		onlineSignalSource := "unknown"
		if nodeView, ok := nodeViewByID[node.NodeID]; ok {
			lastMetricsAt = nodeView.LastMetricsAt
			schedulerReady = nodeView.SchedulerReady
			stateReason = nodeView.StateReason
			onlineSignalSource = resolveNodeOnlineSignalSource(nodeView)
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
			OnlineEstimate:      estimateNodeOnlineWithRegistry(node, lastMetricsAt, registryHeartbeatAt, now),
			OnlineSignalSource:  resolveEffectiveMemberOnlineSignalSource(node, lastMetricsAt, registryHeartbeatAt, now, onlineSignalSource),
			SchedulerReady:      schedulerReady,
			StateReason:         stateReason,
			Source:              source,
			LastHeartbeatAt:     node.LastHeartbeatAt,
			RegistryHeartbeatAt: registryHeartbeatAt,
			LastMetricsAt:       lastMetricsAt,
			WorkerTotal:         workersByNode[node.NodeID],
			WorkerOnlineTotal:   onlineWorkersByNode[node.NodeID],
		}
		result = append(result, view)
	}
	return result
}

func (h *ClusterHandler) workerSummaryByNode(ctx context.Context) (map[uint64]int, map[uint64]int) {
	if h.workerInstanceRepo == nil {
		return map[uint64]int{}, map[uint64]int{}
	}
	return h.workerInstanceRepo.CountByNodeSummary(ctx, time.Now().Add(-clusterNodeOnlineGracePeriod))
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

func estimateNodeOnlineWithRegistry(node mysql.ClusterNodeRecord, lastMetricsAt *time.Time, registryHeartbeatAt *time.Time, now time.Time) bool {
	if estimateNodeOnline(node, lastMetricsAt, now) {
		return true
	}
	if registryHeartbeatAt != nil && !registryHeartbeatAt.IsZero() && now.Sub(*registryHeartbeatAt) <= clusterNodeOnlineGracePeriod {
		return true
	}
	return false
}

func resolveMemberOnlineSignalSource(node mysql.ClusterNodeRecord, lastMetricsAt *time.Time, registryHeartbeatAt *time.Time, now time.Time) string {
	if lastMetricsAt != nil && !lastMetricsAt.IsZero() && now.Sub(*lastMetricsAt) <= clusterNodeOnlineGracePeriod {
		return "metrics"
	}
	if !node.LastHeartbeatAt.IsZero() && now.Sub(node.LastHeartbeatAt) <= clusterNodeOnlineGracePeriod {
		return "node_heartbeat"
	}
	if registryHeartbeatAt != nil && !registryHeartbeatAt.IsZero() && now.Sub(*registryHeartbeatAt) <= clusterNodeOnlineGracePeriod {
		return "registry"
	}
	return "unknown"
}

func resolveEffectiveMemberOnlineSignalSource(node mysql.ClusterNodeRecord, lastMetricsAt *time.Time, registryHeartbeatAt *time.Time, now time.Time, preferred string) string {
	if strings.TrimSpace(preferred) != "" && preferred != "unknown" {
		return preferred
	}
	return resolveMemberOnlineSignalSource(node, lastMetricsAt, registryHeartbeatAt, now)
}

func (h *ClusterHandler) topologySummary(ctx context.Context, cfg config.DynamicRuntimeConfig, nodeViews []ClusterNodeView) map[string]any {
	controlPlaneTotal := 0
	adminAccessibleTotal := 0
	workerOnlyTotal := 0
	controlPlaneNodes := make([]map[string]any, 0)
	recommendedAdminNodeID := uint64(0)
	recommendedAdminBaseURL := ""
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
			if adminAccessible && item.OnlineEstimate && recommendedAdminNodeID == 0 {
				recommendedAdminNodeID = item.NodeID
				recommendedAdminBaseURL = normalizeAdminBaseURL(item.HTTPHost, item.HostIP)
			}
		}
		if adminAccessible {
			adminAccessibleTotal++
		}
		if nodeRole == "cluster-worker" {
			workerOnlyTotal++
		}
	}

	currentNodeRole := h.nodeMode
	currentIsControlPlane, currentAdminAccessible := evaluateNodeManagementCapability(currentNodeRole, true, h.localHTTPAddress)
	if currentIsControlPlane && currentAdminAccessible {
		recommendedAdminNodeID = h.localNodeID
		recommendedAdminBaseURL = normalizeAdminBaseURL(h.localHTTPAddress, h.localAdvertiseIP)
	}
	return map[string]any{
		"current_node_id":            h.localNodeID,
		"current_node_role":          currentNodeRole,
		"current_is_control_plane":   currentIsControlPlane,
		"current_admin_accessible":   currentAdminAccessible,
		"current_http_address":       h.localHTTPAddress,
		"recommended_admin_node_id":  recommendedAdminNodeID,
		"recommended_admin_base_url": recommendedAdminBaseURL,
		"control_plane_total":        controlPlaneTotal,
		"admin_accessible_total":     adminAccessibleTotal,
		"worker_only_total":          workerOnlyTotal,
		"control_plane_nodes":        controlPlaneNodes,
		"generated_at":               time.Now(),
		"db_context_ok":              h.nodeRepository != nil && h.db != nil && ctx != nil,
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

func splitCommaText(raw string) []string {
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, item := range parts {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
	}
	return result
}

type publicGRPCRegistryInspection struct {
	Key             string
	Exists          bool
	ValueJSON       string
	LeaseID         int64
	LeaseTTLSeconds int
}

func (h *ClusterHandler) inspectPublicGRPCRegistryKey(ctx context.Context, record mysql.RegistryEtcdConfigRecord, effectiveEndpoint string) publicGRPCRegistryInspection {
	publishedHost := strings.TrimSpace(h.localAdvertiseIP)
	if publishedHost == "" {
		return publicGRPCRegistryInspection{}
	}
	_, portText, err := net.SplitHostPort(effectiveEndpoint)
	if err != nil {
		return publicGRPCRegistryInspection{}
	}
	endpoint := net.JoinHostPort(publishedHost, portText)
	key := buildPublicGRPCRegistryKeyForAdmin(record.ServiceNamespace, "transcode.v1.TranscodePublicService", endpoint)
	endpoints := splitCommaText(record.Endpoints)
	if len(endpoints) == 0 {
		return publicGRPCRegistryInspection{Key: key}
	}
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   endpoints,
		DialTimeout: resolveRegistryDialTimeoutForAdmin(record),
	})
	if err != nil {
		return publicGRPCRegistryInspection{Key: key}
	}
	defer func() { _ = client.Close() }()
	resp, err := client.Get(ctx, key)
	if err != nil || resp == nil || len(resp.Kvs) == 0 {
		return publicGRPCRegistryInspection{Key: key}
	}
	result := publicGRPCRegistryInspection{
		Key:       key,
		Exists:    true,
		ValueJSON: string(resp.Kvs[0].Value),
		LeaseID:   int64(resp.Kvs[0].Lease),
	}
	if resp.Kvs[0].Lease > 0 {
		leaseResp, err := client.TimeToLive(ctx, clientv3.LeaseID(resp.Kvs[0].Lease))
		if err == nil && leaseResp != nil && leaseResp.TTL > 0 {
			result.LeaseTTLSeconds = int(leaseResp.TTL)
		}
	}
	return result
}

func resolveRegistryDialTimeoutForAdmin(record mysql.RegistryEtcdConfigRecord) time.Duration {
	if record.DialTimeoutMS > 0 {
		return time.Duration(record.DialTimeoutMS) * time.Millisecond
	}
	return 3 * time.Second
}

func buildPublicGRPCRegistryKeyForAdmin(namespace, serviceName, endpoint string) string {
	namespace = strings.Trim(strings.TrimSpace(namespace), "/")
	serviceName = strings.Trim(strings.TrimSpace(serviceName), "/")
	endpoint = strings.Trim(strings.TrimSpace(endpoint), "/")
	if namespace == "" {
		return fmt.Sprintf("/%s/%s", serviceName, endpoint)
	}
	return fmt.Sprintf("/%s/%s/%s", namespace, serviceName, endpoint)
}

func formatGovernanceAuditMessage(payload any) string {
	raw, err := json.Marshal(payload)
	if err != nil {
		return "ok"
	}
	return string(raw)
}

func normalizeAdminBaseURL(httpHost string, hostIP string) string {
	httpHost = strings.TrimSpace(httpHost)
	hostIP = strings.TrimSpace(hostIP)
	if httpHost == "" {
		return ""
	}
	if strings.HasPrefix(httpHost, "http://") || strings.HasPrefix(httpHost, "https://") {
		return httpHost
	}
	if strings.HasPrefix(httpHost, ":") && hostIP != "" {
		return "http://" + hostIP + httpHost
	}
	if host, port, err := net.SplitHostPort(httpHost); err == nil {
		switch strings.TrimSpace(host) {
		case "", "0.0.0.0", "::":
			if hostIP != "" {
				return "http://" + net.JoinHostPort(hostIP, port)
			}
		}
	}
	return "http://" + httpHost
}

func resolveInternalGRPCTarget(hostIP string, grpcHost string) (string, error) {
	grpcHost = strings.TrimSpace(grpcHost)
	if grpcHost == "" {
		return "", fmt.Errorf("target internal grpc host is empty")
	}
	host, portText, err := net.SplitHostPort(grpcHost)
	if err != nil {
		return "", fmt.Errorf("invalid internal grpc host %q: %w", grpcHost, err)
	}
	if strings.TrimSpace(host) == "" || host == "0.0.0.0" || host == "::" {
		host = strings.TrimSpace(hostIP)
	}
	if host == "" {
		return "", fmt.Errorf("target internal grpc host ip is empty")
	}
	return net.JoinHostPort(host, portText), nil
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

func collectNodeIDs(nodes []mysql.ClusterNodeRecord) []uint64 {
	result := make([]uint64, 0, len(nodes))
	seen := make(map[uint64]struct{}, len(nodes))
	for _, node := range nodes {
		if node.NodeID == 0 {
			continue
		}
		if _, exists := seen[node.NodeID]; exists {
			continue
		}
		seen[node.NodeID] = struct{}{}
		result = append(result, node.NodeID)
	}
	return result
}
