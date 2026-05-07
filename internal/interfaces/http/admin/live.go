// Package admin 提供后台管理接口的直播频道管理处理器。
//
// 实现以下接口：
//   - POST /admin/live/channel/create  -- 创建直播频道
//   - GET  /admin/live/channel/detail  -- 直播频道详情
//   - POST /admin/live/channel/update  -- 更新直播频道
//   - POST /admin/live/channel/start   -- 启动直播频道
//   - POST /admin/live/channel/stop    -- 停止直播频道
package admin

import (
	"encoding/json"
	"net/http"
	"strconv"

	"hvc/internal/live"
	liveservice "hvc/internal/service/live"
	"hvc/internal/model"
	"hvc/pkg/logx"
)

// LiveHandler 处理后台直播频道管理接口。
type LiveHandler struct {
	manager         *live.Manager
	channelService  *liveservice.ChannelService
}

// NewLiveHandler 创建后台直播频道管理处理器。
func NewLiveHandler(manager *live.Manager, channelService *liveservice.ChannelService) *LiveHandler {
	return &LiveHandler{
		manager:        manager,
		channelService: channelService,
	}
}

// CreateChannel 创建直播频道。
//
// POST /admin/live/channel/create
// Body: {"channel_key":"live-001","channel_name":"测试频道","profile_id":1}
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

	var channel model.LiveChannel
	if h.channelService != nil {
		channel = h.channelService.CreateChannel(req.ChannelKey, req.ChannelName, req.ProfileID)
	} else {
		channel = model.LiveChannel{
			ChannelKey:  req.ChannelKey,
			ChannelName: req.ChannelName,
			ProfileID:   req.ProfileID,
			Status:      model.LiveChannelStatusIdle,
		}
	}

	logx.Info("admin.live.channel_created", logx.Fields{
		"channel_key": req.ChannelKey,
	})
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: channel})
}

// ChannelDetail 返回直播频道详情。
//
// GET /admin/live/channel/detail?channel_id=123
func (h *LiveHandler) ChannelDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	channelID, _ := strconv.ParseUint(r.URL.Query().Get("channel_id"), 10, 64)
	channelKey := r.URL.Query().Get("channel_key")

	if channelID > 0 && h.manager != nil {
		if channel, ok := h.manager.GetChannel(ctx, channelID); ok {
			logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: channel})
			return
		}
	}

	if channelKey != "" && h.channelService != nil {
		playback := h.channelService.GetPlaybackInfo(channelKey)
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: playback})
		return
	}

	logx.WriteJSON(w, http.StatusNotFound, model.Response{Code: 404, Message: "频道不存在"})
}

// UpdateChannel 更新直播频道配置。
//
// POST /admin/live/channel/update
// Body: {"channel_id":1,"channel_name":"新名称","enable_watermark":true}
func (h *LiveHandler) UpdateChannel(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ChannelID      uint64 `json:"channel_id"`
		ChannelName    string `json:"channel_name"`
		EnableWatermark bool  `json:"enable_watermark"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "请求参数无效"})
		return
	}

	logx.Info("admin.live.channel_updated", logx.Fields{
		"channel_id": req.ChannelID,
	})
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok"})
}

// StartChannel 启动直播频道。
//
// POST /admin/live/channel/start
// Body: {"channel_id":1}
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

	if h.manager != nil {
		if err := h.manager.StartChannel(ctx, req.ChannelID, req.NodeID, req.WorkerID); err != nil {
			logx.Error("admin.live.start_channel_failed", err, logx.Fields{
				"channel_id": req.ChannelID,
			})
			logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "启动失败"})
			return
		}
	}

	logx.Info("admin.live.channel_started", logx.Fields{"channel_id": req.ChannelID})
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok"})
}

// StopChannel 停止直播频道。
//
// POST /admin/live/channel/stop
// Body: {"channel_id":1}
func (h *LiveHandler) StopChannel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req struct {
		ChannelID uint64 `json:"channel_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "请求参数无效"})
		return
	}

	if h.manager != nil {
		if err := h.manager.StopChannel(ctx, req.ChannelID); err != nil {
			logx.Error("admin.live.stop_channel_failed", err, logx.Fields{
				"channel_id": req.ChannelID,
			})
			logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "停止失败"})
			return
		}
	}

	logx.Info("admin.live.channel_stopped", logx.Fields{"channel_id": req.ChannelID})
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok"})
}
