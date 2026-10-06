<template>
  <div>
    <div class="n-layout-page-header"><n-card :bordered="false" title="转码任务" /></div>
    <n-card :bordered="false" class="mt-4 proCard">
      <BasicTable :columns="columns" :request="loadDataTable" :row-key="(row: TranscodeJobRow) => row.jobId" ref="actionRef" :actionColumn="actionColumn"
        :scroll-x="tableScrollX" />
    </n-card>
    <!-- 任务详情弹窗 -->
    <n-modal v-model:show="showDetail" preset="card" title="任务详情" style="width:600px">
      <n-descriptions v-if="detail" :column="1" size="small" bordered>
        <n-descriptions-item label="任务 ID">{{ detail.jobId }}</n-descriptions-item>
        <n-descriptions-item label="状态">{{ detail.statusName }}</n-descriptions-item>
        <n-descriptions-item label="进度">{{ Math.round((detail.progressPermille||0)/10) }}%</n-descriptions-item>
        <n-descriptions-item label="当前阶段">{{ detail.stage || '-' }}</n-descriptions-item>
        <n-descriptions-item label="源 URL">{{ detail.sourceUrl }}</n-descriptions-item>
        <n-descriptions-item label="业务 Key">{{ detail.bizKey }}</n-descriptions-item>
        <n-descriptions-item label="配置 ID">{{ detail.profileId }}</n-descriptions-item>
        <n-descriptions-item label="硬件编码">{{ detail.selectedExecutionHw || '-' }}</n-descriptions-item>
        <n-descriptions-item label="分配节点">{{ detail.assignedNodeId }}</n-descriptions-item>
        <n-descriptions-item label="分配 Worker">{{ detail.assignedWorkerId || '-' }}</n-descriptions-item>
      </n-descriptions>
    </n-modal>
  </div>
</template>

<script lang="ts" setup>
  import { computed, h, ref, reactive } from 'vue';
  import { NTag, useMessage } from 'naive-ui';
  import { BasicTable, TableAction } from '@/components/Table';
  import { listJobs, retryJob, cancelJob, jobDetail } from '@/api/transcode';
  import type { JobDetailData } from '@/types/api';

  const message = useMessage();
  const actionRef = ref();

  /** 转码任务行数据。字段对应后端 ListJobs 返回的 map。 */
  interface TranscodeJobRow { jobId: number; bizKey: string; sourceUrl: string; statusName: string; progressPermille: number; stage: string; assignedNodeId: number; profileId: number; assignedWorkerId: string; selectedExecutionHw: string; createdAt: string; }

  const columns = [
    { title: '任务 ID', key: 'jobId', width: 80 },
    { title: '业务 Key', key: 'bizKey', width: 120 },
    { title: '源 URL', key: 'sourceUrl', width: 200, ellipsis: { tooltip: true } },
    { title: '状态', key: 'statusName', width: 90, render: (row: TranscodeJobRow) => { const m: Record<string, 'success'|'info'|'error'|'warning'|'default'> = {'运行中':'success','已完成':'info','失败':'error','排队中':'warning','已取消':'default'}; return h(NTag, { type: m[row.statusName]||'default' }, () => row.statusName); }},
    { title: '进度', key: 'progressPermille', width: 80, render: (row: TranscodeJobRow) => `${Math.round((row.progressPermille||0)/10)}%` },
    { title: '阶段', key: 'stage', width: 100 },
    { title: '节点', key: 'assignedNodeId', width: 80 },
    { title: '创建时间', key: 'createdAt', width: 170 },
  ];

  const actionColumn = reactive({
    width: 200, title: '操作', key: 'action', fixed: 'right' as const,
    render(record: TranscodeJobRow) { return h(TableAction, { style: 'button', actions: [
      { label: '详情', auth: ['transcode.job.read'], onClick: () => handleDetail(record) },
      { label: '重试', auth: ['transcode.job.retry'], ifShow: () => record.statusName === '失败', onClick: () => handleRetry(record) },
      { label: '取消', auth: ['transcode.job.cancel'], ifShow: () => !['已完成','已取消','失败'].includes(record.statusName), popConfirm: { title: '确认取消此任务?', confirm: () => handleCancel(record) } },
    ]});},
  });

  const loadDataTable = async (res: Record<string, unknown>) => await listJobs(res as Parameters<typeof listJobs>[0]);
  const tableScrollX = computed(() => columns.reduce((s, c) => s + ((c.width as number) || 150), 0));
  function reloadTable() { actionRef.value.reload(); }

  // ===== 详情弹窗 =====
  const showDetail = ref(false);
  const detail = ref<JobDetailData | null>(null);

  /** 打开详情弹窗，加载任务详情数据 */
  async function handleDetail(row: TranscodeJobRow): Promise<void> {
    showDetail.value = true;
    detail.value = null;
    try { detail.value = await jobDetail(row.jobId) as JobDetailData; } catch (e) { /* ignore */ }
  }

  async function handleRetry(row: TranscodeJobRow) { await retryJob(row.jobId); message.success('已提交重试'); reloadTable(); }
  async function handleCancel(row: TranscodeJobRow) { await cancelJob(row.jobId); message.success('已取消'); reloadTable(); }
</script>
