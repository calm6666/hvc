<template>
  <div>
    <div class="n-layout-page-header"><n-card :bordered="false" title="集群成员" /></div>
    <n-card :bordered="false" class="mt-4 proCard">
      <BasicTable :columns="columns" :request="loadDataTable" :row-key="(row: MemberRow) => row.nodeId" ref="actionRef"
        :scroll-x="tableScrollX" />
    </n-card>
  </div>
</template>

<script lang="ts" setup>
  import { computed, h, ref } from 'vue';
  import { NTag } from 'naive-ui';
  import { BasicTable } from '@/components/Table';
  import { listMembers } from '@/api/cluster';

  const actionRef = ref();

  /** 集群成员行数据。字段对应后端 clusterMemberView（经 toCamelCase 转换）。 */
  interface MemberRow { nodeId: number; nodeName: string; nodeRole: string; hostIp: string; enabled: boolean; quarantined: boolean; draining: boolean; controlPlane: boolean; onlineEstimate: boolean; workerTotal: number; workerOnlineTotal: number; lastHeartbeatAt: string; }

  const columns = [
    { title: 'ID', key: 'nodeId', width: 70 },
    { title: '名称', key: 'nodeName', width: 140 },
    { title: '角色', key: 'nodeRole', width: 100, render: (row: MemberRow) => h(NTag, { type: row.controlPlane ? 'info' : 'default' }, () => row.nodeRole) },
    { title: 'IP', key: 'hostIp', width: 130 },
    { title: '启用', key: 'enabled', width: 70, render: (row: MemberRow) => h(NTag, { type: row.enabled ? 'success' : 'default' }, () => row.enabled ? '是' : '否') },
    { title: '在线', key: 'onlineEstimate', width: 70, render: (row: MemberRow) => h(NTag, { type: row.onlineEstimate ? 'success' : 'error' }, () => row.onlineEstimate ? '在线' : '离线') },
    { title: 'Worker', key: 'workerOnlineTotal', width: 100, render: (row: MemberRow) => `${row.workerOnlineTotal}/${row.workerTotal}` },
    { title: '最后心跳', key: 'lastHeartbeatAt', width: 170 },
  ];

  const loadDataTable = async (res: Record<string, unknown>) => await listMembers(res as Parameters<typeof listMembers>[0]);
  const tableScrollX = computed(() => columns.reduce((s, c) => s + ((c.width as number) || 150), 0));
</script>
