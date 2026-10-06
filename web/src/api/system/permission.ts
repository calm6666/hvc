import { Alova } from '@/utils/http/alova/index';
import type { PermissionTreeNode, UpsertPermissionRequest } from '@/types/api';
import type { ItemsData } from '@/types/api/common';

/**
 * 获取权限树（供管理页展示和分配角色权限时使用）。
 * GET /v1/admin/system/permission/tree
 * 权限: system.permission.read
 *
 * 后端返回 writeItemsResponse:
 *   data: { items: permissionTreeNode[], tree: true }
 */
export function permissionTree() {
  return Alova.Get<ItemsData<PermissionTreeNode>>('/system/permission/tree');
}

/**
 * 创建或更新权限点。
 * POST /v1/admin/system/permission/upsert
 * 权限: system.role.permission_bind
 *
 * @param data — 发送前经 toSnakeCase 转换: permKey→perm_key, permName→perm_name, ...
 */
export function upsertPermission(data: UpsertPermissionRequest) {
  return Alova.Post<{ code: number; message: string }>('/system/permission/upsert', data);
}
