import type { ApiEnvelope } from './common';
import type { AdminMenuNode } from './menu';

// ============================================================
// 认证模块 — AuthHandler (internal/interfaces/http/admin/auth.go)
// ============================================================

/**
 * 登录请求体。
 * 后端 AuthHandler.Login 通过 r.ParseForm() 解析 multipart/form-data。
 * contentType: application/x-www-form-urlencoded
 */
export interface LoginRequest {
  /** 管理员用户名 */
  username: string;
  /** 管理员密码 */
  password: string;
  /** OTP 验证码（启用二步验证时必填） */
  otp_code?: string;
}

/**
 * 登录成功响应 data 字段。
 * Go: data := map[string]any{ "authenticated": true, "token_transport": {...} }
 */
export interface LoginResponseData {
  /** 是否认证成功 */
  authenticated: boolean;
  /** Token 传输方式（本项目使用 HttpOnly Cookie） */
  token_transport: {
    /** 传输类型 */
    type: 'cookie';
    /** Cookie 名称 */
    cookie_name: 'admin_session';
    /** 是否 HttpOnly（JS 不可读） */
    http_only: true;
  };
}

/** 登录 API 完整响应（isReturnNativeResponse 模式） */
export type LoginResponse = ApiEnvelope<LoginResponseData>;

/**
 * 管理员用户脱敏信息。
 * 对应后端: adminUserView (internal/interfaces/http/admin/rbac_view.go:13)
 *
 * Go struct → JSON (snake_case) → toCamelCase → TS
 *   AdminUserID     → admin_user_id     → adminUserId
 *   Username        → username          → username
 *   DisplayName     → display_name      → displayName
 *   Status          → status            → status
 *   LastLoginAt     → last_login_at     → lastLoginAt
 *   LastLoginIP     → last_login_ip     → lastLoginIp
 *   CreatedAt       → created_at        → createdAt
 *   UpdatedAt       → updated_at        → updatedAt
 */
export interface AdminUserView {
  adminUserId: number;
  username: string;
  displayName: string;
  /** 状态: 1=启用, 0=禁用 */
  status: number;
  lastLoginAt: string | null;
  lastLoginIp: string;
  createdAt: string;
  updatedAt: string;
}

/**
 * WhoAmI 接口（GET /v1/admin/auth/me）返回的 data 字段。
 * 对应后端: AuthHandler.WhoAmI (internal/interfaces/http/admin/auth.go:56)
 *
 * Go 代码构建的 data map:
 *   data := map[string]any{
 *     "authenticated":   bool,
 *     "admin_user_id":   uint64,
 *     "user":            adminUserView,
 *     "permission_keys": []string,
 *     "menu_tree":       []adminMenuNode,    // 已按当前用户角色过滤
 *     "tree":            true,
 *     "items":           []adminMenuNode,    // 与 menuTree 内容一致
 *   }
 */
export interface WhoAmIData {
  /** 是否已认证 */
  authenticated: boolean;
  /** 当前管理员用户 ID */
  adminUserId: number;
  /** 当前用户脱敏信息 */
  user: AdminUserView | null;
  /** 当前用户拥有的所有权限键列表（字符串数组） */
  permissionKeys: string[];
  /** 当前用户可见的菜单树（已做权限过滤） */
  menuTree: AdminMenuNode[];
  /** 辅助标记：data 中的 items 为树形结构 */
  tree: boolean;
  /** 与 menuTree 内容一致的菜单树（兼容旧版字段名） */
  items: AdminMenuNode[];
}

/** WhoAmI API 完整响应（isReturnNativeResponse 模式） */
export type WhoAmIResponse = ApiEnvelope<WhoAmIData>;
