import { RouteRecordRaw } from 'vue-router';
import { Layout } from '@/router/constant';
import { SettingOutlined } from '@vicons/antd';
import { renderIcon } from '@/utils/index';

/**
 * 系统管理路由（FIXED 模式降级使用，BACK 模式下由后端菜单驱动）。
 */
const routes: Array<RouteRecordRaw> = [
  {
    path: '/system',
    name: 'System',
    redirect: '/system/user',
    component: Layout,
    meta: {
      title: '系统管理',
      icon: renderIcon(SettingOutlined),
      sort: 1,
    },
    children: [
      {
        path: 'user',
        name: 'SystemUser',
        meta: { title: '用户管理' },
        component: () => import('@/views/system/user/user.vue'),
      },
      {
        path: 'role',
        name: 'SystemRole',
        meta: { title: '角色管理' },
        component: () => import('@/views/system/role/role.vue'),
      },
      {
        path: 'menu',
        name: 'SystemMenu',
        meta: { title: '菜单管理' },
        component: () => import('@/views/system/menu/menu.vue'),
      },
      {
        path: 'permission',
        name: 'SystemPermission',
        meta: { title: '权限管理' },
        component: () => import('@/views/system/permission/permission.vue'),
      },
      {
        path: 'audit',
        name: 'SystemAudit',
        meta: { title: '审计日志' },
        component: () => import('@/views/system/audit/audit.vue'),
      },
      {
        path: 'log',
        name: 'SystemLog',
        meta: { title: '运行日志' },
        component: () => import('@/views/system/log/log.vue'),
      },
    ],
  },
];

export default routes;
