import type { RouteRecordRaw } from 'vue-router';
import { Layout } from '@/router/constant';
import { CloudServerOutlined } from '@vicons/antd';
import { renderIcon } from '@/utils/index';

/**
 * 集群管理路由（FIXED 模式降级使用）。
 * BACK 模式下由后端菜单树驱动。
 */
const routes: RouteRecordRaw[] = [
  {
    path: '/cluster', name: 'Cluster', redirect: '/cluster/nodes', component: Layout,
    meta: { title: '集群管理', icon: renderIcon(CloudServerOutlined), sort: 2 },
    children: [
      { path: 'nodes', name: 'ClusterNodes', meta: { title: '节点管理' }, component: () => import('@/views/cluster/nodes.vue') },
      { path: 'workers', name: 'ClusterWorkers', meta: { title: 'Worker 管理' }, component: () => import('@/views/cluster/workers.vue') },
      { path: 'members', name: 'ClusterMembers', meta: { title: '集群成员' }, component: () => import('@/views/cluster/members.vue') },
      { path: 'scheduler', name: 'ClusterScheduler', meta: { title: '调度洞察' }, component: () => import('@/views/cluster/scheduler.vue') },
      { path: 'topology', name: 'ClusterTopology', meta: { title: '集群拓扑' }, component: () => import('@/views/cluster/topology.vue') },
    ],
  },
];
export default routes;
