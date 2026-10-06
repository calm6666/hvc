import type { RouteRecordRaw } from 'vue-router';
import { Layout } from '@/router/constant';
import { PlayCircleOutlined } from '@vicons/antd';
import { renderIcon } from '@/utils/index';

const routes: RouteRecordRaw[] = [
  {
    path: '/live', name: 'Live', redirect: '/live/channels', component: Layout,
    meta: { title: '直播管理', icon: renderIcon(PlayCircleOutlined), sort: 4 },
    children: [
      { path: 'channels', name: 'LiveChannels', meta: { title: '频道管理' }, component: () => import('@/views/live/channels.vue') },
      { path: 'sessions', name: 'LiveSessions', meta: { title: '直播会话' }, component: () => import('@/views/live/sessions.vue') },
    ],
  },
];
export default routes;
