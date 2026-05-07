package admin

import (
	"encoding/json"
	"net/http"
	"time"

	"hvc/internal/infra/db/mysql"
	"hvc/internal/model"
	"hvc/pkg/idgen"
	"hvc/pkg/logx"
)

// CallbackHandler 处理后台回调配置接口。
type CallbackHandler struct {
	repository *mysql.CallbackConfigRepository
}

// NewCallbackHandler 创建回调配置处理器。
func NewCallbackHandler(repository *mysql.CallbackConfigRepository) *CallbackHandler {
	return &CallbackHandler{repository: repository}
}

// List 返回全部回调配置。
func (h *CallbackHandler) List(w http.ResponseWriter, r *http.Request) {
	items := h.repository.ListAll(r.Context())
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{"items": items}})
}

// Upsert 保存回调配置。
func (h *CallbackHandler) Upsert(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CallbackConfigID uint64 `json:"callback_config_id"`
		CallbackName     string `json:"callback_name"`
		CallbackType     int    `json:"callback_type"`
		TargetURL        string `json:"target_url"`
		RPCEndpoint      string `json:"rpc_endpoint"`
		RPCServiceName   string `json:"rpc_service_name"`
		MQExchange       string `json:"mq_exchange"`
		MQRoutingKey     string `json:"mq_routing_key"`
		TimeoutMS        int    `json:"timeout_ms"`
		RetryTimes       int    `json:"retry_times"`
		Enabled          bool   `json:"enabled"`
		Priority         int    `json:"priority"`
		RegistryID       uint64 `json:"registry_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "invalid request"})
		return
	}
	now := time.Now()
	id := req.CallbackConfigID
	if id == 0 {
		id = idgen.Next()
	}
	record := mysql.CallbackConfigRecord{
		CallbackConfigID: id,
		CallbackName:     req.CallbackName,
		CallbackType:     req.CallbackType,
		TargetURL:        req.TargetURL,
		RPCEndpoint:      req.RPCEndpoint,
		RPCServiceName:   req.RPCServiceName,
		MQExchange:       req.MQExchange,
		MQRoutingKey:     req.MQRoutingKey,
		TimeoutMS:        req.TimeoutMS,
		RetryTimes:       req.RetryTimes,
		Enabled:          req.Enabled,
		Priority:         req.Priority,
		RegistryID:       req.RegistryID,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := h.repository.Save(r.Context(), record); err != nil {
		logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "save callback config failed"})
		return
	}
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{"callback_config_id": id}})
}

// SetEnabled 切换回调配置启用状态。
func (h *CallbackHandler) SetEnabled(w http.ResponseWriter, r *http.Request) {
	var req struct {
		CallbackConfigID uint64 `json:"callback_config_id"`
		Enabled          bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "invalid request"})
		return
	}
	if err := h.repository.SetEnabled(r.Context(), req.CallbackConfigID, req.Enabled); err != nil {
		logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "update callback config failed"})
		return
	}
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{"callback_config_id": req.CallbackConfigID, "enabled": req.Enabled}})
}
