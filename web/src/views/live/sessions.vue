<template>
  <div>
    <div class="n-layout-page-header"><n-card :bordered="false" title="直播会话" /></div>
    <n-card :bordered="false" class="mt-4 proCard">
      <BasicTable :columns="columns" :request="loadDataTable" :row-key="(row: SessionRow) => row.sessionId" ref="actionRef"
        :scroll-x="tableScrollX" />
    </n-card>
  </div>
</template>

<script lang="ts" setup>
  import { computed, h, ref } from 'vue';
  import { NTag } from 'naive-ui';
  import { BasicTable } from '@/components/Table';
  import { listSessions } from '@/api/live';

  const actionRef = ref();

  /** 直播会话行数据。字段对应后端 LiveSession。 */
  interface SessionRow { sessionId: number; channelId: number; channelKey: string; status: string; startAt: string; endAt: string | null; }

  const columns = [
    { title: '会话 ID', key: 'sessionId', width: 100 },
    { title: '频道 ID', key: 'channelId', width: 80 },
    { title: '频道标识', key: 'channelKey', width: 140 },
    { title: '状态', key: 'status', width: 90, render: (row: SessionRow) => { const m: Record<string, 'success'|'default'|'error'> = {active:'success',idle:'default',stopped:'error'}; return h(NTag, { type: m[row.status]||'default' }, () => row.status); }},
    { title: '开始时间', key: 'startAt', width: 170 },
    { title: '结束时间', key: 'endAt', width: 170 },
  ];

  const loadDataTable = async (res: Record<string, unknown>) => await listSessions(res as Parameters<typeof listSessions>[0]);
  const tableScrollX = computed(() => columns.reduce((s, c) => s + ((c.width as number) || 150), 0));
</script>
