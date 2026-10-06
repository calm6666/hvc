import type { PageData } from './common';

// ============================================================
// 用户管理 — WriteRBAC / RBACHandler (internal/interfaces/http/admin/)
// ============================================================

/**
 * 管理员用户行数据（列表展示用）。
 * 对应后端: adminUserView (internal/interfaces/http/admin/rbac_view.go:13)
 *
 * Go struct → toCamelCase → TS:
 *   AdminUserID → adminUserId
 *   Username    → username
 *   DisplayName → displayName
 *   Status      → status
 *   LastLoginAt → lastLoginAt
 *   LastLoginIP → lastLoginIp
 *   CreatedAt   → createdAt
 *   UpdatedAt   → updatedAt
 */
export interface AdminUserRow {
  adminUserId: number;
  username: string;
  displayName: string;
  /** 1=启用, 0=禁用 */
  status: number;
  lastLoginAt: string | null;
  lastLoginIp: string;
  createdAt: string;
  updatedAt: string;
}

/** 用户列表 API 响应 data 类型（分页模式，默认拦截器解包） */
export type ListUsersResponse = PageData<AdminUserRow>;

/**
 * 创建/更新用户请求体。
 * 对应后端: WriteRBAC.UpsertUser (rbac_write.go:25)
 */
export interface UpsertUserRequest {
  username: string;
  password?: string;
  displayName?: string;
  /** 1=启用, 0=禁用 */
  status?: number;
}

/**
 * 启用/禁用用户请求体。
 * 对应后端: WriteRBAC.SetUserStatus (rbac_write.go:60)
 */
export interface SetUserStatusRequest {
  adminUserId: number;
  /** 1=启用, 0=禁用 */
  status: number;
}

/**
 * 用户-角色绑定请求体。
 * 对应后端: WriteRBAC.BindUserRole (rbac_write.go:139)
 */
export interface BindUserRoleRequest {
  adminUserId: number;
  roleId: number;
}
