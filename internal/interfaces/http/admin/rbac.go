package admin

import (
	"net/http"

	"hvc/internal/audit"
	"hvc/internal/infra/db/mysql"
	"hvc/internal/model"
	"hvc/pkg/logx"
)

// RBACHandler 处理后台用户、角色、权限和审计接口。
type RBACHandler struct {
	adminRepository *mysql.AdminRepository
	rbacRepository  *mysql.AdminRBACRepository
	auditRepository *audit.Repository
}

// NewRBACHandler 创建后台 RBAC 处理器。
func NewRBACHandler(adminRepository *mysql.AdminRepository, rbacRepository *mysql.AdminRBACRepository, auditRepository *audit.Repository) *RBACHandler {
	return &RBACHandler{adminRepository: adminRepository, rbacRepository: rbacRepository, auditRepository: auditRepository}
}

// ListUsers 返回管理员用户列表。
func (h *RBACHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	items := h.adminRepository.ListUsers(r.Context())
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{"items": items}})
}

// ListRoles 返回角色列表。
func (h *RBACHandler) ListRoles(w http.ResponseWriter, r *http.Request) {
	items := h.rbacRepository.ListRoles(r.Context())
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{"items": items}})
}

// ListPermissions 返回权限点列表。
func (h *RBACHandler) ListPermissions(w http.ResponseWriter, r *http.Request) {
	items := h.rbacRepository.ListPermissions(r.Context())
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{"items": items}})
}

// ListAuditLogs 返回审计日志列表。
func (h *RBACHandler) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	items := h.auditRepository.List(r.Context())
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{"items": items}})
}
