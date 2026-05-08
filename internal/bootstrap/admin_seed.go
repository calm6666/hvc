package bootstrap

import (
	"context"

	"hvc/internal/infra/db/mysql"
)

// ensureAdminRBACSeed 初始化后台最小可用 RBAC 数据。
//
// 目标是保证：
// 1. 默认 admin 用户具备 super_admin 角色；
// 2. super_admin 拥有当前后台接口所需的全部基础权限；
// 3. operator / viewer 角色也能覆盖常见只读与运维场景。
func ensureAdminRBACSeed(ctx context.Context, adminRepo *mysql.AdminRepository, rbacRepo *mysql.AdminRBACRepository) {
	if adminRepo == nil || rbacRepo == nil {
		return
	}
	adminUser, ok := adminRepo.FindUserByUsername(ctx, "admin")
	if !ok {
		return
	}
	role, err := rbacRepo.EnsureRole(ctx, "super_admin", "超级管理员", "拥有后台全部基础权限", 1)
	if err != nil {
		return
	}
	_ = rbacRepo.BindUserRole(ctx, adminUser.AdminUserID, role.RoleID)

	operatorRole, _ := rbacRepo.EnsureRole(ctx, "operator", "运维人员", "可查看集群和任务状态，不可修改配置", 1)
	viewerRole, _ := rbacRepo.EnsureRole(ctx, "viewer", "只读用户", "仅可查看基础信息", 1)

	permissionSeeds := []struct {
		key    string
		name   string
		module string
	}{
		{"auth.session.read", "查看会话", "auth"},
		{"auth.session.write", "管理会话", "auth"},
		{"system.user.read", "查看用户", "system"},
		{"system.user.create", "创建用户", "system"},
		{"system.user.update", "更新用户", "system"},
		{"system.user.delete", "删除用户", "system"},
		{"system.user.role_bind", "绑定用户角色", "system"},
		{"system.user.lock", "锁定/解锁用户", "system"},
		{"system.role.read", "查看角色", "system"},
		{"system.role.update", "更新角色", "system"},
		{"system.permission.read", "查看权限", "system"},
		{"system.role.permission_bind", "绑定角色权限", "system"},
		{"config.version.read", "查看配置版本", "config"},
		{"config.version.publish", "发布配置版本", "config"},
		{"config.runtime.update", "更新运行配置", "config"},
		{"config.callback.read", "查看回调配置", "config"},
		{"config.callback.update", "更新回调配置", "config"},
		{"audit.read", "查看审计日志", "audit"},
		{"cluster.node.read", "查看集群节点", "cluster"},
		{"cluster.node.metrics.read", "查看节点指标", "cluster"},
		{"cluster.node.enable", "启用/禁用节点", "cluster"},
		{"cluster.node.quarantine", "隔离/恢复节点", "cluster"},
		{"cluster.read", "查看集群信息", "cluster"},
		{"transcode.job.read", "查看转码任务", "transcode"},
		{"transcode.job.detail.read", "查看任务详情", "transcode"},
		{"transcode.job.retry", "重试任务", "transcode"},
		{"transcode.job.cancel", "取消任务", "transcode"},
		{"transcode.job.create", "创建转码任务", "transcode"},
		{"live.channel.read", "查看直播频道", "live"},
		{"live.session.read", "查看直播会话", "live"},
		{"live.channel.create", "创建直播频道", "live"},
		{"live.channel.update", "更新直播频道", "live"},
		{"live.channel.start", "启动直播频道", "live"},
		{"live.channel.stop", "停止直播频道", "live"},
		{"live.channel.delete", "删除直播频道", "live"},
	}

	for _, item := range permissionSeeds {
		perm, err := rbacRepo.EnsurePermission(ctx, item.key, item.name, item.name, item.module)
		if err != nil {
			continue
		}
		_ = rbacRepo.BindRolePermission(ctx, role.RoleID, perm.PermID)
	}

	operatorPermissions := []string{
		"cluster.node.read", "cluster.node.metrics.read", "cluster.read",
		"transcode.job.read", "transcode.job.detail.read", "transcode.job.retry",
		"live.channel.read", "live.session.read",
		"audit.read",
		"config.version.read",
	}
	for _, permKey := range operatorPermissions {
		perms := rbacRepo.ListPermissions(ctx)
		for _, p := range perms {
			if p.PermKey == permKey {
				_ = rbacRepo.BindRolePermission(ctx, operatorRole.RoleID, p.PermID)
				break
			}
		}
	}

	viewerPermissions := []string{
		"cluster.node.read", "cluster.read",
		"transcode.job.read",
		"live.channel.read", "live.session.read",
		"config.version.read",
	}
	for _, permKey := range viewerPermissions {
		perms := rbacRepo.ListPermissions(ctx)
		for _, p := range perms {
			if p.PermKey == permKey {
				_ = rbacRepo.BindRolePermission(ctx, viewerRole.RoleID, p.PermID)
				break
			}
		}
	}
}
