<template>
  <div>
    <div class="n-layout-page-header"><n-card :bordered="false" title="配置版本" /></div>
    <n-card :bordered="false" class="mt-4 proCard">
      <BasicTable :columns="columns" :request="loadDataTable" :row-key="(row: VersionRow) => row.configVersion" ref="actionRef" :actionColumn="actionColumn"
        :scroll-x="tableScrollX" />
    </n-card>
  </div>
</template>

<script lang="ts" setup>
  import { computed, h, ref, reactive } from 'vue';
  import { NTag, useMessage } from 'naive-ui';
  import { BasicTable, TableAction } from '@/components/Table';
  import { listRuntimeVersions, publishRuntime } from '@/api/config';

  const message = useMessage();
  const actionRef = ref();

  interface VersionRow { configVersion: number; changeSummary: string; published: boolean; publishedBy: string; publishedAt: string; createdAt: string; }

  const columns = [
    { title: '版本号', key: 'configVersion', width: 100 },
    { title: '变更摘要', key: 'changeSummary', width: 200, ellipsis: { tooltip: true } },
    { title: '已发布', key: 'published', width: 80, render: (row: VersionRow) => h(NTag, { type: row.published ? 'success' : 'warning' }, () => row.published ? '是' : '否') },
    { title: '发布人', key: 'publishedBy', width: 120 },
    { title: '发布时间', key: 'publishedAt', width: 170 },
    { title: '创建时间', key: 'createdAt', width: 170 },
  ];

  const actionColumn = reactive({
    width: 120, title: '操作', key: 'action', fixed: 'right' as const,
    render(record: VersionRow) { return h(TableAction, { style: 'button', actions: [
      { label: '发布', auth: ['config.version.publish'], ifShow: () => !record.published, popConfirm: { title: '确认发布此版本? 发布后新任务将使用新配置。', confirm: () => doPublish(record) } },
    ]});},
  });

  const loadDataTable = async (res: Record<string, unknown>) => await listRuntimeVersions(res as Parameters<typeof listRuntimeVersions>[0]);
  const tableScrollX = computed(() => columns.reduce((s, c) => s + ((c.width as number) || 150), 0));
  function reloadTable() { actionRef.value.reload(); }

  async function doPublish(row: VersionRow) { await publishRuntime({ configVersion: row.configVersion }); message.success('发布成功'); reloadTable(); }
</script>
