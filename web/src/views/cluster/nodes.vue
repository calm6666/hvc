<template>
  <div>
    <div class="n-layout-page-header">
      <n-card :bordered="false" title="集群节点" />
    </div>
    <n-card :bordered="false" class="mt-4 proCard">
      <BasicTable
        :columns="columns"
        :request="loadDataTable"
        :row-key="(row: ClusterNodeRow) => row.nodeId"
        ref="actionRef"
        :actionColumn="actionColumn"
        :scroll-x="tableScrollX"
      />
    </n-card>
    <!-- 节点详情弹窗 -->
    <n-modal v-model:show="showDetail" preset="card" title="节点详情" style="width:550px">
      <n-descriptions v-if="detailNode" :column="2" size="small" bordered>
        <n-descriptions-item label="节点 ID">{{ detailNode.nodeId }}</n-descriptions-item>
        <n-descriptions-item label="名称">{{ detailNode.nodeName }}</n-descriptions-item>
        <n-descriptions-item label="角色">{{ detailNode.nodeRole }}</n-descriptions-item>
        <n-descriptions-item label="IP">{{ detailNode.hostIp }}</n-descriptions-item>
        <n-descriptions-item label="在线">{{ detailNode.onlineEstimate ? '是' : '否' }}</n-descriptions-item>
        <n-descriptions-item label="调度就绪">{{ detailNode.schedulerReady ? '是' : '否' }}</n-descriptions-item>
        <n-descriptions-item label="CPU 核">{{ detailNode.cpuCores }}</n-descriptions-item>
        <n-descriptions-item label="内存(MB)">{{ detailNode.memoryTotalMb }}</n-descriptions-item>
        <n-descriptions-item label="最大转码">{{ detailNode.maxTranscodeSessions }}</n-descriptions-item>
        <n-descriptions-item label="GPU 数">{{ detailNode.gpuSummary?.total ?? 0 }}</n-descriptions-item>
        <n-descriptions-item label="健康 GPU">{{ detailNode.gpuSummary?.healthyTotal ?? 0 }}</n-descriptions-item>
        <n-descriptions-item label="可调度 GPU">{{ detailNode.gpuSummary?.schedulableTotal ?? 0 }}</n-descriptions-item>
        <n-descriptions-item label="最后心跳" :span="2">{{ detailNode.lastHeartbeatAt }}</n-descriptions-item>
        <n-descriptions-item label="原因" :span="2">{{ detailNode.quarantineReason || detailNode.drainReason || '-' }}</n-descriptions-item>
      </n-descriptions>
    </n-modal>
  </div>
</template>

<script lang="ts" setup>
  import { computed, h, ref, reactive } from 'vue';
  import { NTag, useMessage } from 'naive-ui';
  import { BasicTable, TableAction } from '@/components/Table';
  import { listNodes, setNodeEnabled, setNodeQuarantined } from '@/api/cluster';

  const message = useMessage();
  const actionRef = ref();

  /**
   * 集群节点行数据类型。
   * 字段与后端 clusterNodeListItemView（经 toCamelCase 转换）对齐。
   */
  interface ClusterNodeRow {
    nodeId: number;
    nodeName: string;
    nodeRole: string;
    hostIp: string;
    enabled: boolean;
    quarantined: boolean;
    onlineEstimate: boolean;
    cpuCores: number;
    memoryTotalMb: number;
    maxTranscodeSessions: number;
    controlPlane: boolean;
    lastHeartbeatAt: string;
    gpuSummary?: { total: number };
  }

  const columns = [
    { title: 'ID', key: 'nodeId', width: 70 },
    { title: '名称', key: 'nodeName', width: 140 },
    {
      title: '角色', key: 'nodeRole', width: 100,
      render: (row: ClusterNodeRow) =>
        h(NTag, { type: row.controlPlane ? 'info' : 'default' }, () => row.nodeRole),
    },
    {
      title: '启用', key: 'enabled', width: 70,
      render: (row: ClusterNodeRow) =>
        h(NTag, { type: row.enabled ? 'success' : 'default' }, () => (row.enabled ? '是' : '否')),
    },
    {
      title: '隔离', key: 'quarantined', width: 70,
      render: (row: ClusterNodeRow) =>
        h(NTag, { type: row.quarantined ? 'warning' : 'default' }, () => (row.quarantined ? '是' : '否')),
    },
    { title: 'IP', key: 'hostIp', width: 130 },
    {
      title: '在线', key: 'onlineEstimate', width: 70,
      render: (row: ClusterNodeRow) =>
        h(NTag, { type: row.onlineEstimate ? 'success' : 'error' }, () => (row.onlineEstimate ? '在线' : '离线')),
    },
    { title: 'CPU 核', key: 'cpuCores', width: 80 },
    { title: '内存(MB)', key: 'memoryTotalMb', width: 100 },
    { title: '最大转码数', key: 'maxTranscodeSessions', width: 110 },
    {
      title: 'GPU', key: 'gpuSummary', width: 80,
      render: (row: ClusterNodeRow) => row.gpuSummary?.total ?? '-',
    },
    { title: '最后心跳', key: 'lastHeartbeatAt', width: 170 },
  ];

  const actionColumn = reactive({
    width: 170,
    title: '操作',
    key: 'action',
    fixed: 'right' as const,
    render(record: ClusterNodeRow) {
      return h(TableAction, {
        style: 'button',
        actions: [
          { label: '详情', onClick: () => handleDetail(record) },
          {
            label: record.enabled ? '禁用' : '启用',
            auth: ['cluster.node.enable'],
            onClick: () => toggleEnabled(record),
          },
          {
            label: record.quarantined ? '解除隔离' : '隔离',
            auth: ['cluster.node.quarantine'],
            onClick: () => toggleQuarantined(record),
          },
        ],
      });
    },
  });

  /** 表格数据加载函数，供 BasicTable 的 :request 属性调用 */
  const loadDataTable = async (res: Record<string, unknown>) => await listNodes(res as Parameters<typeof listNodes>[0]);
  const tableScrollX = computed(() => columns.reduce((s, c) => s + ((c.width as number) || 150), 0));

  function reloadTable() {
    actionRef.value.reload();
  }

  async function toggleEnabled(row: ClusterNodeRow) {
    await setNodeEnabled({ nodeId: row.nodeId, enabled: !row.enabled });
    message.success('操作成功'); reloadTable();
  }
  async function toggleQuarantined(row: ClusterNodeRow) {
    await setNodeQuarantined({ nodeId: row.nodeId, quarantined: !row.quarantined });
    message.success('操作成功'); reloadTable();
  }

  // ===== 节点详情弹窗 =====
  const showDetail = ref(false);
  const detailNode = ref<ClusterNodeRow | null>(null);

  /** 打开节点详情 */
  function handleDetail(row: ClusterNodeRow): void { detailNode.value = row; showDetail.value = true; }
</script>
