<template>
  <div>
    <div class="n-layout-page-header"><n-card :bordered="false" title="调度洞察" /></div>
    <n-card :bordered="false" class="mt-4 proCard">
      <n-data-table :columns="columns" :data="insightData" :row-key="(row: InsightItem) => row.key" size="small" :scroll-x="tableScrollX" />
    </n-card>
  </div>
</template>

<script lang="ts" setup>
  import { computed, ref, onMounted } from 'vue';
  import { schedulerInsight } from '@/api/cluster';
  import type { SchedulerInsightData } from '@/types/api';

  /** 调度洞察行数据（将后端返回的 candidates/decisions 扁平化） */
  interface InsightItem { key: string; type: string; field: string; value: string; }

  const insightData = ref<InsightItem[]>([]);

  onMounted(async () => {
    try {
      const data = await schedulerInsight() as SchedulerInsightData;
      const rows: InsightItem[] = [];
      if (data?.candidates) {
        (data.candidates as Record<string, unknown>[]).forEach((c, i) => {
          rows.push({ key: `candidate-${i}`, type: '候选', field: '节点', value: String(c.node_name || c.nodeName || '') });
        });
      }
      if (data?.decisions) {
        (data.decisions as Record<string, unknown>[]).forEach((d, i) => {
          rows.push({ key: `decision-${i}`, type: '决策', field: '结果', value: JSON.stringify(d) });
        });
      }
      insightData.value = rows.length ? rows : [{ key: 'empty', type: '-', field: '-', value: '暂无调度数据' }];
    } catch { /* ignore */ }
  });

  const columns = [
    { title: '类型', key: 'type', width: 80 },
    { title: '字段', key: 'field', width: 140 },
    { title: '值', key: 'value', width: 400, ellipsis: { tooltip: true } },
  ];
  const tableScrollX = computed(() => columns.reduce((s, c) => s + ((c.width as number) || 150), 0));
</script>
