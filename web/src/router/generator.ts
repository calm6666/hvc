import { constantRouterIcon } from './icons';
import { RouteRecordRaw } from 'vue-router';
import { Layout, ParentLayout } from '@/router/constant';
import type { AppRouteRecordRaw } from '@/router/types';
import type { AdminMenuNode } from '@/types/api';

/**
 * Iframe 组件懒加载。
 * 当后端菜单类型为 "iframe" 时使用。
 */
const Iframe = () => import('@/views/iframe/index.vue');

/**
 * 布局组件映射表。
 * 特殊字符串 → Vue 组件，由 asyncImportRoute 根据 componentName 匹配替换。
 * 后端可传 "LAYOUT" 表示使用主布局，"IFRAME" 表示 iframe 内嵌页。
 */
const LayoutMap = new Map<string, () => Promise<typeof import('*.vue')>>();
LayoutMap.set('LAYOUT', Layout);
LayoutMap.set('IFRAME', Iframe);

/**
 * 将后端返回的 adminMenuNode 树递归转换为 Vue Router 路由表。
 *
 * 后端菜单节点（经 toCamelCase 转换后）字段：
 *   menuId, parentId, menuKey, menuName, routePath, componentName,
 *   iconName, menuType, permissionKey, sortNo, hidden, status, children
 *
 * 处理规则：
 *   1. status !== 1 的节点不生成路由（已废弃/禁用）
 *   2. menuType === 'button' 的节点不生成路由（按钮权限不由路由控制）
 *   3. 没有 componentName 也没有子路由的节点 → 跳过
 *   4. 没有 componentName 但有子路由的节点 → 使用 ParentLayout 占位
 *   5. componentName 格式如 "/dashboard/console/console"
 *      → asyncImportRoute 通过 import.meta.glob 匹配 views/ 下同名 .vue/.tsx 文件
 *   6. iconName 通过 constantRouterIcon 表映射为渲染函数
 *   7. path 由父子节点的 routePath 递归拼接
 *
 * @param menuNodes — 后端返回的菜单树（必须是树形结构，已由 buildMenuTree 递归组装 children）
 * @param parent    — 父级路由对象（内部递归使用，首次调用时省略）
 * @returns Vue Router 路由配置数组
 */
export const generateRoutes = (menuNodes: AdminMenuNode[], parent?: RouteRecordRaw): RouteRecordRaw[] => {
  if (!menuNodes || !Array.isArray(menuNodes)) return [];

  // 1. 过滤掉禁用/非菜单类型的节点
  // 2. 按 sortNo 升序排列
  return menuNodes
    .filter((node) => {
      // 禁用的菜单不生成路由
      if (node.status !== 1) return false;
      // 按钮类型的节点不生成路由（权限由 v-permission 指令控制）
      if (node.menuType === 'button') return false;
      return true;
    })
    .sort((a, b) => (a.sortNo ?? 0) - (b.sortNo ?? 0))
    .map((node) => {
      // 路由路径：取 routePath 或回退到 menuKey
      const routePath = node.routePath || node.menuKey || '';
      // 拼接父级路径
      const fullPath = parent
        ? `${parent.path}/${routePath}`.replace(/\/+/g, '/')
        : `/${routePath}`.replace(/\/+/g, '/');

      const currentRoute: Record<string, unknown> = {
        path: fullPath,
        // 路由名称：使用 menuKey 作为唯一标识（对应后端 admin_menu.menu_key）
        name: node.menuKey,
        // component 由 asyncImportRoute 通过 import.meta.glob 动态匹配 views/ 下的文件
        component: undefined,
        meta: {
          // 菜单标题（显示在侧边栏和页面标签）
          title: node.menuName,
          // label 用于 NMenu 组件渲染
          label: node.menuName,
          // 图标（通过 constantRouterIcon 表映射为渲染函数）
          icon: constantRouterIcon[node.iconName] || null,
          // 权限标识：转为数组格式，供 v-permission 和路由过滤使用
          permissions: node.permissionKey ? [node.permissionKey] : null,
          // 排序（用于侧边栏菜单排序）
          sort: node.sortNo ?? 0,
          // 是否在侧边栏隐藏
          hidden: node.hidden ?? false,
          // 是否缓存页面
          keepAlive: true,
        },
      };

      // 设置路由的 component
      // 优先使用后端传入的 componentName（如 "/cluster/nodes" → 匹配 views/cluster/nodes.vue）
      if (node.component) {
        currentRoute.component = node.component;
      } else if (node.menuType === 'iframe' && routePath) {
        // iframe 类型：使用 Iframe 组件 + frameSrc 元数据
        currentRoute.component = 'IFRAME';
        currentRoute.meta.frameSrc = routePath;
      }

      // 处理子菜单（递归）
      if (node.children && node.children.length > 0) {
        // 设置 redirect：默认跳转到第一个有效子路由
        if (!node.redirect) {
          const firstChild = node.children.find(
            (c: AdminMenuNode) => c.status === 1 && c.menuType !== 'button'
          );
          if (firstChild) {
            const childPath = firstChild.routePath || firstChild.menuKey || '';
            currentRoute.redirect = `${fullPath}/${childPath}`.replace(/\/+/g, '/');
          }
        }
        // 递归生成子路由
        currentRoute.children = generateRoutes(node.children, currentRoute).filter(Boolean);
      }

      // 如果当前节点既没有 component，也没有子路由 → 跳过（无效路由）
      if (!currentRoute.component && !currentRoute.children?.length) {
        return null;
      }

      // 没有 component 但有子路由 → 使用 ParentLayout 作为占位组件
      if (!currentRoute.component && currentRoute.children?.length) {
        currentRoute.component = undefined; // 由 asyncImportRoute 的 else if (name) 设为 ParentLayout
      }

      return currentRoute;
    })
    .filter(Boolean);
};

/**
 * （已废弃）动态生成菜单 — 旧版：单独请求菜单接口。
 *
 * 新版流程中菜单树由 userStore.getInfo() → WhoAmI 接口一并返回，
 * 不再需要此函数。保留仅供降级模式下参考。
 *
 * @deprecated 请使用 generateRoutes(menuTree) 直接消费 WhoAmI 中的菜单树。
 */
export const generateDynamicRoutes = async (): Promise<RouteRecordRaw[]> => {
  // 旧实现已移除，新流程不依赖此函数
  console.warn('generateDynamicRoutes is deprecated. Use generateRoutes(menuTree) instead.');
  return [];
};

/**
 * 查找 views/ 目录下的 .vue / .tsx 文件，
 * 将路由配置中的 component 字符串映射为实际的 Vue 组件。
 *
 * 匹配逻辑：
 *   componentName = "/cluster/nodes"
 *   → 在 viewsModules 中查找 key 为 "../views/cluster/nodes.vue" 的模块
 *   → 替换为动态 import 函数
 *
 * 特殊处理：
 *   - component === 'LAYOUT'   → 使用主布局组件
 *   - component === 'IFRAME'   → 使用 Iframe 组件
 *   - 无 component 但有 name → 使用 ParentLayout 占位
 *
 * @param routes — generateRoutes 生成的原始路由配置数组
 */
let viewsModules: Record<string, () => Promise<Recordable>>;
export const asyncImportRoute = (routes: AppRouteRecordRaw[] | undefined): void => {
  // 懒加载：只在首次调用时执行 import.meta.glob
  viewsModules = viewsModules || import.meta.glob('../views/**/*.{vue,tsx}');
  if (!routes) return;

  routes.forEach((item) => {
    // iframe 菜单：没有 component 但有 frameSrc → 使用 Iframe 组件
    if (!item.component && item.meta?.frameSrc) {
      item.component = 'IFRAME';
    }

    const { component, name } = item;
    const { children } = item;

    if (component) {
      // 检查是否为布局特殊字符串
      const layoutFound = LayoutMap.get(component as string);
      if (layoutFound) {
        item.component = layoutFound;
      } else {
        // 普通组件路径 → 在 viewsModules 中匹配对应的 .vue/.tsx 文件
        item.component = dynamicImport(viewsModules, component as string);
      }
    } else if (name) {
      // 没有 component 但有 name → 路由占位符（父级路由）
      item.component = ParentLayout;
    }

    // 递归处理子路由
    children && asyncImportRoute(children);
  });
};

/**
 * 在 viewsModules 全局表中匹配组件路径。
 *
 * 示例：
 *   component = "/system/user/user"
 *   → 在 viewsModules 中找到 key = "../views/system/user/user.vue"
 *   → 返回对应的动态 import 函数
 *
 * 防御：
 *   - 同名 .vue 和 .tsx 会导致匹配模糊，打印 warn
 *   - 未匹配到时打印 warn 但不中断
 *
 * @param viewsModules — import.meta.glob 返回的模块映射表
 * @param component    — 后端传入的 componentName（如 "/system/user/user"）
 * @returns 动态 import 函数 或 undefined
 */
export const dynamicImport = (
  viewsModules: Record<string, () => Promise<Recordable>>,
  component: string
) => {
  const keys = Object.keys(viewsModules);
  const matchKeys = keys.filter((key) => {
    let k = key.replace('../views', '');
    const lastIndex = k.lastIndexOf('.');
    k = k.substring(0, lastIndex);
    return k === component;
  });

  if (matchKeys?.length === 1) {
    const matchKey = matchKeys[0];
    return viewsModules[matchKey];
  }

  if (matchKeys?.length > 1) {
    console.warn(
      'Please do not create `.vue` and `.tsx` files with the same file name ' +
        'in the same hierarchical directory under the views folder. ' +
        'This will cause dynamic introduction failure'
    );
    return;
  }

  // 未匹配到组件文件时打印警告（开发期便于排查配置错误）
  if (matchKeys?.length === 0) {
    console.warn(
      `[asyncImportRoute] Component not found for "${component}". ` +
        `Expected a .vue or .tsx file at views${component}.vue or views${component}.tsx`
    );
  }
};
