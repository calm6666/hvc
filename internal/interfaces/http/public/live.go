package public

import (
	"encoding/json"
	"net"
	"net/http"
	"strings"

	authlive "hvc/internal/live"
	livemanager "hvc/internal/live"
	"hvc/internal/model"
	livesvc "hvc/internal/service/live"
	"hvc/pkg/logx"
)

// LiveHandler 处理直播接口。
type LiveHandler struct {
	manager *livemanager.Manager
	service *livesvc.ChannelService
}

// NewLiveHandler 创建直播处理器。
func NewLiveHandler(manager *livemanager.Manager, service *livesvc.ChannelService) *LiveHandler {
	return &LiveHandler{manager: manager, service: service}
}

// GetPlaybackInfo 处理直播播放信息查询。
func (h *LiveHandler) GetPlaybackInfo(w http.ResponseWriter, r *http.Request) {
	channelKey := strings.TrimSpace(r.URL.Query().Get("channel_key"))
	if channelKey == "" {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "channel_key is required"})
		return
	}
	if _, ok := h.service.GetChannelByKeyContext(r.Context(), channelKey); !ok {
		logx.WriteJSON(w, http.StatusNotFound, model.Response{Code: 404, Message: "channel not found"})
		return
	}
	logx.Info("http.live.playback.request", logx.Fields{
		"channel_key": channelKey,
	})
	playback := h.service.GetPlaybackInfoContext(r.Context(), channelKey)
	if err := h.service.RecordIssuedPlaybackToken(r.Context(), channelKey, strings.TrimSpace(r.URL.Query().Get("viewer_id")), playback.PlayToken, playback.ExpireAt); err != nil {
		logx.Error("http.live.playback.token_record_failed", err, logx.Fields{
			"channel_key": channelKey,
		})
	}
	logx.WriteJSON(w, http.StatusOK, model.Response{
		Code:    0,
		Message: "ok",
		Data:    playback,
	})
}

// PushAuth 处理直播推流鉴权请求。
func (h *LiveHandler) PushAuth(w http.ResponseWriter, r *http.Request) {
	channelKey := strings.TrimSpace(r.URL.Query().Get("channel_key"))
	if channelKey == "" {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "channel_key is required"})
		return
	}
	if _, ok := h.service.GetChannelByKeyContext(r.Context(), channelKey); !ok {
		logx.WriteJSON(w, http.StatusNotFound, model.Response{Code: 404, Message: "channel not found"})
		return
	}

	result := authlive.OnPushAuth(h.service.CurrentAuthConfig(), channelKey, r.URL.Query())
	if err := h.service.RecordPublishAuthAttempt(r.Context(), channelKey, strings.TrimSpace(r.URL.Query().Get("stream_key")), requestRemoteIP(r), result.Allowed, result.Reason); err != nil {
		logx.Error("http.live.push_auth.record_failed", err, logx.Fields{
			"channel_key": channelKey,
		})
	}
	statusCode := http.StatusOK
	if !result.Allowed {
		statusCode = http.StatusForbidden
	}
	logx.WriteJSON(w, statusCode, model.Response{Code: 0, Message: "ok", Data: map[string]any{
		"allowed":     result.Allowed,
		"reason":      result.Reason,
		"channel_key": result.ChannelKey,
		"expire_at":   result.ExpireAt,
	}})
}

func requestRemoteIP(r *http.Request) string {
	if r == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}

// PlayAuth 处理直播播放鉴权请求。
func (h *LiveHandler) PlayAuth(w http.ResponseWriter, r *http.Request) {
	channelKey := strings.TrimSpace(r.URL.Query().Get("channel_key"))
	if channelKey == "" {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "channel_key is required"})
		return
	}
	if _, ok := h.service.GetChannelByKeyContext(r.Context(), channelKey); !ok {
		logx.WriteJSON(w, http.StatusNotFound, model.Response{Code: 404, Message: "channel not found"})
		return
	}

	result := authlive.OnPlayAuth(h.service.CurrentAuthConfig(), channelKey, r.URL.Query())
	statusCode := http.StatusOK
	if !result.Allowed {
		statusCode = http.StatusForbidden
	}
	logx.WriteJSON(w, statusCode, model.Response{Code: 0, Message: "ok", Data: map[string]any{
		"allowed":     result.Allowed,
		"reason":      result.Reason,
		"channel_key": result.ChannelKey,
		"expire_at":   result.ExpireAt,
	}})
}

// PublishConnected 处理直播推流连接建立回调。
func (h *LiveHandler) PublishConnected(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "live manager not ready"})
		return
	}
	var req struct {
		ChannelKey     string `json:"channel_key"`
		NodeID         uint64 `json:"node_id"`
		WorkerID       string `json:"worker_id"`
		StreamKey      string `json:"stream_key"`
		PublishIP      string `json:"publish_ip"`
		PushProtocol   string `json:"push_protocol"`
		IngestURL      string `json:"ingest_url"`
		PlaybackHLSURL string `json:"playback_hls_url"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "invalid request"})
		return
	}
	req.ChannelKey = strings.TrimSpace(req.ChannelKey)
	if req.ChannelKey == "" {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "channel_key is required"})
		return
	}
	session, err := h.manager.OnPublish(r.Context(), req.ChannelKey, req.NodeID, req.WorkerID, req.StreamKey, req.PublishIP, req.PushProtocol, req.IngestURL, req.PlaybackHLSURL)
	if err != nil {
		logx.Error("http.live.publish_connected.failed", err, logx.Fields{"channel_key": req.ChannelKey})
		logx.WriteJSON(w, http.StatusConflict, model.Response{Code: 409, Message: err.Error()})
		return
	}
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: session})
}

// PublishDisconnected 处理直播推流断开回调。
func (h *LiveHandler) PublishDisconnected(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "live manager not ready"})
		return
	}
	var req struct {
		ChannelKey string `json:"channel_key"`
		StreamKey  string `json:"stream_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "invalid request"})
		return
	}
	req.ChannelKey = strings.TrimSpace(req.ChannelKey)
	if req.ChannelKey == "" {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "channel_key is required"})
		return
	}
	if err := h.manager.OnUnpublish(r.Context(), req.ChannelKey, strings.TrimSpace(req.StreamKey)); err != nil {
		logx.Error("http.live.publish_disconnected.failed", err, logx.Fields{"channel_key": req.ChannelKey})
		logx.WriteJSON(w, http.StatusConflict, model.Response{Code: 409, Message: err.Error()})
		return
	}
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok"})
}

// PublishInterrupted 处理直播推流中断回调。
func (h *LiveHandler) PublishInterrupted(w http.ResponseWriter, r *http.Request) {
	if h.manager == nil {
		logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "live manager not ready"})
		return
	}
	var req struct {
		ChannelKey string `json:"channel_key"`
		StreamKey  string `json:"stream_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "invalid request"})
		return
	}
	req.ChannelKey = strings.TrimSpace(req.ChannelKey)
	if req.ChannelKey == "" {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "channel_key is required"})
		return
	}
	h.manager.OnInterrupt(r.Context(), req.ChannelKey, strings.TrimSpace(req.StreamKey))
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok"})
}
