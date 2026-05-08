package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	clusterstate "hvc/internal/cluster"
	"hvc/internal/cluster/membership"
	"hvc/internal/config"
	"hvc/internal/configcenter"
	rediscache "hvc/internal/infra/cache/redis"
	"hvc/internal/infra/db/mysql"
	"hvc/internal/model"
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
	db                      *mysql.DB
	internalGRPCConfig      config.InternalGRPCConfig
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
	OnlineEstimate   bool                  `json:"online_estimate"`
	LastMetricsAt    *time.Time            `json:"last_metrics_at,omitempty"`
	Metrics          *model.NodeMetrics    `json:"metrics,omitempty"`
	GPUSummary       ClusterNodeGPUSummary `json:"gpu_summary"`
	GPUDevices       []ClusterNodeGPUView  `json:"gpu_devices"`
}

type clusterOverviewNodeSummary struct {
	NodeID                  uint64 `json:"node_id"`
	NodeName                string `json:"node_name"`
	Enabled                 bool   `json:"enabled"`
	Quarantined             bool   `json:"quarantined"`
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
	db *mysql.DB,
	internalGRPCConfig config.InternalGRPCConfig,
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
		db:                      db,
		internalGRPCConfig:      internalGRPCConfig,
	}
}

// ListNodes 返回节点列表。
func (h *ClusterHandler) ListNodes(w http.ResponseWriter, r *http.Request) {
	items := h.buildNodeViews(r.Context(), h.nodeRepository.List(r.Context()))
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{"items": items}})
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
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: h.buildNodeView(r.Context(), item, h.listGPUByNode(r.Context(), item.NodeID), time.Now())})
}

// ListNodeMetrics 返回当前节点指标概览。
func (h *ClusterHandler) ListNodeMetrics(w http.ResponseWriter, r *http.Request) {
	nodeID, _ := strconv.ParseUint(r.URL.Query().Get("node_id"), 10, 64)
	metrics := make([]model.NodeMetrics, 0)
	if nodeID > 0 {
		if value, ok := h.stateCache.GetNodeMetrics(r.Context(), nodeID); ok {
			metrics = append(metrics, value)
		}
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{"items": metrics}})
		return
	}

	items := h.nodeRepository.List(r.Context())
	metrics = make([]model.NodeMetrics, 0, len(items))
	for _, item := range items {
		if value, ok := h.stateCache.GetNodeMetrics(r.Context(), item.NodeID); ok {
			metrics = append(metrics, value)
		}
	}
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{"items": metrics}})
}

// ListMembers 返回当前内部成员注册表快照。
func (h *ClusterHandler) ListMembers(w http.ResponseWriter, r *http.Request) {
	items := h.registry.List(r.Context())
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{"items": items}})
}

// Overview 返回集群运行模式、节点/GPU 概览和配置版本信息。
//
// 该接口面向运维与排障，尽量一次返回最常用的关键状态，
// 让后台页面和巡检脚本都不需要再自己拼装多次查询结果。
func (h *ClusterHandler) Overview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cfg := h.currentConfig()
	nodes := h.nodeRepository.List(ctx)
	members := h.registry.List(ctx)
	nodeViews := h.buildNodeViews(ctx, nodes)

	nodeEnabledCount := 0
	nodeQuarantinedCount := 0
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
		"module_status": h.moduleStatus(cfg),
		"internal_grpc": map[string]any{
			"listen_address": h.internalGRPCConfig.ListenAddress,
		},
		"cluster": map[string]any{
			"node_total":             len(nodes),
			"node_enabled_total":     nodeEnabledCount,
			"node_quarantined_total": nodeQuarantinedCount,
			"node_online_total":      nodeOnlineCount,
			"member_total":           len(members),
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
	nodes := h.nodeRepository.List(ctx)
	nodeViews := h.buildNodeViews(ctx, nodes)
	members := h.registry.List(ctx)

	activeSessions := 0
	uploadQueueDepth := 0
	nodeEnabledCount := 0
	nodeQuarantinedCount := 0
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
		"module_status": h.moduleStatus(cfg),
		"internal_grpc": map[string]any{
			"listen_address": h.internalGRPCConfig.ListenAddress,
		},
		"version": h.versionSummary(ctx),
		"queue":   h.queueSummary(ctx),
		"cluster": map[string]any{
			"node_total":             len(nodeViews),
			"node_enabled_total":     nodeEnabledCount,
			"node_quarantined_total": nodeQuarantinedCount,
			"node_online_total":      nodeOnlineCount,
			"member_total":           len(members),
			"gpu_total":              gpuTotal,
			"gpu_healthy_total":      gpuHealthyTotal,
			"gpu_schedulable_total":  gpuSchedulableTotal,
			"active_sessions":        activeSessions,
			"upload_queue_depth":     uploadQueueDepth,
			"max_transcode_sessions": maxTranscodeSessions,
		},
		"nodes": nodeViews,
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
	}
	view.OnlineEstimate = estimateNodeOnline(node, view.LastMetricsAt, now)

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
		summary := clusterOverviewNodeSummary{
			NodeID:              item.NodeID,
			NodeName:            item.NodeName,
			Enabled:             item.Enabled,
			Quarantined:         item.Quarantined,
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

func estimateNodeOnline(node mysql.ClusterNodeRecord, lastMetricsAt *time.Time, now time.Time) bool {
	if lastMetricsAt != nil && !lastMetricsAt.IsZero() && now.Sub(*lastMetricsAt) <= clusterNodeOnlineGracePeriod {
		return true
	}
	if !node.LastHeartbeatAt.IsZero() && now.Sub(node.LastHeartbeatAt) <= clusterNodeOnlineGracePeriod {
		return true
	}
	return false
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
