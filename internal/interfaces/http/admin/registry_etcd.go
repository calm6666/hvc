package admin

import (
	"net/http"
	"strings"
	"time"

	"hvc/internal/infra/db/mysql"
	"hvc/internal/model"
	"hvc/pkg/idgen"
	"hvc/pkg/logx"
)

// RegistryEtcdHandler 处理 etcd 注册配置接口。
type RegistryEtcdHandler struct {
	repository *mysql.RegistryEtcdConfigRepository
}

// NewRegistryEtcdHandler 创建 etcd 注册配置处理器。
func NewRegistryEtcdHandler(repository *mysql.RegistryEtcdConfigRepository) *RegistryEtcdHandler {
	return &RegistryEtcdHandler{repository: repository}
}

// List 返回 etcd 注册配置分页列表。
func (h *RegistryEtcdHandler) List(w http.ResponseWriter, r *http.Request) {
	page, pageSize := parsePageParams(r)
	items, total, err := h.repository.ListPage(r.Context(), page, pageSize, r.URL.Query().Get("registry_name"), parseOptionalBool(r.URL.Query().Get("enabled")))
	if err != nil {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "查询 etcd 注册配置列表失败"})
		return
	}
	views := make([]registryEtcdConfigView, 0, len(items))
	for _, item := range items {
		views = append(views, toRegistryEtcdConfigView(item))
	}
	writePageResponse(w, page, pageSize, total, views)
}

// Upsert 保存 etcd 注册配置。
func (h *RegistryEtcdHandler) Upsert(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RegistryID       uint64 `json:"registry_id"`
		RegistryName     string `json:"registry_name"`
		Endpoints        string `json:"endpoints"`
		ServiceNamespace string `json:"service_namespace"`
		LeaseTTLSec      int    `json:"lease_ttl_sec"`
		DialTimeoutMS    int    `json:"dial_timeout_ms"`
		Enabled          bool   `json:"enabled"`
		Priority         int    `json:"priority"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.RegistryName) == "" {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: "registry_name 不能为空"})
		return
	}
	if strings.TrimSpace(req.Endpoints) == "" {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: "endpoints 不能为空"})
		return
	}
	now := time.Now()
	id := req.RegistryID
	if id == 0 {
		id = idgen.Next()
	}
	record := mysql.RegistryEtcdConfigRecord{
		RegistryID:       id,
		RegistryName:     req.RegistryName,
		Endpoints:        req.Endpoints,
		ServiceNamespace: req.ServiceNamespace,
		LeaseTTLSec:      req.LeaseTTLSec,
		DialTimeoutMS:    req.DialTimeoutMS,
		Enabled:          req.Enabled,
		Priority:         req.Priority,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := h.repository.Save(r.Context(), record); err != nil {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "保存 etcd 注册配置失败"})
		return
	}
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{"registry_id": id}})
}

// SetEnabled 切换 etcd 注册配置启用状态。
func (h *RegistryEtcdHandler) SetEnabled(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RegistryID uint64 `json:"registry_id"`
		Enabled    bool   `json:"enabled"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if err := h.repository.SetEnabled(r.Context(), req.RegistryID, req.Enabled); err != nil {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "更新 etcd 注册配置状态失败"})
		return
	}
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{"registry_id": req.RegistryID, "enabled": req.Enabled}})
}
