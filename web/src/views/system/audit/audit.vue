<template>
  <div>
    <div class="n-layout-page-header"><n-card :bordered="false" title="审计日志" /></div>
    <n-card :bordered="false" class="mt-4 proCard">
      <BasicTable :columns="columns" :request="loadDataTable" :row-key="(row: AuditLogRow) => row.auditLogId" ref="actionRef"
        :scroll-x="tableScrollX" />
    </n-card>
  </div>
</template>

<script lang="ts" setup>
  import { computed, h, ref } from 'vue';
  import { NTag } from 'naive-ui';
  import { BasicTable } from '@/components/Table';
  import { listAuditLogs } from '@/api/system/audit';

  const actionRef = ref();

  /** 审计日志行数据类型。字段与后端 auditLogView（经 toCamelCase 转换）对齐。 */
  interface AuditLogRow {
    auditLogId: number; username: string; actionName: string; targetType: string;
    targetId: string; resultCode: number; resultMessage: string; requestIp: string; createdAt: string;
  }

  const columns = [
    { title: 'ID', key: 'auditLogId', width: 80 },
    { title: '操作用户', key: 'username', width: 120 },
    { title: '操作名称', key: 'actionName', width: 160 },
    { title: '目标类型', key: 'targetType', width: 120 },
    { title: '目标 ID', key: 'targetId', width: 120 },
    { title: '结果', key: 'resultCode', width: 80,
      render: (row: AuditLogRow) => h(NTag, { type: row.resultCode === 0 ? 'success' : 'error' }, () => row.resultCode === 0 ? '成功' : '失败') },
    { title: '结果信息', key: 'resultMessage', width: 200, ellipsis: { tooltip: true } },
    { title: '请求 IP', key: 'requestIp', width: 140 },
    { title: '操作时间', key: 'createdAt', width: 170 },
  ];

  const loadDataTable = async (res: Record<string, unknown>) => await listAuditLogs(res as Parameters<typeof listAuditLogs>[0]);
  const tableScrollX = computed(() => columns.reduce((s, c) => s + ((c.width as number) || 150), 0));
</script>
