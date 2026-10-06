package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"hvc/internal/infra/db/mysql"
	"hvc/internal/live"
	"hvc/internal/model"
	liveservice "hvc/internal/service/live"
	"hvc/pkg/logx"
)

// LiveHandler 处理后台直播频道与会话管理接口。
type LiveHandler struct {
	manager        *live.Manager
	channelService *liveservice.ChannelService
	nodeRepository *mysql.ClusterNodeRepository
	internalToken  string
	localNodeID    uint64
	httpClient     *http.Client
}

type liveControlDispatchResult struct {
	ControlPath    string `json:"control_path"`
	TargetNodeID   uint64 `json:"target_node_id"`
	TargetWorkerID string `json:"target_worker_id,omitempty"`
}

// NewLiveHandler 创建直播频道管理处理器。
func NewLiveHandler(manager *live.Manager, channelService *liveservice.ChannelService) *LiveHandler {
	return &LiveHandler{
		manager:        manager,
		channelService: channelService,
		httpClient:     &http.Client{Timeout: 5 * time.Second},
	}
}

// ConfigureInternalControl 配置直播内部控制链路。
//
// 后台管理接口只负责决定“在哪个节点启动/停止频道”，
// 真正执行动作的请求通过这里配置的节点仓储和内部认证令牌下发到目标节点。
func (h *LiveHandler) ConfigureInternalControl(nodeRepository *mysql.ClusterNodeRepository, internalToken string, localNodeID uint64) {
	if h == nil {
		return
	}
	h.nodeRepository = nodeRepository
	h.internalToken = strings.TrimSpace(internalToken)
	h.localNodeID = localNodeID
	if h.httpClient == nil {
		h.httpClient = &http.Client{Timeout: 5 * time.Second}
	}
}

// ListChannels 返回直播频道列表，分页和过滤直接下推到数据库。
func (h *LiveHandler) ListChannels(w http.ResponseWriter, r *http.Request) {
	if h.channelService == nil {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "频道服务未初始化"})
		return
	}

	page, pageSize := parsePageParams(r)
	status := r.URL.Query().Get("status")
	channelKey := r.URL.Query().Get("channel_key")

	items, total, err := h.channelService.ListChannelsContext(r.Context(), liveservice.ChannelListFilter{
		Page:       page,
		PageSize:   pageSize,
		Status:     status,
		ChannelKey: channelKey,
	})
	if err != nil {
		logx.Error("admin.live.list_channels_failed", err, logx.Fields{
			"page":        page,
			"page_size":   pageSize,
			"status":      status,
			"channel_key": channelKey,
		})
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "查询频道列表失败"})
		return
	}

	writePageResponse(w, page, pageSize, total, items)
}

// ListSessions 返回直播会话列表，支持按频道和状态过滤。
func (h *LiveHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	if h.channelService == nil {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "频道服务未初始化"})
		return
	}

	page, pageSize := parsePageParams(r)
	channelID, _ := strconv.ParseUint(r.URL.Query().Get("channel_id"), 10, 64)
	channelKey := r.URL.Query().Get("channel_key")
	status := r.URL.Query().Get("status")

	items, total, err := h.channelService.ListSessionsContext(r.Context(), liveservice.SessionListFilter{
		Page:       page,
		PageSize:   pageSize,
		ChannelID:  channelID,
		ChannelKey: channelKey,
		Status:     status,
	})
	if err != nil {
		logx.Error("admin.live.list_sessions_failed", err, logx.Fields{
			"page":        page,
			"page_size":   pageSize,
			"channel_id":  channelID,
			"channel_key": channelKey,
			"status":      status,
		})
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "查询会话列表失败"})
		return
	}

	writePageResponse(w, page, pageSize, total, items)
}

// CreateChannel 创建直播频道。
func (h *LiveHandler) CreateChannel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChannelKey  string `json:"channel_key"`
		ChannelName string `json:"channel_name"`
		ProfileID   uint64 `json:"profile_id"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.ChannelKey == "" {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: "channel_key 不能为空"})
		return
	}

	channel := model.LiveChannel{
		ChannelKey:  req.ChannelKey,
		ChannelName: req.ChannelName,
		ProfileID:   req.ProfileID,
		Status:      model.LiveChannelStatusIdle,
	}
	if h.channelService != nil {
		created, err := h.channelService.CreateChannelContext(r.Context(), req.ChannelKey, req.ChannelName, req.ProfileID)
		if err != nil {
			logx.Error("admin.live.channel_create_failed", err, logx.Fields{"channel_key": req.ChannelKey})
			logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "创建频道失败"})
			return
		}
		channel = created
	}

	logx.Info("admin.live.channel_created", logx.Fields{"channel_key": req.ChannelKey})
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: channel})
}

// ChannelDetail 查询频道详情，并附带当前播放信息和活跃会话。
func (h *LiveHandler) ChannelDetail(w http.ResponseWriter, r *http.Request) {
	if h.channelService == nil {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "频道服务未初始化"})
		return
	}

	ctx := r.Context()
	channelID, _ := strconv.ParseUint(r.URL.Query().Get("channel_id"), 10, 64)
	channelKey := r.URL.Query().Get("channel_key")

	var (
		channel model.LiveChannel
		ok      bool
	)
	switch {
	case channelID > 0:
		channel, ok = h.channelService.GetChannelByIDContext(ctx, channelID)
	case channelKey != "":
		channel, ok = h.channelService.GetChannelByKeyContext(ctx, channelKey)
	default:
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: "channel_id 或 channel_key 至少传一个"})
		return
	}
	if !ok {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 404, Message: "频道不存在"})
		return
	}

	detail := map[string]any{
		"channel":  channel,
		"playback": h.channelService.GetPlaybackInfoContext(ctx, channel.ChannelKey),
	}
	if h.manager != nil {
		if session, found := h.manager.GetActiveSession(ctx, channel.ChannelKey); found {
			detail["active_session"] = session
		}
	}

	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: detail})
}

// UpdateChannel 更新频道配置，只修改请求体中明确传入的字段。
func (h *LiveHandler) UpdateChannel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChannelID             uint64  `json:"channel_id"`
		ChannelName           *string `json:"channel_name"`
		ProfileID             *uint64 `json:"profile_id"`
		EnableSourceRendition *bool   `json:"enable_source_rendition"`
		EnableWatermark       *bool   `json:"enable_watermark"`
		PlayDomain            *string `json:"play_domain"`
		PushDomain            *string `json:"push_domain"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.ChannelID == 0 {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: "channel_id 不能为空"})
		return
	}
	if h.channelService == nil {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "频道服务未初始化"})
		return
	}

	channel, ok, err := h.channelService.UpdateChannelContext(r.Context(), liveservice.ChannelPatch{
		ChannelID:             req.ChannelID,
		ChannelName:           req.ChannelName,
		ProfileID:             req.ProfileID,
		EnableSourceRendition: req.EnableSourceRendition,
		EnableWatermark:       req.EnableWatermark,
		PlayDomain:            req.PlayDomain,
		PushDomain:            req.PushDomain,
	})
	if err != nil {
		logx.Error("admin.live.channel_update_failed", err, logx.Fields{"channel_id": req.ChannelID})
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "更新频道失败"})
		return
	}
	if !ok {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 404, Message: "频道不存在"})
		return
	}

	logx.Info("admin.live.channel_updated", logx.Fields{"channel_id": req.ChannelID})
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: channel})
}

// StartChannel 启动直播频道。
func (h *LiveHandler) StartChannel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req struct {
		ChannelID uint64 `json:"channel_id"`
		NodeID    uint64 `json:"node_id"`
		WorkerID  string `json:"worker_id"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.ChannelID == 0 {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: "channel_id 不能为空"})
		return
	}
	result, err := h.dispatchStartChannel(ctx, req.ChannelID, req.NodeID, req.WorkerID)
	if err != nil {
		logx.Error("admin.live.start_channel_failed", err, logx.Fields{
			"channel_id": req.ChannelID,
			"node_id":    req.NodeID,
			"worker_id":  req.WorkerID,
		})
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "启动失败"})
		return
	}

	logx.Info("admin.live.channel_started", logx.Fields{"channel_id": req.ChannelID})
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{
		"channel_id":        req.ChannelID,
		"control_path":      result.ControlPath,
		"target_node_id":    result.TargetNodeID,
		"target_worker_id":  result.TargetWorkerID,
	}})
}

// StopChannel 停止直播频道。
func (h *LiveHandler) StopChannel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req struct {
		ChannelID uint64 `json:"channel_id"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.ChannelID == 0 {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: "channel_id 不能为空"})
		return
	}
	result, err := h.dispatchStopChannel(ctx, req.ChannelID)
	if err != nil {
		logx.Error("admin.live.stop_channel_failed", err, logx.Fields{"channel_id": req.ChannelID})
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "停止失败"})
		return
	}

	logx.Info("admin.live.channel_stopped", logx.Fields{"channel_id": req.ChannelID})
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{
		"channel_id":       req.ChannelID,
		"control_path":     result.ControlPath,
		"target_node_id":   result.TargetNodeID,
		"target_worker_id": result.TargetWorkerID,
	}})
}

// DeleteChannel 删除直播频道。
func (h *LiveHandler) DeleteChannel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChannelID uint64 `json:"channel_id"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.ChannelID == 0 {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: "channel_id 不能为空"})
		return
	}
	if h.channelService == nil {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "频道服务未初始化"})
		return
	}

	deleted, err := h.channelService.DeleteChannelContext(r.Context(), req.ChannelID)
	if err != nil {
		logx.Error("admin.live.channel_delete_failed", err, logx.Fields{"channel_id": req.ChannelID})
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 409, Message: err.Error()})
		return
	}
	if !deleted {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 404, Message: "频道不存在"})
		return
	}

	logx.Info("admin.live.channel_deleted", logx.Fields{"channel_id": req.ChannelID})
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok"})
}

func (h *LiveHandler) dispatchStartChannel(ctx context.Context, channelID uint64, nodeID uint64, workerID string) (liveControlDispatchResult, error) {
	workerID = strings.TrimSpace(workerID)
	if nodeID == 0 || nodeID == h.localNodeID || h.nodeRepository == nil {
		if h.manager == nil {
			return liveControlDispatchResult{
				ControlPath:    "local",
				TargetNodeID:   h.resolveLocalTargetNodeID(nodeID),
				TargetWorkerID: workerID,
			}, nil
		}
		if err := h.manager.StartChannel(ctx, channelID, nodeID, workerID); err != nil {
			return liveControlDispatchResult{}, err
		}
		return liveControlDispatchResult{
			ControlPath:    "local",
			TargetNodeID:   h.resolveLocalTargetNodeID(nodeID),
			TargetWorkerID: workerID,
		}, nil
	}
	node, ok := h.nodeRepository.FindByID(ctx, nodeID)
	if !ok {
		if h.manager == nil {
			return liveControlDispatchResult{}, fmt.Errorf("目标节点不存在且本地直播管理器未初始化")
		}
		if err := h.manager.StartChannel(ctx, channelID, nodeID, workerID); err != nil {
			return liveControlDispatchResult{}, err
		}
		return liveControlDispatchResult{
			ControlPath:    "local_fallback",
			TargetNodeID:   h.resolveLocalTargetNodeID(nodeID),
			TargetWorkerID: workerID,
		}, nil
	}
	if node.HTTPHost == "" || node.NodeID == h.localNodeID {
		if h.manager == nil {
			return liveControlDispatchResult{
				ControlPath:    "local",
				TargetNodeID:   h.resolveLocalTargetNodeID(node.NodeID),
				TargetWorkerID: workerID,
			}, nil
		}
		if err := h.manager.StartChannel(ctx, channelID, nodeID, workerID); err != nil {
			return liveControlDispatchResult{}, err
		}
		return liveControlDispatchResult{
			ControlPath:    "local",
			TargetNodeID:   h.resolveLocalTargetNodeID(node.NodeID),
			TargetWorkerID: workerID,
		}, nil
	}
	if err := h.postInternalControl(ctx, normalizeAdminBaseURL(node.HTTPHost, node.HostIP)+"/v1/internal/live/channel/start", map[string]any{
		"channel_id": channelID,
		"node_id":    nodeID,
		"worker_id":  workerID,
	}); err != nil {
		return liveControlDispatchResult{}, err
	}
	return liveControlDispatchResult{
		ControlPath:    "internal_http_forward",
		TargetNodeID:   node.NodeID,
		TargetWorkerID: workerID,
	}, nil
}

func (h *LiveHandler) dispatchStopChannel(ctx context.Context, channelID uint64) (liveControlDispatchResult, error) {
	if h.manager == nil {
		return liveControlDispatchResult{
			ControlPath:  "local",
			TargetNodeID: h.localNodeID,
		}, nil
	}
	channel, ok := h.manager.GetChannel(ctx, channelID)
	if !ok || channel == nil || channel.AssignedNodeID == 0 || channel.AssignedNodeID == h.localNodeID || h.nodeRepository == nil {
		targetNodeID := h.localNodeID
		targetWorkerID := ""
		if channel != nil {
			targetNodeID = h.resolveLocalTargetNodeID(channel.AssignedNodeID)
			targetWorkerID = strings.TrimSpace(channel.AssignedWorkerID)
		}
		if err := h.manager.StopChannel(ctx, channelID); err != nil {
			return liveControlDispatchResult{}, err
		}
		return liveControlDispatchResult{
			ControlPath:    "local",
			TargetNodeID:   targetNodeID,
			TargetWorkerID: targetWorkerID,
		}, nil
	}
	node, found := h.nodeRepository.FindByID(ctx, channel.AssignedNodeID)
	if !found || strings.TrimSpace(node.HTTPHost) == "" || node.NodeID == h.localNodeID {
		if err := h.manager.StopChannel(ctx, channelID); err != nil {
			return liveControlDispatchResult{}, err
		}
		return liveControlDispatchResult{
			ControlPath:    "local",
			TargetNodeID:   h.resolveLocalTargetNodeID(channel.AssignedNodeID),
			TargetWorkerID: strings.TrimSpace(channel.AssignedWorkerID),
		}, nil
	}
	if err := h.postInternalControl(ctx, normalizeAdminBaseURL(node.HTTPHost, node.HostIP)+"/v1/internal/live/channel/stop", map[string]any{
		"channel_id": channelID,
	}); err != nil {
		return liveControlDispatchResult{}, err
	}
	return liveControlDispatchResult{
		ControlPath:    "internal_http_forward",
		TargetNodeID:   node.NodeID,
		TargetWorkerID: strings.TrimSpace(channel.AssignedWorkerID),
	}, nil
}

func (h *LiveHandler) postInternalControl(ctx context.Context, endpoint string, payload any) error {
	if h == nil {
		return nil
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token := strings.TrimSpace(h.internalToken); token != "" {
		req.Header.Set("X-HVC-Internal-Token", token)
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := h.httpClient
	if client == nil {
		client = &http.Client{Timeout: 5 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("内部直播控制调用失败，status=%d", resp.StatusCode)
	}
	return nil
}

func (h *LiveHandler) resolveLocalTargetNodeID(requestedNodeID uint64) uint64 {
	if requestedNodeID > 0 {
		return requestedNodeID
	}
	return h.localNodeID
}
