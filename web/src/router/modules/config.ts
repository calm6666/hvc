import type { RouteRecordRaw } from 'vue-router';
import { Layout } from '@/router/constant';
import { SettingOutlined } from '@vicons/antd';
import { renderIcon } from '@/utils/index';

const routes: RouteRecordRaw[] = [
  {
    path: '/config', name: 'Config', redirect: '/config/versions', component: Layout,
    meta: { title: '配置管理', icon: renderIcon(SettingOutlined), sort: 5 },
    children: [
      { path: 'runtime', name: 'ConfigRuntime', meta: { title: '运行配置' }, component: () => import('@/views/config/runtime.vue') },
      { path: 'versions', name: 'ConfigVersions', meta: { title: '版本管理' }, component: () => import('@/views/config/versions.vue') },
      { path: 'callback', name: 'ConfigCallback', meta: { title: '回调配置' }, component: () => import('@/views/config/callback.vue') },
      { path: 'naming-template', name: 'ConfigNamingTemplate', meta: { title: '命名模板' }, component: () => import('@/views/config/namingTemplate.vue') },
      { path: 'config-center', name: 'ConfigCenter', meta: { title: '配置中心' }, component: () => import('@/views/config/configCenter.vue') },
      { path: 'etcd', name: 'RegistryEtcd', meta: { title: 'Etcd 注册中心' }, component: () => import('@/views/config/registryEtcd.vue') },
    ],
  },
];
export default routes;
