<template>
  <div>
    <div class="n-layout-page-header"><n-card :bordered="false" title="Worker 管理" /></div>
    <n-card :bordered="false" class="mt-4 proCard">
      <BasicTable :columns="columns" :request="loadDataTable" :row-key="(row: WorkerRow) => row.id" ref="actionRef" :actionColumn="actionColumn"
        :scroll-x="tableScrollX" />
    </n-card>
  </div>
</template>

<script lang="ts" setup>
  import { computed, h, ref, reactive } from 'vue';
  import { NTag, useMessage } from 'naive-ui';
  import { BasicTable, TableAction } from '@/components/Table';
  import { listWorkers, setWorkerOffline, setWorkerExited } from '@/api/cluster';

  const message = useMessage();
  const actionRef = ref();

  interface WorkerRow { id: number; workerId: string; nodeId: number; statusName: string; version: string; startAt: string; lastHeartbeatAt: string; }

  const columns = [
    { title: 'ID', key: 'id', width: 80 },
    { title: 'Worker ID', key: 'workerId', width: 170 },
    { title: '节点 ID', key: 'nodeId', width: 80 },
    { title: '状态', key: 'statusName', width: 80, render: (row: WorkerRow) => h(NTag, { type: ({online:'success',offline:'default',exited:'error'} as Record<string,'success'|'default'|'error'>)[row.statusName]||'default' }, () => row.statusName) },
    { title: '版本', key: 'version', width: 100 },
    { title: '启动时间', key: 'startAt', width: 170 },
    { title: '最后心跳', key: 'lastHeartbeatAt', width: 170 },
  ];

  const actionColumn = reactive({
    width: 170, title: '操作', key: 'action', fixed: 'right' as const,
    render(record: WorkerRow) { return h(TableAction, { style: 'button', actions: [
      { label: '下线', auth: ['cluster.worker.offline'], ifShow: () => record.statusName !== 'offline', popConfirm: { title: '确认下线此 Worker?', confirm: () => offline(record) } },
      { label: '退出', auth: ['cluster.worker.exit'], ifShow: () => record.statusName !== 'exited', popConfirm: { title: '确认强制退出此 Worker?', confirm: () => exitWorker(record) } },
    ]});},
  });

  const loadDataTable = async (res: Record<string, unknown>) => await listWorkers(res as Parameters<typeof listWorkers>[0]);
  const tableScrollX = computed(() => columns.reduce((s, c) => s + ((c.width as number) || 150), 0));
  function reloadTable() { actionRef.value.reload(); }
  async function offline(row: WorkerRow) { await setWorkerOffline({ workerId: row.workerId }); message.success('已提交下线'); reloadTable(); }
  async function exitWorker(row: WorkerRow) { await setWorkerExited({ workerId: row.workerId }); message.success('已提交退出'); reloadTable(); }
</script>
