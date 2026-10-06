import { Alova } from '@/utils/http/alova/index';
import type { ListUsersResponse, UpsertUserRequest, SetUserStatusRequest, BindUserRoleRequest } from '@/types/api';

/** 管理员用户分页列表。GET /v1/admin/system/user/list。权限: system.user.read */
export function listUsers(params: { page: number; pageSize: number; username?: string; status?: number }) {
  return Alova.Get<ListUsersResponse>('/system/user/list', { params });
}

/** 创建/更新管理员用户。POST /v1/admin/system/user/upsert。权限: system.user.create */
export function upsertUser(data: UpsertUserRequest) {
  return Alova.Post<{ code: number; message: string }>('/system/user/upsert', data);
}

/** 启用/禁用管理员用户。POST /v1/admin/system/user/status。权限: system.user.update */
export function setUserStatus(data: SetUserStatusRequest) {
  return Alova.Post<{ code: number; message: string }>('/system/user/status', data);
}

/** 为用户绑定角色。POST /v1/admin/system/user-role/bind。权限: system.user.role_bind */
export function bindUserRole(data: BindUserRoleRequest) {
  return Alova.Post<{ code: number; message: string }>('/system/user-role/bind', data);
}
