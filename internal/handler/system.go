package handler

import (
	"net/http"

	"hvc/internal/model"
	"hvc/internal/service"
	"hvc/pkg/logx"
)

// SystemHandler 处理系统相关请求。
type SystemHandler struct {
	systemService *service.SystemService
}

// NewSystemHandler 创建系统处理器。
func NewSystemHandler(systemService *service.SystemService) *SystemHandler {
	return &SystemHandler{systemService: systemService}
}

// Health 处理健康检查请求。
func (h *SystemHandler) Health(w http.ResponseWriter, r *http.Request) {
	logx.Info("http.health", logx.Fields{
		"method": r.Method,
		"path":   r.URL.Path,
	})
	logx.WriteJSON(w, http.StatusOK, model.Response{
		Code:    0,
		Message: "ok",
		Data:    h.systemService.Health(),
	})
}
