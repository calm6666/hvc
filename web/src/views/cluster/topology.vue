<template>
  <div>
    <div class="n-layout-page-header"><n-card :bordered="false" title="集群拓扑" /></div>
    <n-card :bordered="false" class="mt-4 proCard">
      <n-data-table :columns="columns" :data="topologyData" :row-key="(row: TopoItem) => row.key" size="small" :scroll-x="tableScrollX" />
    </n-card>
  </div>
</template>

<script lang="ts" setup>
  import { computed, h, ref, onMounted } from 'vue';
  import { NTag } from 'naive-ui';
  import { clusterTopology } from '@/api/cluster';

  /** 拓扑展示行 */
  interface TopoItem { key: string; node: string; role: string; online: boolean; workers: string; }

  const topologyData = ref<TopoItem[]>([]);

  onMounted(async () => {
    try {
      const data = await clusterTopology() as Record<string, unknown>;
      const items = (data.items || data?.topology || []) as Record<string, unknown>[];
      topologyData.value = items.map((n, i) => ({
        key: String(n.nodeId || n.node_id || i),
        node: String(n.nodeName || n.node_name || ''),
        role: String(n.nodeRole || n.node_role || ''),
        online: Boolean(n.onlineEstimate ?? n.online_estimate),
        workers: `${n.workerOnlineTotal ?? n.worker_online_total ?? 0}/${n.workerTotal ?? n.worker_total ?? 0}`,
      }));
    } catch { /* ignore */ }
  });

  const columns = [
    { title: '节点', key: 'node', width: 160 },
    { title: '角色', key: 'role', width: 120 },
    { title: '在线', key: 'online', width: 80, render: (row: TopoItem) => h(NTag, { type: row.online ? 'success' : 'error' }, () => row.online ? '在线' : '离线') },
    { title: 'Worker (在线/总数)', key: 'workers', width: 160 },
  ];
  const tableScrollX = computed(() => columns.reduce((s, c) => s + ((c.width as number) || 150), 0));
</script>
