import { RouteRecordRaw } from 'vue-router';
import { Layout } from '@/router/constant';
import { DashboardOutlined } from '@vicons/antd';
import { renderIcon } from '@/utils/index';

const routeName = 'dashboard';

/**
 * 仪表盘路由（FIXED 模式降级使用，BACK 模式下由后端菜单驱动）。
 * 仅保留主控台页面，数据来自 GET /v1/admin/cluster/overview。
 */
const routes: Array<RouteRecordRaw> = [
  {
    path: '/dashboard',
    name: routeName,
    redirect: '/dashboard/console',
    component: Layout,
    meta: {
      title: '仪表盘',
      icon: renderIcon(DashboardOutlined),
      sort: 0,
    },
    children: [
      {
        path: 'console',
        name: `${routeName}_console`,
        meta: {
          title: '主控台',
          affix: true,
        },
        component: () => import('@/views/dashboard/console/console.vue'),
      },
    ],
  },
];

export default routes;
