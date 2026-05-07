package admin

import (
	"encoding/json"
	"net/http"
	"time"

	"hvc/internal/configcenter"
	"hvc/internal/infra/db/mysql"
	"hvc/internal/model"
	"hvc/pkg/idgen"
	"hvc/pkg/logx"
)

// ConfigHandler 处理后台动态配置接口。
type ConfigHandler struct {
	runtimeConfigRepository *mysql.RuntimeConfigRepository
	effectiveConfig         *configcenter.EffectiveConfig
}

// NewConfigHandler 创建后台动态配置处理器。
func NewConfigHandler(runtimeConfigRepository *mysql.RuntimeConfigRepository, effectiveConfig *configcenter.EffectiveConfig) *ConfigHandler {
	return &ConfigHandler{runtimeConfigRepository: runtimeConfigRepository, effectiveConfig: effectiveConfig}
}

// ListRuntimeVersions 返回运行配置版本列表。
func (h *ConfigHandler) ListRuntimeVersions(w http.ResponseWriter, r *http.Request) {
	items := h.runtimeConfigRepository.ListVersions(r.Context())
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{"items": items}})
}

// UpdateRuntime 创建新的运行配置版本。
func (h *ConfigHandler) UpdateRuntime(w http.ResponseWriter, r *http.Request) {
	base, ok := h.runtimeConfigRepository.LatestPublished(r.Context())
	if !ok {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "no base runtime config"})
		return
	}
	var req struct {
		DefaultProfileID                uint64 `json:"default_profile_id"`
		MaxGlobalTranscodeSessions      int    `json:"max_global_transcode_sessions"`
		JobLeaseTTLSec                  int    `json:"job_lease_ttl_sec"`
		WorkerHeartbeatTimeoutSec       int    `json:"worker_heartbeat_timeout_sec"`
		AllowRequestOverrideProfile     bool   `json:"allow_request_override_profile"`
		AllowRequestOverrideSegmentDur  bool   `json:"allow_request_override_segment_duration"`
		AllowRequestOverrideHWAccel     bool   `json:"allow_request_override_hwaccel"`
		ChangeSummary                   string `json:"change_summary"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "invalid request"})
		return
	}
	now := time.Now()
	record := base
	record.ConfigVersion = idgen.Next()
	record.DefaultProfileID = req.DefaultProfileID
	record.MaxGlobalTranscodeSessions = req.MaxGlobalTranscodeSessions
	record.JobLeaseTTLSeconds = req.JobLeaseTTLSec
	record.WorkerHeartbeatTimeoutSec = req.WorkerHeartbeatTimeoutSec
	record.AllowRequestOverrideProfile = req.AllowRequestOverrideProfile
	record.AllowRequestOverrideSegmentDur = req.AllowRequestOverrideSegmentDur
	record.AllowRequestOverrideHWAccel = req.AllowRequestOverrideHWAccel
	record.ChangeSummary = req.ChangeSummary
	record.Published = false
	record.CreatedAt = now
	record.UpdatedAt = now
	if err := h.runtimeConfigRepository.Save(r.Context(), record); err != nil {
		logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "save runtime config failed"})
		return
	}
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{"pending_config_version": record.ConfigVersion, "published": false}})
}

// PublishRuntime 发布指定运行配置版本并替换当前生效配置。
func (h *ConfigHandler) PublishRuntime(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ConfigVersion uint64 `json:"config_version"`
		PublishReason string `json:"publish_reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "invalid request"})
		return
	}
	record, ok := h.runtimeConfigRepository.FindByVersion(r.Context(), req.ConfigVersion)
	if !ok {
		logx.WriteJSON(w, http.StatusNotFound, model.Response{Code: 404, Message: "config version not found"})
		return
	}
	if err := h.runtimeConfigRepository.MarkPublished(r.Context(), req.ConfigVersion, "admin"); err != nil {
		logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "publish config failed"})
		return
	}
	h.effectiveConfig.Replace(mysql.ToDynamicRuntimeConfig(record))
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{
		"config_version": req.ConfigVersion,
		"published":      true,
		"effective_scope": map[string]any{
			"new_jobs":    true,
			"queued_jobs": true,
			"running_jobs": false,
		},
	}})
}
