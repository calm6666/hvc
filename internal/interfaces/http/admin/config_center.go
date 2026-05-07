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

// ConfigCenterHandler 处理后台配置中心绑定接口。
type ConfigCenterHandler struct {
	repository *mysql.ConfigCenterBindingRepository
	effective  *configcenter.EffectiveConfig
}

// NewConfigCenterHandler 创建配置中心绑定处理器。
func NewConfigCenterHandler(repository *mysql.ConfigCenterBindingRepository, effective *configcenter.EffectiveConfig) *ConfigCenterHandler {
	return &ConfigCenterHandler{repository: repository, effective: effective}
}

// List 返回全部配置中心绑定。
func (h *ConfigCenterHandler) List(w http.ResponseWriter, r *http.Request) {
	items := h.repository.ListAll(r.Context())
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{"items": items}})
}

// Upsert 保存配置中心绑定。
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
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{"binding_id": id}})
}

// SetEnabled 切换配置中心绑定启用状态。
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
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{"binding_id": req.BindingID, "enabled": req.Enabled}})
}
