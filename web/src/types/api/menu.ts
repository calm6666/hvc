// ============================================================
// 菜单管理 — AdminMenuRecord (internal/infra/db/mysql/admin_models.go:101)
// ============================================================

/**
 * 后台菜单节点。
 * 对应后端: AdminMenuRecord (GORM) → adminMenuNode (view)
 *   internal/infra/db/mysql/admin_models.go:101
 *   internal/interfaces/http/admin/rbac_view.go:43
 *
 * Go struct → JSON (snake_case) → toCamelCase → TS
 *   MenuID        → menu_id           → menuId
 *   ParentID      → parent_id         → parentId
 *   MenuKey       → menu_key          → menuKey
 *   MenuName      → menu_name         → menuName
 *   RoutePath     → route_path        → routePath
 *   ComponentName → component_name    → componentName
 *   IconName      → icon_name         → iconName
 *   MenuType      → menu_type         → menuType       // 'menu' | 'button' | 'iframe'
 *   PermissionKey → permission_key    → permissionKey
 *   SortNo        → sort_no           → sortNo
 *   Hidden        → hidden            → hidden
 *   Status        → status            → status         // 1=启用, 0=禁用
 */
export interface AdminMenuNode {
  /** 菜单 ID */
  menuId: number;
  /** 父级菜单 ID（0 表示顶级菜单） */
  parentId: number;
  /** 菜单唯一标识（用作路由 name） */
  menuKey: string;
  /** 菜单显示名称 */
  menuName: string;
  /** 前端路由路径片段（如 "user"） */
  routePath: string;
  /** 前端组件路径（如 "/system/user/user"，对应 views/system/user/user.vue） */
  componentName: string;
  /** 图标名称（如 "DashboardOutlined"，对应 router/icons.ts 中的映射） */
  iconName: string;
  /** 菜单类型: 'menu'=侧边栏菜单, 'button'=权限按钮, 'iframe'=内嵌页 */
  menuType: 'menu' | 'button' | 'iframe';
  /** 权限标识（如 "system.user.read"） */
  permissionKey: string;
  /** 排序序号（越小越靠前） */
  sortNo: number;
  /** 是否在侧边栏隐藏 */
  hidden: boolean;
  /** 状态: 1=启用, 0=禁用 */
  status: number;
  /** 子菜单 */
  children?: AdminMenuNode[];
}

/**
 * 创建/更新菜单请求体。
 * 对应后端: WriteRBAC.UpsertMenu (internal/interfaces/http/admin/rbac_write.go:193)
 *
 * 发送时由 HTTP 拦截器 toSnakeCase 自动转换:
 *   menuKey → menu_key
 *   menuName → menu_name
 *   等等...
 */
export interface UpsertMenuRequest {
  /** 菜单 ID（编辑时传入，新建时留空） */
  menuId?: number;
  /** 父级菜单 ID */
  parentId?: number;
  /** 菜单标识 */
  menuKey: string;
  /** 菜单名称 */
  menuName: string;
  /** 路由路径 */
  routePath?: string;
  /** 组件路径 */
  componentName?: string;
  /** 图标名称 */
  iconName?: string;
  /** 菜单类型 */
  menuType: string;
  /** 权限标识 */
  permissionKey?: string;
  /** 排序序号 */
  sortNo?: number;
  /** 是否隐藏 */
  hidden?: boolean;
  /** 状态 */
  status?: number;
}

/** 删除菜单请求体 */
export interface DeleteMenuRequest {
  menuId: number;
}
