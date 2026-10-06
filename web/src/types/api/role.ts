import type { PageData, ItemsData } from './common';
import type { AdminMenuNode } from './menu';
import type { PermissionTreeNode } from './permission';

// ============================================================
// 角色管理 — AdminRoleRecord / RBACHandler (internal/interfaces/http/admin/)
// ============================================================

/**
 * 角色行数据。
 * 对应后端: adminRoleView (internal/interfaces/http/admin/rbac_view.go:24)
 *
 * Go struct → toCamelCase → TS:
 *   RoleID    → roleId
 *   RoleKey   → roleKey
 *   RoleName  → roleName
 *   RoleDesc  → roleDesc
 *   Status    → status
 *   CreatedAt → createdAt
 *   UpdatedAt → updatedAt
 */
export interface AdminRoleRow {
  roleId: number;
  /** 角色标识（如 "admin", "operator"） */
  roleKey: string;
  /** 角色名称 */
  roleName: string;
  /** 角色描述 */
  roleDesc: string;
  /** 1=启用, 0=禁用 */
  status: number;
  createdAt: string;
  updatedAt: string;
}

/** 角色分页列表响应 data 类型 */
export type ListRolesResponse = PageData<AdminRoleRow>;

/** 全部角色列表响应 data 类型（不分页） */
export type AllRolesResponse = ItemsData<AdminRoleRow>;

/**
 * 创建/更新角色请求体。
 * 对应后端: WriteRBAC.UpsertRole (rbac_write.go:76)
 */
export interface UpsertRoleRequest {
  roleKey: string;
  roleName: string;
  roleDesc?: string;
  status?: number;
}

/**
 * 角色菜单树响应 data 类型。
 * 对应后端: RBACHandler.RoleMenuTree (rbac.go:101)
 *
 * Go writeItemsResponse:
 *   data: { items: []adminMenuNode, tree: true, role_id: uint64, menu_ids: []uint64 }
 */
export interface RoleMenuTreeData {
  /** 完整菜单树 */
  items: AdminMenuNode[];
  /** 辅助标记 */
  tree: boolean;
  /** 角色 ID */
  roleId: number;
  /** 该角色已绑定的菜单 ID 列表 */
  menuIds: number[];
}

/**
 * 分配角色菜单请求体。
 * 对应后端: WriteRBAC.AssignRoleMenus (rbac_write.go:268)
 */
export interface AssignRoleMenusRequest {
  roleId: number;
  /** 菜单 ID 列表（覆盖模式） */
  menuIds: number[];
}

/**
 * 绑定角色权限请求体。
 * 对应后端: WriteRBAC.BindRolePermission (rbac_write.go:168)
 */
export interface BindRolePermissionRequest {
  roleId: number;
  permId: number;
}
