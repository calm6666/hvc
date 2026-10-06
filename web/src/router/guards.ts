import { PageEnum } from '@/enums/pageEnum';
import { ErrorPageRoute } from '@/router/base';
import { useAsyncRoute } from '@/store/modules/asyncRoute';
import { useUser } from '@/store/modules/user';
import type { RouteRecordRaw } from 'vue-router';
import { isNavigationFailure, Router } from 'vue-router';
import { RedirectName } from './constant';

const LOGIN_PATH = PageEnum.BASE_LOGIN;

/**
 * 不需要登录即可访问的路由白名单。
 */
const whitePathList: string[] = [LOGIN_PATH];

/**
 * 创建路由守卫。
 *
 * 鉴权流程（Cookie 模式）：
 * 1. 白名单路由（login）直接放行
 * 2. ignoreAuth 标记的路由免鉴权放行
 * 3. 动态路由已添加 → 直接放行
 * 4. 动态路由未添加 → 调用 userStore.getInfo() 验证登录态
 *    - 成功：从返回的 menuTree 生成动态路由 → router.addRoute() → next()
 *    - 失败（401 等）：重定向到登录页
 *
 * 与旧版（Header Token 模式）的区别：
 *   - 不再从 localStorage 读取 ACCESS_TOKEN 判断登录态
 *   - 改为直接请求 WhoAmI 接口，由后端通过 Cookie 中的 admin_session 识别用户
 *   - 登录失效时后端自动清除 Cookie，前端无需手动 storage.clear()
 */
export function createRouterGuards(router: Router) {
  const userStore = useUser();
  const asyncRouteStore = useAsyncRoute();

  router.beforeEach(async (to, from, next) => {
    const Loading = window['$loading'] || null;
    Loading && Loading.start();

    // 从登录页跳转到错误页 → 重定向到首页
    if (from.path === LOGIN_PATH && to.name === 'errorPage') {
      next(PageEnum.BASE_HOME);
      return;
    }

    // 白名单路由直接放行（如 /login）
    if (whitePathList.includes(to.path as PageEnum)) {
      next();
      return;
    }

    // meta.ignoreAuth 标记的路由免鉴权（如 404 等公开页面）
    if (to.meta.ignoreAuth) {
      next();
      return;
    }

    // 动态路由已添加 → 直接放行，避免重复请求
    if (asyncRouteStore.getIsDynamicRouteAdded) {
      next();
      return;
    }

    // ---- 动态路由未添加，需要验证登录态并生成路由 ----
    try {
      // 调用 WhoAmI 接口验证登录态
      // - Cookie 有效：返回 { permissions, menuTree, user }
      // - Cookie 无效/过期：后端返回 code=401，拦截器处理后抛出异常
      const userInfo = await userStore.getInfo();

      // 根据返回的菜单树生成可访问路由表
      const routes = await asyncRouteStore.generateRoutes(userInfo);

      // 将生成的动态路由逐条注册到 Vue Router
      routes.forEach((item) => {
        router.addRoute(item as unknown as RouteRecordRaw);
      });

      // 注册 404 兜底路由（确保在所有动态路由之后）
      const isErrorPage = router
        .getRoutes()
        .findIndex((item) => item.name === ErrorPageRoute.name);
      if (isErrorPage === -1) {
        router.addRoute(ErrorPageRoute as unknown as RouteRecordRaw);
      }

      // 处理登录后重定向：优先跳转 login 时传递的 redirect 参数
      const redirectPath = (from.query.redirect || to.path) as string;
      const redirect = decodeURIComponent(redirectPath);
      const nextData =
        to.path === redirect ? { ...to, replace: true } : { path: redirect };

      asyncRouteStore.setDynamicRouteAdded(true);
      next(nextData);
    } catch (_error) {
      // WhoAmI 返回 401 或其他网络异常
      // Cookie 可能已过期或不存在 → 重定向到登录页
      next({
        path: LOGIN_PATH,
        replace: true,
        query: { redirect: to.path },
      });
    } finally {
      Loading && Loading.finish();
    }
  });

  /**
   * 路由后置守卫：
   * - 设置页面标题（来自路由 meta.title）
   * - 管理 KeepAlive 缓存组件列表
   */
  router.afterEach((to, _, failure) => {
    document.title = (to?.meta?.title as string) || document.title;
    if (isNavigationFailure(failure)) {
      // 导航失败时不处理 keepAlive
    }
    const asyncRouteStore = useAsyncRoute();
    const keepAliveComponents = asyncRouteStore.keepAliveComponents;
    const currentComName: string | undefined = to.matched.find((item) => item.name == to.name)?.name as string | undefined;

    if (
      currentComName &&
      !keepAliveComponents.includes(currentComName) &&
      to.meta?.keepAlive
    ) {
      // 需要缓存的组件：添加到 keepAlive 列表中
      keepAliveComponents.push(currentComName);
    } else if (!to.meta?.keepAlive || to.name == RedirectName) {
      // 不需要缓存的组件：从 keepAlive 列表中移除
      const index = asyncRouteStore.keepAliveComponents.findIndex(
        (name) => name == currentComName
      );
      if (index != -1) {
        keepAliveComponents.splice(index, 1);
      }
    }
    asyncRouteStore.setKeepAliveComponents(keepAliveComponents);

    const Loading = window['$loading'] || null;
    Loading && Loading.finish();
  });

  router.onError((error) => {
    console.log(error, '路由错误');
  });
}
