package admin

import (
	"encoding/json"
	"net/http"

	"hvc/internal/infra/db/mysql"
	"hvc/internal/model"
	"hvc/pkg/logx"
)

// WriteRBAC 处理后台 RBAC 写接口。
type WriteRBAC struct {
	rbacRepository  *mysql.AdminRBACRepository
	adminRepository *mysql.AdminRepository
}

// NewWriteRBAC 创建 RBAC 写处理器。
func NewWriteRBAC(rbacRepository *mysql.AdminRBACRepository, adminRepository *mysql.AdminRepository) *WriteRBAC {
	return &WriteRBAC{rbacRepository: rbacRepository, adminRepository: adminRepository}
}

// UpsertUser 创建或更新管理员用户。
func (h *WriteRBAC) UpsertUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username     string `json:"username"`
		PasswordHash string `json:"password_hash"`
		DisplayName  string `json:"display_name"`
		Status       int    `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "invalid request"})
		return
	}
	record, err := h.adminRepository.SaveUser(r.Context(), req.Username, req.PasswordHash, req.DisplayName, req.Status)
	if err != nil {
		logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "save user failed"})
		return
	}
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: record})
}

// SetUserStatus 启用或禁用管理员用户。
func (h *WriteRBAC) SetUserStatus(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AdminUserID uint64 `json:"admin_user_id"`
		Status      int    `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "invalid request"})
		return
	}
	if err := h.adminRepository.SetUserStatus(r.Context(), req.AdminUserID, req.Status); err != nil {
		logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "update user status failed"})
		return
	}
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok"})
}

// UpsertRole 创建或更新角色。
func (h *WriteRBAC) UpsertRole(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RoleKey  string `json:"role_key"`
		RoleName string `json:"role_name"`
		RoleDesc string `json:"role_desc"`
		Status   int    `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "invalid request"})
		return
	}
	record, err := h.rbacRepository.EnsureRole(r.Context(), req.RoleKey, req.RoleName, req.RoleDesc, req.Status)
	if err != nil {
		logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "save role failed"})
		return
	}
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: record})
}

// UpsertPermission 创建或更新权限点。
func (h *WriteRBAC) UpsertPermission(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PermKey  string `json:"perm_key"`
		PermName string `json:"perm_name"`
		PermDesc string `json:"perm_desc"`
		Module   string `json:"module"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "invalid request"})
		return
	}
	record, err := h.rbacRepository.EnsurePermission(r.Context(), req.PermKey, req.PermName, req.PermDesc, req.Module)
	if err != nil {
		logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "save permission failed"})
		return
	}
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: record})
}

// BindUserRole 绑定用户与角色。
func (h *WriteRBAC) BindUserRole(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID uint64 `json:"user_id"`
		RoleID uint64 `json:"role_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "invalid request"})
		return
	}
	if err := h.rbacRepository.BindUserRole(r.Context(), req.UserID, req.RoleID); err != nil {
		logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "bind user role failed"})
		return
	}
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok"})
}

// BindRolePermission 绑定角色与权限点。
func (h *WriteRBAC) BindRolePermission(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RoleID uint64 `json:"role_id"`
		PermID uint64 `json:"perm_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logx.WriteJSON(w, http.StatusBadRequest, model.Response{Code: 400, Message: "invalid request"})
		return
	}
	if err := h.rbacRepository.BindRolePermission(r.Context(), req.RoleID, req.PermID); err != nil {
		logx.WriteJSON(w, http.StatusInternalServerError, model.Response{Code: 500, Message: "bind role permission failed"})
		return
	}
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok"})
}
