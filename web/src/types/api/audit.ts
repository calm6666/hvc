import type { PageData } from './common';

// ============================================================
// 审计日志 — auditLogView (internal/interfaces/http/admin/rbac_view.go:59)
// ============================================================

/**
 * 审计日志行数据。
 * 对应后端: auditLogView (internal/interfaces/http/admin/rbac_view.go:59)
 *
 * Go struct → toCamelCase → TS:
 *   AuditLogID       → auditLogId
 *   AdminUserID      → adminUserId
 *   Username         → username
 *   ActionName       → actionName
 *   TargetType       → targetType
 *   TargetID         → targetId
 *   RequestID        → requestId
 *   RequestIP        → requestIp
 *   RequestUserAgent → requestUserAgent
 *   ResultCode       → resultCode
 *   ResultMessage    → resultMessage
 *   CreatedAt        → createdAt
 */
export interface AuditLogRow {
  auditLogId: number;
  adminUserId: number;
  /** 操作的管理员用户名 */
  username: string;
  /** 操作名称 */
  actionName: string;
  /** 目标类型 */
  targetType: string;
  /** 目标 ID */
  targetId: string;
  /** 请求追踪 ID */
  requestId: string;
  /** 请求来源 IP */
  requestIp: string;
  /** 请求 User-Agent */
  requestUserAgent: string;
  /** 操作结果码（0=成功） */
  resultCode: number;
  /** 操作结果信息 */
  resultMessage: string;
  createdAt: string;
}

/** 审计日志分页列表响应 data 类型 */
export type ListAuditLogsResponse = PageData<AuditLogRow>;

/**
 * 审计日志查询参数。
 * 对应后端: RBACHandler.ListAuditLogs (rbac.go:118)
 */
export interface ListAuditLogsParams {
  page: number;
  pageSize: number;
  adminUserId?: number;
  actionName?: string;
  /** RFC3339 格式，如 "2024-01-01T00:00:00Z" */
  startTime?: string;
  /** RFC3339 格式 */
  endTime?: string;
}
