package admin

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	internalauth "hvc/internal/auth"

	"hvc/internal/audit"
	"hvc/internal/infra/db/mysql"
	"hvc/internal/model"
	"hvc/internal/opslog"
	"hvc/pkg/logx"
)

// RBACHandler 处理后台用户、角色、权限和审计接口。
type RBACHandler struct {
	adminRepository  *mysql.AdminRepository
	rbacRepository   *mysql.AdminRBACRepository
	auditRepository  *audit.Repository
	opsLogRepository *opslog.Repository
}

// NewRBACHandler 创建后台 RBAC 处理器。
func NewRBACHandler(adminRepository *mysql.AdminRepository, rbacRepository *mysql.AdminRBACRepository, auditRepository *audit.Repository, opsLogRepository *opslog.Repository) *RBACHandler {
	return &RBACHandler{adminRepository: adminRepository, rbacRepository: rbacRepository, auditRepository: auditRepository, opsLogRepository: opsLogRepository}
}

// ListUsers 返回管理员用户分页列表，并屏蔽敏感字段。
func (h *RBACHandler) ListUsers(w http.ResponseWriter, r *http.Request) {
	page, pageSize := parsePageParams(r)
	status, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("status")))
	items, total, err := h.adminRepository.ListUsersPage(r.Context(), page, pageSize, r.URL.Query().Get("username"), status)
	if err != nil {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "查询管理员列表失败"})
		return
	}
	views := make([]adminUserView, 0, len(items))
	for _, item := range items {
		views = append(views, toAdminUserView(item))
	}
	writePageResponse(w, page, pageSize, total, views)
}

// ListRoles 返回角色分页列表。
func (h *RBACHandler) ListRoles(w http.ResponseWriter, r *http.Request) {
	page, pageSize := parsePageParams(r)
	status, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("status")))
	items, total, err := h.rbacRepository.ListRolesPage(r.Context(), page, pageSize, r.URL.Query().Get("role_key"), status)
	if err != nil {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "查询角色列表失败"})
		return
	}
	views := make([]adminRoleView, 0, len(items))
	for _, item := range items {
		views = append(views, toAdminRoleView(item))
	}
	writePageResponse(w, page, pageSize, total, views)
}

// ListAllRoles 返回全部角色列表，不分页，供角色选择器使用。
func (h *RBACHandler) ListAllRoles(w http.ResponseWriter, r *http.Request) {
	items := h.rbacRepository.ListRolesAll(r.Context())
	views := make([]adminRoleView, 0, len(items))
	for _, item := range items {
		views = append(views, toAdminRoleView(item))
	}
	writeItemsResponse(w, views, nil)
}

// ListPermissions 返回权限树。
func (h *RBACHandler) ListPermissions(w http.ResponseWriter, r *http.Request) {
	items := h.rbacRepository.ListPermissions(r.Context())
	writeItemsResponse(w, buildPermissionTree(items), map[string]any{"tree": true})
}

// PermissionTree 返回权限树。
func (h *RBACHandler) PermissionTree(w http.ResponseWriter, r *http.Request) {
	h.ListPermissions(w, r)
}

// ListMenuTree 返回全部菜单树。
func (h *RBACHandler) ListMenuTree(w http.ResponseWriter, r *http.Request) {
	items := h.rbacRepository.ListMenus(r.Context(), false)
	writeItemsResponse(w, buildMenuTree(items), map[string]any{"tree": true})
}

// CurrentMenuTree 返回当前管理员可见菜单树。
func (h *RBACHandler) CurrentMenuTree(w http.ResponseWriter, r *http.Request) {
	userID := internalauth.CurrentAdminUserID(r.Context(), r)
	if userID == 0 {
		logx.WriteJSON(w, http.StatusUnauthorized, model.Response{Code: 401, Message: "未登录或登录已失效"})
		return
	}
	items := h.adminRepository.ListMenusByUserID(r.Context(), userID)
	writeItemsResponse(w, buildMenuTree(items), map[string]any{"tree": true})
}

// RoleMenuTree 返回角色已绑定菜单 ID 和完整菜单树。
func (h *RBACHandler) RoleMenuTree(w http.ResponseWriter, r *http.Request) {
	roleID, _ := strconv.ParseUint(strings.TrimSpace(r.URL.Query().Get("role_id")), 10, 64)
	if roleID == 0 {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 400, Message: "role_id 不能为空"})
		return
	}
	items := h.rbacRepository.ListMenus(r.Context(), false)
	menuIDs := h.rbacRepository.ListMenuIDsByRoleID(r.Context(), roleID)
	writeItemsResponse(w, buildMenuTree(items), map[string]any{
		"tree":     true,
		"role_id":  roleID,
		"menu_ids": menuIDs,
	})
}

// ListAuditLogs 返回审计日志分页列表。
func (h *RBACHandler) ListAuditLogs(w http.ResponseWriter, r *http.Request) {
	page, pageSize := parsePageParams(r)
	userID, _ := strconv.ParseUint(strings.TrimSpace(r.URL.Query().Get("admin_user_id")), 10, 64)
	actionName := strings.TrimSpace(r.URL.Query().Get("action_name"))
	startTime := parseRFC3339Time(strings.TrimSpace(r.URL.Query().Get("start_time")))
	endTime := parseRFC3339Time(strings.TrimSpace(r.URL.Query().Get("end_time")))
	items, total := h.auditRepository.ListPaged(r.Context(), page, pageSize, actionName, userID, startTime, endTime)
	views := make([]auditLogView, 0, len(items))
	for _, item := range items {
		views = append(views, toAuditLogView(item))
	}
	writePageResponse(w, page, pageSize, total, views)
}

// ListRuntimeLogs 返回运行日志分页列表。
func (h *RBACHandler) ListRuntimeLogs(w http.ResponseWriter, r *http.Request) {
	if h.opsLogRepository == nil {
		logx.WriteJSON(w, http.StatusOK, model.Response{Code: 500, Message: "运行日志仓储未初始化"})
		return
	}
	page, pageSize := parsePageParams(r)
	level := strings.TrimSpace(r.URL.Query().Get("level"))
	actionName := strings.TrimSpace(r.URL.Query().Get("action_name"))
	startTime := parseRFC3339Time(strings.TrimSpace(r.URL.Query().Get("start_time")))
	endTime := parseRFC3339Time(strings.TrimSpace(r.URL.Query().Get("end_time")))
	items, total := h.opsLogRepository.ListPaged(r.Context(), page, pageSize, level, actionName, startTime, endTime)
	views := make([]runtimeLogView, 0, len(items))
	for _, item := range items {
		views = append(views, toRuntimeLogView(item))
	}
	writePageResponse(w, page, pageSize, total, views)
}

func parseRFC3339Time(value string) *time.Time {
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil
	}
	return &parsed
}
