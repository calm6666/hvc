import { defineStore } from 'pinia';
import { store } from '@/store';
import { ResultEnum } from '@/enums/httpEnum';

import { loginApi, logoutApi, whoAmI } from '@/api/auth';

/**
 * 当前登录用户的脱敏信息（来自 WhoAmI 接口 data.user 字段）。
 *
 * 字段名经 toCamelCase 转换后与后端 adminUserView 对齐：
 *   { adminUserId, username, displayName, status, lastLoginAt, lastLoginIp, createdAt, updatedAt }
 */
export interface UserInfoType {
  adminUserId: number;
  username: string;
  displayName: string;
  status: number;
  lastLoginAt: string | null;
  lastLoginIp: string;
  createdAt: string;
  updatedAt: string;
}

export interface IUserState {
  /** 登录用户名 */
  username: string;
  /** 显示名称（昵称） */
  displayName: string;
  /** 后端返回的 permission_keys 字符串数组，如 ["cluster.read", "live.channel.create"] */
  permissions: string[];
  /** 当前登录用户的完整脱敏信息 */
  info: UserInfoType | null;
}

export const useUserStore = defineStore({
  id: 'app-user',
  state: (): IUserState => ({
    username: '',
    displayName: '',
    permissions: [],
    info: null,
  }),
  getters: {
    /** 当前用户名（登录账号） */
    getUsername(): string {
      return this.username || this.info?.username || '';
    },
    /** 当前显示名（昵称），优先显示名，其次用户名 */
    getDisplayName(): string {
      return this.displayName || this.info?.displayName || this.username;
    },
    /** 当前用户拥有的权限键列表（string[]） */
    getPermissions(): string[] {
      return this.permissions;
    },
    /** 当前登录用户完整脱敏信息 */
    getUserInfo(): UserInfoType | null {
      return this.info;
    },
  },
  actions: {
    setPermissions(permissions: string[]) {
      this.permissions = permissions;
    },
    setUserInfo(info: UserInfoType | null) {
      this.info = info;
    },

    /**
     * 管理员登录。
     *
     * 鉴权流程：
     * 1. 前端将 username / password 以 application/x-www-form-urlencoded 格式发送到后端
     * 2. 后端验证通过后，通过 Set-Cookie 下发 HttpOnly 的 admin_session Cookie
     * 3. 后续所有请求由浏览器自动携带该 Cookie，前端不存储任何 token
     *
     * 登录成功后不在此处调用 getInfo()，而是由路由守卫统一调用，
     * 避免登录跳转后再次请求用户信息造成重复。
     *
     * @param params { username: string; password: string; otp_code?: string }
     * @returns 后端原始响应 { code, message, data }
     */
    async login(params: { username: string; password: string; otp_code?: string }) {
      const res = await loginApi(params);
      const { code, data } = res;
      if (code === ResultEnum.SUCCESS) {
        // Cookie 已由后端 Set-Cookie 写入浏览器，后续请求自动携带
        // 用户详细信息（权限/菜单树）在路由守卫中通过 getInfo() 拉取
      }
      return res;
    },

    /**
     * 获取当前登录用户的完整会话信息（WhoAmI）。
     *
     * 调用 GET /v1/admin/auth/me，后端通过请求中的 admin_session Cookie 识别当前用户，
     * 无需前端传参。返回的 data 包含：
     *
     *   {
     *     authenticated:  true,
     *     adminUserId:    1,
     *     user:           { adminUserId, username, displayName, status, ... },
     *     permissionKeys: ["cluster.read", "live.channel.create", ...],
     *     menuTree:       [ { menuId, parentId, menuKey, menuName, routePath,
     *                         componentName, iconName, permissionKey, children: [...] }, ... ]
     *   }
     *
     * 其中 menuTree 已由后端按当前用户角色做过权限过滤，前端直接用于生成路由表。
     *
     * @returns {{ permissions, menuTree, user }}
     *   - permissions: 已转为 asyncRoute store 期望的 { value, label }[] 格式
     *   - menuTree: 后端返回的菜单树（已过滤当前用户可见节点）
     *   - user: 当前用户脱敏信息
     */
    async getInfo() {
      const res = await whoAmI();
      const { code, data } = res;

      if (code === ResultEnum.SUCCESS && data) {
        const { user, permissionKeys, menuTree, items } = data;

        // 存储用户基本信息到 state（供 Header 组件等展示）
        if (user) {
          this.username = user.username || '';
          this.displayName = user.displayName || '';
          this.setUserInfo(user);
        }

        // 存储权限键列表（string[]），供 hasPermission 等判断
        if (permissionKeys && Array.isArray(permissionKeys)) {
          this.setPermissions(permissionKeys);
        }

        // 返回给路由守卫，用于动态路由生成
        // - permissions: 转为 { value, label }[] 格式（兼容 asyncRoute store）
        // - menuTree: 优先使用 menuTree 字段，回退 items（后端两个字段内容一致）
        return {
          permissions: (permissionKeys || []).map((key: string) => ({
            value: key,
            label: key,
          })),
          menuTree: menuTree || items || [],
          user: user || null,
        };
      }

      throw new Error('获取用户信息失败');
    },

    /**
     * 管理员登出。
     *
     * 调用 POST /v1/admin/auth/logout，后端清除 admin_session Cookie
     * （Set-Cookie: admin_session=; Max-Age=-1）。前端清空 state 中所有用户数据。
     */
    async logout() {
      try {
        await logoutApi();
      } finally {
        // 无论后端是否成功，前端一律清空本地状态
        this.username = '';
        this.displayName = '';
        this.setPermissions([]);
        this.setUserInfo(null);
      }
    },
  },
});

/**
 * 在 setup 外部使用（路由守卫等场景）。
 * 使用方式：const userStore = useUser();
 */
export function useUser() {
  return useUserStore(store);
}
