<template>
  <div>
    <div class="n-layout-page-header">
      <n-card :bordered="false" title="转码监控">
        <template #header-extra>
          <n-tag :type="connected ? 'success' : 'default'">{{ connected ? '已连接' : '未连接' }}</n-tag>
        </template>
      </n-card>
    </div>
    <n-card :bordered="false" class="mt-4 proCard">
      <n-data-table :columns="columns" :data="snapshotList" :row-key="(row: SnapshotItem) => row.jobId" size="small" :scroll-x="tableScrollX"
        :max-height="500" />
    </n-card>
  </div>
</template>

<script lang="ts" setup>
  import { computed, ref, h, onMounted, onUnmounted } from 'vue';
  import { NTag } from 'naive-ui';
  import useWebSocket from '@/utils/websocket';

  /**
   * 转码监控实时快照数据类型。
   * 对应后端 wsmonitor.SnapshotHandler.Snapshot / HandleWS 推送的数据。
   */
  interface SnapshotItem {
    jobId: number;
    bizKey: string;
    statusName: string;
    progressPermille: number;
    stage: string;
    currentFps?: number;
    currentBitrateKbps?: number;
    elapsedMs?: number;
  }

  const connected = ref(false);
  const snapshotList = ref<SnapshotItem[]>([]);

  /** 使用泛型 SnapshotItem[] 指定 message 回调数据类型 */
  const ws = useWebSocket<SnapshotItem[]>({
    url: `ws://${window.location.host}/v1/admin/transcode/monitor/ws`,
    reconnectTimeout: 3000,
  });

  ws.on('open', () => { connected.value = true; });
  ws.on('close', () => { connected.value = false; });
  ws.on('message', (data) => { snapshotList.value = data || []; });

  onMounted(() => ws.open());
  onUnmounted(() => ws.destroy());

  const columns = [
    { title: '任务 ID', key: 'jobId', width: 80 },
    { title: '业务 Key', key: 'bizKey', width: 120 },
    { title: '状态', key: 'statusName', width: 90, render: (row: SnapshotItem) => h(NTag, { type: 'info' }, () => row.statusName) },
    { title: '进度', key: 'progressPermille', width: 80, render: (row: SnapshotItem) => `${Math.round((row.progressPermille||0)/10)}%` },
    { title: '阶段', key: 'stage', width: 100 },
    { title: 'FPS', key: 'currentFps', width: 70, render: (row: SnapshotItem) => row.currentFps?.toFixed(1) ?? '-' },
    { title: '码率(Kbps)', key: 'currentBitrateKbps', width: 110, render: (row: SnapshotItem) => row.currentBitrateKbps ?? '-' },
    { title: '耗时(ms)', key: 'elapsedMs', width: 100, render: (row: SnapshotItem) => row.elapsedMs ?? '-' },
  ];
  const tableScrollX = computed(() => columns.reduce((s, c) => s + ((c.width as number) || 150), 0));
</script>
