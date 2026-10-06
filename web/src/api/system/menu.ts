import { Alova } from '@/utils/http/alova/index';
import type { AdminMenuNode, UpsertMenuRequest, DeleteMenuRequest } from '@/types/api';
import type { ItemsData } from '@/types/api/common';

/**
 * 完整菜单树（管理页使用，不做权限过滤）。
 * GET /v1/admin/system/menu/tree
 * 权限: system.menu.read
 *
 * 后端返回 writeItemsResponse:
 *   data: { items: adminMenuNode[], tree: true }
 *
 * @returns Promise<ItemsData<AdminMenuNode>> — 默认拦截器解包，只返回 data
 */
export function getMenuTree() {
  return Alova.Get<ItemsData<AdminMenuNode>>('/system/menu/tree');
}

/**
 * 创建或更新菜单。
 * POST /v1/admin/system/menu/upsert
 * 权限: system.menu.update
 *
 * @param data — 发送前经 HTTP 拦截器 toSnakeCase 自动转换 key → snake_case
 */
export function upsertMenu(data: UpsertMenuRequest) {
  return Alova.Post<AdminMenuNode>('/system/menu/upsert', data);
}

/**
 * 删除菜单。
 * POST /v1/admin/system/menu/delete
 * 权限: system.menu.delete
 */
export function deleteMenu(data: DeleteMenuRequest) {
  return Alova.Post<{ code: number; message: string }>('/system/menu/delete', data);
}
