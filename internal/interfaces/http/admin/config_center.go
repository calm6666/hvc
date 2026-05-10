package admin

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"hvc/internal/infra/db/mysql"
	"hvc/internal/model"
	"hvc/pkg/idgen"
	"hvc/pkg/logx"
)

const configCenterBindingScopeBootstrap = "bootstrap"

// ConfigCenterHandler 处理后台 bootstrap 配置源绑定接口。
type ConfigCenterHandler struct {
	repository *mysql.ConfigCenterBindingRepository
}

// NewConfigCenterHandler 创建 bootstrap 配置源绑定处理器。
func NewConfigCenterHandler(repository *mysql.ConfigCenterBindingRepository) *ConfigCenterHandler {
	return &ConfigCenterHandler{repository: repository}
}

// List 返回 bootstrap 配置源绑定分页列表。
func (h *ConfigCenterHandler) List(w http.ResponseWriter, r *http.Request) {
	page, pageSize := parsePageParams(r)
	items, total, err := h.repository.ListPage(r.Context(), page, pageSize, r.URL.Query().Get("provider_type"), parseOptionalBool(r.URL.Query().Get("enabled")))
	if err != nil {
		logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "query config center bindings failed"})
		return
	}
	views := make([]configCenterBindingView, 0, len(items))
	for _, item := range items {
		views = append(views, buildConfigCenterBindingView(sanitizeConfigCenterBindingRecord(item)))
	}
	writePageResponseWithMeta(w, page, pageSize, total, views, map[string]any{
		"config_scope":         configCenterBindingScopeBootstrap,
		"affects_runtime":      false,
		"runtime_update_path":  "/v1/admin/config/runtime/update",
		"runtime_publish_path": "/v1/admin/config/publish",
	})
}

// Upsert 保存 bootstrap 配置源绑定。
func (h *ConfigCenterHandler) Upsert(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BindingID    uint64 `json:"binding_id"`
		BindingName  string `json:"binding_name"`
		ProviderType string `json:"provider_type"`
		Endpoint     string `json:"endpoint"`
		Namespace    string `json:"namespace"`
		AuthMode     string `json:"auth_mode"`
		AccessKey    string `json:"access_key"`
		SecretKey    string `json:"secret_key"`
		Token        string `json:"token"`
		Enabled      bool   `json:"enabled"`
		Priority     int    `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "invalid request"})
		return
	}
	if strings.TrimSpace(req.BindingName) == "" {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "binding_name is required"})
		return
	}
	if strings.TrimSpace(req.ProviderType) == "" {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "provider_type is required"})
		return
	}
	now := time.Now()
	id := req.BindingID
	if id == 0 {
		id = idgen.Next()
	}
	record := mysql.ConfigCenterBindingRecord{
		BindingID:    id,
		BindingName:  req.BindingName,
		ProviderType: req.ProviderType,
		Endpoint:     req.Endpoint,
		Namespace:    req.Namespace,
		AuthMode:     req.AuthMode,
		AccessKey:    req.AccessKey,
		SecretKey:    req.SecretKey,
		Token:        req.Token,
		Enabled:      req.Enabled,
		Priority:     req.Priority,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := h.repository.Save(r.Context(), record); err != nil {
		logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "save config center binding failed"})
		return
	}
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{
		"binding_id":      id,
		"config_scope":    configCenterBindingScopeBootstrap,
		"affects_runtime": false,
	}})
}

// SetEnabled 切换 bootstrap 配置源绑定启用状态。
func (h *ConfigCenterHandler) SetEnabled(w http.ResponseWriter, r *http.Request) {
	var req struct {
		BindingID uint64 `json:"binding_id"`
		Enabled   bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "invalid request"})
		return
	}
	if err := h.repository.SetEnabled(r.Context(), req.BindingID, req.Enabled); err != nil {
		logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "update config center binding failed"})
		return
	}
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{
		"binding_id":      req.BindingID,
		"enabled":         req.Enabled,
		"config_scope":    configCenterBindingScopeBootstrap,
		"affects_runtime": false,
	}})
}
