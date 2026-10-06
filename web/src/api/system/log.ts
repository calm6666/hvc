import { Alova } from '@/utils/http/alova/index';
import type { ListRuntimeLogsResponse, ListRuntimeLogsParams } from '@/types/api';

/**
 * 运行日志分页列表。
 * GET /v1/admin/system/log/list
 * 权限: system.log.read
 *
 * 后端返回 writePageResponse:
 *   data: { page, pageSize, total, items: runtimeLogView[] }
 */
export function listRuntimeLogs(params: ListRuntimeLogsParams) {
  return Alova.Get<ListRuntimeLogsResponse>('/system/log/list', { params });
}
