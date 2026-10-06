import type { PageData } from './common';

// ============================================================
// 运行日志 — runtimeLogView (internal/interfaces/http/admin/rbac_view.go:74)
// ============================================================

/**
 * 运行日志行数据。
 * 对应后端: runtimeLogView (internal/interfaces/http/admin/rbac_view.go:74)
 *
 * Go struct → toCamelCase → TS:
 *   LogID      → logId
 *   Service    → serviceName     (json tag: "service_name")
 *   Level      → logLevel        (json tag: "log_level")
 *   ActionName → actionName      (json tag: "action_name")
 *   Fields     → fields          (map[string]any)
 *   LoggedAt   → loggedAt        (json tag: "logged_at")
 */
export interface RuntimeLogRow {
  logId: number;
  /** 服务名 */
  serviceName: string;
  /** 日志级别: "info" | "warn" | "error" */
  logLevel: string;
  /** 操作名称 */
  actionName: string;
  /** 日志详情字段（map 或 JSON 字符串） */
  fields: Record<string, unknown> | string | null;
  /** 日志时间 */
  loggedAt: string;
}

/** 运行日志分页列表响应 data 类型 */
export type ListRuntimeLogsResponse = PageData<RuntimeLogRow>;

/**
 * 运行日志查询参数。
 * 对应后端: RBACHandler.ListRuntimeLogs (rbac.go:133)
 */
export interface ListRuntimeLogsParams {
  page: number;
  pageSize: number;
  /** 日志级别筛选 */
  level?: string;
  /** 操作名称筛选 */
  actionName?: string;
  /** RFC3339 开始时间 */
  startTime?: string;
  /** RFC3339 结束时间 */
  endTime?: string;
}
