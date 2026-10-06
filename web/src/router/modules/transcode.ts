import type { RouteRecordRaw } from 'vue-router';
import { Layout } from '@/router/constant';
import { VideoCameraOutlined } from '@vicons/antd';
import { renderIcon } from '@/utils/index';

const routes: RouteRecordRaw[] = [
  {
    path: '/transcode', name: 'Transcode', redirect: '/transcode/jobs', component: Layout,
    meta: { title: '转码管理', icon: renderIcon(VideoCameraOutlined), sort: 3 },
    children: [
      { path: 'jobs', name: 'TranscodeJobs', meta: { title: '转码任务' }, component: () => import('@/views/transcode/jobs.vue') },
      { path: 'monitor', name: 'TranscodeMonitor', meta: { title: '实时监控' }, component: () => import('@/views/transcode/monitor.vue') },
    ],
  },
];
export default routes;
