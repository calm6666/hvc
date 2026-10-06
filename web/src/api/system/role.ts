import { Alova } from '@/utils/http/alova/index';
import type { ListRolesResponse, AllRolesResponse, UpsertRoleRequest, RoleMenuTreeData, AssignRoleMenusRequest, BindRolePermissionRequest } from '@/types/api';

/** 角色分页列表。GET /v1/admin/system/role/list。权限: system.role.read */
export function getRoleList(params: { page: number; pageSize: number; roleKey?: string; status?: number }) {
  return Alova.Get<ListRolesResponse>('/system/role/list', { params });
}

/** 全部角色（不分页，供下拉选择）。GET /v1/admin/system/role/all。权限: system.role.read */
export function getAllRoles() {
  return Alova.Get<AllRolesResponse>('/system/role/all');
}

/** 创建/更新角色。POST /v1/admin/system/role/upsert。权限: system.role.update */
export function upsertRole(data: UpsertRoleRequest) {
  return Alova.Post<{ code: number; message: string }>('/system/role/upsert', data);
}

/** 获取角色菜单树（含已绑定菜单 ID）。GET /v1/admin/system/role/menu/tree?role_id=xxx。权限: system.menu.read */
export function getRoleMenuTree(roleId: number) {
  return Alova.Get<RoleMenuTreeData>('/system/role/menu/tree', { params: { roleId } });
}

/** 覆盖角色菜单绑定。POST /v1/admin/system/role-menu/assign。权限: system.role.menu_bind */
export function assignRoleMenus(data: AssignRoleMenusRequest) {
  return Alova.Post<{ code: number; message: string }>('/system/role-menu/assign', data);
}

/** 绑定角色与权限点。POST /v1/admin/system/role-permission/bind。权限: system.role.permission_bind */
export function bindRolePermission(data: BindRolePermissionRequest) {
  return Alova.Post<{ code: number; message: string }>('/system/role-permission/bind', data);
}
