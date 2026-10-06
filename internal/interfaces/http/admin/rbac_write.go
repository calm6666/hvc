package admin

import (
	"fmt"
	"net/http"
	"strings"

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
		Password     string `json:"password"`
		PasswordHash string `json:"password_hash"`
		DisplayName  string `json:"display_name"`
		Status       int    `json:"status"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: "username 不能为空"})
		return
	}
	passwordHash, passwordSalt, err := resolveAdminPasswordCredential(req.Password, req.PasswordHash)
	if err != nil {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: err.Error()})
		return
	}
	if passwordHash == "" {
		if _, exists := h.adminRepository.FindUserByUsername(r.Context(), req.Username); !exists {
			logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: "创建管理员时 password 不能为空"})
			return
		}
	}
	record, err := h.adminRepository.SaveUser(r.Context(), req.Username, passwordHash, passwordSalt, req.DisplayName, req.Status)
	if err != nil {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "保存管理员失败"})
		return
	}
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: toAdminUserView(record)})
}

// SetUserStatus 启用或禁用管理员用户。
func (h *WriteRBAC) SetUserStatus(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AdminUserID uint64 `json:"admin_user_id"`
		Status      int    `json:"status"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if err := h.adminRepository.SetUserStatus(r.Context(), req.AdminUserID, req.Status); err != nil {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "更新管理员状态失败"})
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
	if !decodeJSONBody(w, r, &req) {
		return
	}
	req.RoleKey = strings.TrimSpace(req.RoleKey)
	req.RoleName = strings.TrimSpace(req.RoleName)
	if req.RoleKey == "" {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: "role_key 不能为空"})
		return
	}
	if req.RoleName == "" {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: "role_name 不能为空"})
		return
	}
	record, err := h.rbacRepository.EnsureRole(r.Context(), req.RoleKey, req.RoleName, req.RoleDesc, req.Status)
	if err != nil {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "保存角色失败"})
		return
	}
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: toAdminRoleView(record)})
}

// UpsertPermission 创建或更新权限点。
func (h *WriteRBAC) UpsertPermission(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PermKey  string `json:"perm_key"`
		PermName string `json:"perm_name"`
		PermDesc string `json:"perm_desc"`
		Module   string `json:"module"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	req.PermKey = strings.TrimSpace(req.PermKey)
	req.PermName = strings.TrimSpace(req.PermName)
	req.Module = strings.TrimSpace(req.Module)
	if req.PermKey == "" {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: "perm_key 不能为空"})
		return
	}
	if req.PermName == "" {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: "perm_name 不能为空"})
		return
	}
	if req.Module == "" {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: "module 不能为空"})
		return
	}
	record, err := h.rbacRepository.EnsurePermission(r.Context(), req.PermKey, req.PermName, req.PermDesc, req.Module)
	if err != nil {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "保存权限失败"})
		return
	}
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: toAdminPermissionView(record)})
}

// BindUserRole 绑定用户与角色。
func (h *WriteRBAC) BindUserRole(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AdminUserID uint64 `json:"admin_user_id"`
		UserID      uint64 `json:"user_id"`
		RoleID      uint64 `json:"role_id"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	userID := req.AdminUserID
	if userID == 0 {
		userID = req.UserID
	}
	if userID == 0 {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: "admin_user_id 不能为空"})
		return
	}
	if req.RoleID == 0 {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: "role_id 不能为空"})
		return
	}
	if err := h.rbacRepository.BindUserRole(r.Context(), userID, req.RoleID); err != nil {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "绑定用户角色失败"})
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
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.RoleID == 0 {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: "role_id 不能为空"})
		return
	}
	if req.PermID == 0 {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: "perm_id 不能为空"})
		return
	}
	if err := h.rbacRepository.BindRolePermission(r.Context(), req.RoleID, req.PermID); err != nil {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "绑定角色权限失败"})
		return
	}
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok"})
}

// UpsertMenu 创建或更新菜单。
func (h *WriteRBAC) UpsertMenu(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MenuID        uint64 `json:"menu_id"`
		ParentID      uint64 `json:"parent_id"`
		MenuKey       string `json:"menu_key"`
		MenuName      string `json:"menu_name"`
		RoutePath     string `json:"route_path"`
		Component     string `json:"component"`
		ComponentName string `json:"component_name"`
		IconName      string `json:"icon_name"`
		MenuType      string `json:"menu_type"`
		PermissionKey string `json:"permission_key"`
		SortNo        int    `json:"sort_no"`
		Hidden        bool   `json:"hidden"`
		Status        int    `json:"status"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	req.MenuKey = strings.TrimSpace(req.MenuKey)
	req.MenuName = strings.TrimSpace(req.MenuName)
	req.MenuType = strings.TrimSpace(req.MenuType)
	if req.MenuKey == "" {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: "menu_key 不能为空"})
		return
	}
	if req.MenuName == "" {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: "menu_name 不能为空"})
		return
	}
	if req.MenuType == "" {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: "menu_type 不能为空"})
		return
	}
	component := strings.TrimSpace(req.Component)
	if component == "" {
		component = strings.TrimSpace(req.ComponentName)
	}
	record, err := h.rbacRepository.EnsureMenu(r.Context(), mysql.AdminMenuRecord{
		MenuID:        req.MenuID,
		ParentID:      req.ParentID,
		MenuKey:       req.MenuKey,
		MenuName:      req.MenuName,
		RoutePath:     req.RoutePath,
		Component:     component,
		IconName:      req.IconName,
		MenuType:      req.MenuType,
		PermissionKey: req.PermissionKey,
		SortNo:        req.SortNo,
		Hidden:        req.Hidden,
		Status:        req.Status,
	})
	if err != nil {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "保存菜单失败"})
		return
	}
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: toAdminMenuNode(record)})
}

// DeleteMenu 删除菜单。
func (h *WriteRBAC) DeleteMenu(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MenuID uint64 `json:"menu_id"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if err := h.rbacRepository.DeleteMenu(r.Context(), req.MenuID); err != nil {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 409, Message: "删除菜单失败，当前菜单下可能仍存在子菜单"})
		return
	}
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok"})
}

// AssignRoleMenus 覆盖角色菜单绑定。
func (h *WriteRBAC) AssignRoleMenus(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RoleID  uint64   `json:"role_id"`
		MenuIDs []uint64 `json:"menu_ids"`
	}
	if !decodeJSONBody(w, r, &req) {
		return
	}
	if req.RoleID == 0 {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: "role_id 不能为空"})
		return
	}
	if err := h.rbacRepository.ReplaceRoleMenus(r.Context(), req.RoleID, req.MenuIDs); err != nil {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "分配角色菜单失败"})
		return
	}
	logx.WriteJSON(w, http.StatusOK, model.Response{Code: 0, Message: "ok", Data: map[string]any{
		"role_id":  req.RoleID,
		"menu_ids": req.MenuIDs,
	}})
}

func resolveAdminPasswordCredential(password string, legacyPasswordHash string) (string, string, error) {
	password = strings.TrimSpace(password)
	legacyPasswordHash = strings.TrimSpace(legacyPasswordHash)
	if password != "" {
		salt := mysql.GenerateAdminPasswordSalt()
		if salt == "" {
			return "", "", fmt.Errorf("生成密码盐失败")
		}
		return mysql.HashAdminPasswordWithSalt(password, salt), salt, nil
	}
	if legacyPasswordHash != "" {
		// 兼容旧客户端短期内继续传 password_hash，避免升级窗口直接失败。
		return legacyPasswordHash, "", nil
	}
	return "", "", nil
}
