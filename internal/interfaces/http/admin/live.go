package admin

import (
	"encoding/json"
	"net/http"
	"strconv"

	"hvc/internal/live"
	"hvc/internal/model"
	liveservice "hvc/internal/service/live"
	"hvc/pkg/logx"
)

// LiveHandler 处理后台直播频道与会话管理接口。
type LiveHandler struct {
	manager        *live.Manager
	channelService *liveservice.ChannelService
}

// NewLiveHandler 创建直播频道管理处理器。
func NewLiveHandler(manager *live.Manager, channelService *liveservice.ChannelService) *LiveHandler {
	return &LiveHandler{
		manager:        manager,
		channelService: channelService,
	}
}

// ListChannels 返回直播频道列表，分页和过滤直接下推到数据库。
func (h *LiveHandler) ListChannels(w http.ResponseWriter, r *http.Request) {
	if h.channelService == nil {
		logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "频道服务未初始化"})
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
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
		logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "查询频道列表失败"})
		return
	}

	page, pageSize = normalizePage(page, pageSize)
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"items":     items,
	}})
}

// ListSessions 返回直播会话列表，支持按频道和状态过滤。
func (h *LiveHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	if h.channelService == nil {
		logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "频道服务未初始化"})
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
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
		logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "查询会话列表失败"})
		return
	}

	page, pageSize = normalizePage(page, pageSize)
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"items":     items,
	}})
}

// CreateChannel 创建直播频道。
func (h *LiveHandler) CreateChannel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChannelKey  string `json:"channel_key"`
		ChannelName string `json:"channel_name"`
		ProfileID   uint64 `json:"profile_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "请求参数无效"})
		return
	}
	if req.ChannelKey == "" {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "channel_key 不能为空"})
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
			logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "创建频道失败"})
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
		logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "频道服务未初始化"})
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
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "channel_id 或 channel_key 至少传一个"})
		return
	}
	if !ok {
		logx.WriteJSON(w, http.StatusNotFound, model.Response{Code: 404, Message: "频道不存在"})
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "请求参数无效"})
		return
	}
	if req.ChannelID == 0 {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "channel_id 不能为空"})
		return
	}
	if h.channelService == nil {
		logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "频道服务未初始化"})
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
		logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "更新频道失败"})
		return
	}
	if !ok {
		logx.WriteJSON(w, http.StatusNotFound, model.Response{Code: 404, Message: "频道不存在"})
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
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "请求参数无效"})
		return
	}
	if req.ChannelID == 0 {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "channel_id 不能为空"})
		return
	}
	if h.manager != nil {
		if err := h.manager.StartChannel(ctx, req.ChannelID, req.NodeID, req.WorkerID); err != nil {
			logx.Error("admin.live.start_channel_failed", err, logx.Fields{"channel_id": req.ChannelID})
			logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "启动失败"})
			return
		}
	}

	logx.Info("admin.live.channel_started", logx.Fields{"channel_id": req.ChannelID})
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok"})
}

// StopChannel 停止直播频道。
func (h *LiveHandler) StopChannel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req struct {
		ChannelID uint64 `json:"channel_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "请求参数无效"})
		return
	}
	if req.ChannelID == 0 {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "channel_id 不能为空"})
		return
	}
	if h.manager != nil {
		if err := h.manager.StopChannel(ctx, req.ChannelID); err != nil {
			logx.Error("admin.live.stop_channel_failed", err, logx.Fields{"channel_id": req.ChannelID})
			logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "停止失败"})
			return
		}
	}

	logx.Info("admin.live.channel_stopped", logx.Fields{"channel_id": req.ChannelID})
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok"})
}

// DeleteChannel 删除直播频道。
func (h *LiveHandler) DeleteChannel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChannelID uint64 `json:"channel_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "请求参数无效"})
		return
	}
	if req.ChannelID == 0 {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "channel_id 不能为空"})
		return
	}
	if h.channelService == nil {
		logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "频道服务未初始化"})
		return
	}

	deleted, err := h.channelService.DeleteChannelContext(r.Context(), req.ChannelID)
	if err != nil {
		logx.Error("admin.live.channel_delete_failed", err, logx.Fields{"channel_id": req.ChannelID})
		logx.WriteJSON(w, http.StatusConflict, model.Response{Code: 409, Message: err.Error()})
		return
	}
	if !deleted {
		logx.WriteJSON(w, http.StatusNotFound, model.Response{Code: 404, Message: "频道不存在"})
		return
	}

	logx.Info("admin.live.channel_deleted", logx.Fields{"channel_id": req.ChannelID})
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok"})
}

func normalizePage(page int, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}
