import { Alova } from '@/utils/http/alova/index';

/**
 * 仪表盘数据 — 复用集群总览接口。
 * GET /v1/admin/cluster/overview
 * 权限: cluster.read
 */
export function getConsoleInfo() {
  return Alova.Get('/cluster/overview');
}
