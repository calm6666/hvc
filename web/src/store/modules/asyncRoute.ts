import { toRaw, unref } from 'vue';
import { defineStore } from 'pinia';
import { RouteRecordRaw } from 'vue-router';
import { store } from '@/store';
import { asyncRoutes, constantRouter } from '@/router/index';
import { generateRoutes as generateRoutesFromMenu, asyncImportRoute } from '@/router/generator';
import { useProjectSetting } from '@/hooks/setting/useProjectSetting';

interface TreeHelperConfig {
  id: string;
  children: string;
  pid: string;
}

const DEFAULT_CONFIG: TreeHelperConfig = {
  id: 'id',
  children: 'children',
  pid: 'pid',
};

const getConfig = (config: Partial<TreeHelperConfig>) => Object.assign({}, DEFAULT_CONFIG, config);

export interface IAsyncRouteState {
  menus: RouteRecordRaw[];
  routers: RouteRecordRaw[];
  routersAdded: RouteRecordRaw[];
  keepAliveComponents: string[];
  /** 动态路由是否已添加，避免重复注册 */
  isDynamicRouteAdded: boolean;
}

/**
 * 树形数据递归过滤。
 * 用于 FIXED 权限模式下从前端静态路由中筛出当前用户有权访问的节点。
 */
function filter<T = any>(
  tree: T[],
  func: (n: T) => boolean,
  config: Partial<TreeHelperConfig> = {}
): T[] {
  config = getConfig(config);
  const children = config.children as string;

  function listFilter(list: T[]) {
    return list
      .map((node: T) => ({ ...node }))
      .filter((node) => {
        node[children] = node[children] && listFilter(node[children]);
        return func(node) || (node[children] && node[children].length);
      });
  }

  return listFilter(tree);
}

export const useAsyncRouteStore = defineStore({
  id: 'app-async-route',
  state: (): IAsyncRouteState => ({
    menus: [],
    routers: constantRouter,
    routersAdded: [],
    keepAliveComponents: [],
    isDynamicRouteAdded: false,
  }),
  getters: {
    getMenus(): RouteRecordRaw[] {
      return this.menus;
    },
    getIsDynamicRouteAdded(): boolean {
      return this.isDynamicRouteAdded;
    },
  },
  actions: {
    getRouters() {
      return toRaw(this.routersAdded);
    },
    setDynamicRouteAdded(added: boolean) {
      this.isDynamicRouteAdded = added;
    },
    /** 将生成的动态路由合并到完整路由表中 */
    setRouters(routers: RouteRecordRaw[]) {
      this.routersAdded = routers;
      this.routers = constantRouter.concat(routers);
    },
    /** 存储菜单数据，供菜单组件渲染 */
    setMenus(menus: RouteRecordRaw[]) {
      this.menus = menus;
    },
    setKeepAliveComponents(compNames: string[]) {
      this.keepAliveComponents = compNames;
    },

    /**
     * 根据用户权限信息生成可访问的动态路由表。
     *
     * 两种权限模式（由 settings/projectSetting.ts 中的 permissionMode 控制）：
     *
     *   BACK 模式（推荐，对接真实后端）：
     *     - 直接消费后端 WhoAmI 返回的 menuTree（已在 userStore.getInfo() 中获取）
     *     - 调用 generateRoutesFromMenu() 将后端 adminMenuNode 树转为 Vue Router 路由表
     *     - 菜单树已由后端按当前用户角色做好权限过滤，前端无需再过滤
     *
     *   FIXED 模式（降级/开发备用）：
     *     - 使用前端 router/modules/*.ts 中定义的静态路由
     *     - 按 data.permissions 过滤出当前用户有权访问的路由
     *
     * @param data — userStore.getInfo() 的返回值
     *   { permissions: { value, label }[], menuTree: adminMenuNode[], user }
     */
    async generateRoutes(data: {
      permissions?: { value: string; label: string }[];
      menuTree?: RouteRecordRaw[];
    }) {
      let accessedRouters: RouteRecordRaw[];

      const permissionsList: { value: string; label: string }[] = data.permissions ?? [];

      // 路由过滤函数：如果路由 meta 中未定义 permissions，默认允许访问；
      // 否则要求当前用户拥有至少一个匹配的权限键
      const routeFilter = (route: RouteRecordRaw) => {
        const { meta } = route;
        const { permissions } = meta || {};
        if (!permissions) return true;
        return permissionsList.some((item) => permissions.includes(item.value));
      };

      const { permissionMode } = useProjectSetting();

      if (unref(permissionMode) === 'BACK') {
        // ---- BACK 模式：后端动态菜单 ----
        // menuTree 由 userStore.getInfo() 中调用 WhoAmI 获取
        // 后端已根据当前用户角色做好权限过滤，前端直接转为路由表
        const menuTree = data.menuTree || [];
        accessedRouters = generateRoutesFromMenu(menuTree);
        // 通过 import.meta.glob 将 componentName 映射为 views/ 下的 .vue 文件
        asyncImportRoute(accessedRouters);
      } else {
        // ---- FIXED 模式：前端静态路由 + 权限过滤 ----
        // 使用 router/modules/*.ts 中定义的完整路由表，
        // 根据用户拥有的权限键列表过滤出可访问路由
        accessedRouters = filter(asyncRoutes, routeFilter);
      }

      // 二次过滤：移除路由表中被权限过滤后为空的节点
      accessedRouters = accessedRouters.filter(routeFilter);

      this.setRouters(accessedRouters);
      this.setMenus(accessedRouters);
      return toRaw(accessedRouters);
    },
  },
});

/**
 * 在 setup 外部使用（路由守卫等场景）。
 */
export function useAsyncRoute() {
  return useAsyncRouteStore(store);
}
