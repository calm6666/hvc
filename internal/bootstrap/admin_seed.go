package bootstrap

import (
	"context"

	"hvc/internal/infra/db/mysql"
)

// ensureAdminRBACSeed 初始化后台最小可用 RBAC 数据。
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
		{"system.log.read", "查看运行日志", "system"},
		{"system.role.permission_bind", "绑定角色权限", "system"},
		{"system.menu.read", "查看菜单", "system"},
		{"system.menu.update", "更新菜单", "system"},
		{"system.menu.delete", "删除菜单", "system"},
		{"system.role.menu_bind", "绑定角色菜单", "system"},
		{"config.version.read", "查看配置版本", "config"},
		{"config.version.publish", "发布配置版本", "config"},
		{"config.runtime.update", "更新运行配置", "config"},
		{"config.callback.read", "查看回调配置", "config"},
		{"config.callback.update", "更新回调配置", "config"},
		{"config.naming_template.read", "查看命名模板", "config"},
		{"config.naming_template.update", "更新命名模板", "config"},
		{"audit.read", "查看审计日志", "audit"},
		{"cluster.node.read", "查看集群节点", "cluster"},
		{"cluster.node.metrics.read", "查看节点指标", "cluster"},
		{"cluster.node.enable", "启用/禁用节点", "cluster"},
		{"cluster.node.quarantine", "隔离/恢复节点", "cluster"},
		{"cluster.node.drain", "排空/恢复节点", "cluster"},
		{"cluster.worker.offline", "下线 Worker", "cluster"},
		{"cluster.worker.exit", "标记 Worker 退出", "cluster"},
		{"cluster.job.takeover", "强制接管任务", "cluster"},
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
		"auth.session.read", "system.log.read",
		"cluster.node.read", "cluster.node.metrics.read", "cluster.read",
		"cluster.node.drain", "cluster.worker.offline", "cluster.worker.exit", "cluster.job.takeover",
		"transcode.job.read", "transcode.job.detail.read", "transcode.job.retry",
		"live.channel.read", "live.session.read",
		"audit.read", "config.version.read", "config.callback.read", "config.naming_template.read",
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
		"auth.session.read", "cluster.node.read", "cluster.read",
		"transcode.job.read", "live.channel.read", "live.session.read", "config.version.read",
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

	ensureAdminMenuSeed(ctx, rbacRepo, map[string]uint64{
		"super_admin": role.RoleID,
		"operator":    operatorRole.RoleID,
		"viewer":      viewerRole.RoleID,
	})
}

func ensureAdminMenuSeed(ctx context.Context, rbacRepo *mysql.AdminRBACRepository, roleIDs map[string]uint64) {
	if rbacRepo == nil {
		return
	}
	// 一级菜单：有子菜单的用 LAYOUT 映射到 layout/index.vue（侧边栏+Header+内容区）
	menuSeeds := []mysql.AdminMenuRecord{
		{ParentID: 0, MenuKey: "dashboard", MenuName: "仪表盘", RoutePath: "/dashboard", Component: "LAYOUT", IconName: "DashboardOutlined", MenuType: "menu", SortNo: 0, Status: 1},
		{ParentID: 0, MenuKey: "system", MenuName: "系统管理", RoutePath: "/system", Component: "LAYOUT", IconName: "SettingOutlined", MenuType: "menu", SortNo: 10, Status: 1},
		{ParentID: 0, MenuKey: "cluster", MenuName: "集群管理", RoutePath: "/cluster", Component: "LAYOUT", IconName: "CloudServerOutlined", MenuType: "menu", SortNo: 20, Status: 1},
		{ParentID: 0, MenuKey: "transcode", MenuName: "转码管理", RoutePath: "/transcode", Component: "LAYOUT", IconName: "VideoCameraOutlined", MenuType: "menu", SortNo: 30, Status: 1},
		{ParentID: 0, MenuKey: "live", MenuName: "直播管理", RoutePath: "/live", Component: "LAYOUT", IconName: "PlayCircleOutlined", MenuType: "menu", SortNo: 40, Status: 1},
		{ParentID: 0, MenuKey: "config", MenuName: "配置管理", RoutePath: "/config", Component: "LAYOUT", IconName: "ApiOutlined", MenuType: "menu", SortNo: 50, Status: 1},
	}

	menuIndex := make(map[string]mysql.AdminMenuRecord, 16)
	for _, seed := range menuSeeds {
		record, err := rbacRepo.EnsureMenu(ctx, seed)
		if err != nil {
			continue
		}
		menuIndex[seed.MenuKey] = record
	}

	children := []mysql.AdminMenuRecord{
		{ParentID: menuIndex["dashboard"].MenuID, MenuKey: "dashboard_console", MenuName: "主控台", RoutePath: "console", Component: "/dashboard/console/console", MenuType: "menu", PermissionKey: "cluster.read", SortNo: 10, Status: 1},
		{ParentID: menuIndex["system"].MenuID, MenuKey: "system_users", MenuName: "用户管理", RoutePath: "user", Component: "/system/user/user", MenuType: "menu", PermissionKey: "system.user.read", SortNo: 10, Status: 1},
		{ParentID: menuIndex["system"].MenuID, MenuKey: "system_roles", MenuName: "角色管理", RoutePath: "role", Component: "/system/role/role", MenuType: "menu", PermissionKey: "system.role.read", SortNo: 20, Status: 1},
		{ParentID: menuIndex["system"].MenuID, MenuKey: "system_menus", MenuName: "菜单管理", RoutePath: "menu", Component: "/system/menu/menu", MenuType: "menu", PermissionKey: "system.menu.read", SortNo: 30, Status: 1},
		{ParentID: menuIndex["system"].MenuID, MenuKey: "system_permissions", MenuName: "权限管理", RoutePath: "permission", Component: "/system/permission/permission", MenuType: "menu", PermissionKey: "system.permission.read", SortNo: 40, Status: 1},
		{ParentID: menuIndex["system"].MenuID, MenuKey: "system_audit", MenuName: "审计日志", RoutePath: "audit", Component: "/system/audit/audit", MenuType: "menu", PermissionKey: "audit.read", SortNo: 50, Status: 1},
		{ParentID: menuIndex["system"].MenuID, MenuKey: "system_log", MenuName: "运行日志", RoutePath: "log", Component: "/system/log/log", MenuType: "menu", PermissionKey: "system.log.read", SortNo: 60, Status: 1},
		{ParentID: menuIndex["config"].MenuID, MenuKey: "config_versions", MenuName: "版本管理", RoutePath: "versions", Component: "/config/versions", MenuType: "menu", PermissionKey: "config.version.read", SortNo: 10, Status: 1},
		{ParentID: menuIndex["config"].MenuID, MenuKey: "config_runtime", MenuName: "运行配置", RoutePath: "runtime", Component: "/config/runtime", MenuType: "menu", PermissionKey: "config.version.read", SortNo: 20, Status: 1},
		{ParentID: menuIndex["config"].MenuID, MenuKey: "config_callback", MenuName: "回调配置", RoutePath: "callback", Component: "/config/callback", MenuType: "menu", PermissionKey: "config.callback.read", SortNo: 30, Status: 1},
		{ParentID: menuIndex["config"].MenuID, MenuKey: "config_naming", MenuName: "命名模板", RoutePath: "naming-template", Component: "/config/namingTemplate", MenuType: "menu", PermissionKey: "config.naming_template.read", SortNo: 40, Status: 1},
		{ParentID: menuIndex["config"].MenuID, MenuKey: "config_center", MenuName: "配置中心", RoutePath: "config-center", Component: "/config/configCenter", MenuType: "menu", PermissionKey: "config.version.read", SortNo: 50, Status: 1},
		{ParentID: menuIndex["config"].MenuID, MenuKey: "registry_etcd", MenuName: "Etcd", RoutePath: "etcd", Component: "/config/registryEtcd", MenuType: "menu", PermissionKey: "config.version.read", SortNo: 60, Status: 1},
		{ParentID: menuIndex["cluster"].MenuID, MenuKey: "cluster_nodes", MenuName: "节点管理", RoutePath: "nodes", Component: "/cluster/nodes", MenuType: "menu", PermissionKey: "cluster.node.read", SortNo: 10, Status: 1},
		{ParentID: menuIndex["cluster"].MenuID, MenuKey: "cluster_workers", MenuName: "Worker", RoutePath: "workers", Component: "/cluster/workers", MenuType: "menu", PermissionKey: "cluster.read", SortNo: 20, Status: 1},
		{ParentID: menuIndex["cluster"].MenuID, MenuKey: "cluster_members", MenuName: "成员管理", RoutePath: "members", Component: "/cluster/members", MenuType: "menu", PermissionKey: "cluster.read", SortNo: 30, Status: 1},
		{ParentID: menuIndex["cluster"].MenuID, MenuKey: "cluster_scheduler", MenuName: "调度洞察", RoutePath: "scheduler", Component: "/cluster/scheduler", MenuType: "menu", PermissionKey: "cluster.read", SortNo: 40, Status: 1},
		{ParentID: menuIndex["cluster"].MenuID, MenuKey: "cluster_topology", MenuName: "集群拓扑", RoutePath: "topology", Component: "/cluster/topology", MenuType: "menu", PermissionKey: "cluster.read", SortNo: 50, Status: 1},
		{ParentID: menuIndex["transcode"].MenuID, MenuKey: "transcode_jobs", MenuName: "转码任务", RoutePath: "jobs", Component: "/transcode/jobs", MenuType: "menu", PermissionKey: "transcode.job.read", SortNo: 10, Status: 1},
		{ParentID: menuIndex["transcode"].MenuID, MenuKey: "transcode_monitor", MenuName: "实时监控", RoutePath: "monitor", Component: "/transcode/monitor", MenuType: "menu", PermissionKey: "transcode.job.read", SortNo: 20, Status: 1},
		{ParentID: menuIndex["live"].MenuID, MenuKey: "live_channels", MenuName: "频道管理", RoutePath: "channels", Component: "/live/channels", MenuType: "menu", PermissionKey: "live.channel.read", SortNo: 10, Status: 1},
		{ParentID: menuIndex["live"].MenuID, MenuKey: "live_sessions", MenuName: "直播会话", RoutePath: "sessions", Component: "/live/sessions", MenuType: "menu", PermissionKey: "live.session.read", SortNo: 20, Status: 1},
	}
	for _, seed := range children {
		record, err := rbacRepo.EnsureMenu(ctx, seed)
		if err != nil {
			continue
		}
		menuIndex[seed.MenuKey] = record
	}

	assignRoleMenus(ctx, rbacRepo, roleIDs["super_admin"], menuIndex,
		"dashboard",
		"system", "system_users", "system_roles", "system_menus", "system_permissions", "system_audit", "system_log",
		"config", "config_versions", "config_runtime", "config_callback", "config_naming", "config_center", "registry_etcd",
		"cluster", "cluster_nodes", "cluster_workers", "cluster_members", "cluster_scheduler", "cluster_topology",
		"transcode", "transcode_jobs", "transcode_monitor",
		"live", "live_channels", "live_sessions",
	)
	assignRoleMenus(ctx, rbacRepo, roleIDs["operator"], menuIndex,
		"dashboard",
		"system", "system_audit", "system_log",
		"config", "config_versions", "config_runtime", "config_callback", "config_naming",
		"cluster", "cluster_nodes", "cluster_workers", "cluster_members",
		"transcode", "transcode_jobs", "transcode_monitor",
		"live", "live_channels", "live_sessions",
	)
	assignRoleMenus(ctx, rbacRepo, roleIDs["viewer"], menuIndex,
		"dashboard",
		"cluster", "cluster_nodes",
		"transcode", "transcode_jobs",
		"live", "live_channels", "live_sessions",
	)
}

func assignRoleMenus(ctx context.Context, rbacRepo *mysql.AdminRBACRepository, roleID uint64, menuIndex map[string]mysql.AdminMenuRecord, menuKeys ...string) {
	if roleID == 0 || rbacRepo == nil {
		return
	}
	menuIDs := make([]uint64, 0, len(menuKeys))
	for _, menuKey := range menuKeys {
		if item, ok := menuIndex[menuKey]; ok && item.MenuID != 0 {
			menuIDs = append(menuIDs, item.MenuID)
		}
	}
	_ = rbacRepo.ReplaceRoleMenus(ctx, roleID, menuIDs)
}
