import { Alova } from '@/utils/http/alova/index';
import type { ListAuditLogsResponse, ListAuditLogsParams } from '@/types/api';

/**
 * 审计日志分页列表。
 * GET /v1/admin/audit/list
 * 权限: audit.read
 *
 * 后端返回 writePageResponse:
 *   data: { page, pageSize, total, items: auditLogView[] }
 */
export function listAuditLogs(params: ListAuditLogsParams) {
  return Alova.Get<ListAuditLogsResponse>('/audit/list', { params });
}
