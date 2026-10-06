import { Alova } from '@/utils/http/alova/index';
import type { LoginRequest, LoginResponse, WhoAmIResponse } from '@/types/api';

/**
 * 管理员登录。
 * POST /v1/admin/auth/login
 * 后端接受 application/x-www-form-urlencoded，响应通过 HttpOnly Cookie 下发 admin_session。
 *
 * @returns Promise<LoginResponse> — isReturnNativeResponse 模式，返回完整信封 { code, message, data }
 */
export function loginApi(params: LoginRequest) {
  const body = new URLSearchParams(params as unknown as Record<string, string>).toString();
  return Alova.Post<LoginResponse>('/auth/login', body, {
    meta: { isReturnNativeResponse: true },
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
  });
}

/**
 * 管理员登出。
 * POST /v1/admin/auth/logout
 * 后端清除 admin_session Cookie（Set-Cookie: admin_session=; Max-Age=-1）。
 */
export function logoutApi() {
  return Alova.Post<{ code: number; message: string }>('/auth/logout', undefined, {
    meta: { isReturnNativeResponse: true },
  });
}

/**
 * 获取当前管理员会话信息（WhoAmI）。
 * GET /v1/admin/auth/me
 * 权限: auth.session.read
 *
 * 后端通过 Cookie 中的 admin_session 识别当前用户，
 * 返回 user、permission_keys、menu_tree 等完整信息。
 *
 * @returns Promise<WhoAmIResponse> — isReturnNativeResponse 模式
 */
export function whoAmI() {
  return Alova.Get<WhoAmIResponse>('/auth/me', {
    meta: { isReturnNativeResponse: true },
  });
}

/** 健康检查。GET /v1/admin/ping。权限: system.user.read */
export function ping() {
  return Alova.Get<{ code: number; message: string }>('/ping');
}
