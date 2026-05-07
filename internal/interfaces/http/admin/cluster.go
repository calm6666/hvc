package admin

import (
	"encoding/json"
	"net/http"

	clusterstate "hvc/internal/cluster"
	"hvc/internal/cluster/membership"
	"hvc/internal/infra/db/mysql"
	"hvc/internal/model"
	"hvc/pkg/logx"
)

// ClusterHandler 处理后台集群管理接口。
type ClusterHandler struct {
	nodeRepository *mysql.ClusterNodeRepository
	stateCache     *clusterstate.StateCache
	registry       *membership.Registry
}

// NewClusterHandler 创建后台集群处理器。
func NewClusterHandler(nodeRepository *mysql.ClusterNodeRepository, stateCache *clusterstate.StateCache, registry *membership.Registry) *ClusterHandler {
	return &ClusterHandler{nodeRepository: nodeRepository, stateCache: stateCache, registry: registry}
}

// ListNodes 返回节点列表。
func (h *ClusterHandler) ListNodes(w http.ResponseWriter, r *http.Request) {
	items := h.nodeRepository.List(r.Context())
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{"items": items}})
}

// GetNodeDetail 返回节点详情。
func (h *ClusterHandler) GetNodeDetail(w http.ResponseWriter, r *http.Request) {
	var req struct{ NodeID uint64 `json:"node_id"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "invalid request"})
		return
	}
	item, ok := h.nodeRepository.FindByNodeID(r.Context(), req.NodeID)
	if !ok {
		logx.WriteJSON(w, http.StatusNotFound, model.Response{Code: 404, Message: "node not found"})
		return
	}
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: item})
}

// ListNodeMetrics 返回当前节点指标概览。
func (h *ClusterHandler) ListNodeMetrics(w http.ResponseWriter, r *http.Request) {
	items := h.nodeRepository.List(r.Context())
	metrics := make([]model.NodeMetrics, 0, len(items))
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
